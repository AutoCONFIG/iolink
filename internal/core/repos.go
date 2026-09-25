package core

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

// ---- domain.Repository implementations over pgx ----
// appapi sees only the interfaces; swap to gRPC here when modules split.

type pondRepo struct{ pool *pgxpool.Pool }

func (r *pondRepo) ListByUser(ctx context.Context, userID int64) ([]domain.Pond, error) {
	const q = `SELECT p.id, p.farm_id, p.name, coalesce(p.area_mu,0), p.created_at
		FROM ponds p
		JOIN farms f ON f.id = p.farm_id
		WHERE f.owner_id = $1 ORDER BY p.id`
	rows, err := r.pool.Query(ctx, q, userID)
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

func (r *pondRepo) Get(ctx context.Context, id int64) (domain.Pond, error) {
	var p domain.Pond
	err := r.pool.QueryRow(ctx,
		`SELECT id, farm_id, name, coalesce(area_mu,0), created_at FROM ponds WHERE id=$1`, id).
		Scan(&p.ID, &p.FarmID, &p.Name, &p.AreaMu, &p.CreatedAt)
	return p, err
}

func (r *pondRepo) GetByUser(ctx context.Context, id, userID int64) (domain.Pond, error) {
	var p domain.Pond
	err := r.pool.QueryRow(ctx, `SELECT p.id,p.farm_id,p.name,coalesce(p.area_mu,0),p.created_at FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE p.id=$1 AND f.owner_id=$2`, id, userID).Scan(&p.ID, &p.FarmID, &p.Name, &p.AreaMu, &p.CreatedAt)
	return p, err
}

type deviceRepo struct{ pool *pgxpool.Pool }

func (r *deviceRepo) ListByPond(ctx context.Context, pondID int64) ([]domain.Device, error) {
	const q = `SELECT id, pond_id, device_no, coalesce(name,''), coalesce(model,''), status, last_seen_at, created_at, disabled_at, coalesce(report_interval,60)
		FROM devices WHERE pond_id=$1 AND disabled_at IS NULL ORDER BY id`
	rows, err := r.pool.Query(ctx, q, pondID)
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

func (r *deviceRepo) GetByDeviceNo(ctx context.Context, no string) (domain.Device, error) {
	var d domain.Device
	err := r.pool.QueryRow(ctx,
		`SELECT id, pond_id, device_no, coalesce(name,''), coalesce(model,''), status, last_seen_at, created_at,disabled_at,coalesce(report_interval,$2)
		 FROM devices WHERE device_no=$1`, no, int(60)).
		Scan(&d.ID, &d.PondID, &d.DeviceNo, &d.Name, &d.Model, &d.Status, &d.LastSeenAt, &d.CreatedAt, &d.DisabledAt, &d.ReportInterval)
	return d, err
}

func (r *deviceRepo) GetByDeviceNoForUser(ctx context.Context, no string, userID int64) (domain.Device, error) {
	var d domain.Device
	err := r.pool.QueryRow(ctx, `SELECT d.id,d.pond_id,d.device_no,coalesce(d.name,''),coalesce(d.model,''),d.status,d.last_seen_at,d.created_at,d.disabled_at,coalesce(d.report_interval,60) FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id WHERE d.device_no=$1 AND f.owner_id=$2 AND d.disabled_at IS NULL`, no, userID).Scan(&d.ID, &d.PondID, &d.DeviceNo, &d.Name, &d.Model, &d.Status, &d.LastSeenAt, &d.CreatedAt, &d.DisabledAt, &d.ReportInterval)
	return d, err
}

func (r *deviceRepo) UpdateStatus(ctx context.Context, no string, s domain.DeviceStatus) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE devices SET status=$2, last_seen_at=now() WHERE device_no=$1`, no, string(s))
	return err
}

type telemetryRepo struct {
	pool            *pgxpool.Pool
	defaultInterval time.Duration
}

// Latest reads the device shadow (kept fresh by every report) instead of
// scanning the wide time-series table.
func (r *telemetryRepo) Latest(ctx context.Context, no string) (domain.Reading, error) {
	var raw []byte
	var ts time.Time
	var stamps []byte
	var pond int64
	var interval int
	err := r.pool.QueryRow(ctx,
		`SELECT s.last,s.ts,s.timestamps,s.pond_id,coalesce(d.report_interval,$2) FROM device_shadows s JOIN devices d ON d.device_no=s.device_no AND d.pond_id=s.pond_id WHERE s.device_no=$1 AND d.disabled_at IS NULL`, no, int(r.defaultInterval.Seconds())).
		Scan(&raw, &ts, &stamps, &pond, &interval)
	if err != nil {
		return domain.Reading{}, err
	}
	var m map[string]float64
	if err := json.Unmarshal(raw, &m); err != nil {
		return domain.Reading{}, err
	}
	var timestamps map[string]time.Time
	if err := json.Unmarshal(stamps, &timestamps); err != nil {
		return domain.Reading{}, err
	}
	var signal *int
	if v, ok := m["signal"]; ok {
		x := int(v)
		signal = &x
	}
	ptr := func(k string) *float64 {
		if v, ok := m[k]; ok {
			return &v
		}
		return nil
	}
	return domain.Reading{
		DeviceNo: no,
		PondID:   pond, Timestamps: timestamps, ReportInterval: interval, Battery: ptr("battery"), Signal: signal,
		Timestamp:   ts,
		Temperature: ptr("temperature"),
		DO:          ptr("dissolved_oxygen"),
		PH:          ptr("ph"),
		Turbidity:   ptr("turbidity"),
		Salinity:    ptr("salinity"),
	}, nil
}

func (r *telemetryRepo) History(ctx context.Context, no, metric string, from, to time.Time, maxPoints int) ([]domain.MetricPoint, error) {
	col, ok := domain.MetricColumns[metric]
	if !ok {
		return nil, domain.ErrUnknownMetric
	}
	if maxPoints < 1 || maxPoints > 200 || !to.After(from) {
		return nil, domain.ErrInvalidRange
	}
	// Origin at from avoids an extra bucket at either endpoint; [from,to).
	width := int64(math.Ceil(float64(to.Sub(from).Microseconds()) / float64(maxPoints)))
	if width < 1 {
		width = 1
	}
	bucket := fmt.Sprintf("%d microseconds", width)
	q := `SELECT date_bin($4::interval,ts,$2::timestamptz),avg(` + col + `) FROM sensor_data WHERE device_no=$1 AND pond_id=(SELECT pond_id FROM devices WHERE device_no=$1) AND ts >= $2 AND ts < $3 AND ` + col + ` IS NOT NULL GROUP BY 1 ORDER BY 1 LIMIT $5`
	rows, err := r.pool.Query(ctx, q, no, from, to, bucket, maxPoints)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.MetricPoint{}
	for rows.Next() {
		var p domain.MetricPoint
		if err := rows.Scan(&p.Ts, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type alarmRepo struct{ pool *pgxpool.Pool }

func (r *alarmRepo) ListByUser(ctx context.Context, userID int64, limit int) ([]domain.Alarm, error) {
	const q = `SELECT a.id, a.device_no, a.pond_id, a.metric, a.current_value, a.threshold,
		a.level, coalesce(a.message,''), a.confirmed_at, a.created_at
		FROM alarms a
		JOIN ponds p ON p.id = a.pond_id
		JOIN farms f ON f.id = p.farm_id
		WHERE f.owner_id = $1
		ORDER BY a.created_at DESC LIMIT $2`
	rows, err := r.pool.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Alarm
	for rows.Next() {
		var al domain.Alarm
		if err := rows.Scan(&al.ID, &al.DeviceNo, &al.PondID, &al.Metric, &al.CurrentValue,
			&al.Threshold, &al.Level, &al.Message, &al.ConfirmedAt, &al.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, al)
	}
	return out, rows.Err()
}

func (r *alarmRepo) Confirm(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE alarms SET confirmed_at=now() WHERE id=$1 AND confirmed_at IS NULL`, id)
	return err
}

func (r *alarmRepo) ConfirmByUser(ctx context.Context, id, userID int64) error {
	ct, err := r.pool.Exec(ctx, `UPDATE alarms a SET confirmed_at=now() FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE a.id=$1 AND a.pond_id=p.id AND f.owner_id=$2 AND a.confirmed_at IS NULL`, id, userID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type alarmRuleRepo struct{ pool *pgxpool.Pool }

func (r *alarmRuleRepo) RulesForDevice(ctx context.Context, no string) ([]domain.AlarmRule, error) {
	const q = `SELECT r.id, r.pond_id, r.metric, r.min_value, r.max_value, r.level, r.enabled
		FROM alarm_rules r JOIN devices d ON d.pond_id = r.pond_id
		WHERE d.device_no=$1 AND r.enabled`
	rows, err := r.pool.Query(ctx, q, no)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AlarmRule
	for rows.Next() {
		var ar domain.AlarmRule
		if err := rows.Scan(&ar.ID, &ar.PondID, &ar.Metric, &ar.Min, &ar.Max, &ar.Level, &ar.Enabled); err != nil {
			return nil, err
		}
		out = append(out, ar)
	}
	return out, rows.Err()
}
