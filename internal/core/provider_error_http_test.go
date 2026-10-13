package core_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/appapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
)

func TestBaselineHTTPProviderErrorsWithRealPostgres(t *testing.T) {
	// Given: both authenticated APIs share real storage and a safe dump sink.
	svc, pool := setup(t)
	execute(t, pool, `UPDATE users SET username='provider-owner',password_hash=$1,authority='USER' WHERE id=1`, platform.HashPassword("Provider-test-only-1234"))
	dir := t.TempDir()
	logger, sink, err := observability.New(observability.Config{Dir: dir, Version: "test", Level: "error", MaxMB: 1, Backups: 1}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sink.Close(); err != nil {
			t.Error(err)
		}
	})
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	admin := adminapi.New(adminapi.Config{SecretKey: "provider-http-test", JWT: time.Hour}, adminapi.Deps{Store: svc, Telemetry: svc.Telemetry(), Policy: policy, Logger: logger})
	app := appapi.New(appapi.Config{SecretKey: "provider-http-test", JWT: time.Hour}, appapi.Deps{Users: svc, Ponds: svc.Ponds(), Devices: svc.Devices(), Telemetry: svc.Telemetry(), Alarms: svc.Alarms()}, logger)
	app.SetWechatExchanger(func(string) (string, error) { return "test-user", nil })
	mux := http.NewServeMux()
	mux.Handle("/user/", admin.UserRoutes())
	mux.Handle("/api/", app.Routes())
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	var user, mini struct {
		Token string `json:"token"`
	}
	m6bHTTPRequest(t, server.URL, "", "POST", "/user/v1/login", `{"username":"provider-owner","password":"Provider-test-only-1234"}`, 200, &user)
	m6bHTTPRequest(t, server.URL, "", "POST", "/api/v1/auth/login", `{"code":"local-provider-fixture"}`, 200, &mini)
	ids := make(map[string]bool)
	for _, tc := range []struct {
		table string
		paths []string
	}{
		{"farms", []string{"/user/v1/farms", "/user/v1/stats", "/api/v1/ponds", "/api/v1/stats/summary"}},
		{"alarms", []string{"/user/v1/alarms", "/api/v1/alarms"}},
		{"alarm_rules", []string{"/user/v1/alarm-rules"}},
	} {
		t.Run(tc.table, func(t *testing.T) {
			execute(t, pool, "ALTER TABLE "+tc.table+" RENAME TO provider_test_unavailable")
			defer execute(t, pool, "ALTER TABLE provider_test_unavailable RENAME TO "+tc.table)
			// When: a real relation failure reaches each existing HTTP handler.
			for _, path := range tc.paths {
				req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
				if err != nil {
					t.Fatal(err)
				}
				token := user.Token
				if strings.HasPrefix(path, "/api/") {
					token = mini.Token
				}
				req.Header.Set("Authorization", "Bearer "+token)
				resp, err := server.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				raw, err := io.ReadAll(resp.Body)
				closeErr := resp.Body.Close()
				if err != nil || closeErr != nil {
					t.Fatalf("read/close: %v %v", err, closeErr)
				}
				// Then: only the contract error is public; a server ID correlates it.
				id := resp.Header.Get("X-Request-ID")
				if resp.StatusCode != 500 || string(raw) != `{"error":"internal_error"}` || id == "" {
					t.Fatalf("%s status=%d body=%s request_id=%s", path, resp.StatusCode, raw, id)
				}
				ids[id] = false
			}
		})
	}
	server.Close()
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "test", "iolinkd.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"provider_test_unavailable", "Provider-test-only-1234", user.Token, mini.Token} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatal("unsafe provider diagnostic")
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var record struct {
			Stage, Level, Err string
			RequestID         string `json:"request_id"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if record.Stage == "http_provider" && record.Level == "ERROR" && record.Err == "postgres_42P01" {
			if _, ok := ids[record.RequestID]; ok {
				ids[record.RequestID] = true
			}
		}
	}
	for id, found := range ids {
		if !found {
			t.Fatalf("missing safe SQLSTATE diagnostic for %s", id)
		}
	}
}
