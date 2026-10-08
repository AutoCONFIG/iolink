package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/persistence"
)

// tenantMembershipAuthority rejects legacy ordinary grants to global platform admins.
const tenantMembershipAuthority = ` AND EXISTS (SELECT 1 FROM users membership_user WHERE membership_user.id=tm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND tm.role='support' AND tm.expires_at IS NOT NULL)))`

// FindByOpenID implements the auth-facing user lookup. Satisfies
// appapi.UserStore (together with EnsureUser below) via Go structural
// typing — core never imports the appapi module.
func (s *Service) FindByOpenID(ctx context.Context, openID string) (*domain.User, error) {
	const q = `SELECT id, open_id, coalesce(nickname,'') FROM users WHERE open_id=$1`
	u := &domain.User{}
	err := s.pool.QueryRow(ctx, q, openID).Scan(&u.ID, &u.OpenID, &u.Nickname)
	if err != nil {
		return nil, fmt.Errorf("user by openid: %w", err)
	}
	return u, nil
}

// EnsureUser creates the user on first WeChat login (idempotent).
func (s *Service) EnsureUser(ctx context.Context, openID string) (*domain.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("ensure user transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	const q = `INSERT INTO users (open_id) VALUES ($1)
		ON CONFLICT (open_id) DO UPDATE SET open_id = EXCLUDED.open_id WHERE users.authority='USER'
		RETURNING id, open_id, coalesce(nickname,'')`
	u := &domain.User{}
	err = tx.QueryRow(ctx, q, openID).Scan(&u.ID, &u.OpenID, &u.Nickname)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ensure user: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("ensure user commit: %w", err)
	}
	return u, nil
}

func (s *Service) UserTokenVersion(ctx context.Context, id int64) (int, error) {
	var version int
	err := s.pool.QueryRow(ctx, `SELECT token_version FROM users WHERE id=$1 AND authority='USER'`, id).Scan(&version)
	return version, err
}

func (s *Service) DefaultTenantForUser(ctx context.Context, id int64) (int64, error) {
	var tenantID int64
	err := s.pool.QueryRow(ctx, `SELECT tm.tenant_id FROM tenant_memberships tm JOIN tenants t ON t.id=tm.tenant_id WHERE tm.user_id=$1 AND tm.active AND t.active AND (tm.expires_at IS NULL OR tm.expires_at>now())`+tenantMembershipAuthority+` ORDER BY CASE tm.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, tm.tenant_id LIMIT 1`, id).Scan(&tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		var membershipExists bool
		if existsErr := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenant_memberships tm WHERE user_id=$1`+tenantMembershipAuthority+`)`, id).Scan(&membershipExists); existsErr != nil {
			return 0, existsErr
		}
		if membershipExists {
			return 0, domain.ErrInactiveTenant
		}
		return 0, domain.ErrNotFound
	}
	return tenantID, err
}

func (s *Service) TenantMembershipVersion(ctx context.Context, userID, tenantID int64) (int64, error) {
	var version int64
	err := s.pool.QueryRow(ctx, `SELECT tm.permission_version FROM tenant_memberships tm JOIN tenants t ON t.id=tm.tenant_id WHERE tm.user_id=$1 AND tm.tenant_id=$2 AND tm.active AND t.active AND (tm.expires_at IS NULL OR tm.expires_at>now())`+tenantMembershipAuthority, userID, tenantID).Scan(&version)
	return version, err
}

func (s *Service) TenantRole(ctx context.Context, userID, tenantID int64) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx, `SELECT tm.role FROM tenant_memberships tm JOIN tenants t ON t.id=tm.tenant_id WHERE tm.user_id=$1 AND tm.tenant_id=$2 AND tm.active AND t.active AND (tm.expires_at IS NULL OR tm.expires_at>now())`+tenantMembershipAuthority, userID, tenantID).Scan(&role)
	return role, err
}

func (s *Service) ListTenants(ctx context.Context) ([]domain.Tenant, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,name,active,permission_version FROM tenants WHERE name <> '__iolink_system__' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Tenant, 0)
	for rows.Next() {
		var item domain.Tenant
		if err := rows.Scan(&item.ID, &item.Name, &item.Active, &item.PermissionVersion); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) SetTenantActive(ctx context.Context, tenantID int64, active bool, actorID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := persistence.AuthorizePlatformWrite(ctx, tx, actorID); err != nil {
		return err
	}
	ct, err := tx.Exec(ctx, `UPDATE tenants SET active=$2,permission_version=permission_version+1 WHERE id=$1 AND name <> '__iolink_system__'`, tenantID, active)
	if err != nil {
		return err
	}
	if ct.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	if _, err = tx.Exec(ctx, `UPDATE tenant_memberships SET permission_version=permission_version+1 WHERE tenant_id=$1`, tenantID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id,metadata) VALUES($1::bigint,$2::bigint,'tenant.status_changed','tenant',$3::text,$4::jsonb)`, tenantID, actorID, fmt.Sprint(tenantID), fmt.Sprintf(`{"active":%t}`, active)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
