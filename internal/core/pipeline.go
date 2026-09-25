package core

import (
	"context"
	"encoding/json"
	"errors"
	"git.hyhy.fun/rsplab/iolink/internal/event"
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
	ranges := map[string][2]float64{"temperature": {0, 50}, "dissolved_oxygen": {0, 20}, "ph": {0, 14}, "turbidity": {0, 1000}, "salinity": {0, 50}, "battery": {0, 100}, "signal": {-120, 0}}
	for k, v := range e.Properties {
		r, ok := ranges[k]
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < r[0] || v > r[1] || (k == "signal" && v != math.Trunc(v)) {
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
	var pond int64
	// Serialize this device with moves/disables and concurrent reports.
	if err = tx.QueryRow(ctx, "SELECT pond_id FROM devices WHERE device_no=$1 AND disabled_at IS NULL FOR UPDATE", e.DeviceNo).Scan(&pond); err != nil {
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
	_, err = tx.Exec(ctx, `INSERT INTO sensor_data(ts,device_no,pond_id,temperature,dissolved_oxygen,ph,turbidity,salinity,battery,signal) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, e.Ts, e.DeviceNo, pond, get("temperature"), get("dissolved_oxygen"), get("ph"), get("turbidity"), get("salinity"), get("battery"), sig)
	if err != nil {
		return err
	}
	if err = shadowUpsert(ctx, tx, e, pond); err != nil {
		return err
	}
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
	for _, a := range alarms {
		MetricAlarmsTotal.Inc()
		if s.al.notifier != nil {
			go s.al.notifier(a)
		}
	}
	return nil
}
func shadowUpsert(ctx context.Context, tx pgx.Tx, e event.Event, pond int64) error {
	values := map[string]float64{}
	stamps := map[string]time.Time{}
	var raw, times []byte
	var oldPond *int64
	latest := e.Ts
	err := tx.QueryRow(ctx, "SELECT last,timestamps,ts,pond_id FROM device_shadows WHERE device_no=$1", e.DeviceNo).Scan(&raw, &times, &latest, &oldPond)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil && oldPond != nil && *oldPond == pond {
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
	_, err = tx.Exec(ctx, `INSERT INTO device_shadows(device_no,last,signal,ts,pond_id,timestamps) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(device_no) DO UPDATE SET last=excluded.last,signal=excluded.signal,ts=excluded.ts,pond_id=excluded.pond_id,timestamps=excluded.timestamps`, e.DeviceNo, raw, signal, latest, pond, times)
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
	if err := tx.QueryRow(ctx, "SELECT status FROM devices WHERE device_no=$1 AND disabled_at IS NULL FOR UPDATE", e.DeviceNo).Scan(&previous); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE devices SET status=$2::varchar,last_seen_at=CASE WHEN $2::varchar='online' THEN GREATEST(last_seen_at,$3) ELSE last_seen_at END WHERE device_no=$1 AND disabled_at IS NULL`, e.DeviceNo, status, e.Ts)
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
