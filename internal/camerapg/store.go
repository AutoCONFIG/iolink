// Package camerapg implements transactional camera configuration storage.
package camerapg

import (
	"context"
	"errors"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

var _ camera.CameraStore = (*Store)(nil)

type transaction struct {
	tx                pgx.Tx
	tenantID, actorID int64
	role, authority   string
}

func (s *Store) Run(ctx context.Context, operation func(camera.CameraTransaction) error) error {
	if s == nil || s.pool == nil {
		return camera.ErrUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return camera.ErrUnavailable
	}
	defer tx.Rollback(ctx)
	if err = operation(&transaction{tx: tx}); err != nil {
		return safeError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return camera.ErrUnavailable
	}
	return nil
}

func safeError(err error) error {
	for _, safe := range []error{domain.ErrForbidden, domain.ErrNotFound, camera.ErrInvalid, camera.ErrUnavailable, camera.ErrInternal, context.Canceled, context.DeadlineExceeded} {
		if errors.Is(err, safe) {
			return safe
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return camera.ErrInternal
}

func (t *transaction) LicenseSnapshot(ctx context.Context) (camera.LicenseSnapshot, error) {
	var state camera.LicenseSnapshot
	err := t.tx.QueryRow(ctx, `SELECT deployment_id FROM deployment_config WHERE singleton FOR SHARE`).Scan(&state.DeploymentID)
	if err == nil {
		err = t.tx.QueryRow(ctx, `SELECT payload,signature,coalesce(payload_sha256,'') FROM license_state WHERE singleton FOR SHARE`).Scan(&state.Payload, &state.Signature, &state.SHA256)
	}
	if err == nil {
		err = t.tx.QueryRow(ctx, `SELECT coalesce(max_seen_at,'epoch'::timestamptz),clock_error,clock_timestamp() FROM license_clock WHERE singleton FOR SHARE`).Scan(&state.MaxSeenAt, &state.ClockError, &state.Now)
	}
	if err != nil {
		return camera.LicenseSnapshot{}, camera.ErrUnavailable
	}
	return state, nil
}
