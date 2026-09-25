package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
)

// AdminStore implementation: satisfies adminapi.AdminStore structurally —
// core never imports adminapi; cmd/iolinkd passes the Service in.

func (s *Service) FindAdminByLogin(ctx context.Context, login string) (*domain.User, error) {
	const q = `SELECT id, coalesce(username,''), coalesce(open_id,''), coalesce(password_hash,''), coalesce(authority,'USER')
		FROM users WHERE username=$1 AND authority='ADMIN'`
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
	err := s.pool.QueryRow(ctx, `SELECT token_version FROM users WHERE id=$1 AND authority='ADMIN'`, id).Scan(&version)
	return version, err
}

func (s *Service) UpgradeAdminPassword(ctx context.Context, id int64, hash string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE id=$1 AND authority='ADMIN'`, id, hash)
	return err
}

// ---- farms ----

func (s *Service) ListFarms(ctx context.Context) ([]domain.Farm, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, owner_id, name, coalesce(location,''), created_at FROM farms ORDER BY id`)
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
	if ownerID != nil {
		var ok bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND authority='USER' AND open_id<>'')`, *ownerID).Scan(&ok); err != nil {
			return domain.Farm{}, err
		}
		if !ok {
			return domain.Farm{}, domain.ErrNotFound
		}
	}
	var f domain.Farm
	err := s.pool.QueryRow(ctx,
		`INSERT INTO farms (owner_id, name, location) VALUES ($1,$2,$3)
		 RETURNING id, owner_id, name, coalesce(location,''), created_at`,
		ownerID, name, location).Scan(&f.ID, &f.OwnerID, &f.Name, &f.Location, &f.CreatedAt)
	return f, normalizeDBError(err)
}

func (s *Service) SearchUsers(ctx context.Context, query string, limit, offset int) ([]domain.User, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, open_id, coalesce(nickname,''), token_version FROM users WHERE authority='USER' AND (id::text=$1 OR nickname ILIKE '%'||$1||'%') ORDER BY id LIMIT $2 OFFSET $3`, query, limit, offset)
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

func (s *Service) SetFarmOwner(ctx context.Context, id int64, ownerID *int64) error {
	if ownerID != nil {
		var ok bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND authority='USER' AND open_id<>'')`, *ownerID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return domain.ErrNotFound
		}
	}
	ct, err := s.pool.Exec(ctx, `UPDATE farms SET owner_id=$2 WHERE id=$1`, id, ownerID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Service) UpdateFarm(ctx context.Context, id int64, name, location string) error {
	ct, err := s.pool.Exec(ctx, `UPDATE farms SET name=$2, location=$3 WHERE id=$1`, id, name, location)
	if err == nil && ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (s *Service) FindAdminByID(ctx context.Context, id int64) (*domain.User, error) {
	u := &domain.User{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, coalesce(username,''), coalesce(open_id,''), coalesce(password_hash,''), coalesce(authority,'USER')
		 FROM users WHERE id=$1 AND authority='ADMIN'`, id).
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
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM ponds WHERE farm_id=$1`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return domain.ErrFarmHasPonds
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM farms WHERE id=$1`, id)
	return err
}

// ---- ponds ----

func (s *Service) ListPonds(ctx context.Context) ([]domain.Pond, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, farm_id, name, coalesce(area_mu,0), created_at FROM ponds ORDER BY id`)
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

func (s *Service) CreatePond(ctx context.Context, farmID int64, name string, areaMu float64) (domain.Pond, error) {
	var ok bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM farms WHERE id=$1)`, farmID).Scan(&ok); err != nil {
		return domain.Pond{}, err
	}
	if !ok {
		return domain.Pond{}, domain.ErrNotFound
	}
	var p domain.Pond
	err := s.pool.QueryRow(ctx,
		`INSERT INTO ponds (farm_id, name, area_mu) VALUES ($1,$2,$3)
		 RETURNING id, farm_id, name, coalesce(area_mu,0), created_at`,
		farmID, name, areaMu).Scan(&p.ID, &p.FarmID, &p.Name, &p.AreaMu, &p.CreatedAt)
	return p, normalizeDBError(err)
}

func (s *Service) UpdatePond(ctx context.Context, id int64, name string, areaMu float64) error {
	ct, err := s.pool.Exec(ctx, `UPDATE ponds SET name=$2, area_mu=$3 WHERE id=$1`, id, name, areaMu)
	if err == nil && ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (s *Service) DeletePond(ctx context.Context, id int64) error {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM devices WHERE pond_id=$1 OR EXISTS(SELECT 1 FROM sensor_data WHERE pond_id=$1) OR EXISTS(SELECT 1 FROM alarms WHERE pond_id=$1) OR EXISTS(SELECT 1 FROM alarm_rules WHERE pond_id=$1)`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return domain.ErrPondHasDevices
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM ponds WHERE id=$1`, id)
	return err
}

// ---- devices ----

// RegisterDevice generates a unique device_no and a one-time secret
// (only sha256 is persisted).
func (s *Service) RegisterDevice(ctx context.Context, pondID int64, name, model string, reportInterval int) (domain.Device, string, error) {
	if reportInterval != 0 && reportInterval != 60 && reportInterval != 300 {
		return domain.Device{}, "", domain.ErrInvalidRange
	}
	if reportInterval == 0 {
		reportInterval = int(s.defaultInterval.Seconds())
	}
	var ok bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ponds WHERE id=$1)`, pondID).Scan(&ok); err != nil {
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
		if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM devices WHERE device_no=$1`, no).Scan(&exists); err != nil {
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
	err := s.pool.QueryRow(ctx,
		`INSERT INTO devices (pond_id, device_no, secret_hash, name, model, report_interval)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, pond_id, device_no, coalesce(name,''), coalesce(model,''), status, last_seen_at, created_at, disabled_at, coalesce(report_interval,$6)`,
		pondID, no, hash, name, model, reportInterval).
		Scan(&d.ID, &d.PondID, &d.DeviceNo, &d.Name, &d.Model, &d.Status, &d.LastSeenAt, &d.CreatedAt, &d.DisabledAt, &d.ReportInterval)
	if err != nil {
		return domain.Device{}, "", err
	}
	return d, secHex, nil
}

func normalizeDBError(err error) error {
	if err == nil {
		return nil
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
	rows, err := s.pool.Query(ctx,
		`SELECT id, pond_id, device_no, coalesce(name,''), coalesce(model,''), status, last_seen_at, created_at, disabled_at, coalesce(report_interval,$1)
		 FROM devices WHERE ($2 OR disabled_at IS NULL) AND ($3=0 OR pond_id=$3) ORDER BY id LIMIT $4 OFFSET $5`, int(s.defaultInterval.Seconds()), includeDisabled, pondID, limit, offset)
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
	err := s.pool.QueryRow(ctx, `SELECT id,pond_id,device_no,coalesce(name,''),coalesce(model,''),status,last_seen_at,created_at,disabled_at,coalesce(report_interval,$2) FROM devices WHERE device_no=$1`, deviceNo, int(s.defaultInterval.Seconds())).Scan(&d.ID, &d.PondID, &d.DeviceNo, &d.Name, &d.Model, &d.Status, &d.LastSeenAt, &d.CreatedAt, &d.DisabledAt, &d.ReportInterval)
	return d, err
}

func (s *Service) MoveDevice(ctx context.Context, deviceNo string, pondID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id int64
	var disabled *time.Time
	if err = tx.QueryRow(ctx, `SELECT id,disabled_at FROM devices WHERE device_no=$1 FOR UPDATE`, deviceNo).Scan(&id, &disabled); err != nil {
		return err
	}
	if disabled != nil {
		return domain.ErrConflict
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ponds WHERE id=$1)`, pondID).Scan(&exists); err != nil {
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
	_, err := s.pool.Exec(ctx, `UPDATE devices SET disabled_at=coalesce(disabled_at,now()),status='offline',session_version=session_version+1 WHERE device_no=$1`, deviceNo)
	return err
}

// ---- alarm rules ----

const ruleCols = `id, pond_id, metric, min_value, max_value, level, enabled`

func scanRule(row interface{ Scan(...any) error }) (domain.AlarmRule, error) {
	var r domain.AlarmRule
	err := row.Scan(&r.ID, &r.PondID, &r.Metric, &r.Min, &r.Max, &r.Level, &r.Enabled)
	return r, err
}

func (s *Service) ListRules(ctx context.Context) ([]domain.AlarmRule, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+ruleCols+` FROM alarm_rules ORDER BY id`)
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
	if err := validateRule(rule); err != nil {
		return domain.AlarmRule{}, err
	}
	q := `INSERT INTO alarm_rules (pond_id, metric, min_value, max_value, level)
	      VALUES ($1,$2,$3,$4,$5) RETURNING ` + ruleCols
	r,err:=scanRule(s.pool.QueryRow(ctx, q,
		rule.PondID, rule.Metric, rule.Min, rule.Max, string(rule.Level)))
	if err!=nil{return domain.AlarmRule{},normalizeDBError(err)};return r,nil
}

func (s *Service) UpdateRule(ctx context.Context, rule domain.AlarmRule) error {
	if err := validateRule(rule); err != nil {
		return err
	}
	ct, err := s.pool.Exec(ctx,
		`UPDATE alarm_rules SET pond_id=$2, metric=$3, min_value=$4, max_value=$5, level=$6, enabled=$7 WHERE id=$1`,
		rule.ID, rule.PondID, rule.Metric, rule.Min, rule.Max, string(rule.Level), rule.Enabled)
	if err==nil&&ct.RowsAffected()==0{return domain.ErrNotFound};return normalizeDBError(err)
}

func (s *Service) DeleteRule(ctx context.Context, id int64) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM alarm_rules WHERE id=$1`, id)
	if err==nil&&ct.RowsAffected()==0{return domain.ErrNotFound};return err
}

// ---- alarms (admin view) ----

func (s *Service) ListAllAlarms(ctx context.Context, limit int) ([]domain.Alarm, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, device_no, pond_id, metric, current_value, threshold,
		        level, coalesce(message,''), confirmed_at, created_at
		 FROM alarms ORDER BY created_at DESC LIMIT $1`, limit)
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
	ct, err := s.pool.Exec(ctx,
		`UPDATE alarms SET confirmed_at=now() WHERE id=$1 AND confirmed_at IS NULL`, id)
	if err == nil && ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (s *Service) BatchConfirm(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, domain.ErrInvalidRange
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var total, open int
	if err = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE confirmed_at IS NULL) FROM alarms WHERE id=ANY($1)`, ids).Scan(&total, &open); err != nil {
		return 0, err
	}
	if total != len(ids) || open != len(ids) {
		return 0, domain.ErrConflict
	}
	ct, err := tx.Exec(ctx, `UPDATE alarms SET confirmed_at=now() WHERE id=ANY($1) AND confirmed_at IS NULL`, ids)
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

// ---- stats ----

func (s *Service) Stats(ctx context.Context) (domain.Stats, error) {
	var st domain.Stats
	err := s.pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM devices),
		  (SELECT count(*) FROM devices WHERE status='online'),
		  (SELECT count(*) FROM devices WHERE status='offline'),
		  (SELECT count(*) FROM alarms WHERE confirmed_at IS NULL)`).
		Scan(&st.DevicesTotal, &st.Online, &st.Offline, &st.OpenAlarms)
	if errors.Is(err, sql.ErrNoRows) {
		return st, nil
	}
	return st, err
}
