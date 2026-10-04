package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
)

func tenantFilter(ctx context.Context, alias string, argIndex int) (string, []any) {
	tenantID, ok := domain.TenantID(ctx)
	if !ok {
		return "", nil
	}
	clause := fmt.Sprintf(" AND %s.tenant_id=$%d", alias, argIndex)
	args := []any{tenantID}
	role := domain.TenantRole(ctx)
	actorID, actorOK := domain.TenantUserID(ctx)
	if role != "" && role != "owner" && role != "admin" {
		if !actorOK {
			return clause + " AND FALSE", args
		}
		clause += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=%s.id AND fm.tenant_id=%s.tenant_id AND fm.user_id=$%d AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()))", alias, alias, argIndex+1)
		args = append(args, actorID)
	}
	return clause, args
}

// AdminStore implementation: satisfies adminapi.AdminStore structurally —
// core never imports adminapi; cmd/iolinkd passes the Service in.

func (s *Service) FindAdminByLogin(ctx context.Context, login string) (*domain.User, error) {
	const q = `SELECT id, coalesce(username,''), coalesce(open_id,''), coalesce(password_hash,''), coalesce(authority,'USER')
		FROM users WHERE username=$1 AND authority IN ('ADMIN','USER')`
	u := &domain.User{}
	err := s.pool.QueryRow(ctx, q, login).
		Scan(&u.ID, &u.Username, &u.OpenID, &u.PasswordHash, &u.Authority)
	if err != nil {
		return nil, fmt.Errorf("admin by login: %w", err)
	}
	return u, nil
}

func (s *Service) AdminTokenVersion(ctx context.Context, id int64) (int, error) {
	var version int
	err := s.pool.QueryRow(ctx, `SELECT token_version FROM users WHERE id=$1 AND authority IN ('ADMIN','USER')`, id).Scan(&version)
	return version, err
}

func (s *Service) UpgradeAdminPassword(ctx context.Context, id int64, hash string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE id=$1 AND authority IN ('ADMIN','USER')`, id, hash)
	return err
}

// ---- farms ----

func (s *Service) ListFarms(ctx context.Context) ([]domain.Farm, error) {
	q := `SELECT f.id, f.owner_id, f.name, coalesce(f.location,''), f.created_at FROM farms f WHERE true`
	clause, args := tenantFilter(ctx, "f", 1)
	q += clause + ` ORDER BY f.id`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Farm
	for rows.Next() {
		var f domain.Farm
		if err := rows.Scan(&f.ID, &f.OwnerID, &f.Name, &f.Location, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Service) CreateFarm(ctx context.Context, ownerID *int64, name, location string) (domain.Farm, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Farm{}, err
	}
	defer tx.Rollback(ctx)
	if err := s.authorizeTenantWrite(ctx, tx, "farms", "write"); err != nil {
		return domain.Farm{}, err
	}
	var tenantID int64
	if scopedTenant, scoped := domain.TenantID(ctx); scoped {
		tenantID = scopedTenant
	}
	if ownerID != nil {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND authority='USER' AND open_id<>'')`, *ownerID).Scan(&ok); err != nil {
			return domain.Farm{}, err
		}
		if !ok {
			return domain.Farm{}, domain.ErrNotFound
		}
		var memberships int
		membershipQ := `SELECT count(*), COALESCE(min(tm.tenant_id),
			(SELECT id FROM tenants WHERE name='__iolink_system__' AND active))
			FROM tenant_memberships tm
			JOIN tenants t ON t.id=tm.tenant_id
			WHERE tm.user_id=$1 AND tm.active AND t.active AND (tm.expires_at IS NULL OR tm.expires_at>now())`
		membershipArgs := []any{*ownerID}
		if tenantID > 0 {
			membershipQ = `SELECT count(*), $2 FROM tenant_memberships tm JOIN tenants t ON t.id=tm.tenant_id WHERE tm.user_id=$1 AND tm.tenant_id=$2 AND tm.active AND t.active AND (tm.expires_at IS NULL OR tm.expires_at>now())`
			membershipArgs = append(membershipArgs, tenantID)
		}
		if err := tx.QueryRow(ctx, membershipQ, membershipArgs...).Scan(&memberships, &tenantID); err != nil {
			return domain.Farm{}, normalizeDBError(err)
		}
		if (domainTenantID(ctx) > 0 && memberships == 0) || (domainTenantID(ctx) == 0 && memberships > 1) {
			return domain.Farm{}, domain.ErrConflict
		}
		membershipResult, err := tx.Exec(ctx, `INSERT INTO tenant_memberships(tenant_id,user_id,role,active,expires_at) VALUES($1,$2,'member',true,NULL)
			ON CONFLICT (tenant_id,user_id) DO UPDATE SET expires_at=NULL
			WHERE tenant_memberships.active AND (tenant_memberships.expires_at IS NULL OR tenant_memberships.expires_at>now())`, tenantID, *ownerID)
		if err != nil {
			return domain.Farm{}, normalizeDBError(err)
		}
		if membershipResult.RowsAffected() == 0 {
			return domain.Farm{}, domain.ErrConflict
		}
	} else if tenantID == 0 {
		if err := tx.QueryRow(ctx, `SELECT id FROM tenants WHERE name='__iolink_system__' AND active`).Scan(&tenantID); err != nil {
			return domain.Farm{}, normalizeDBError(err)
		}
	}
	if tenantID == 0 {
		return domain.Farm{}, domain.ErrNotFound
	}
	var f domain.Farm
	err = tx.QueryRow(ctx,
		`INSERT INTO farms (owner_id, tenant_id, name, location) VALUES ($1,$2,$3,$4)
				 RETURNING id, owner_id, name, coalesce(location,''), created_at`,
		ownerID, tenantID, name, location).Scan(&f.ID, &f.OwnerID, &f.Name, &f.Location, &f.CreatedAt)
	if err != nil {
		return domain.Farm{}, normalizeDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Farm{}, err
	}
	return f, nil
}

func domainTenantID(ctx context.Context) int64 {
	id, _ := domain.TenantID(ctx)
	return id
}

func (s *Service) SearchUsers(ctx context.Context, query string, limit, offset int) ([]domain.User, error) {
	q := `SELECT u.id, u.open_id, coalesce(u.nickname,''), u.token_version FROM users u WHERE u.authority='USER' AND (u.id::text=$1 OR u.nickname ILIKE '%'||$1||'%')`
	args := []any{query}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q += ` AND EXISTS (SELECT 1 FROM tenant_memberships tm WHERE tm.tenant_id=$2 AND tm.user_id=u.id AND tm.active AND (tm.expires_at IS NULL OR tm.expires_at>now()))`
		args = append(args, tenantID)
	}
	q += fmt.Sprintf(" ORDER BY u.id LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.User{}
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.OpenID, &u.Nickname, &u.TokenVersion); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Service) ListFarmMembers(ctx context.Context, farmID int64) ([]domain.FarmMembership, error) {
	tenantID, scoped := domain.TenantID(ctx)
	actorID, actorOK := domain.TenantUserID(ctx)
	if s.policy == nil || !scoped || !actorOK {
		return nil, domain.ErrForbidden
	}
	role := domain.TenantRole(ctx)
	allowed, err := s.policy.Allow(role, "farm_members", "read")
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, domain.ErrForbidden
	}
	var active bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenant_memberships tm JOIN tenants t ON t.id=tm.tenant_id WHERE tm.tenant_id=$1 AND tm.user_id=$2 AND tm.role=$3 AND tm.active AND t.active AND (tm.expires_at IS NULL OR tm.expires_at>now()))`, tenantID, actorID, role).Scan(&active); err != nil {
		return nil, err
	}
	if !active {
		return nil, domain.ErrForbidden
	}
	q := `SELECT fm.tenant_id,fm.farm_id,fm.user_id,fm.role,fm.active,fm.expires_at FROM farm_memberships fm JOIN farms f ON f.id=fm.farm_id WHERE fm.farm_id=$1 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now())`
	args := []any{farmID}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q += ` AND f.tenant_id=$2`
		args = append(args, tenantID)
	}
	var exists bool
	existsQ := `SELECT EXISTS(SELECT 1 FROM farms WHERE id=$1`
	existsArgs := []any{farmID}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		existsQ += ` AND tenant_id=$2`
		existsArgs = append(existsArgs, tenantID)
	}
	existsQ += `)`
	if err := s.pool.QueryRow(ctx, existsQ, existsArgs...).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.ErrNotFound
	}
	rows, err := s.pool.Query(ctx, q+` ORDER BY fm.user_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.FarmMembership{}
	for rows.Next() {
		var item domain.FarmMembership
		if err := rows.Scan(&item.TenantID, &item.FarmID, &item.UserID, &item.Role, &item.Active, &item.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) SetFarmMember(ctx context.Context, farmID, userID int64, role string, active bool, expiresAt *time.Time, actorID int64) error {
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
	var tenantID int64
	q := `SELECT tenant_id FROM farms WHERE id=$1`
	args := []any{farmID}
	if scopedTenant, scoped := domain.TenantID(ctx); scoped {
		q += ` AND tenant_id=$2`
		args = append(args, scopedTenant)
	}
	if err := tx.QueryRow(ctx, q, args...).Scan(&tenantID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	var userExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND (authority='USER' OR (authority='ADMIN' AND $2='support')))`, userID, role).Scan(&userExists); err != nil {
		return err
	}
	if !userExists {
		return domain.ErrNotFound
	}
	var tenantMember bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenant_memberships WHERE tenant_id=$1 AND user_id=$2 AND active AND (expires_at IS NULL OR expires_at>now()))`, tenantID, userID).Scan(&tenantMember); err != nil {
		return err
	}
	if !tenantMember {
		return domain.ErrNotFound
	}
	if err := s.authorizeTenantWrite(ctx, tx, "farm_members", "write"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role,active,expires_at) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(farm_id,user_id) DO UPDATE SET tenant_id=excluded.tenant_id,role=excluded.role,active=excluded.active,expires_at=excluded.expires_at`, tenantID, farmID, userID, role, active, expiresAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id,metadata) VALUES($1,$2,'farm.member_changed','farm',$3,$4::jsonb)`, tenantID, actorID, fmt.Sprint(farmID), fmt.Sprintf(`{"user_id":%d,"role":%q,"active":%t}`, userID, role, active)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) SetFarmOwner(ctx context.Context, id int64, ownerID *int64) error {
	return s.SetFarmOwnerByActor(ctx, id, ownerID, 0)
}

func (s *Service) SetFarmOwnerByActor(ctx context.Context, id int64, ownerID *int64, actorID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var previous *int64
	var tenantID int64
	q := `SELECT owner_id,tenant_id FROM farms WHERE id=$1`
	args := []any{id}
	if scopedTenant, scoped := domain.TenantID(ctx); scoped {
		q += ` AND tenant_id=$2`
		args = append(args, scopedTenant)
	}
	q += ` FOR UPDATE`
	if err := tx.QueryRow(ctx, q, args...).Scan(&previous, &tenantID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	if ownerID != nil {
		var ok bool
		ownerQ := `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND authority='USER' AND open_id<>'')`
		ownerArgs := []any{*ownerID}
		if scopedTenant, scoped := domain.TenantID(ctx); scoped {
			ownerQ = `SELECT EXISTS(SELECT 1 FROM users u JOIN tenant_memberships tm ON tm.user_id=u.id WHERE u.id=$1 AND u.authority='USER' AND u.open_id<>'' AND tm.tenant_id=$2 AND tm.active AND (tm.expires_at IS NULL OR tm.expires_at>now()))`
			ownerArgs = append(ownerArgs, scopedTenant)
		}
		if err := tx.QueryRow(ctx, ownerQ, ownerArgs...).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return domain.ErrNotFound
		}
		if err := s.authorizeTenantWrite(ctx, tx, "farms", "write"); err != nil {
			return err
		}
		var activeMembership, anyMembership bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenant_memberships WHERE tenant_id=$1 AND user_id=$2 AND active AND (expires_at IS NULL OR expires_at>now())), EXISTS(SELECT 1 FROM tenant_memberships WHERE tenant_id=$1 AND user_id=$2)`, tenantID, *ownerID).Scan(&activeMembership, &anyMembership); err != nil {
			return err
		}
		if !activeMembership && anyMembership {
			return domain.ErrConflict
		}
		if !activeMembership {
			if _, err := tx.Exec(ctx, `INSERT INTO tenant_memberships(tenant_id,user_id,role,active,expires_at) VALUES($1,$2,'member',true,NULL)`, tenantID, *ownerID); err != nil {
				return err
			}
		}
	} else if err := s.authorizeTenantWrite(ctx, tx, "farms", "write"); err != nil {
		return err
	}
	updateQ := `UPDATE farms SET owner_id=$2 WHERE id=$1`
	updateArgs := []any{id, ownerID}
	if scopedTenant, scoped := domain.TenantID(ctx); scoped {
		updateQ += ` AND tenant_id=$3`
		updateArgs = append(updateArgs, scopedTenant)
	}
	ct, err := tx.Exec(ctx, updateQ, updateArgs...)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if previous != nil && (ownerID == nil || *previous != *ownerID) {
		if _, err := tx.Exec(ctx, `UPDATE farm_memberships SET active=false WHERE farm_id=$1 AND user_id=$2 AND tenant_id=$3`, id, *previous, tenantID); err != nil {
			return err
		}
	}
	if ownerID != nil {
		if _, err := tx.Exec(ctx, `INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role,active,expires_at) VALUES($1,$2,$3,'owner',true,NULL) ON CONFLICT(farm_id,user_id) DO UPDATE SET tenant_id=excluded.tenant_id,role='owner',active=true,expires_at=NULL`, tenantID, id, *ownerID); err != nil {
			return normalizeDBError(err)
		}
	}
	metadata := fmt.Sprintf(`{"previous_owner_id":%s,"new_owner_id":%s}`, nullableID(previous), nullableID(ownerID))
	var actor any
	if actorID > 0 {
		actor = actorID
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id,metadata) VALUES($1,$2,'farm.owner_changed','farm',$3,$4::jsonb)`, tenantID, actor, fmt.Sprint(id), metadata); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func nullableID(id *int64) string {
	if id == nil {
		return "null"
	}
	return fmt.Sprint(*id)
}

func (s *Service) UpdateFarm(ctx context.Context, id int64, name, location string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := s.authorizeTenantWrite(ctx, tx, "farms", "write"); err != nil {
		return err
	}
	q := `UPDATE farms SET name=$2, location=$3 WHERE id=$1`
	args := []any{id, name, location}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q += ` AND tenant_id=$4`
		args = append(args, tenantID)
	}
	ct, err := tx.Exec(ctx, q, args...)
	if err == nil && ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) FindAdminByID(ctx context.Context, id int64) (*domain.User, error) {
	u := &domain.User{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, coalesce(username,''), coalesce(open_id,''), coalesce(password_hash,''), coalesce(authority,'USER')
		 FROM users WHERE id=$1 AND authority IN ('ADMIN','USER')`, id).
		Scan(&u.ID, &u.Username, &u.OpenID, &u.PasswordHash, &u.Authority)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// ChangeAdminPassword verifies the old password, then stores the new hash.
func (s *Service) ChangeAdminPassword(ctx context.Context, id int64, oldPassword, newPassword string) error {
	u, err := s.FindAdminByID(ctx, id)
	if err != nil {
		return err
	}
	if u.PasswordHash == nil || !platform.CheckPassword(*u.PasswordHash, oldPassword) {
		return domain.ErrOldPasswordMismatch
	}
	_, err = s.pool.Exec(ctx, `UPDATE users SET password_hash=$2, token_version=token_version+1, must_change_password=false WHERE id=$1`,
		id, platform.HashPassword(newPassword))
	return err
}

func (s *Service) DeleteFarm(ctx context.Context, id int64) error {
	tx, err := s.beginTenantWrite(ctx, "farms")
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var n int
	q := `SELECT count(*) FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE p.farm_id=$1`
	args := []any{id}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q += ` AND f.tenant_id=$2`
		args = append(args, tenantID)
	}
	if err := tx.QueryRow(ctx, q, args...).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return domain.ErrFarmHasPonds
	}
	q = `DELETE FROM farms WHERE id=$1`
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q += ` AND tenant_id=$2`
		args = append(args[:1], tenantID)
	}
	ct, err := tx.Exec(ctx, q, args...)
	if err == nil && ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ---- ponds ----

func (s *Service) ListPonds(ctx context.Context) ([]domain.Pond, error) {
	q := `SELECT p.id, p.farm_id, p.name, coalesce(p.area_mu,0), p.created_at FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE true`
	clause, args := tenantFilter(ctx, "f", 1)
	q += clause + ` ORDER BY p.id`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Pond
	for rows.Next() {
		var p domain.Pond
		if err := rows.Scan(&p.ID, &p.FarmID, &p.Name, &p.AreaMu, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) FarmExists(ctx context.Context, id int64) (bool, error) {
	q := `SELECT EXISTS(SELECT 1 FROM farms WHERE id=$1`
	args := []any{id}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q += ` AND tenant_id=$2`
		args = append(args, tenantID)
	}
	q += `)`
	var exists bool
	err := s.pool.QueryRow(ctx, q, args...).Scan(&exists)
	return exists, err
}

func (s *Service) CreatePond(ctx context.Context, farmID int64, name string, areaMu float64) (domain.Pond, error) {
	tx, err := s.beginTenantWrite(ctx, "ponds")
	if err != nil {
		return domain.Pond{}, err
	}
	defer tx.Rollback(ctx)
	var ok bool
	q := `SELECT EXISTS(SELECT 1 FROM farms WHERE id=$1)`
	args := []any{farmID}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q = `SELECT EXISTS(SELECT 1 FROM farms WHERE id=$1 AND tenant_id=$2)`
		args = append(args, tenantID)
	}
	if err := tx.QueryRow(ctx, q, args...).Scan(&ok); err != nil {
		return domain.Pond{}, err
	}
	if !ok {
		return domain.Pond{}, domain.ErrNotFound
	}
	var p domain.Pond
	err = tx.QueryRow(ctx,
		`INSERT INTO ponds (farm_id, name, area_mu) VALUES ($1,$2,$3)
		 RETURNING id, farm_id, name, coalesce(area_mu,0), created_at`,
		farmID, name, areaMu).Scan(&p.ID, &p.FarmID, &p.Name, &p.AreaMu, &p.CreatedAt)
	if err := normalizeDBError(err); err != nil {
		return p, err
	}
	if err := tx.Commit(ctx); err != nil {
		return p, err
	}
	return p, nil
}

func (s *Service) UpdatePond(ctx context.Context, id int64, name string, areaMu float64) error {
	tx, err := s.beginTenantWrite(ctx, "ponds")
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := `UPDATE ponds p SET name=$2, area_mu=$3 FROM farms f WHERE p.farm_id=f.id AND p.id=$1`
	args := []any{id, name, areaMu}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q += ` AND f.tenant_id=$4`
		args = append(args, tenantID)
	}
	ct, err := tx.Exec(ctx, q, args...)
	if err == nil && ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) DeletePond(ctx context.Context, id int64) error {
	tx, err := s.beginTenantWrite(ctx, "ponds")
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var n int
	q := `SELECT count(*) FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE p.id=$1 AND (EXISTS(SELECT 1 FROM devices WHERE pond_id=$1) OR EXISTS(SELECT 1 FROM sensor_data WHERE pond_id=$1) OR EXISTS(SELECT 1 FROM alarms WHERE pond_id=$1) OR EXISTS(SELECT 1 FROM alarm_rules WHERE pond_id=$1))`
	args := []any{id}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q += ` AND f.tenant_id=$2`
		args = append(args, tenantID)
	}
	if err := tx.QueryRow(ctx, q, args...).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return domain.ErrPondHasDevices
	}
	q = `DELETE FROM ponds p USING farms f WHERE p.id=$1 AND p.farm_id=f.id`
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q += ` AND f.tenant_id=$2`
		args = append(args[:1], tenantID)
	}
	ct, err := tx.Exec(ctx, q, args...)
	if err == nil && ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ---- devices ----

// RegisterDevice generates a unique device_no and a one-time secret
// (only sha256 is persisted).
func (s *Service) RegisterDevice(ctx context.Context, pondID int64, name, model string, reportInterval int) (domain.Device, string, error) {
	if err := s.observeLicenseClock(ctx); err != nil {
		return domain.Device{}, "", err
	}
	tx, err := s.beginTenantWrite(ctx, "devices")
	if err != nil {
		return domain.Device{}, "", err
	}
	defer tx.Rollback(ctx)
	if err := s.checkDeviceAdmission(ctx, tx); err != nil {
		return domain.Device{}, "", err
	}
	if reportInterval != 0 && reportInterval != 60 && reportInterval != 300 {
		return domain.Device{}, "", domain.ErrInvalidRange
	}
	if reportInterval == 0 {
		reportInterval = int(s.defaultInterval.Seconds())
	}
	var ok bool
	q := `SELECT EXISTS(SELECT 1 FROM ponds p WHERE p.id=$1)`
	args := []any{pondID}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q = `SELECT EXISTS(SELECT 1 FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE p.id=$1 AND f.tenant_id=$2)`
		args = append(args, tenantID)
	}
	if err := tx.QueryRow(ctx, q, args...).Scan(&ok); err != nil {
		return domain.Device{}, "", err
	}
	if !ok {
		return domain.Device{}, "", domain.ErrNotFound
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return domain.Device{}, "", normalizeDBError(err)
	}
	secHex := hex.EncodeToString(secret)
	sum := sha256.Sum256([]byte(secHex))
	hash := hex.EncodeToString(sum[:])

	var no string
	for attempt := 0; attempt < 5; attempt++ {
		cand := make([]byte, 4)
		if _, err := rand.Read(cand); err != nil {
			return domain.Device{}, "", err
		}
		no = "dev-" + hex.EncodeToString(cand)
		var exists int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM devices WHERE device_no=$1`, no).Scan(&exists); err != nil {
			return domain.Device{}, "", err
		}
		if exists == 0 {
			break
		}
		no = ""
	}
	if no == "" {
		return domain.Device{}, "", errors.New("could not allocate device_no")
	}

	var d domain.Device
	err = tx.QueryRow(ctx,
		`INSERT INTO devices (pond_id, device_no, secret_hash, name, model, report_interval)
			 VALUES ($1,$2,$3,$4,$5,$6)
			 RETURNING id, pond_id, device_no, coalesce(name,''), coalesce(model,''), status, last_seen_at, created_at, disabled_at, coalesce(report_interval,$6)`,
		pondID, no, hash, name, model, reportInterval).
		Scan(&d.ID, &d.PondID, &d.DeviceNo, &d.Name, &d.Model, &d.Status, &d.LastSeenAt, &d.CreatedAt, &d.DisabledAt, &d.ReportInterval)
	if err != nil {
		return domain.Device{}, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Device{}, "", err
	}
	return d, secHex, nil
}

func (s *Service) RestoreDevice(ctx context.Context, deviceNo string) error {
	if err := s.observeLicenseClock(ctx); err != nil {
		return err
	}
	tx, err := s.beginTenantWrite(ctx, "devices")
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := `SELECT d.id,d.disabled_at,d.status FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id WHERE d.device_no=$1`
	args := []any{deviceNo}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q += ` AND f.tenant_id=$2`
		args = append(args, tenantID)
	}
	var id int64
	var disabledAt *time.Time
	var status string
	if err := tx.QueryRow(ctx, q+` FOR UPDATE`, args...).Scan(&id, &disabledAt, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	if disabledAt == nil {
		return nil
	}
	if status != string(domain.DeviceOffline) {
		return domain.ErrConflict
	}
	if err := s.checkDeviceAdmission(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE devices SET disabled_at=NULL,status='offline',last_seen_at=NULL,session_version=session_version+1 WHERE id=$1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM device_shadows WHERE device_no=$1`, deviceNo); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func normalizeDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		if pgerr.Code == "23505" {
			return domain.ErrConflict
		}
		if pgerr.Code == "23503" {
			return domain.ErrNotFound
		}
	}
	return err
}

func (s *Service) ListDevices(ctx context.Context, includeDisabled bool, pondID int64, limit, offset int) ([]domain.Device, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	q := `SELECT d.id, d.pond_id, d.device_no, coalesce(d.name,''), coalesce(d.model,''), d.status, d.last_seen_at, d.created_at, d.disabled_at, coalesce(d.report_interval,$1)
		 FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id WHERE ($2 OR d.disabled_at IS NULL) AND ($3=0 OR d.pond_id=$3)`
	args := []any{int(s.defaultInterval.Seconds()), includeDisabled, pondID}
	clause, scopedArgs := tenantFilter(ctx, "f", len(args)+1)
	q += clause
	args = append(args, scopedArgs...)
	q += fmt.Sprintf(` ORDER BY d.id LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Device
	for rows.Next() {
		var d domain.Device
		if err := rows.Scan(&d.ID, &d.PondID, &d.DeviceNo, &d.Name, &d.Model, &d.Status, &d.LastSeenAt, &d.CreatedAt, &d.DisabledAt, &d.ReportInterval); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Service) GetDevice(ctx context.Context, deviceNo string) (domain.Device, error) {
	var d domain.Device
	q := `SELECT d.id,d.pond_id,d.device_no,coalesce(d.name,''),coalesce(d.model,''),d.status,d.last_seen_at,d.created_at,d.disabled_at,coalesce(d.report_interval,$2) FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id WHERE d.device_no=$1`
	args := []any{deviceNo, int(s.defaultInterval.Seconds())}
	clause, scopedArgs := tenantFilter(ctx, "f", len(args)+1)
	q += clause
	args = append(args, scopedArgs...)
	err := s.pool.QueryRow(ctx, q, args...).Scan(&d.ID, &d.PondID, &d.DeviceNo, &d.Name, &d.Model, &d.Status, &d.LastSeenAt, &d.CreatedAt, &d.DisabledAt, &d.ReportInterval)
	return d, err
}

func (s *Service) MoveDevice(ctx context.Context, deviceNo string, pondID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := s.authorizeTenantWrite(ctx, tx, "devices", "write"); err != nil {
		return err
	}
	var id int64
	var disabled *time.Time
	q := `SELECT d.id,d.disabled_at FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id WHERE d.device_no=$1`
	args := []any{deviceNo}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q += ` AND f.tenant_id=$2`
		args = append(args, tenantID)
	}
	q += ` FOR UPDATE OF d`
	if err = tx.QueryRow(ctx, q, args...).Scan(&id, &disabled); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	if disabled != nil {
		return domain.ErrConflict
	}
	var exists bool
	pondQ := `SELECT EXISTS(SELECT 1 FROM ponds p WHERE p.id=$1)`
	pondArgs := []any{pondID}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		pondQ = `SELECT EXISTS(SELECT 1 FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE p.id=$1 AND f.tenant_id=$2)`
		pondArgs = append(pondArgs, tenantID)
	}
	if err = tx.QueryRow(ctx, pondQ, pondArgs...).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return domain.ErrNotFound
	}
	if _, err = tx.Exec(ctx, `UPDATE devices SET pond_id=$2,status='offline',last_seen_at=NULL,session_version=session_version+1 WHERE id=$1`, id, pondID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM device_shadows WHERE device_no=$1`, deviceNo); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) DeleteDevice(ctx context.Context, deviceNo string) error {
	tx, err := s.beginTenantWrite(ctx, "devices")
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := `UPDATE devices d SET disabled_at=coalesce(d.disabled_at,now()),status='offline',session_version=session_version+1 FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE d.pond_id=p.id AND d.device_no=$1`
	args := []any{deviceNo}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q += ` AND f.tenant_id=$2`
		args = append(args, tenantID)
	}
	ct, err := tx.Exec(ctx, q, args...)
	if err == nil && ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ---- alarm rules ----

const ruleCols = `id, pond_id, metric, min_value, max_value, level, enabled`

func scanRule(row interface{ Scan(...any) error }) (domain.AlarmRule, error) {
	var r domain.AlarmRule
	err := row.Scan(&r.ID, &r.PondID, &r.Metric, &r.Min, &r.Max, &r.Level, &r.Enabled)
	return r, err
}

func (s *Service) ListRules(ctx context.Context) ([]domain.AlarmRule, error) {
	q := `SELECT r.` + strings.ReplaceAll(ruleCols, ", ", ", r.") + ` FROM alarm_rules r JOIN ponds p ON p.id=r.pond_id JOIN farms f ON f.id=p.farm_id WHERE true`
	clause, args := tenantFilter(ctx, "f", 1)
	q += clause + ` ORDER BY r.id`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AlarmRule
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) CreateRule(ctx context.Context, rule domain.AlarmRule) (domain.AlarmRule, error) {
	tx, err := s.beginTenantWrite(ctx, "alarm_rules")
	if err != nil {
		return domain.AlarmRule{}, err
	}
	defer tx.Rollback(ctx)
	if err := validateRule(rule); err != nil {
		return domain.AlarmRule{}, err
	}
	q := `INSERT INTO alarm_rules (pond_id, metric, min_value, max_value, level)
	      VALUES ($1,$2,$3,$4,$5) RETURNING ` + ruleCols
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q = `INSERT INTO alarm_rules (pond_id, metric, min_value, max_value, level)
			SELECT $1,$2,$3,$4,$5 WHERE EXISTS (SELECT 1 FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE p.id=$1 AND f.tenant_id=$6)
			RETURNING ` + ruleCols
		r, err := scanRule(tx.QueryRow(ctx, q, rule.PondID, rule.Metric, rule.Min, rule.Max, string(rule.Level), tenantID))
		if err != nil {
			return domain.AlarmRule{}, normalizeDBError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.AlarmRule{}, err
		}
		return r, nil
	}
	r, err := scanRule(tx.QueryRow(ctx, q, rule.PondID, rule.Metric, rule.Min, rule.Max, string(rule.Level)))
	if err != nil {
		return domain.AlarmRule{}, normalizeDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AlarmRule{}, err
	}
	return r, nil
}

func (s *Service) UpdateRule(ctx context.Context, rule domain.AlarmRule) error {
	tx, err := s.beginTenantWrite(ctx, "alarm_rules")
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := validateRule(rule); err != nil {
		return err
	}
	q := `UPDATE alarm_rules r SET pond_id=$2, metric=$3, min_value=$4, max_value=$5, level=$6, enabled=$7 FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE r.id=$1 AND p.id=r.pond_id`
	args := []any{rule.ID, rule.PondID, rule.Metric, rule.Min, rule.Max, string(rule.Level), rule.Enabled}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q = `UPDATE alarm_rules r SET pond_id=$2, metric=$3, min_value=$4, max_value=$5, level=$6, enabled=$7 FROM ponds oldp JOIN farms f ON f.id=oldp.farm_id WHERE r.id=$1 AND oldp.id=r.pond_id AND f.tenant_id=$8 AND EXISTS (SELECT 1 FROM ponds newp JOIN farms newf ON newf.id=newp.farm_id WHERE newp.id=$2 AND newf.tenant_id=$8)`
		args = append(args, tenantID)
	}
	ct, err := tx.Exec(ctx, q, args...)
	if err == nil && ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if err := normalizeDBError(err); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) DeleteRule(ctx context.Context, id int64) error {
	tx, err := s.beginTenantWrite(ctx, "alarm_rules")
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := `DELETE FROM alarm_rules r USING ponds p JOIN farms f ON f.id=p.farm_id WHERE r.id=$1 AND r.pond_id=p.id`
	args := []any{id}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q += ` AND f.tenant_id=$2`
		args = append(args, tenantID)
	}
	ct, err := tx.Exec(ctx, q, args...)
	if err == nil && ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ---- alarms (admin view) ----

func (s *Service) ListAllAlarms(ctx context.Context, limit int) ([]domain.Alarm, error) {
	q := `SELECT a.id, a.device_no, a.pond_id, a.metric, a.current_value, a.threshold,
		a.level, coalesce(a.message,''), a.confirmed_at, a.created_at
	 FROM alarms a JOIN ponds p ON p.id=a.pond_id JOIN farms f ON f.id=p.farm_id WHERE true`
	clause, args := tenantFilter(ctx, "f", 1)
	q += clause
	q += ` ORDER BY a.created_at DESC, a.id DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT $%d", len(args)+1)
		args = append(args, limit)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Alarm
	for rows.Next() {
		var a domain.Alarm
		if err := rows.Scan(&a.ID, &a.DeviceNo, &a.PondID, &a.Metric, &a.CurrentValue,
			&a.Threshold, &a.Level, &a.Message, &a.ConfirmedAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Service) ConfirmAlarm(ctx context.Context, id int64) error {
	if domain.HasTenantScope(ctx) {
		actorID, _ := domain.TenantUserID(ctx)
		return s.ConfirmAlarmByActor(ctx, id, actorID)
	}
	q := `UPDATE alarms a SET confirmed_at=now() FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE a.id=$1 AND a.pond_id=p.id AND a.confirmed_at IS NULL`
	args := []any{id}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		q += ` AND f.tenant_id=$2`
		args = append(args, tenantID)
	}
	ct, err := s.pool.Exec(ctx, q, args...)
	if err == nil && ct.RowsAffected() == 0 {
		var exists bool
		existsQ := `SELECT EXISTS(SELECT 1 FROM alarms a JOIN ponds p ON p.id=a.pond_id JOIN farms f ON f.id=p.farm_id WHERE a.id=$1 AND a.confirmed_at IS NOT NULL`
		existsArgs := []any{id}
		if tenantID, scoped := domain.TenantID(ctx); scoped {
			existsQ += ` AND f.tenant_id=$2`
			existsArgs = append(existsArgs, tenantID)
		}
		existsQ += `)`
		if err = s.pool.QueryRow(ctx, existsQ, existsArgs...).Scan(&exists); err == nil && exists {
			return nil
		}
		if err == nil {
			err = domain.ErrNotFound
		}
	}
	return err
}

func (s *Service) ConfirmAlarmByActor(ctx context.Context, id, actorID int64) error {
	_, scoped := domain.TenantID(ctx)
	contextActor, actorOK := domain.TenantUserID(ctx)
	if !scoped || !actorOK || actorID <= 0 || contextActor != actorID || domain.TenantRole(ctx) == "" {
		return domain.ErrForbidden
	}
	return (&alarmRepo{s.pool}).ConfirmByUser(ctx, id, actorID)
}

func (s *Service) BatchConfirm(ctx context.Context, ids []int64) (int64, error) {
	if domain.HasTenantScope(ctx) {
		actorID, _ := domain.TenantUserID(ctx)
		return s.BatchConfirmByActor(ctx, ids, actorID)
	}
	return s.batchConfirm(ctx, ids, 0)
}

func (s *Service) BatchConfirmByActor(ctx context.Context, ids []int64, actorID int64) (int64, error) {
	tenantID, scoped := domain.TenantID(ctx)
	contextActor, actorOK := domain.TenantUserID(ctx)
	if !scoped || !actorOK || contextActor != actorID || tenantID <= 0 || s.policy == nil {
		return 0, domain.ErrForbidden
	}
	allowed, err := s.policy.Allow(domain.TenantRole(ctx), "alarms", "confirm")
	if err != nil {
		return 0, err
	}
	if !allowed {
		return 0, domain.ErrForbidden
	}
	return s.batchConfirm(ctx, ids, actorID)
}

func (s *Service) batchConfirm(ctx context.Context, ids []int64, actorID int64) (int64, error) {
	if len(ids) == 0 || len(ids) > 200 {
		return 0, domain.ErrInvalidRange
	}
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return 0, domain.ErrInvalidRange
		}
		if _, duplicate := seen[id]; duplicate {
			return 0, domain.ErrInvalidRange
		}
		seen[id] = struct{}{}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var total, open int
	countQ := `SELECT a.id,a.confirmed_at IS NULL FROM alarms a JOIN ponds p ON p.id=a.pond_id JOIN farms f ON f.id=p.farm_id WHERE a.id=ANY($1)`
	countArgs := []any{ids}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		countQ += ` AND f.tenant_id=$2`
		countArgs = append(countArgs, tenantID)
	}
	if actorID > 0 {
		countQ += ` AND ` + appFarmConfirmScope(ctx, "f", 3)
		countArgs = append(countArgs, actorID)
	}
	rows, err := tx.Query(ctx, countQ+` ORDER BY a.id FOR UPDATE OF a`, countArgs...)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var id int64
		var unconfirmed bool
		if err := rows.Scan(&id, &unconfirmed); err != nil {
			rows.Close()
			return 0, err
		}
		total++
		if unconfirmed {
			open++
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if total != len(ids) {
		return 0, domain.ErrNotFound
	}
	updateQ := `UPDATE alarms a SET confirmed_at=now() FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE a.id=ANY($1) AND a.pond_id=p.id AND a.confirmed_at IS NULL`
	updateArgs := []any{ids}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		updateQ += ` AND f.tenant_id=$2`
		updateArgs = append(updateArgs, tenantID)
	}
	if actorID > 0 {
		updateQ += ` AND ` + appFarmConfirmScope(ctx, "f", 3)
		updateArgs = append(updateArgs, actorID)
	}
	ct, err := tx.Exec(ctx, updateQ, updateArgs...)
	if err != nil {
		return 0, err
	}
	if actorID > 0 && ct.RowsAffected() != int64(open) {
		return 0, domain.ErrConflict
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

// ---- stats ----

func (s *Service) Stats(ctx context.Context) (domain.Stats, error) {
	var st domain.Stats
	q := `SELECT count(*) FILTER (WHERE d.disabled_at IS NULL), count(*) FILTER (WHERE d.disabled_at IS NULL AND d.status='online'), count(*) FILTER (WHERE d.disabled_at IS NULL AND d.status='offline'), (SELECT count(*) FROM alarms a JOIN ponds ap ON ap.id=a.pond_id JOIN farms af ON af.id=ap.farm_id WHERE a.confirmed_at IS NULL`
	alarmClause, args := tenantFilter(ctx, "af", 1)
	q += alarmClause
	q += `) FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id`
	deviceClause, _ := tenantFilter(ctx, "f", 1)
	q += ` WHERE true` + deviceClause
	err := s.pool.QueryRow(ctx, q, args...).Scan(&st.DevicesTotal, &st.Online, &st.Offline, &st.OpenAlarms)
	if errors.Is(err, sql.ErrNoRows) {
		return st, nil
	}
	return st, err
}
