// Package iolink — main service assembly.
//
// This is the ONLY place where all modules meet: core (here) is handed to
// access as event.Handler+Authenticator and to appapi as repositories.
package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/access"
	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/appapi"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"git.hyhy.fun/rsplab/iolink/web"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := platform.Config{
		HTTPAddr:     envOr("IOLINK_HTTP_ADDR", ":8080"),
		MQTTAddr:     envOr("IOLINK_MQTT_ADDR", ":1883"),
		PgDSN:        envOr("IOLINK_PG_DSN", "postgres://iolink:iolink@localhost:5432/iolink"),
		PgMaxConns:   20,
		QueryTimeout: 5 * time.Second,
		SecretKey:    envOr("IOLINK_SECRET_KEY", "dev-only-change-me"),
		WXAppID:      os.Getenv("IOLINK_WX_APPID"),
		WXSecret:     os.Getenv("IOLINK_WX_SECRET"),
		WXTemplateID: os.Getenv("IOLINK_WX_TEMPLATE_ID"),
	}
	log := slog.Default()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := platform.DB(ctx, cfg)
	if err != nil {
		log.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	svc, err := core.New(ctx, pool, log)
	if err != nil {
		log.Error("core init", "err", err)
		os.Exit(1)
	}

	// --- access: embedded MQTT broker, events -> core ---
	acc := access.New(access.Config{
		MQTTAddr:           cfg.MQTTAddr,
		ReportInterval:     1 * time.Minute,
		OfflineGraceFactor: 3,
	}, svc, svc, log) // svc is both event.Handler and Authenticator
	go func() {
		if err := acc.Serve(); err != nil {
			log.Error("mqtt broker stopped", "err", err)
			stop()
		}
	}()
	go acc.Run(ctx)

	// --- adminapi: /admin/v1 for the management console ---
	admin := adminapi.New(adminapi.Config{
		SecretKey: cfg.SecretKey,
		JWT:       12 * time.Hour,
	}, adminapi.Deps{Store: svc}) // svc implements AdminStore

	// --- appapi: /api/v1 for the mini program, backed by core repos ---
	api := appapi.New(appapi.Config{
		Addr:      cfg.HTTPAddr,
		SecretKey: cfg.SecretKey,
		JWT:       7 * 24 * time.Hour,
		Wechat: appapi.WechatConfig{
			AppID:  os.Getenv("IOLINK_WX_APPID"),
			Secret: os.Getenv("IOLINK_WX_SECRET"),
		},
	}, appapi.Deps{
		Ponds:     svc.Ponds(),
		Devices:   svc.Devices(),
		Telemetry: svc.Telemetry(),
		Alarms:    svc.Alarms(),
		Users:     svc, // svc implements UserStore
	}, log)

	// single port: root mux mounts admin API, app API and health endpoint
	// notifier: WeChat subscribe message when configured
	if cfg.WXAppID != "" && cfg.WXSecret != "" && cfg.WXTemplateID != "" {
		svc.SetNotifier(core.NewWeChatNotifier(cfg.WXAppID, cfg.WXSecret, cfg.WXTemplateID,
			envOr("IOLINK_WX_PAGE", "pages/alarms/index"), log))
		log.Info("wechat notifier enabled")
	}

	root := http.NewServeMux()
	root.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, "db down", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})
	root.Handle("/metrics", promhttp.Handler())
	root.Handle("/admin/v1/", admin.Routes())
	adminFS, err := web.Admin()
	if err != nil {
		log.Error("admin frontend embed", "err", err)
		os.Exit(1)
	}
	root.Handle("/api/v1/", api.Routes())
	root.Handle("/", spaHandler(adminFS))

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: root, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("iolinkd started", "http", cfg.HTTPAddr, "mqtt", cfg.MQTTAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http stopped", "err", err)
			stop()
		}
	}()
	<-ctx.Done()
	log.Info("shutting down")
	if err := acc.Close(); err != nil {
		log.Warn("mqtt close", "err", err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// spaHandler serves the embedded admin SPA, falling back to index.html for
// client-side routes.
func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(fsys, path); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
