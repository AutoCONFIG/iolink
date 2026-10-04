package core

import (
	"context"
	"errors"
	"fmt"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
	"github.com/jackc/pgx/v5"
	"time"
)

func (s *Service) ImportLicenseRaw(ctx context.Context, raw []byte, envelope license.Envelope, actorID int64) (resultErr error) {
	if s.license == nil || s.license.PublicKey == nil || s.license.KeyID == "" {
		return license.ErrUnavailable
	}
	if err := s.observeLicenseClock(ctx); err != nil {
		if errors.Is(err, license.ErrClockError) {
			if auditErr := s.recordLicenseRejection(ctx, raw, actorID, "license_clock_error"); auditErr != nil {
				return fmt.Errorf("license rejection audit: %w", auditErr)
			}
		}
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("license import transaction: %w", err)
	}
	defer s.finishLicenseTransaction(ctx, tx, &resultErr)
	if actor, ok := domain.PlatformActorFromContext(ctx); ok {
		var current int
		if err := tx.QueryRow(ctx, `SELECT token_version FROM users WHERE id=$1 AND authority='ADMIN' FOR SHARE`, actor.ID).Scan(&current); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrForbidden
			}
			return fmt.Errorf("license actor: %w", err)
		}
		if current != actor.TokenVersion {
			return domain.ErrForbidden
		}
	}
	reject := func(reason string, cause error) error {
		_ = tx.Rollback(ctx)
		if auditErr := s.recordLicenseRejection(ctx, raw, actorID, reason); auditErr != nil {
			return fmt.Errorf("license rejection audit: %w", auditErr)
		}
		return cause
	}
	var deployment string
	if err := tx.QueryRow(ctx, `SELECT deployment_id FROM deployment_config WHERE singleton=TRUE FOR UPDATE`).Scan(&deployment); err != nil {
		return fmt.Errorf("license deployment: %w", err)
	}
	if err := lockLicenseState(ctx, tx); err != nil {
		return err
	}
	if err := checkLicenseClock(ctx, tx); err != nil {
		return reject("license_clock_error", err)
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
		cause := license.ErrInvalidPayload
		if state == license.StateClockError {
			reason = "license_clock_error"
		} else if state == license.StateNotBefore {
			reason = "license_not_before"
			cause = license.ErrNotBefore
		} else if state == license.StateExpired {
			reason = "license_expired"
			cause = license.ErrExpired
		}
		return reject(reason, cause)
	}
	var existingSHA string
	if err := tx.QueryRow(ctx, `SELECT coalesce(payload_sha256,'') FROM license_state WHERE singleton=TRUE FOR UPDATE`).Scan(&existingSHA); err != nil {
		return fmt.Errorf("license existing state: %w", err)
	}
	if existingSHA == verified.SHA256 {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("license import commit: %w", err)
		}
		return nil
	}
	if _, err := tx.Exec(ctx, `UPDATE license_state SET payload=$1,signature=$2,payload_sha256=$3,imported_at=now(),imported_by=$4 WHERE singleton=TRUE`, verified.PayloadRaw, verified.Signature, verified.SHA256, actorID); err != nil {
		return fmt.Errorf("license persist: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id,metadata) VALUES((SELECT id FROM tenants WHERE name='__iolink_system__'),$1,'license.imported','license',$2,$3::jsonb)`, actorID, verified.Payload.LicenseID, fmt.Sprintf(`{"sha256":%q,"key_id":%q,"max_devices":%d}`, verified.SHA256, verified.Payload.KeyID, verified.Payload.MaxDevices)); err != nil {
		return fmt.Errorf("license audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("license import commit: %w", err)
	}
	return nil
}
