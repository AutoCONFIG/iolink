package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func (r *telemetryRepo) authorizeTelemetryWrite(ctx context.Context, tx pgx.Tx, tenantID, userID int64) error {
	scopedTenant, scoped := domain.TenantID(ctx)
	actor, actorPresent := domain.TenantUserID(ctx)
	claimedRole := domain.TenantRole(ctx)
	if !scoped || scopedTenant != tenantID || !actorPresent || actor != userID || claimedRole == "" {
		return domain.ErrForbidden
	}
	if r.policy == nil {
		return domain.ErrForbidden
	}
	var role string
	err := tx.QueryRow(ctx, `SELECT tm.role FROM tenant_memberships tm
 JOIN tenants t ON t.id=tm.tenant_id AND t.active
 JOIN users u ON u.id=tm.user_id AND u.authority='USER'
 WHERE tm.tenant_id=$1 AND tm.user_id=$2 AND tm.active
 AND (tm.expires_at IS NULL OR tm.expires_at>now()) FOR SHARE OF tm,t,u`, tenantID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrForbidden
	}
	if err != nil {
		return fmt.Errorf("load telemetry write membership: %w", err)
	}
	if claimedRole != role {
		return domain.ErrForbidden
	}
	allowed, err := r.policy.Allow(role, "telemetry", "write")
	if err != nil {
		return fmt.Errorf("authorize telemetry write: %w", err)
	}
	if !allowed {
		return domain.ErrForbidden
	}
	return nil
}
