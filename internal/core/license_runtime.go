package core

import (
	"context"
	"crypto/rsa"
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
	defer tx.Rollback(context.Background())
	var deployment string
	if err := tx.QueryRow(ctx, `SELECT deployment_id FROM deployment_config WHERE singleton=TRUE`).Scan(&deployment); err != nil {
		return license.Status{}, fmt.Errorf("license deployment: %w", err)
	}
	var payloadRaw, signature []byte
	var sha string
	var payload license.Payload
	var features []byte
	if err := tx.QueryRow(ctx, `SELECT license_id,deployment_id,key_id,issued_at,not_before,expires_at,max_devices,features,payload,signature,coalesce(payload_sha256,'') FROM license_state WHERE singleton=TRUE`).Scan(&payload.LicenseID, &payload.DeploymentID, &payload.KeyID, &payload.IssuedAt, &payload.NotBefore, &payload.ExpiresAt, &payload.MaxDevices, &features, &payloadRaw, &signature, &sha); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return license.Status{State: license.StateMissing, DeploymentID: &deployment, Features: []string{}}, nil
		}
		return license.Status{}, fmt.Errorf("license state: %w", err)
	}
	if err := json.Unmarshal(features, &payload.Features); err != nil {
		return license.Status{}, fmt.Errorf("license features: %w", err)
	}
	status := license.Status{State: license.StateInvalid, DeploymentID: &deployment, LicenseID: &payload.LicenseID, KeyID: &payload.KeyID, IssuedAt: &payload.IssuedAt, NotBefore: &payload.NotBefore, ExpiresAt: payload.ExpiresAt, MaxDevices: payload.MaxDevices, Features: payload.Features}
	if s.license == nil || s.license.PublicKey == nil || len(payloadRaw) == 0 || len(signature) == 0 {
		status.State = license.StateInvalid
	} else if verified, verifyErr := license.Verify(license.Envelope{PayloadB64: base64.StdEncoding.EncodeToString(payloadRaw), SignatureB64: base64.StdEncoding.EncodeToString(signature)}, s.license.PublicKey); verifyErr == nil && verified.SHA256 == sha && verified.Payload.KeyID == s.license.KeyID {
		status.State = license.Evaluate(verified.Payload, deployment, time.Now().UTC(), time.Time{}, false)
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
		return nil
	}
	if s.license.PublicKey == nil || s.license.KeyID == "" {
		return license.ErrUnavailable
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
	defer tx.Rollback(context.Background())
	var deployment string
	if err := tx.QueryRow(ctx, `SELECT deployment_id FROM deployment_config WHERE singleton=TRUE FOR UPDATE`).Scan(&deployment); err != nil {
		return fmt.Errorf("license deployment: %w", err)
	}
	verified, err := license.Verify(envelope, s.license.PublicKey)
	if err != nil {
		return err
	}
	if verified.Payload.KeyID != s.license.KeyID {
		return license.ErrInvalidPayload
	}
	state := license.Evaluate(verified.Payload, deployment, time.Now().UTC(), time.Time{}, false)
	if state == license.StateInstanceMismatch {
		return license.ErrInstanceMismatch
	}
	if state == license.StateNotBefore || state == license.StateExpired || state == license.StateClockError {
		return license.ErrInvalidPayload
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
