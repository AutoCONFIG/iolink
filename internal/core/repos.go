package core

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"iolink/contracts/domain"
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

type deviceRepo struct{ pool *pgxpool.Pool }

func (r *deviceRepo) ListByPond(ctx context.Context, pondID int64) ([]domain.Device, error) {
	const q = `SELECT id, pond_id, device_no, coalesce(model,''), status, last_seen_at, created_at
		FROM devices WHERE pond_id=$1 ORDER BY id`
	rows, err := r.pool.Query(ctx, q, pondID)
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

func (r *deviceRepo) GetByDeviceNo(ctx context.Context, no string) (domain.Device, error) {
	var d domain.Device
	err := r.pool.QueryRow(ctx,
		`SELECT id, pond_id, device_no, coalesce(model,''), status, last_seen_at, created_at
		 FROM devices WHERE device_no=$1`, no).
		Scan(&d.ID, &d.PondID, &d.DeviceNo, &d.Model, &d.Status, &d.LastSeenAt, &d.CreatedAt)
	return d, err
}

func (r *deviceRepo) UpdateStatus(ctx context.Context, no string, s domain.DeviceStatus) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE devices SET status=$2, last_seen_at=now() WHERE device_no=$1`, no, string(s))
	return err
}

type telemetryRepo struct{ pool *pgxpool.Pool }

func (r *telemetryRepo) Latest(ctx context.Context, no string) (domain.Reading, error) {
	var rd domain.Reading
	rd.DeviceNo = no
	err := r.pool.QueryRow(ctx, `
		SELECT ts, temperature, dissolved_oxygen, ph, turbidity, salinity
		FROM sensor_data WHERE device_no=$1 ORDER BY ts DESC LIMIT 1`, no).
		Scan(&rd.Timestamp, &rd.Temperature, &rd.DO, &rd.PH, &rd.Turbidity, &rd.Salinity)
	return rd, err
}

func (r *telemetryRepo) History(ctx context.Context, no, metric string, from, to time.Time, maxPoints int) ([]domain.MetricPoint, error) {
	col, ok := metricColumns[metric]
	if !ok {
		return nil, domain.ErrUnknownMetric
	}
	// bucket to keep points bounded for charting
	bucket := "1 minute"
	if d := to.Sub(from); d > 48*time.Hour {
		bucket = "1 hour"
	}
	if d := to.Sub(from); d > 30*24*time.Hour {
		bucket = "1 day"
	}
	q := `SELECT time_bucket($4, ts), avg(` + col + `)
		FROM sensor_data
		WHERE device_no=$1 AND ts BETWEEN $2 AND $3 AND ` + col + ` IS NOT NULL
		GROUP BY 1 ORDER BY 1`
	rows, err := r.pool.Query(ctx, q, no, from, to, bucket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.MetricPoint
	for rows.Next() {
		var p domain.MetricPoint
		if err := rows.Scan(&p.Ts, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

var metricColumns = map[string]string{
	"temperature":     "temperature",
	"dissolved_oxygen": "dissolved_oxygen",
	"ph":              "ph",
	"turbidity":       "turbidity",
	"salinity":        "salinity",
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
