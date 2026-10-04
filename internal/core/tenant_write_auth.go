package core

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func (s *Service) authorizeTenantWrite(ctx context.Context, tx pgx.Tx, resource, action string) error {
	tenantID, tenantOK := domain.TenantID(ctx)
	actorID, actorOK := domain.TenantUserID(ctx)
	claimedRole := domain.TenantRole(ctx)
	if !tenantOK || !actorOK || claimedRole == "" || s.policy == nil {
		return nil
	}
	var liveRole string
	err := tx.QueryRow(ctx, `SELECT tm.role
		FROM tenant_memberships tm
		JOIN tenants t ON t.id=tm.tenant_id AND t.active
		JOIN users u ON u.id=tm.user_id
		WHERE tm.tenant_id=$1 AND tm.user_id=$2 AND tm.active
		  AND (tm.expires_at IS NULL OR tm.expires_at>now())
		  AND (u.authority='USER' OR (u.authority='ADMIN' AND tm.role='support' AND tm.expires_at IS NOT NULL))
		FOR SHARE OF tm,t,u`, tenantID, actorID).Scan(&liveRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrForbidden
		}
		return err
	}
	if liveRole != claimedRole {
		return domain.ErrForbidden
	}
	allowed, err := s.policy.Allow(liveRole, resource, action)
	if err != nil {
		return err
	}
	if !allowed {
		return domain.ErrForbidden
	}
	return nil
}

func (s *Service) authorizeTenantWriteNow(ctx context.Context, resource, action string) error {
	if _, scoped := domain.TenantID(ctx); !scoped || domain.TenantRole(ctx) == "" {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := s.authorizeTenantWrite(ctx, tx, resource, action); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
