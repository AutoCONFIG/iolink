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
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/access"
	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/appapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/notifications"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
	"git.hyhy.fun/rsplab/iolink/internal/operations"
	"git.hyhy.fun/rsplab/iolink/internal/persistence"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"git.hyhy.fun/rsplab/iolink/internal/web"
	"git.hyhy.fun/rsplab/iolink/internal/wechat"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "iolinkd stopped; check configuration and diagnostics")
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) (runErr error) {
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
	logConfig, err := observability.FromEnv(os.Getenv)
	if err != nil {
		return err
	}
	log, err := observability.Console(logConfig, os.Stderr)
	if err != nil {
		return err
	}
	var logSink *observability.Sink
	if serving {
		log, logSink, err = observability.New(logConfig, os.Stderr)
		if err != nil {
			return err
		}
	}
	defer func() {
		if runErr != nil {
			log.Error("iolinkd operation failed", "stage", "process", "err", runErr)
		}
		if logSink != nil {
			if err := logSink.Close(); err != nil {
				fmt.Fprintln(os.Stderr, "diagnostic log close failed")
			}
		}
	}()
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

	policy, err := authorization.New()
	if err != nil {
		return fmt.Errorf("tenant policy: %w", err)
	}
	svc, err := core.NewWithPolicy(ctx, pool, log, policy, cfg.ReportInterval)
	if err != nil {
		return err
	}

	notificationSender := wechat.NewSender(wechat.Config{
		AppID: cfg.WXAppID, Secret: cfg.WXSecret, TemplateID: cfg.WXTemplateID,
		Page:         envOr("IOLINK_WX_PAGE", "pages/alarms/index"),
		MessageField: envOr("IOLINK_WX_MESSAGE_FIELD", "thing1"),
		ValueField:   envOr("IOLINK_WX_VALUE_FIELD", "number2"),
		TimeField:    envOr("IOLINK_WX_TIME_FIELD", "time3"),
	})
	notificationWorker, err := notifications.NewWorker(notifications.Config{
		Store: persistence.NewNotificationStore(pool), Sender: notificationSender, Logger: log,
		OnSent: core.MetricNotificationsSent.Inc, OnFailure: core.MetricNotificationsFailed.Inc,
	})
	if err != nil {
		return err
	}
	notificationDone := make(chan struct{})
	go func() {
		defer close(notificationDone)
		notificationWorker.Run(ctx)
	}()

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
	}, adminapi.Deps{Store: svc, Telemetry: svc.Telemetry(), Catalog: svc.Products(), Policy: policy, Logger: log})

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
	health := operations.NewHealth(operations.Checks{
		Ping: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, cfg.QueryTimeout)
			defer cancel()
			return pool.Ping(pingCtx)
		},
		Ready: func(ctx context.Context) error {
			if !logSink.Healthy() {
				return errors.New("diagnostic log unavailable")
			}
			checkCtx, cancel := context.WithTimeout(ctx, cfg.QueryTimeout)
			defer cancel()
			if err := pool.Ping(checkCtx); err != nil {
				return err
			}
			return migrate.CheckLatest(checkCtx, pool)
		},
	})
	root.Handle("/healthz", health.Healthz())
	root.Handle("/readyz", health.Readyz())
	root.Handle("/metrics", promhttp.Handler())
	root.Handle("/admin/v1/", admin.Routes())
	adminFS, err := web.Admin()
	if err != nil {
		return err
	}
	mountApplicationAPI(root, api.Routes())
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
	health.SetReady(false)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := acc.CloseContext(shutdownCtx); err != nil {
		log.Warn("mqtt close", "err", err)
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close()
		log.Warn("http shutdown deadline", "err", err)
	}
	select {
	case <-notificationDone:
	case <-shutdownCtx.Done():
		log.Warn("notification shutdown deadline")
	}
	select {
	case err := <-serviceErrors:
		return err
	default:
		return nil
	}
}

func mountApplicationAPI(root *http.ServeMux, handler http.Handler) {
	root.Handle("/api/v1/", handler)
	root.Handle("/api/v2/", handler)
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
