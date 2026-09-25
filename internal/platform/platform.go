package platform

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config is the process-wide configuration, loaded from env/yaml in main.
type Config struct {
	HTTPAddr string // appapi listen addr, e.g. ":8080"
	MQTTAddr string // access MQTT listener, e.g. ":1883"

	PgDSN        string // e.g. postgres://iolink:iolink@localhost:5432/iolink
	PgMaxConns   int32
	QueryTimeout time.Duration

	SecretKey      string // JWT signing key (production: env only)
	ReportInterval time.Duration
	OfflineGrace   int

	WXAppID      string // WeChat mini program; empty = notifier off
	WXSecret     string
	WXTemplateID string // subscribe-message template for alarms
}

// DB wraps the pgx pool shared by core (and appapi via core repositories).
func DB(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.PgDSN)
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	pcfg.MaxConns = cfg.PgMaxConns
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, errors.New("cannot create database pool")
	}
	pingCtx, cancel := context.WithTimeout(ctx, cfg.QueryTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, errors.New("database unavailable; check connection configuration")
	}
	return pool, nil
}

func Logger() *slog.Logger {
	return slog.Default()
}
