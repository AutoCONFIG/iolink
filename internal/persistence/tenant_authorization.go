package persistence

import (
	"context"
	"errors"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/jackc/pgx/v5"
)

func AuthorizeTenantWrite(ctx context.Context, tx pgx.Tx, policy domain.PermissionPolicy, resource, action string) error {
	if !domain.HasTenantScope(ctx) {
		if _, platform := domain.PlatformActorFromContext(ctx); platform {
			return domain.ErrForbidden
		}
		return nil
	}
	tenantID, tenantOK := domain.TenantID(ctx)
	actorID, actorOK := domain.TenantUserID(ctx)
	claimedRole := domain.TenantRole(ctx)
	if !tenantOK || !actorOK || claimedRole == "" || policy == nil {
		return domain.ErrForbidden
	}
	lock := " FOR SHARE"
	if resource == "tenant_members" {
		lock = " FOR UPDATE"
	}
	var active bool
	err := tx.QueryRow(ctx, `SELECT active FROM tenants WHERE id=$1`+lock, tenantID).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !active) {
		return domain.ErrForbidden
	}
	if err != nil {
		return err
	}
	var liveRole string
	var version int64
	err = tx.QueryRow(ctx, `SELECT tm.role,tm.permission_version FROM tenant_memberships tm
		JOIN users u ON u.id=tm.user_id
		WHERE tm.tenant_id=$1 AND tm.user_id=$2 AND tm.active
		AND (tm.expires_at IS NULL OR tm.expires_at>now())
		AND (u.authority='USER' OR (u.authority='ADMIN' AND tm.role='support' AND tm.expires_at IS NOT NULL))
		FOR SHARE OF tm,u`, tenantID, actorID).Scan(&liveRole, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrForbidden
	}
	if err != nil {
		return err
	}
	if liveRole != claimedRole {
		return domain.ErrForbidden
	}
	if claimedVersion, present := domain.TenantPermissionVersion(ctx); present && claimedVersion != version {
		return domain.ErrForbidden
	}
	allowed, err := policy.Allow(liveRole, resource, action)
	if err != nil {
		return err
	}
	if !allowed {
		return domain.ErrForbidden
	}
	return nil
}

func AuthorizeTenantTarget(ctx context.Context, tenantID, actorID int64) error {
	if !domain.HasTenantScope(ctx) {
		return nil
	}
	scopedTenant, tenantOK := domain.TenantID(ctx)
	actor, actorOK := domain.TenantUserID(ctx)
	if !tenantOK || !actorOK || actorID != actor {
		return domain.ErrForbidden
	}
	if tenantID != scopedTenant {
		return domain.ErrNotFound
	}
	return nil
}

func AuthorizePlatformWrite(ctx context.Context, tx pgx.Tx, actorID int64) error {
	actor, present := domain.PlatformActorFromContext(ctx)
	if !present {
		if domain.HasTenantScope(ctx) {
			return domain.ErrForbidden
		}
		return nil
	}
	if actor.ID <= 0 || actor.ID != actorID {
		return domain.ErrForbidden
	}
	var version int
	err := tx.QueryRow(ctx, `SELECT token_version FROM users WHERE id=$1 AND authority='ADMIN' FOR SHARE`, actorID).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && actor.TokenVersion != version) {
		return domain.ErrForbidden
	}
	return err
}
