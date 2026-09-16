package core

import (
	"context"
	"fmt"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

// FindByOpenID implements the auth-facing user lookup. Satisfies
// appapi.UserStore (together with EnsureUser below) via Go structural
// typing — core never imports the appapi module.
func (s *Service) FindByOpenID(ctx context.Context, openID string) (*domain.User, error) {
	const q = `SELECT id, open_id, coalesce(nickname,'') FROM users WHERE open_id=$1`
	u := &domain.User{}
	err := s.pool.QueryRow(ctx, q, openID).Scan(&u.ID, &u.OpenID, &u.Nickname)
	if err != nil {
		return nil, fmt.Errorf("user by openid: %w", err)
	}
	return u, nil
}

// EnsureUser creates the user on first WeChat login (idempotent).
func (s *Service) EnsureUser(ctx context.Context, openID string) (*domain.User, error) {
	const q = `INSERT INTO users (open_id) VALUES ($1)
		ON CONFLICT (open_id) DO UPDATE SET open_id = EXCLUDED.open_id
		RETURNING id, open_id, coalesce(nickname,'')`
	u := &domain.User{}
	err := s.pool.QueryRow(ctx, q, openID).Scan(&u.ID, &u.OpenID, &u.Nickname)
	if err != nil {
		return nil, fmt.Errorf("ensure user: %w", err)
	}
	return u, nil
}
