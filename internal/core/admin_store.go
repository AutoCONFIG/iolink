package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
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

func (s *Service) CreateFarm(ctx context.Context, ownerID int64, name, location string) (domain.Farm, error) {
	var f domain.Farm
	err := s.pool.QueryRow(ctx,
		`INSERT INTO farms (owner_id, name, location) VALUES ($1,$2,$3)
		 RETURNING id, owner_id, name, coalesce(location,''), created_at`,
		ownerID, name, location).Scan(&f.ID, &f.OwnerID, &f.Name, &f.Location, &f.CreatedAt)
	return f, err
}

func (s *Service) UpdateFarm(ctx context.Context, id int64, name, location string) error {
	_, err := s.pool.Exec(ctx, `UPDATE farms SET name=$2, location=$3 WHERE id=$1`, id, name, location)
	return err
}

func (s *Service) DeleteFarm(ctx context.Context, id int64) error {
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
	var p domain.Pond
	err := s.pool.QueryRow(ctx,
		`INSERT INTO ponds (farm_id, name, area_mu) VALUES ($1,$2,$3)
		 RETURNING id, farm_id, name, coalesce(area_mu,0), created_at`,
		farmID, name, areaMu).Scan(&p.ID, &p.FarmID, &p.Name, &p.AreaMu, &p.CreatedAt)
	return p, err
}

func (s *Service) UpdatePond(ctx context.Context, id int64, name string, areaMu float64) error {
	_, err := s.pool.Exec(ctx, `UPDATE ponds SET name=$2, area_mu=$3 WHERE id=$1`, id, name, areaMu)
	return err
}

func (s *Service) DeletePond(ctx context.Context, id int64) error {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM devices WHERE pond_id=$1`, id).Scan(&n); err != nil {
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
func (s *Service) RegisterDevice(ctx context.Context, pondID int64, model string) (domain.Device, string, error) {
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		return domain.Device{}, "", err
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
		`INSERT INTO devices (pond_id, device_no, secret_hash, model)
		 VALUES ($1,$2,$3,$4)
		 RETURNING id, pond_id, device_no, coalesce(model,''), status, last_seen_at, created_at`,
		pondID, no, hash, model).
		Scan(&d.ID, &d.PondID, &d.DeviceNo, &d.Model, &d.Status, &d.LastSeenAt, &d.CreatedAt)
	if err != nil {
		return domain.Device{}, "", err
	}
	return d, secHex, nil
}

func (s *Service) ListDevices(ctx context.Context) ([]domain.Device, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, pond_id, device_no, coalesce(model,''), status, last_seen_at, created_at
		 FROM devices ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Device
	for rows.Next() {
		var d domain.Device
		if err := rows.Scan(&d.ID, &d.PondID, &d.DeviceNo, &d.Model, &d.Status, &d.LastSeenAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Service) DeleteDevice(ctx context.Context, deviceNo string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM devices WHERE device_no=$1`, deviceNo)
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
	q := `INSERT INTO alarm_rules (pond_id, metric, min_value, max_value, level)
	      VALUES ($1,$2,$3,$4,$5) RETURNING ` + ruleCols
	return scanRule(s.pool.QueryRow(ctx, q,
		rule.PondID, rule.Metric, rule.Min, rule.Max, string(rule.Level)))
}

func (s *Service) UpdateRule(ctx context.Context, rule domain.AlarmRule) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE alarm_rules SET pond_id=$2, metric=$3, min_value=$4, max_value=$5, level=$6, enabled=$7 WHERE id=$1`,
		rule.ID, rule.PondID, rule.Metric, rule.Min, rule.Max, string(rule.Level), rule.Enabled)
	return err
}

func (s *Service) DeleteRule(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM alarm_rules WHERE id=$1`, id)
	return err
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
	_, err := s.pool.Exec(ctx,
		`UPDATE alarms SET confirmed_at=now() WHERE id=$1 AND confirmed_at IS NULL`, id)
	return err
}

func (s *Service) BatchConfirm(ctx context.Context, ids []int64) (int64, error) {
	ct, err := s.pool.Exec(ctx,
		`UPDATE alarms SET confirmed_at=now() WHERE id=ANY($1) AND confirmed_at IS NULL`, ids)
	if err != nil {
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

// shadowUpsert keeps the latest reading per device (device_shadows).
func (s *Service) shadowUpsert(deviceNo string, props map[string]float64, ts time.Time) error {
	raw, err := json.Marshal(props)
	if err != nil {
		return err
	}
	var sig *int
	if v, ok := props["signal"]; ok {
		i := int(v)
		sig = &i
	}
	_, err = s.pool.Exec(context.Background(),
		`INSERT INTO device_shadows (device_no, last, signal, ts)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (device_no) DO UPDATE SET last=$2, signal=$3, ts=$4`,
		deviceNo, raw, sig, ts)
	return err
}
