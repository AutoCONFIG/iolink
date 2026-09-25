// Package testdb provides isolated databases for integration tests only.
package testdb

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"os"
	"testing"
	"time"
)

func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("IOLINK_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("integration: set IOLINK_TEST_PG_DSN to a disposable Timescale instance")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("invalid test DSN")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("invalid test database config")
	}
	name := fmt.Sprintf("iolink_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	var pool *pgxpool.Pool
	t.Cleanup(func() {
		if pool != nil {
			pool.Close()
		}
		_, err := admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
		admin.Close()
		if err != nil {
			t.Errorf("test database cleanup: %v", err)
		}
	})
	u.Path = "/" + name
	pool, err = pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal("test pool creation failed")
	}
	return pool
}
