package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/event"
	"github.com/jackc/pgx/v5"
)

func (s *Service) storeGenericReading(e event.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if e.MessageID != "" {
		if _, err = tx.Exec(ctx, `DELETE FROM ingest_messages WHERE received_at <= now()-interval '24 hours'`); err != nil {
			return err
		}
		ct, dedupErr := tx.Exec(ctx, `INSERT INTO ingest_messages(device_no,message_id,received_at) VALUES($1,$2,now()) ON CONFLICT DO NOTHING`, e.DeviceNo, e.MessageID)
		if dedupErr != nil {
			return dedupErr
		}
		if ct.RowsAffected() == 0 {
			return tx.Commit(ctx)
		}
	}
	var pond, tenant, productID int64
	var version int
	var rawSchema []byte
	err = tx.QueryRow(ctx, `SELECT d.pond_id,f.tenant_id,d.product_id,d.model_version,m.schema FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id JOIN tenants t ON t.id=f.tenant_id AND t.active JOIN product_models m ON m.product_id=d.product_id AND m.version=d.model_version AND m.published_at IS NOT NULL WHERE d.device_no=$1 AND d.disabled_at IS NULL FOR UPDATE OF d`, e.DeviceNo).Scan(&pond, &tenant, &productID, &version, &rawSchema)
	if err != nil {
		return err
	}
	var schema struct {
		Fields []domain.ModelField `json:"fields"`
	}
	if err = json.Unmarshal(rawSchema, &schema); err != nil {
		return err
	}
	accepted, rejected := domain.ValidateTelemetryProperties(schema.Fields, e.GenericProperties)
	if len(rejected) > 0 || len(accepted) == 0 {
		return domain.ErrInvalidProductModel
	}
	properties, err := json.Marshal(accepted)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO telemetry(ts,device_no,pond_id,tenant_id,product_id,model_version,properties) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (ts,device_no) DO NOTHING`, e.Ts, e.DeviceNo, pond, tenant, productID, version, properties); err != nil {
		return err
	}
	if err = genericShadowUpsert(ctx, tx, e, pond, tenant, productID, version, accepted); err != nil {
		return err
	}
	numeric := make(map[string]float64)
	for name, value := range accepted {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			continue
		}
		var number float64
		if json.Unmarshal(value, &number) == nil {
			numeric[name] = number
		}
	}
	alarms, err := s.al.evaluate(ctx, tx, event.Event{DeviceNo: e.DeviceNo, ProductID: productID, Ts: e.Ts, Properties: numeric}, pond)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE devices SET last_seen_at=GREATEST(last_seen_at,$2) WHERE device_no=$1`, e.DeviceNo, e.Ts); err != nil {
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

func genericShadowUpsert(ctx context.Context, tx pgx.Tx, e event.Event, pond, tenant, productID int64, version int, accepted map[string]json.RawMessage) error {
	values := map[string]json.RawMessage{}
	stamps := map[string]time.Time{}
	var raw, stampRaw []byte
	var oldPond *int64
	var oldProduct *int64
	err := tx.QueryRow(ctx, `SELECT last,timestamps,pond_id,product_id FROM device_shadows WHERE device_no=$1`, e.DeviceNo).Scan(&raw, &stampRaw, &oldPond, &oldProduct)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil && oldPond != nil && *oldPond == pond && oldProduct != nil && *oldProduct == productID {
		if json.Unmarshal(raw, &values) != nil || json.Unmarshal(stampRaw, &stamps) != nil {
			return errors.New("invalid shadow")
		}
	}
	for name, value := range accepted {
		if prior, ok := stamps[name]; !ok || !e.Ts.Before(prior) {
			values[name] = value
			stamps[name] = e.Ts
		}
	}
	last, err := json.Marshal(values)
	if err != nil {
		return err
	}
	timestamps, err := json.Marshal(stamps)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO device_shadows(device_no,last,signal,ts,pond_id,tenant_id,timestamps,model_version,product_id) VALUES($1,$2,NULL,$3,$4,$5,$6,$7,$8) ON CONFLICT(device_no) DO UPDATE SET last=excluded.last,ts=GREATEST(device_shadows.ts,excluded.ts),pond_id=excluded.pond_id,tenant_id=excluded.tenant_id,timestamps=excluded.timestamps,model_version=excluded.model_version,product_id=excluded.product_id`, e.DeviceNo, last, e.Ts, pond, tenant, timestamps, version, productID)
	return err
}
