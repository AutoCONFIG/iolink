// Package iolink — main service assembly.
//
// This is the ONLY place where all modules meet: core (here) is handed to
// access as event.Handler+Authenticator and to appapi as repositories.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"git.hyhy.fun/rsplab/iolink/web"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		slog.Error("iolinkd stopped", "error", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "help") {
		fmt.Fprintln(stdout, "iolinkd [serve | migrate up | migrate status | migrate adopt-legacy | admin init USER | admin reset-password USER]\nAdmin commands read a new password from redirected stdin, never argv. Back up before adopt-legacy or upgrades.")
		return nil
	}
	serving := len(args) == 0 || (len(args) == 1 && args[0] == "serve")
	valid := serving || (len(args) == 2 && args[0] == "migrate" && (args[1] == "up" || args[1] == "status" || args[1] == "adopt-legacy")) || (len(args) == 3 && args[0] == "admin" && (args[1] == "init" || args[1] == "reset-password"))
	if !valid {
		return errors.New("unknown command; use iolinkd --help")
	}
	cfg, err := platform.LoadConfig(os.Getenv, serving)
	if err != nil {
		return err
	}
	log := slog.Default()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pool, err := platform.DB(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	if !serving {
		cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if args[0] == "migrate" {
			switch args[1] {
			case "up":
				return migrate.Up(cmdCtx, pool)
			case "adopt-legacy":
				return migrate.AdoptLegacy(cmdCtx, pool)
			case "status":
				st, e := migrate.Status(cmdCtx, pool)
				if e != nil {
					return e
				}
				return json.NewEncoder(stdout).Encode(st)
			}
		}
		if err := migrate.CheckLatest(cmdCtx, pool); err != nil {
			return err
		}
		if f, ok := stdin.(*os.File); ok {
			if st, e := f.Stat(); e == nil && st.Mode()&os.ModeCharDevice != 0 {
				return errors.New("redirect password from a protected file on stdin; interactive echo is disabled")
			}
		}
		raw, err := io.ReadAll(io.LimitReader(stdin, 259))
		if err != nil {
			return errors.New("cannot read password from stdin")
		}
		if len(raw) > 258 {
			return errors.New("password input exceeds 256 bytes plus line ending")
		}
		password := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
		return platform.BootstrapAdmin(cmdCtx, pool, args[2], password, args[1] == "reset-password")
	}
	if err := migrate.CheckLatest(ctx, pool); err != nil {
		return err
	}
	if err := platform.CheckAdminReady(ctx, pool); err != nil {
		return err
	}

	svc, err := core.New(ctx, pool, log, cfg.ReportInterval)
	if err != nil {
		return err
	}

	// notifier: WeChat subscribe message when configured
	if cfg.WXAppID != "" && cfg.WXSecret != "" && cfg.WXTemplateID != "" {
		svc.SetNotifier(core.NewWeChatNotifier(cfg.WXAppID, cfg.WXSecret, cfg.WXTemplateID,
			envOr("IOLINK_WX_PAGE", "pages/alarms/index"), log))
		log.Info("wechat notifier enabled")
	}

	// --- access: embedded MQTT broker, events -> core ---
	acc := access.New(access.Config{
		MQTTAddr:           cfg.MQTTAddr,
		ReportInterval:     cfg.ReportInterval,
		OfflineGraceFactor: cfg.OfflineGrace,
	}, svc, svc, log) // svc is both event.Handler and Authenticator
	serviceErrors := make(chan error, 2)
	go func() {
		if err := acc.Serve(); err != nil {
			serviceErrors <- fmt.Errorf("mqtt listener: %w", err)
			stop()
		}
	}()
	go acc.Run(ctx)

	// --- adminapi: /admin/v1 for the management console ---
	admin := adminapi.New(adminapi.Config{
		SecretKey: cfg.SecretKey,
		JWT:       12 * time.Hour,
	}, adminapi.Deps{Store: svc, Telemetry: svc.Telemetry()}) // svc implements AdminStore

	// --- appapi: /api/v1 for the mini program, backed by core repos ---
	api := appapi.New(appapi.Config{
		Addr:      cfg.HTTPAddr,
		SecretKey: cfg.SecretKey,
		JWT:       7 * 24 * time.Hour,
		Wechat: appapi.WechatConfig{
			AppID:  cfg.WXAppID,
			Secret: cfg.WXSecret,
		},
	}, appapi.Deps{
		Ponds:     svc.Ponds(),
		Devices:   svc.Devices(),
		Telemetry: svc.Telemetry(),
		Alarms:    svc.Alarms(),
		Users:     svc, // svc implements UserStore
	}, log)

	// single port: root mux mounts admin API, app API and health endpoint

	root := http.NewServeMux()
	root.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), cfg.QueryTimeout)
		defer cancel()
		if err := pool.Ping(pingCtx); err != nil {
			http.Error(w, "db down", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})
	root.Handle("/metrics", promhttp.Handler())
	root.Handle("/admin/v1/", admin.Routes())
	adminFS, err := web.Admin()
	if err != nil {
		return err
	}
	root.Handle("/api/v1/", api.Routes())
	root.Handle("/", spaHandler(adminFS))

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: root, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Info("iolinkd started", "http", cfg.HTTPAddr, "mqtt", cfg.MQTTAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serviceErrors <- err
			stop()
		}
	}()
	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close()
		log.Warn("http shutdown deadline", "err", err)
	}
	if err := acc.Close(); err != nil {
		log.Warn("mqtt close", "err", err)
	}
	select {
	case err := <-serviceErrors:
		return err
	default:
		return nil
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
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/admin/v1") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":"not found"}`)
			return
		}
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
