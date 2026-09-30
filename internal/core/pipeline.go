package core

import (
	"context"
	"encoding/json"
	"errors"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/ingestion"
	"github.com/jackc/pgx/v5"
	"math"
	"time"
	"unicode/utf8"
)

func (s *Service) storeReading(e event.Event) error {
	if len(e.Properties) == 0 {
		return nil
	}
	if e.Ts.IsZero() || !utf8.ValidString(e.MessageID) || utf8.RuneCountInString(e.MessageID) > 128 {
		return errors.New("invalid event")
	}
	for k, v := range e.Properties {
		low, high, ok := ingestion.MetricRange(k)
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < low || v > high || (k == "signal" && v != math.Trunc(v)) {
			return errors.New("invalid normalized metric")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var pond, tenant, productID int64
	var modelVersion int
	var modelSchema []byte
	// Serialize this device with moves/disables and concurrent reports.
	if err = tx.QueryRow(ctx, `SELECT d.pond_id, f.tenant_id, d.product_id, d.model_version, m.schema
		FROM devices d
		JOIN ponds p ON p.id=d.pond_id
		JOIN farms f ON f.id=p.farm_id
		JOIN tenants t ON t.id=f.tenant_id AND t.active
		JOIN product_models m ON m.product_id=d.product_id AND m.version=d.model_version AND m.published_at IS NOT NULL
		WHERE d.device_no=$1 AND d.disabled_at IS NULL
		FOR UPDATE OF d`, e.DeviceNo).Scan(&pond, &tenant, &productID, &modelVersion, &modelSchema); err != nil {
		return err
	}
	if e.MessageID != "" {
		// Retention uses receipt wall-clock, never an untrusted event timestamp.
		if _, err = tx.Exec(ctx, "DELETE FROM ingest_messages WHERE received_at <= now()-interval '24 hours'"); err != nil {
			return err
		}
		ct, err := tx.Exec(ctx, `INSERT INTO ingest_messages(device_no,message_id,received_at) VALUES($1,$2,now()) ON CONFLICT DO NOTHING`, e.DeviceNo, e.MessageID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return tx.Commit(ctx)
		}
	}
	get := func(k string) *float64 {
		v, ok := e.Properties[k]
		if ok {
			return &v
		}
		return nil
	}
	var sig *int
	if v, ok := e.Properties["signal"]; ok {
		x := int(v)
		sig = &x
	}
	_, err = tx.Exec(ctx, `INSERT INTO sensor_data(ts,device_no,pond_id,tenant_id,temperature,dissolved_oxygen,ph,turbidity,salinity,battery,signal) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, e.Ts, e.DeviceNo, pond, tenant, get("temperature"), get("dissolved_oxygen"), get("ph"), get("turbidity"), get("salinity"), get("battery"), sig)
	if err != nil {
		return err
	}
	var schema struct {
		Fields []domain.ModelField `json:"fields"`
	}
	if err = json.Unmarshal(modelSchema, &schema); err != nil {
		return err
	}
	allowed := make(map[string]struct{}, len(schema.Fields))
	for _, field := range schema.Fields {
		allowed[field.Identifier] = struct{}{}
	}
	generic := make(map[string]float64, len(e.Properties))
	for name, value := range e.Properties {
		if _, ok := allowed[name]; ok {
			generic[name] = value
		}
	}
	properties, err := json.Marshal(generic)
	if err != nil {
		return err
	}
	if len(generic) > 0 {
		_, err = tx.Exec(ctx, `INSERT INTO telemetry(ts,device_no,pond_id,tenant_id,product_id,model_version,properties) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (ts,device_no) DO NOTHING`, e.Ts, e.DeviceNo, pond, tenant, productID, modelVersion, properties)
	}
	if err != nil {
		return err
	}
	if err = shadowUpsert(ctx, tx, e, pond, tenant, productID, modelVersion); err != nil {
		return err
	}
	e.ProductID = productID
	alarms, err := s.al.evaluate(ctx, tx, e, pond)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE devices SET last_seen_at=GREATEST(last_seen_at,$2) WHERE device_no=$1", e.DeviceNo, e.Ts); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	MetricTelemetryTotal.Inc()
	for range alarms {
		MetricAlarmsTotal.Inc()
	}
	return nil
}
func shadowUpsert(ctx context.Context, tx pgx.Tx, e event.Event, pond, tenant, productID int64, modelVersion int) error {
	values := map[string]float64{}
	stamps := map[string]time.Time{}
	var raw, times []byte
	var oldPond *int64
	var oldProduct *int64
	latest := e.Ts
	err := tx.QueryRow(ctx, "SELECT last,timestamps,ts,pond_id,product_id FROM device_shadows WHERE device_no=$1", e.DeviceNo).Scan(&raw, &times, &latest, &oldPond, &oldProduct)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil && oldPond != nil && *oldPond == pond && oldProduct != nil && *oldProduct == productID {
		if err = json.Unmarshal(raw, &values); err != nil {
			return err
		}
		if err = json.Unmarshal(times, &stamps); err != nil {
			return err
		}
	} else {
		latest = e.Ts
	}
	for k, v := range e.Properties {
		if previous, ok := stamps[k]; !ok || !e.Ts.Before(previous) {
			values[k] = v
			stamps[k] = e.Ts
		}
	}
	if e.Ts.After(latest) {
		latest = e.Ts
	}
	raw, err = json.Marshal(values)
	if err != nil {
		return err
	}
	times, err = json.Marshal(stamps)
	if err != nil {
		return err
	}
	var signal *int
	if v, ok := values["signal"]; ok {
		x := int(v)
		signal = &x
	}
	_, err = tx.Exec(ctx, `INSERT INTO device_shadows(device_no,last,signal,ts,pond_id,tenant_id,timestamps,model_version,product_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(device_no) DO UPDATE SET last=excluded.last,signal=excluded.signal,ts=excluded.ts,pond_id=excluded.pond_id,tenant_id=excluded.tenant_id,timestamps=excluded.timestamps,model_version=excluded.model_version,product_id=excluded.product_id`, e.DeviceNo, raw, signal, latest, pond, tenant, times, modelVersion, productID)
	return err
}
func (s *Service) storeStatus(e event.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	status := "offline"
	if e.Online {
		status = "online"
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var previous string
	if err := tx.QueryRow(ctx, "SELECT d.status FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id JOIN tenants t ON t.id=f.tenant_id WHERE d.device_no=$1 AND d.disabled_at IS NULL AND t.active FOR UPDATE OF d", e.DeviceNo).Scan(&previous); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE devices d SET status=$2::varchar,last_seen_at=CASE WHEN $2::varchar='online' THEN GREATEST(d.last_seen_at,$3) ELSE d.last_seen_at END FROM ponds p JOIN farms f ON f.id=p.farm_id JOIN tenants t ON t.id=f.tenant_id WHERE d.pond_id=p.id AND d.device_no=$1 AND d.disabled_at IS NULL AND t.active`, e.DeviceNo, status, e.Ts)
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if previous != status {
		if e.Online {
			MetricDevicesOnline.Inc()
		} else {
			MetricDevicesOnline.Dec()
		}
	}
	return nil
}
