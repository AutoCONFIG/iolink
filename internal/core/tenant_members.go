package core

import (
	"context"
	"errors"
	"fmt"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/persistence"
	"github.com/jackc/pgx/v5"
	"time"
)

func (s *Service) ListTenantMembers(ctx context.Context, tenantID int64) ([]domain.TenantMembership, error) {
	var tenantExists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenants WHERE id=$1 AND name <> '__iolink_system__')`, tenantID).Scan(&tenantExists); err != nil {
		return nil, err
	}
	if !tenantExists {
		return nil, domain.ErrNotFound
	}
	rows, err := s.pool.Query(ctx, `SELECT tm.tenant_id,tm.user_id,coalesce(u.nickname,u.open_id,''),tm.role,tm.active,tm.expires_at FROM tenant_memberships tm JOIN users u ON u.id=tm.user_id WHERE tm.tenant_id=$1 ORDER BY tm.user_id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.TenantMembership, 0)
	for rows.Next() {
		var item domain.TenantMembership
		if err := rows.Scan(&item.TenantID, &item.UserID, &item.Name, &item.Role, &item.Active, &item.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) SetTenantMember(ctx context.Context, tenantID, userID int64, role string, active bool, expiresAt *time.Time, actorID int64) error {
	platformActor, platform := domain.PlatformActorFromContext(ctx)
	if platform && (platformActor.ID != actorID || role == "support") {
		return domain.ErrForbidden
	}
	if !platform {
		if err := persistence.AuthorizeTenantTarget(ctx, tenantID, actorID); err != nil {
			return err
		}
	}
	if role != "owner" && role != "admin" && role != "member" && role != "viewer" && role != "support" {
		return domain.ErrInvalidProductModel
	}
	if role == "support" && (expiresAt == nil || !expiresAt.After(time.Now())) {
		return domain.ErrInvalidProductModel
	}
	if role != "support" {
		expiresAt = nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var authorizeErr error
	if platform {
		authorizeErr = persistence.AuthorizePlatformWrite(ctx, tx, platformActor.ID)
	} else {
		authorizeErr = s.authorizeTenantWrite(ctx, tx, "tenant_members", "write")
	}
	if authorizeErr != nil {
		return authorizeErr
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenants WHERE id=$1 AND (name <> '__iolink_system__' OR $2) FOR UPDATE)`, tenantID, !platform).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return domain.ErrNotFound
	}
	var authority string
	if err = tx.QueryRow(ctx, `SELECT authority FROM users WHERE id=$1 AND authority IN ('USER','ADMIN')`, userID).Scan(&authority); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	if authority == "ADMIN" && role != "support" {
		return domain.ErrInvalidProductModel
	}
	if _, err = tx.Exec(ctx, `INSERT INTO tenant_memberships(tenant_id,user_id,role,active,expires_at,permission_version) VALUES($1,$2,$3,$4,$5,1) ON CONFLICT(tenant_id,user_id) DO UPDATE SET role=excluded.role,active=excluded.active,expires_at=excluded.expires_at,permission_version=tenant_memberships.permission_version+1`, tenantID, userID, role, active, expiresAt); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE tenants SET permission_version=permission_version+1 WHERE id=$1`, tenantID); err != nil {
		return err
	}
	metadata := fmt.Sprintf(`{"role":%q,"active":%t,"expires_at":%s}`, role, active, nullableTimeJSON(expiresAt))
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id,metadata) VALUES($1::bigint,$2::bigint,'tenant.member_changed','user',$3::text,$4::jsonb)`, tenantID, actorID, fmt.Sprint(userID), metadata); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) ListUserTenants(ctx context.Context, userID int64) ([]domain.TenantMembership, error) {
	rows, err := s.pool.Query(ctx, `SELECT tm.tenant_id,tm.user_id,t.name,tm.role,tm.active,tm.expires_at FROM tenant_memberships tm JOIN tenants t ON t.id=tm.tenant_id WHERE tm.user_id=$1 AND tm.active AND t.active AND (tm.expires_at IS NULL OR tm.expires_at>now())`+tenantMembershipAuthority+` ORDER BY t.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.TenantMembership
	for rows.Next() {
		var item domain.TenantMembership
		if err := rows.Scan(&item.TenantID, &item.UserID, &item.Name, &item.Role, &item.Active, &item.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func nullableTimeJSON(value *time.Time) string {
	if value == nil {
		return "null"
	}
	return fmt.Sprintf(`%q`, value.UTC().Format(time.RFC3339Nano))
}
