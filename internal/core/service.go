// Package core implements the business heart: telemetry ingestion pipeline,
// alarm engine, and the repositories promised by git.hyhy.fun/rsplab/iolink/internal/domain.
//
// core consumes events from access (git.hyhy.fun/rsplab/iolink/internal/event.Handler) and
// serves appapi through domain repository interfaces. It never imports
// access or appapi.
package core

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/event"
)

// Service is the assembled core. cmd/iolinkd constructs it and hands the
// same instance to access (as event.Handler) and appapi (as repositories).
type Service struct {
	pool            *pgxpool.Pool
	log             *slog.Logger
	al              *alarmEngine
	defaultInterval time.Duration
}

func New(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger, interval ...time.Duration) (*Service, error) {
	d := time.Minute
	if len(interval) > 0 && interval[0] > 0 {
		d = interval[0]
	}
	s := &Service{pool: pool, log: log, al: &alarmEngine{log: log}, defaultInterval: d}
	// MQTT sessions do not survive a process restart. Retain last_seen timestamps.
	if _, err := pool.Exec(ctx, "UPDATE devices SET status='offline' WHERE status='online'"); err != nil {
		return nil, err
	}
	MetricDevicesOnline.Set(0)
	return s, nil
}

// HandleEvent implements event.Handler: validate, persist, evaluate alarms.
func (s *Service) HandleEvent(e event.Event) error {
	switch e.Kind {
	case event.KindProperties:
		return s.storeReading(e)
	case event.KindStatusChange:
		return s.storeStatus(e)
	default:
		s.log.Warn("unknown event kind", "kind", e.Kind)
		return nil
	}
}

// --- Repository accessors for appapi wiring ---

func (s *Service) Ponds() domain.PondRepo           { return &pondRepo{s.pool} }
func (s *Service) Devices() domain.DeviceRepo       { return &deviceRepo{s.pool} }
func (s *Service) Telemetry() domain.TelemetryRepo  { return &telemetryRepo{s.pool, s.defaultInterval} }
func (s *Service) Alarms() domain.AlarmRepo         { return &alarmRepo{s.pool} }
func (s *Service) AlarmRules() domain.AlarmRuleRepo { return &alarmRuleRepo{s.pool} }
