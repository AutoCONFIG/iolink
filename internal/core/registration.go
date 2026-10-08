package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
)

func (s *Service) RegisterUser(ctx context.Context, input domain.Registration) error {
	if input.Username == "" || len(input.Username) > 64 || strings.TrimSpace(input.Username) != input.Username || len(input.Password) < 12 || len(input.Password) > 256 || strings.TrimSpace(input.Password) != input.Password {
		return domain.ErrBootstrapInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var ready bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE authority='ADMIN')`).Scan(&ready); err != nil {
		return err
	}
	if !ready {
		return domain.ErrForbidden
	}
	entropy := make([]byte, 16)
	if _, err := rand.Read(entropy); err != nil {
		return err
	}
	var userID int64
	err = tx.QueryRow(ctx, `INSERT INTO users(open_id,username,password_hash,authority,nickname) VALUES($1,$2,$3,'USER',$2) RETURNING id`, "web-"+hex.EncodeToString(entropy), input.Username, platform.HashPassword(input.Password)).Scan(&userID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrConflict
	}
	if err != nil {
		return err
	}
	ct, err := tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id) SELECT id,$1,'user.registered','user',$2 FROM tenants WHERE name='__iolink_system__'`, userID, fmt.Sprint(userID))
	if err != nil {
		return err
	}
	if ct.RowsAffected() != 1 {
		return domain.ErrForbidden
	}
	return tx.Commit(ctx)
}
