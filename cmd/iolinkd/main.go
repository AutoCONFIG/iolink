// Package iolink — main service assembly.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
)

// Access and AppApi are defined in their own git repositories and wired here
// as interfaces (see contracts/). Phase-1 skeleton: log the intended wiring
// and run the HTTP health endpoint so deployment is testable end to end.
//
// import access "iolink/access"    // submodule, provides access.New(cfg, handler)
// import appapi  "iolink/appapi"   // submodule, provides appapi.New(core repos)

func main() {
	cfg := platform.Config{
		HTTPAddr:     envOr("IOLINK_HTTP_ADDR", ":8080"),
		MQTTAddr:     envOr("IOLINK_MQTT_ADDR", ":1883"),
		PgDSN:        envOr("IOLINK_PG_DSN", "postgres://iolink:iolink@localhost:5432/iolink"),
		PgMaxConns:   20,
		QueryTimeout: 5 * time.Second,
		SecretKey:    envOr("IOLINK_SECRET_KEY", "dev-only-change-me"),
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := platform.DB(ctx, cfg)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	svc, err := core.New(ctx, pool, platform.Logger())
	if err != nil {
		log.Fatalf("core init: %v", err)
	}
	_ = svc // consumed by access (event.Handler) and appapi (repos) once submodules land

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, "db down", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("iolinkd listening on %s (mqtt %s planned)", cfg.HTTPAddr, cfg.MQTTAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	<-ctx.Done()
	shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shut)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
