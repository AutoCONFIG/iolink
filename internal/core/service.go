// Package core implements the business heart: telemetry ingestion pipeline,
// alarm engine, and the repositories promised by git.hyhy.fun/rsplab/iolink/contracts/domain.
//
// core consumes events from access (git.hyhy.fun/rsplab/iolink/contracts/event.Handler) and
// serves appapi through domain repository interfaces. It never imports
// access or appapi.
package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"git.hyhy.fun/rsplab/iolink/contracts/domain"
	"git.hyhy.fun/rsplab/iolink/contracts/event"
)

// Service is the assembled core. cmd/iolinkd constructs it and hands the
// same instance to access (as event.Handler) and appapi (as repositories).
type Service struct {
	pool *pgxpool.Pool
	log  *slog.Logger
	al   *alarmEngine
}

func New(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) (*Service, error) {
	s := &Service{pool: pool, log: log, al: &alarmEngine{pool: pool, log: log}}
	return s, nil
}

// HandleEvent implements event.Handler: validate, persist, evaluate alarms.
func (s *Service) HandleEvent(e event.Event) error {
	switch e.Kind {
	case event.KindProperties:
		if err := s.storeReading(e); err != nil {
			return fmt.Errorf("store reading: %w", err)
		}
		return s.al.evaluate(e)
	case event.KindStatusChange:
		return s.storeStatus(e)
	default:
		s.log.Warn("unknown event kind", "kind", e.Kind)
		return nil
	}
}

// --- Repository accessors for appapi wiring ---

func (s *Service) Ponds() domain.PondRepo          { return &pondRepo{s.pool} }
func (s *Service) Devices() domain.DeviceRepo      { return &deviceRepo{s.pool} }
func (s *Service) Telemetry() domain.TelemetryRepo { return &telemetryRepo{s.pool} }
func (s *Service) Alarms() domain.AlarmRepo        { return &alarmRepo{s.pool} }
func (s *Service) AlarmRules() domain.AlarmRuleRepo { return &alarmRuleRepo{s.pool} }
