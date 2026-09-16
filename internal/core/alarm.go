package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/event"
)

// alarmEngine evaluates pond-scoped threshold rules against each incoming
// properties event. Rules live in alarm_rules (per pond per metric); this
// keeps threshold tuning a data change, not a code change.
type alarmEngine struct {
	pool     *pgxpool.Pool
	log      *slog.Logger
	notifier func(domain.Alarm) // late-bound by Service.SetNotifier; nil = off
}

type rule struct {
	id       int64
	metric   string
	minValue *float64
	maxValue *float64
	level    string
}

func (a *alarmEngine) evaluate(e event.Event) error {
	if len(e.Properties) == 0 {
		return nil
	}
	rules, err := a.rulesForDevice(e.DeviceNo)
	if err != nil {
		return fmt.Errorf("load rules: %w", err)
	}
	for _, r := range rules {
		v, ok := e.Properties[r.metric]
		if !ok {
			continue
		}
		if breached(r, v) {
			if err := a.raise(e, r, v); err != nil {
				a.log.Error("raise alarm", "device", e.DeviceNo, "metric", r.metric, "err", err)
			}
		}
	}
	return nil
}

func breached(r rule, v float64) bool {
	if r.minValue != nil && v < *r.minValue {
		return true
	}
	if r.maxValue != nil && v > *r.maxValue {
		return true
	}
	return false
}

func (a *alarmEngine) rulesForDevice(deviceNo string) ([]rule, error) {
	const q = `SELECT r.id, r.metric, r.min_value, r.max_value, r.level
		FROM alarm_rules r
		JOIN devices d ON d.pond_id = r.pond_id
		WHERE d.device_no = $1 AND r.enabled`
	rows, err := a.pool.Query(context.Background(), q, deviceNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []rule
	for rows.Next() {
		var r rule
		if err := rows.Scan(&r.id, &r.metric, &r.minValue, &r.maxValue, &r.level); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// raise inserts an alarm unless an unconfirmed alarm for the same
// device+metric is already open (dedup, avoid notification storms).
func (a *alarmEngine) raise(e event.Event, r rule, v float64) error {
	const q = `INSERT INTO alarms
		(device_no, pond_id, metric, current_value, threshold, level, message)
		SELECT $1::varchar, d.pond_id, $2::varchar, $3, $4, $5::varchar, $6::varchar
		FROM devices d WHERE d.device_no = $1::varchar
		  AND NOT EXISTS (
			SELECT 1 FROM alarms a
			WHERE a.device_no = $1::varchar AND a.metric = $2::varchar AND a.confirmed_at IS NULL)`
	threshold := 0.0
	msg := ""
	if r.minValue != nil {
		threshold = *r.minValue
		msg = fmt.Sprintf("%s 低于阈值 %.1f", r.metric, *r.minValue)
	}
	if r.maxValue != nil {
		threshold = *r.maxValue
		msg = fmt.Sprintf("%s 高于阈值 %.1f", r.metric, *r.maxValue)
	}
	ct, err := a.pool.Exec(context.Background(), q,
		e.DeviceNo, r.metric, v, threshold, r.level, msg)
	if err != nil {
		return err
	}
	if ct.RowsAffected() > 0 {
		MetricAlarmsTotal.Inc()
		a.log.Warn("ALARM raised", "device", e.DeviceNo, "metric", r.metric, "value", v, "ts", time.Now())
		a.notify(e, r, v)
	}
	return nil
}

// notify resolves the alarm row we just inserted and hands it to the
// notifier (asynchronous; never blocks the ingest path).

// notify resolves the just-inserted alarm row and hands it to the notifier
// asynchronously (never blocks the ingest path). openIDs are resolved by the
// Service wrapper via dispatchAlarmNotifications.
func (a *alarmEngine) notify(e event.Event, r rule, v float64) {
	if a.notifier == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var al domain.Alarm
		err := a.pool.QueryRow(ctx, `
			SELECT id, device_no, pond_id, metric, current_value, threshold, level,
			       coalesce(message,''), created_at
			FROM alarms WHERE device_no=$1 AND metric=$2 AND confirmed_at IS NULL
			ORDER BY created_at DESC LIMIT 1`, e.DeviceNo, r.metric).
			Scan(&al.ID, &al.DeviceNo, &al.PondID, &al.Metric, &al.CurrentValue,
				&al.Threshold, &al.Level, &al.Message, &al.CreatedAt)
		if err != nil {
			a.log.Warn("notify: load alarm", "err", err)
			return
		}
		a.notifier(al)
	}()
}
