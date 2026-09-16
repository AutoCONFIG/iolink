package core

import (
	"context"
	"fmt"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/event"
)

// storeReading persists a normalized telemetry row into the sensor_data
// hypertable. Validation of ranges already happened in access; core does a
// light re-check of field count only (cheap, keeps DB clean).
func (s *Service) storeReading(e event.Event) error {
	get := func(k string) *float64 {
		if v, ok := e.Properties[k]; ok {
			return &v
		}
		return nil
	}
	var sig *int
	if v, ok := e.Properties["signal"]; ok {
		i := int(v)
		sig = &i
	}
	const q = `INSERT INTO sensor_data
		(ts, device_no, temperature, dissolved_oxygen, ph, turbidity, salinity, battery, signal)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	if _, err := s.pool.Exec(context.Background(), q,
		e.Ts, e.DeviceNo,
		get("temperature"), get("dissolved_oxygen"), get("ph"),
		get("turbidity"), get("salinity"), get("battery"), sig); err != nil {
		return fmt.Errorf("insert sensor_data: %w", err)
	}
	if err := s.shadowUpsert(e.DeviceNo, e.Properties, e.Ts); err != nil {
		return fmt.Errorf("upsert shadow: %w", err)
	}
	return nil
}

func (s *Service) storeStatus(e event.Event) error {
	const q = `UPDATE devices SET status=$2, last_seen_at=$3 WHERE device_no=$1`
	status := "offline"
	if e.Online {
		status = "online"
	}
	_, err := s.pool.Exec(context.Background(), q, e.DeviceNo, status, time.Now())
	return err
}
