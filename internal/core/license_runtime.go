package core

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"git.hyhy.fun/rsplab/iolink/internal/license"
)

type LicenseRuntime struct {
	PublicKey *rsa.PublicKey
	KeyID     string
}

func (s *Service) LicenseStatus(ctx context.Context) (license.Status, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return license.Status{}, fmt.Errorf("license status transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	var deployment string
	if err := tx.QueryRow(ctx, `SELECT deployment_id FROM deployment_config WHERE singleton=TRUE`).Scan(&deployment); err != nil {
		return license.Status{}, fmt.Errorf("license deployment: %w", err)
	}
	var maxSeen *time.Time
	var clockError bool
	if err := tx.QueryRow(ctx, `SELECT max_seen_at,clock_error FROM license_clock WHERE singleton=TRUE`).Scan(&maxSeen, &clockError); err != nil {
		return license.Status{}, fmt.Errorf("license clock: %w", err)
	}
	var payloadRaw, signature []byte
	var sha string
	var licenseID, licenseDeploymentID, keyID *string
	var issuedAt, notBefore, expiresAt *time.Time
	var maxDevices int64
	var features []byte
	if err := tx.QueryRow(ctx, `SELECT license_id,deployment_id,key_id,issued_at,not_before,expires_at,max_devices,features,payload,signature,coalesce(payload_sha256,'') FROM license_state WHERE singleton=TRUE`).Scan(&licenseID, &licenseDeploymentID, &keyID, &issuedAt, &notBefore, &expiresAt, &maxDevices, &features, &payloadRaw, &signature, &sha); err != nil {
		return license.Status{}, fmt.Errorf("license state: %w", err)
	}
	if licenseID == nil && len(payloadRaw) == 0 && len(signature) == 0 {
		state := license.StateMissing
		if clockError {
			state = license.StateClockError
		}
		return license.Status{State: state, DeploymentID: &deployment, Features: []string{}}, nil
	}
	var payloadFeatures []string
	if err := json.Unmarshal(features, &payloadFeatures); err != nil {
		return license.Status{}, fmt.Errorf("license features: %w", err)
	}
	payload := license.Payload{Features: payloadFeatures, MaxDevices: maxDevices}
	if licenseID != nil {
		payload.LicenseID = *licenseID
	}
	if licenseDeploymentID != nil {
		payload.DeploymentID = *licenseDeploymentID
	}
	if keyID != nil {
		payload.KeyID = *keyID
	}
	if issuedAt != nil {
		payload.IssuedAt = *issuedAt
	}
	if notBefore != nil {
		payload.NotBefore = *notBefore
	}
	payload.ExpiresAt = expiresAt
	status := license.Status{State: license.StateInvalid, DeploymentID: &deployment, LicenseID: licenseID, KeyID: keyID, IssuedAt: issuedAt, NotBefore: notBefore, ExpiresAt: expiresAt, MaxDevices: maxDevices, Features: payloadFeatures, PayloadSHA256: &sha}
	if s.license == nil || s.license.PublicKey == nil || len(payloadRaw) == 0 || len(signature) == 0 {
		status.State = license.StateInvalid
	} else if verified, verifyErr := license.Verify(license.Envelope{PayloadB64: base64.StdEncoding.EncodeToString(payloadRaw), SignatureB64: base64.StdEncoding.EncodeToString(signature)}, s.license.PublicKey); verifyErr == nil && verified.SHA256 == sha && verified.Payload.KeyID == s.license.KeyID {
		lastSeen := time.Time{}
		if maxSeen != nil {
			lastSeen = *maxSeen
		}
		status.State = license.Evaluate(verified.Payload, deployment, time.Now().UTC(), lastSeen, clockError)
	} else if clockError {
		status.State = license.StateClockError
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM devices WHERE disabled_at IS NULL`).Scan(&status.UsedDevices); err != nil {
		return license.Status{}, fmt.Errorf("license devices: %w", err)
	}
	if status.UsedDevices > status.MaxDevices {
		status.Overage = status.UsedDevices - status.MaxDevices
		if status.State == license.StateValid || status.State == license.StatePermanent {
			status.State = license.StateOverage
		}
	}
	return status, nil
}

func (s *Service) checkDeviceAdmission(ctx context.Context, tx pgx.Tx) error {
	if s.license == nil {
		return license.ErrUnavailable
	}
	if s.license.PublicKey == nil || s.license.KeyID == "" {
		return license.ErrUnavailable
	}
	if err := observeLicenseClock(ctx, tx); err != nil {
		return err
	}
	var deployment string
	if err := tx.QueryRow(ctx, `SELECT deployment_id FROM deployment_config WHERE singleton=TRUE`).Scan(&deployment); err != nil {
		return fmt.Errorf("license deployment: %w", err)
	}
	var payloadRaw, signature []byte
	var payloadSHA string
	if err := tx.QueryRow(ctx, `SELECT payload,signature,coalesce(payload_sha256,'') FROM license_state WHERE singleton=TRUE FOR UPDATE`).Scan(&payloadRaw, &signature, &payloadSHA); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return license.ErrRequired
		}
		return fmt.Errorf("license state: %w", err)
	}
	verified, err := license.Verify(license.Envelope{PayloadB64: base64.StdEncoding.EncodeToString(payloadRaw), SignatureB64: base64.StdEncoding.EncodeToString(signature)}, s.license.PublicKey)
	if err != nil || verified.SHA256 != payloadSHA || verified.Payload.KeyID != s.license.KeyID {
		return license.ErrRequired
	}
	state := license.Evaluate(verified.Payload, deployment, time.Now().UTC(), time.Time{}, false)
	if state != license.StateValid && state != license.StatePermanent {
		return license.ErrRequired
	}
	var used int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM devices WHERE disabled_at IS NULL`).Scan(&used); err != nil {
		return fmt.Errorf("license device count: %w", err)
	}
	if used >= verified.Payload.MaxDevices {
		return license.ErrQuotaExceeded
	}
	return nil
}

func (s *Service) ImportLicense(ctx context.Context, envelope license.Envelope, actorID int64) error {
	if s.license == nil || s.license.PublicKey == nil || s.license.KeyID == "" {
		return license.ErrUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("license import transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	reject := func(reason string, cause error) error {
		_ = tx.Rollback(ctx)
		_ = s.recordLicenseRejection(ctx, envelope, actorID, reason)
		return cause
	}
	var deployment string
	if err := tx.QueryRow(ctx, `SELECT deployment_id FROM deployment_config WHERE singleton=TRUE FOR UPDATE`).Scan(&deployment); err != nil {
		return fmt.Errorf("license deployment: %w", err)
	}
	if err := observeLicenseClock(ctx, tx); err != nil {
		if errors.Is(err, license.ErrClockError) {
			return reject("license_clock_error", err)
		}
		return err
	}
	verified, err := license.Verify(envelope, s.license.PublicKey)
	if err != nil {
		reason := "license_invalid"
		if errors.Is(err, license.ErrInvalidSignature) {
			reason = "license_signature_invalid"
		}
		return reject(reason, err)
	}
	if verified.Payload.KeyID != s.license.KeyID {
		return reject("license_invalid", license.ErrInvalidPayload)
	}
	state := license.Evaluate(verified.Payload, deployment, time.Now().UTC(), time.Time{}, false)
	if state == license.StateInstanceMismatch {
		return reject("license_instance_mismatch", license.ErrInstanceMismatch)
	}
	if state == license.StateNotBefore || state == license.StateExpired || state == license.StateClockError {
		reason := "license_invalid"
		if state == license.StateClockError {
			reason = "license_clock_error"
		}
		return reject(reason, license.ErrInvalidPayload)
	}
	features, err := json.Marshal(verified.Payload.Features)
	if err != nil {
		return fmt.Errorf("license features: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE license_state SET license_id=$1,deployment_id=$2,key_id=$3,issued_at=$4,not_before=$5,expires_at=$6,max_devices=$7,features=$8,payload=$9,signature=$10,payload_sha256=$11,state=$12,imported_at=now(),imported_by=$13 WHERE singleton=TRUE`, verified.Payload.LicenseID, verified.Payload.DeploymentID, verified.Payload.KeyID, verified.Payload.IssuedAt, verified.Payload.NotBefore, verified.Payload.ExpiresAt, verified.Payload.MaxDevices, features, verified.PayloadRaw, verified.Signature, verified.SHA256, state, actorID); err != nil {
		return fmt.Errorf("license persist: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id,metadata) VALUES((SELECT id FROM tenants WHERE name='__iolink_system__'),$1,'license.imported','license',$2,$3::jsonb)`, actorID, verified.Payload.LicenseID, fmt.Sprintf(`{"sha256":%q,"key_id":%q}`, verified.SHA256, verified.Payload.KeyID)); err != nil {
		return fmt.Errorf("license audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("license import commit: %w", err)
	}
	return nil
}

func (s *Service) RecordLicenseRejection(ctx context.Context, raw []byte, actorID int64, reason string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	digest := sha256.Sum256(raw)
	digestHex := fmt.Sprintf("%x", digest)
	metadata, err := json.Marshal(map[string]string{"sha256": digestHex, "reason": reason})
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id,metadata) VALUES((SELECT id FROM tenants WHERE name='__iolink_system__'),$1,'license.import_rejected','license',$2,$3::jsonb)`, actorID, digestHex, metadata); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) recordLicenseRejection(ctx context.Context, envelope license.Envelope, actorID int64, reason string) error {
	payload := []byte(envelope.PayloadB64)
	if decoded, decodeErr := base64.StdEncoding.DecodeString(envelope.PayloadB64); decodeErr == nil {
		payload = decoded
	}
	return s.RecordLicenseRejection(ctx, payload, actorID, reason)
}

func observeLicenseClock(ctx context.Context, tx pgx.Tx) error {
	var maxSeen *time.Time
	var clockError bool
	if err := tx.QueryRow(ctx, `SELECT max_seen_at,clock_error FROM license_clock WHERE singleton=TRUE FOR UPDATE`).Scan(&maxSeen, &clockError); err != nil {
		return fmt.Errorf("license clock: %w", err)
	}
	now := time.Now().UTC()
	if clockError || (maxSeen != nil && now.Before(maxSeen.Add(-5*time.Minute))) {
		if !clockError {
			if _, err := tx.Exec(ctx, `UPDATE license_clock SET clock_error=TRUE,updated_at=$1 WHERE singleton=TRUE`, now); err != nil {
				return fmt.Errorf("license clock latch: %w", err)
			}
		}
		return license.ErrClockError
	}
	if maxSeen == nil || now.After(*maxSeen) {
		if _, err := tx.Exec(ctx, `UPDATE license_clock SET max_seen_at=$1,updated_at=$1 WHERE singleton=TRUE`, now); err != nil {
			return fmt.Errorf("license clock observe: %w", err)
		}
	}
	return nil
}
