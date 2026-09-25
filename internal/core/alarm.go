package core

import (
	"context"
	"errors"
	"fmt"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/event"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"math"
)

type alarmEngine struct {
	log      *slog.Logger
	notifier func(domain.Alarm)
}
type rule struct {
	metric             string
	minValue, maxValue *float64
	level              string
}

func breached(r rule, v float64) bool {
	return r.minValue != nil && v < *r.minValue || r.maxValue != nil && v > *r.maxValue
}
func (a *alarmEngine) evaluate(ctx context.Context, tx pgx.Tx, e event.Event, pond int64) ([]domain.Alarm, error) {
	rows, err := tx.Query(ctx, "SELECT metric,min_value,max_value,level FROM alarm_rules WHERE pond_id=$1 AND enabled", pond)
	if err != nil {
		return nil, err
	}
	var rules []rule
	for rows.Next() {
		var r rule
		if err = rows.Scan(&r.metric, &r.minValue, &r.maxValue, &r.level); err != nil {
			rows.Close()
			return nil, err
		}
		rules = append(rules, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var out []domain.Alarm
	for _, r := range rules {
		v, ok := e.Properties[r.metric]
		if !ok || !breached(r, v) {
			continue
		}
		threshold := 0.0
		direction := "低于"
		if r.minValue != nil && v < *r.minValue {
			threshold = *r.minValue
		} else {
			threshold = *r.maxValue
			direction = "高于"
		}
		al := domain.Alarm{DeviceNo: e.DeviceNo, PondID: pond, Metric: r.metric, CurrentValue: v, Threshold: threshold, Level: domain.AlarmLevel(r.level), Message: fmt.Sprintf("%s %s阈值 %g", r.metric, direction, threshold)}
		err = tx.QueryRow(ctx, `INSERT INTO alarms(device_no,pond_id,metric,current_value,threshold,level,message,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(device_no,pond_id,metric) WHERE confirmed_at IS NULL DO NOTHING RETURNING id,created_at`, al.DeviceNo, pond, al.Metric, v, threshold, r.level, al.Message, e.Ts).Scan(&al.ID, &al.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO notification_outbox(alarm_id) VALUES($1)", al.ID); err != nil {
			return nil, err
		}
		out = append(out, al)
	}
	return out, nil
}
func validateRule(r domain.AlarmRule) error {
	if !domain.ValidMetric(r.Metric) || (r.Level != domain.AlarmWarning && r.Level != domain.AlarmCritical) || (r.Min == nil && r.Max == nil) {
		return domain.ErrInvalidRule
	}
	for _, p := range []*float64{r.Min, r.Max} {
		if p != nil && (math.IsNaN(*p) || math.IsInf(*p, 0)) {
			return domain.ErrInvalidRule
		}
	}
	if r.Min != nil && r.Max != nil && *r.Min >= *r.Max {
		return domain.ErrInvalidRule
	}
	return nil
}
