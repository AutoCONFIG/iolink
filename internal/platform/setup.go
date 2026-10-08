package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SetupInput struct {
	PlatformUsername string `json:"platform_username"`
	PlatformPassword string `json:"platform_password"`
}

type SetupStatus struct {
	DeploymentID   string `json:"deployment_id"`
	PlatformReady  bool   `json:"platform_ready"`
	TenantReady    bool   `json:"tenant_ready"`
	LicensePresent bool   `json:"license_present"`
}

func (s SetupInput) validate() error {
	for name, value := range map[string]string{"platform_username": s.PlatformUsername} {
		if len(value) < 1 || len(value) > 64 || strings.TrimSpace(value) != value {
			return fmt.Errorf("%s must be 1..64 characters without surrounding whitespace", name)
		}
	}
	for name, value := range map[string]string{"platform_password": s.PlatformPassword} {
		if err := validateSetupPassword(name, value); err != nil {
			return err
		}
	}
	return nil
}

func validateSetupPassword(name, value string) error {
	if len(value) < 12 || len(value) > 256 || strings.TrimSpace(value) != value {
		return fmt.Errorf("%s must be 12..256 bytes without surrounding whitespace", name)
	}
	var hasLetter, hasNonLetter bool
	for _, r := range value {
		if unicode.IsLetter(r) {
			hasLetter = true
		} else {
			hasNonLetter = true
		}
	}
	if !hasLetter || !hasNonLetter {
		return fmt.Errorf("%s must contain letters and non-letters", name)
	}
	return nil
}

func SetupStatusJSON(ctx context.Context, pool *pgxpool.Pool) ([]byte, error) {
	var status SetupStatus
	if err := pool.QueryRow(ctx, `SELECT deployment_id FROM deployment_config WHERE singleton=TRUE`).Scan(&status.DeploymentID); err != nil {
		return nil, err
	}
	var admins, users, licenses int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE authority='ADMIN'), count(*) FILTER (WHERE authority='USER' AND EXISTS (SELECT 1 FROM tenant_memberships tm WHERE tm.user_id=users.id AND tm.active)) FROM users`).Scan(&admins, &users); err != nil {
		return nil, err
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM license_state WHERE singleton=TRUE AND payload IS NOT NULL`).Scan(&licenses); err != nil {
		return nil, err
	}
	status.PlatformReady = admins > 0
	status.TenantReady = users > 0
	status.LicensePresent = licenses == 1
	return json.Marshal(status)
}

func SetupInit(ctx context.Context, pool *pgxpool.Pool, input SetupInput) (setupErr error) {
	if err := input.validate(); err != nil {
		return errors.Join(err, recordSetupFailure(ctx, pool, "validation"))
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
		if setupErr != nil {
			setupErr = errors.Join(setupErr, recordSetupFailure(ctx, pool, "initialization"))
		}
	}()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1232047152)`); err != nil {
		return err
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE authority='ADMIN'`).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return errors.New("setup already initialized")
	}
	var systemTenantID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM tenants WHERE name='__iolink_system__' AND active FOR UPDATE`).Scan(&systemTenantID); err != nil {
		return errors.New("system tenant unavailable; run migrations first")
	}
	adminHash := HashPassword(input.PlatformPassword)
	var adminID int64
	if err := tx.QueryRow(ctx, `INSERT INTO users(open_id,username,password_hash,authority,nickname) VALUES('internal-setup-admin',$1,$2,'ADMIN','Platform administrator') RETURNING id`, input.PlatformUsername, adminHash).Scan(&adminID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id) VALUES($1,$2,'setup.initialized','user',$3)`, systemTenantID, adminID, fmt.Sprint(adminID)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func recordSetupFailure(ctx context.Context, pool *pgxpool.Pool, reason string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var systemTenantID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM tenants WHERE name='__iolink_system__' AND active`).Scan(&systemTenantID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,action,resource_type,resource_id,metadata) VALUES($1,'setup.init_rejected','setup','bootstrap',$2::jsonb)`, systemTenantID, fmt.Sprintf(`{"reason":%q}`, reason)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func RecordSetupInputRejection(ctx context.Context, pool *pgxpool.Pool) error {
	return recordSetupFailure(ctx, pool, "invalid_input")
}
