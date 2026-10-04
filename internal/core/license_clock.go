package core

import (
	"context"
	"errors"
	"fmt"
	"git.hyhy.fun/rsplab/iolink/internal/license"
	"github.com/jackc/pgx/v5"
	"time"
)

func (s *Service) observeLicenseClock(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("license clock transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	observationErr := observeLicenseClock(ctx, tx)
	if observationErr != nil && !errors.Is(observationErr, license.ErrClockError) {
		return observationErr
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("license clock commit: %w", err)
	}
	return observationErr
}

// ObserveLicenseClock records the process clock high-water mark in its own
// short transaction so a later business rollback cannot erase it.
func (s *Service) ObserveLicenseClock(ctx context.Context) error { return s.observeLicenseClock(ctx) }

func (s *Service) ReconcileLicenseClock(ctx context.Context, actorID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("license clock reconcile transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	var maxSeen *time.Time
	var clockError bool
	if err := tx.QueryRow(ctx, `SELECT max_seen_at,clock_error FROM license_clock WHERE singleton=TRUE FOR UPDATE`).Scan(&maxSeen, &clockError); err != nil {
		return fmt.Errorf("license clock reconcile state: %w", err)
	}
	now := time.Now().UTC()
	if maxSeen != nil && now.Before(*maxSeen) {
		return license.ErrClockError
	}
	if !clockError {
		return nil
	}
	if _, err := tx.Exec(ctx, `UPDATE license_clock SET clock_error=FALSE,updated_at=$1 WHERE singleton=TRUE`, now); err != nil {
		return fmt.Errorf("license clock reconcile update: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id,metadata) VALUES((SELECT id FROM tenants WHERE name='__iolink_system__'),$1,'license.clock_reconciled','license','clock','{}'::jsonb)`, actorID); err != nil {
		return fmt.Errorf("license clock reconcile audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("license clock reconcile commit: %w", err)
	}
	return nil
}

func observeLicenseClock(ctx context.Context, tx pgx.Tx) error {
	var maxSeen *time.Time
	var clockError bool
	if err := tx.QueryRow(ctx, `SELECT max_seen_at,clock_error FROM license_clock WHERE singleton=TRUE FOR UPDATE`).Scan(&maxSeen, &clockError); err != nil {
		return fmt.Errorf("license clock: %w", err)
	}
	now := time.Now().UTC()
	if clockError || (maxSeen != nil && now.Before(maxSeen.Add(-5*time.Minute))) {
		if !clockError {
			if _, err := tx.Exec(ctx, `UPDATE license_clock SET clock_error=TRUE,updated_at=$1 WHERE singleton=TRUE`, now); err != nil {
				return fmt.Errorf("license clock latch: %w", err)
			}
		}
		return license.ErrClockError
	}
	if maxSeen == nil || now.After(*maxSeen) {
		if _, err := tx.Exec(ctx, `UPDATE license_clock SET max_seen_at=$1,updated_at=$1 WHERE singleton=TRUE`, now); err != nil {
			return fmt.Errorf("license clock observe: %w", err)
		}
	}
	return nil
}
