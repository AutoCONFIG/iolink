package core

import (
	"context"
	"errors"
	"fmt"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/persistence"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
)

func (s *Service) ListPlatformUsers(ctx context.Context, query string) ([]domain.PlatformUser, error) {
	actor, ok := domain.PlatformActorFromContext(ctx)
	if !ok {
		return nil, domain.ErrForbidden
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := persistence.AuthorizePlatformWrite(ctx, tx, actor.ID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id,coalesce(username,''),coalesce(nickname,'') FROM users WHERE authority='USER' AND ($1='' OR strpos(lower(coalesce(username,'')),lower($1))>0 OR strpos(lower(coalesce(nickname,'')),lower($1))>0 OR id::text=$1) ORDER BY id LIMIT 200`, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]domain.PlatformUser, 0)
	for rows.Next() {
		var user domain.PlatformUser
		if err := rows.Scan(&user.ID, &user.Username, &user.Name); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Service) CreateTenant(ctx context.Context, name string, actorID int64) (domain.Tenant, error) {
	if actor, ok := domain.PlatformActorFromContext(ctx); !ok || actor.ID != actorID {
		return domain.Tenant{}, domain.ErrForbidden
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 128 || name == "__iolink_system__" {
		return domain.Tenant{}, domain.ErrInvalidProductModel
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Tenant{}, err
	}
	defer tx.Rollback(ctx)
	if err := persistence.AuthorizePlatformWrite(ctx, tx, actorID); err != nil {
		return domain.Tenant{}, err
	}
	var tenant domain.Tenant
	err = tx.QueryRow(ctx, `INSERT INTO tenants(name) VALUES($1) RETURNING id,name,active,permission_version`, name).Scan(&tenant.ID, &tenant.Name, &tenant.Active, &tenant.PermissionVersion)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.Tenant{}, domain.ErrConflict
		}
		return domain.Tenant{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id) VALUES($1,$2,'tenant.created','tenant',$3)`, tenant.ID, actorID, fmt.Sprint(tenant.ID)); err != nil {
		return domain.Tenant{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Tenant{}, err
	}
	return tenant, nil
}

func (s *Service) PlatformStats(ctx context.Context) (domain.PlatformStats, error) {
	actor, ok := domain.PlatformActorFromContext(ctx)
	if !ok {
		return domain.PlatformStats{}, domain.ErrForbidden
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PlatformStats{}, err
	}
	defer tx.Rollback(ctx)
	if err := persistence.AuthorizePlatformWrite(ctx, tx, actor.ID); err != nil {
		return domain.PlatformStats{}, err
	}
	var stats domain.PlatformStats
	err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM tenants WHERE name <> '__iolink_system__'), (SELECT count(*) FROM users WHERE authority='USER'), (SELECT count(*) FROM users u WHERE authority='USER' AND EXISTS (SELECT 1 FROM tenant_memberships tm JOIN tenants t ON t.id=tm.tenant_id WHERE tm.user_id=u.id AND tm.active AND t.active AND t.name <> '__iolink_system__' AND (tm.expires_at IS NULL OR tm.expires_at>now()))), (SELECT count(*) FROM devices), (SELECT count(*) FROM devices WHERE status='online')`).Scan(&stats.Tenants, &stats.Users, &stats.ActiveUsers, &stats.Devices, &stats.OnlineDevices)
	return stats, err
}
