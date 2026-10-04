package core

import (
	"context"

	"git.hyhy.fun/rsplab/iolink/internal/persistence"
	"github.com/jackc/pgx/v5"
)

func (s *Service) authorizeTenantWrite(ctx context.Context, tx pgx.Tx, resource, action string) error {
	return persistence.AuthorizeTenantWrite(ctx, tx, s.policy, resource, action)
}

func (s *Service) beginTenantWrite(ctx context.Context, resource string) (pgx.Tx, error) {
	tx, err := s.beginWriteTransaction(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeTenantWrite(ctx, tx, resource, "write"); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return nil, rollbackErr
		}
		return nil, err
	}
	return tx, nil
}

func (s *Service) beginWriteTransaction(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
