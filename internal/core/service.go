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
	"git.hyhy.fun/rsplab/iolink/internal/ingestion"
)

// Service is the assembled core. cmd/iolinkd constructs it and hands the
// same instance to access (as event.Handler) and appapi (as repositories).
type Service struct {
	pool            *pgxpool.Pool
	log             *slog.Logger
	al              *alarmEngine
	policy          domain.PermissionPolicy
	defaultInterval time.Duration
}

func New(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger, interval ...time.Duration) (*Service, error) {
	return NewWithPolicy(ctx, pool, log, nil, interval...)
}

func NewWithPolicy(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger, policy domain.PermissionPolicy, interval ...time.Duration) (*Service, error) {
	d := time.Minute
	if len(interval) > 0 && interval[0] > 0 {
		d = interval[0]
	}
	s := &Service{pool: pool, log: log, al: &alarmEngine{log: log}, policy: policy, defaultInterval: d}
	// MQTT sessions do not survive a process restart. Retain last_seen timestamps.
	if _, err := pool.Exec(ctx, "UPDATE devices SET status='offline' WHERE status='online'"); err != nil {
		return nil, err
	}
	MetricDevicesOnline.Set(0)
	return s, nil
}

// HandleEvent implements event.Handler: validate, persist, evaluate alarms.
func (s *Service) HandleEvent(e event.Event) (err error) {
	started := time.Now()
	defer func() {
		if err != nil {
			s.log.Debug("device event rejected", "device", e.DeviceNo, "stage", "ingestion", "err", err, "duration_ms", time.Since(started).Milliseconds())
		} else {
			s.log.Debug("device event persisted", "device", e.DeviceNo, "kind", e.Kind, "fields", len(e.Properties)+len(e.GenericProperties), "duration_ms", time.Since(started).Milliseconds())
		}
	}()
	if err := ingestion.Validate(e); err != nil {
		return err
	}
	switch e.Kind {
	case event.KindProperties:
		if e.GenericProperties != nil {
			return s.storeGenericReading(e)
		}
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
