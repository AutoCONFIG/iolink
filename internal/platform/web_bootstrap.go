package platform

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WebBootstrap struct{ Pool *pgxpool.Pool }

func (s WebBootstrap) Required(ctx context.Context) (bool, error) {
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE authority='ADMIN')`).Scan(&exists); err != nil {
		return false, fmt.Errorf("read bootstrap state: %w", err)
	}
	return !exists, nil
}

func (s WebBootstrap) Initialize(ctx context.Context, username, password string) error {
	return BootstrapAdmin(ctx, s.Pool, username, password, false)
}
