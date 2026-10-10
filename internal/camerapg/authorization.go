package camerapg

import (
	"context"
	"errors"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (t *transaction) Authorize(ctx context.Context, write bool) error {
	var tenantOK, actorOK bool
	t.tenantID, tenantOK = domain.TenantID(ctx)
	t.actorID, actorOK = domain.TenantUserID(ctx)
	if !tenantOK || !actorOK || domain.TenantRole(ctx) == "" {
		return domain.ErrForbidden
	}
	var active bool
	err := t.tx.QueryRow(ctx, `SELECT active FROM tenants WHERE id=$1 FOR SHARE`, t.tenantID).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !active {
		return domain.ErrForbidden
	}
	if err != nil {
		return err
	}
	err = t.tx.QueryRow(ctx, `SELECT authority FROM users WHERE id=$1 FOR SHARE`, t.actorID).Scan(&t.authority)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrForbidden
	}
	if err != nil {
		return err
	}
	var version int64
	err = t.tx.QueryRow(ctx, `SELECT role,permission_version FROM tenant_memberships WHERE tenant_id=$1 AND user_id=$2 AND active
 AND (expires_at IS NULL OR expires_at>clock_timestamp()) AND ($3='USER' OR ($3='ADMIN' AND role='support' AND expires_at IS NOT NULL)) FOR SHARE`, t.tenantID, t.actorID, t.authority).Scan(&t.role, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrForbidden
	}
	if err != nil {
		return err
	}
	if t.role != domain.TenantRole(ctx) {
		return domain.ErrForbidden
	}
	if claimed, present := domain.TenantPermissionVersion(ctx); present && claimed != version {
		return domain.ErrForbidden
	}
	switch t.role {
	case "owner", "admin":
		if t.authority != "USER" {
			return domain.ErrForbidden
		}
	case "member", "viewer", "support":
		if write {
			return domain.ErrForbidden
		}
	default:
		return domain.ErrForbidden
	}
	return nil
}

// farmScope is always evaluated against live membership rows. Values are bound
// parameters, never caller-provided SQL or cached permission decisions.
const farmScope = `($3 IN ('owner','admin') AND $4='USER' OR f.owner_id=$2 AND $4='USER' OR EXISTS (
 SELECT 1 FROM farm_memberships fm WHERE fm.farm_id=f.id AND fm.tenant_id=f.tenant_id AND fm.user_id=$2
 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>clock_timestamp())
 AND ($4='USER' OR (fm.role='support' AND fm.expires_at IS NOT NULL))))`

func (t *transaction) lockFarm(ctx context.Context, id int64) error {
	var farmID int64
	err := t.tx.QueryRow(ctx, `SELECT f.id FROM farms f WHERE f.tenant_id=$1 AND f.id=$5 AND `+farmScope+` FOR SHARE OF f`, t.tenantID, t.actorID, t.role, t.authority, id).Scan(&farmID)
	if err != nil {
		return err
	}
	// Hold a present farm grant against concurrent revocation. Managers and owners
	// have their own live row locks and do not require an explicit grant.
	rows, err := t.tx.Query(ctx, `SELECT user_id FROM farm_memberships WHERE tenant_id=$1 AND farm_id=$2 AND user_id=$3 FOR SHARE`, t.tenantID, id, t.actorID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var userID int64
		if err = rows.Scan(&userID); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}
