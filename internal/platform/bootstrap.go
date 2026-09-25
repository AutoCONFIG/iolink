package platform

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BootstrapAdmin is deliberately a local CLI operation. Credentials arrive over
// stdin, not command arguments/environment variables, and are never logged.
func BootstrapAdmin(ctx context.Context, pool *pgxpool.Pool, username, password string, reset bool) error {
	if len(username) < 1 || len(username) > 64 || strings.TrimSpace(username) != username {
		return errors.New("username must be 1..64 characters without surrounding whitespace")
	}
	if len(password) < 12 || len(password) > 256 || strings.TrimSpace(password) != password || password == "admin123" {
		return errors.New("bootstrap password must be 12..256 bytes without surrounding whitespace")
	}
	hash := HashPassword(password)
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1232047152)"); err != nil {
		return err
	}
	if reset {
		ct, e := tx.Exec(ctx, `UPDATE users SET password_hash=$2,must_change_password=false,token_version=token_version+1 WHERE username=$1 AND authority='ADMIN'`, username, hash)
		if e != nil {
			return e
		}
		if ct.RowsAffected() != 1 {
			return errors.New("administrator not found")
		}
	} else {
		var n int
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM users WHERE authority='ADMIN'").Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return errors.New("administrator already exists; use admin reset-password for local recovery")
		}
		if _, err = tx.Exec(ctx, `INSERT INTO users(open_id,username,password_hash,authority,nickname) VALUES('internal-admin',$1,$2,'ADMIN','Administrator')`, username, hash); err != nil {
			return err
		}
	}
	action := "admin.bootstrap"
	if reset {
		action = "admin.local-password-reset"
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(action,resource_type,resource_id) VALUES($1,'user',$2)`, action, username); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func CheckAdminReady(ctx context.Context, pool *pgxpool.Pool) error {
	var total, pending int
	if err := pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE must_change_password OR password_hash IS NULL OR password_hash='1cd663ce3300b9f52a357c4ae4e114064b0fa066071728aca1d7a98f5916f2e0') FROM users WHERE authority='ADMIN'`).Scan(&total, &pending); err != nil {
		return err
	}
	if total == 0 {
		return errors.New("no administrator configured; run iolinkd admin init with a password on stdin")
	}
	if pending > 0 {
		return errors.New("initial administrator password requires local reset before serving")
	}
	return nil
}
