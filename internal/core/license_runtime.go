package core

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"time"
)

type LicenseRuntime struct {
	PublicKey *rsa.PublicKey
	KeyID     string
}

func NewLicenseService(pool *pgxpool.Pool, log *slog.Logger) *Service {
	return &Service{pool: pool, log: log}
}

func (s *Service) LicenseStatus(ctx context.Context) (license.Status, error) {
	if err := s.observeLicenseClock(ctx); err != nil && !errors.Is(err, license.ErrClockError) {
		return license.Status{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return license.Status{}, fmt.Errorf("license status transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if actor, ok := domain.PlatformActorFromContext(ctx); ok {
		var current int
		if err := tx.QueryRow(ctx, `SELECT token_version FROM users WHERE id=$1 AND authority='ADMIN' FOR SHARE`, actor.ID).Scan(&current); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return license.Status{}, domain.ErrForbidden
			}
			return license.Status{}, fmt.Errorf("license actor: %w", err)
		}
		if current != actor.TokenVersion {
			return license.Status{}, domain.ErrForbidden
		}
	}
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
	if err := tx.QueryRow(ctx, `SELECT payload,signature,coalesce(payload_sha256,'') FROM license_state WHERE singleton=TRUE`).Scan(&payloadRaw, &signature, &sha); err != nil {
		return license.Status{}, fmt.Errorf("license state: %w", err)
	}
	status := license.Status{State: license.StateInvalid, DeploymentID: &deployment, Features: []string{}}
	if len(payloadRaw) == 0 && len(signature) == 0 {
		status.State = license.StateMissing
		if clockError {
			status.State = license.StateClockError
		}
	} else if s.license == nil || s.license.PublicKey == nil || len(payloadRaw) == 0 || len(signature) == 0 {
		status.State = license.StateInvalid
	} else if verified, verifyErr := license.Verify(license.Envelope{PayloadB64: base64.StdEncoding.EncodeToString(payloadRaw), SignatureB64: base64.StdEncoding.EncodeToString(signature)}, s.license.PublicKey); verifyErr == nil && verified.SHA256 == sha && verified.Payload.KeyID == s.license.KeyID {
		payload := verified.Payload
		status.LicenseID = &payload.LicenseID
		status.KeyID = &payload.KeyID
		status.IssuedAt = &payload.IssuedAt
		status.NotBefore = &payload.NotBefore
		status.ExpiresAt = payload.ExpiresAt
		status.MaxDevices = payload.MaxDevices
		status.Features = payload.Features
		status.PayloadSHA256 = &verified.SHA256
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
	if err := lockLicenseState(ctx, tx); err != nil {
		return err
	}
	if err := checkLicenseClock(ctx, tx); err != nil {
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

func lockLicenseState(ctx context.Context, tx pgx.Tx) error {
	var singleton bool
	if err := tx.QueryRow(ctx, `SELECT singleton FROM license_state WHERE singleton=TRUE FOR UPDATE`).Scan(&singleton); err != nil {
		return fmt.Errorf("license lock: %w", err)
	}
	return nil
}

func checkLicenseClock(ctx context.Context, tx pgx.Tx) error {
	var maxSeen *time.Time
	var latched bool
	if err := tx.QueryRow(ctx, `SELECT max_seen_at,clock_error FROM license_clock WHERE singleton=TRUE FOR SHARE`).Scan(&maxSeen, &latched); err != nil {
		return fmt.Errorf("license clock recheck: %w", err)
	}
	if latched || (maxSeen != nil && time.Now().UTC().Before(maxSeen.Add(-5*time.Minute))) {
		return license.ErrClockError
	}
	return nil
}

func (s *Service) finishLicenseTransaction(ctx context.Context, tx pgx.Tx, resultErr *error) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		*resultErr = errors.Join(*resultErr, fmt.Errorf("license rollback: %w", err))
	}
	if errors.Is(*resultErr, license.ErrClockError) {
		if err := s.observeLicenseClock(ctx); err != nil && !errors.Is(err, license.ErrClockError) {
			*resultErr = errors.Join(*resultErr, err)
		}
	}
}

// RequireLicenseFeature is the shared guard for future optional use cases.
// Callers must also enforce their resource authorization before external effects.
func (s *Service) RequireLicenseFeature(ctx context.Context, feature string) error {
	if s.license == nil || s.license.PublicKey == nil || s.license.KeyID == "" {
		return license.ErrUnavailable
	}
	status, err := s.LicenseStatus(ctx)
	if err != nil {
		return err
	}
	return status.AllowsFeature(feature)
}
