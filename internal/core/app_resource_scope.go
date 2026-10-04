package core

import (
	"context"
	"fmt"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func appFarmScope(ctx context.Context, farm string, userArg int) string {
	manager := "false"
	actor, actorPresent := domain.TenantUserID(ctx)
	_, tenantPresent := domain.TenantID(ctx)
	role := domain.TenantRole(ctx)
	if tenantPresent && actorPresent && (role == "owner" || role == "admin") {
		manager = fmt.Sprintf("(scope_user.authority='USER' AND scope_tm.role='%s' AND scope_tm.user_id=%d)", role, actor)
	}
	actorScope := ""
	if actorPresent {
		actorScope = fmt.Sprintf(" AND scope_tm.user_id=%d", actor)
	}
	return fmt.Sprintf(`EXISTS (SELECT 1 FROM tenant_memberships scope_tm
 JOIN tenants scope_t ON scope_t.id=scope_tm.tenant_id AND scope_t.active
 JOIN users scope_user ON scope_user.id=scope_tm.user_id
 WHERE scope_tm.tenant_id=%[1]s.tenant_id AND scope_tm.user_id=$%[2]d
 AND scope_tm.active AND (scope_tm.expires_at IS NULL OR scope_tm.expires_at>now())
 AND (scope_user.authority='USER' OR (scope_user.authority='ADMIN' AND scope_tm.role='support' AND scope_tm.expires_at IS NOT NULL))%[4]s
 AND (%[3]s OR (%[1]s.owner_id=scope_tm.user_id AND scope_user.authority='USER') OR EXISTS (
 SELECT 1 FROM farm_memberships fm WHERE fm.farm_id=%[1]s.id AND fm.tenant_id=%[1]s.tenant_id
 AND fm.user_id=scope_tm.user_id AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now())
 AND (scope_user.authority='USER' OR (fm.role='support' AND fm.expires_at IS NOT NULL)))))`, farm, userArg, manager, actorScope)
}

func appFarmConfirmScope(ctx context.Context, farm string, userArg int) string {
	roleScope := "role IN ('owner','admin','member','support')"
	switch role := domain.TenantRole(ctx); role {
	case "":
	case "owner", "admin", "member", "support":
		roleScope += fmt.Sprintf(" AND role='%s'", role)
	default:
		roleScope = "false"
	}
	if version, present := domain.TenantPermissionVersion(ctx); present {
		roleScope += fmt.Sprintf(" AND permission_version=%d", version)
	}
	return appFarmScope(ctx, farm, userArg) + fmt.Sprintf(` AND EXISTS (
 SELECT 1 FROM tenant_memberships WHERE tenant_id=%s.tenant_id AND user_id=$%d AND %s)`, farm, userArg, roleScope)
}
