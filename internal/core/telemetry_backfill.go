package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrBackfillInput = errors.New("invalid telemetry backfill input")

type BackfillProgress struct {
	DeviceNo  string
	Timestamp time.Time
	Rows      int
}

func BackfillWaterTelemetry(ctx context.Context, pool *pgxpool.Pool, job string, batch int) (BackfillProgress, error) {
	return backfillWaterTelemetry(ctx, pool, job, batch, -1)
}

func backfillWaterTelemetry(ctx context.Context, pool *pgxpool.Pool, job string, batch, failAfter int) (BackfillProgress, error) {
	if job == "" || !strings.Contains(job, "fixture") || batch < 1 || batch > 10000 {
		return BackfillProgress{}, ErrBackfillInput
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return BackfillProgress{}, err
	}
	defer tx.Rollback(ctx)
	var lastDevice string
	var lastTS time.Time
	err = tx.QueryRow(ctx, `SELECT last_device_no,last_ts FROM telemetry_backfill_checkpoints WHERE job_name=$1 FOR UPDATE`, job).Scan(&lastDevice, &lastTS)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return BackfillProgress{}, err
	}
	rows, err := tx.Query(ctx, `SELECT s.ts,s.device_no,s.pond_id,s.tenant_id,p.id,1,jsonb_strip_nulls(jsonb_build_object('temperature',s.temperature,'dissolved_oxygen',s.dissolved_oxygen,'ph',s.ph,'turbidity',s.turbidity,'salinity',s.salinity,'battery',s.battery,'signal',s.signal)) FROM sensor_data s CROSS JOIN products p JOIN tenants st ON st.id=p.tenant_id AND st.name='__iolink_system__' WHERE p.name='water-quality' AND (s.device_no,s.ts)>($1,$2) ORDER BY s.device_no,s.ts LIMIT $3`, lastDevice, lastTS, batch)
	if err != nil {
		return BackfillProgress{}, err
	}
	defer rows.Close()
	progress := BackfillProgress{DeviceNo: lastDevice, Timestamp: lastTS}
	type item struct {
		ts           time.Time
		device       string
		pond, tenant int64
		product      int64
		version      int
		properties   []byte
	}
	items := make([]item, 0, batch)
	for rows.Next() {
		var ts time.Time
		var device string
		var pond, tenant, product int64
		var version int
		var properties []byte
		if err := rows.Scan(&ts, &device, &pond, &tenant, &product, &version, &properties); err != nil {
			return progress, err
		}
		items = append(items, item{ts, device, pond, tenant, product, version, properties})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return progress, err
	}
	for _, value := range items {
		if failAfter >= 0 && progress.Rows >= failAfter {
			return progress, errors.New("fixture backfill interrupted")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO telemetry(ts,device_no,pond_id,tenant_id,product_id,model_version,properties) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, value.ts, value.device, value.pond, value.tenant, value.product, value.version, value.properties); err != nil {
			return progress, err
		}
		progress.DeviceNo, progress.Timestamp, progress.Rows = value.device, value.ts, progress.Rows+1
	}
	if progress.Rows > 0 {
		_, err = tx.Exec(ctx, `INSERT INTO telemetry_backfill_checkpoints(job_name,last_device_no,last_ts) VALUES($1,$2,$3) ON CONFLICT(job_name) DO UPDATE SET last_device_no=excluded.last_device_no,last_ts=excluded.last_ts,updated_at=now()`, job, progress.DeviceNo, progress.Timestamp)
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO telemetry_backfill_checkpoints(job_name) VALUES($1) ON CONFLICT DO NOTHING`, job)
	}
	if err != nil {
		return progress, err
	}
	if err = tx.Commit(ctx); err != nil {
		return progress, err
	}
	return progress, nil
}
