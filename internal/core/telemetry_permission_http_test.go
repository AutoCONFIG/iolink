package core_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/appapi"
)

func TestAppTelemetryWriteRoleBoundary(t *testing.T) {
	for _, tc := range []struct {
		role string
		want int
	}{
		{"owner", 202}, {"admin", 202}, {"viewer", 403}, {"member", 403}, {"support", 403},
	} {
		t.Run(tc.role, func(t *testing.T) {
			// Given: real Timescale storage and the production app login and telemetry routes.
			f := newTelemetryPermissionFixture(t)
			f.role(t, tc.role, true)
			previous := appapi.WechatExchanger
			appapi.WechatExchanger = func(string) (string, error) { return "v2-actor", nil }
			t.Cleanup(func() { appapi.WechatExchanger = previous })
			server := httptest.NewServer(appapi.New(appapi.Config{SecretKey: strings.Repeat("v", 32), JWT: time.Hour}, appapi.Deps{Users: f.svc, Ponds: f.svc.Ponds(), Devices: f.svc.Devices(), Telemetry: f.svc.Telemetry(), Alarms: f.svc.Alarms()}, slog.New(slog.NewTextHandler(io.Discard, nil))).Routes())
			defer server.Close()
			var login struct{ Token string }
			m6bHTTPRequest(t, server.URL, "", http.MethodPost, "/api/v1/auth/login", `{"code":"fixture"}`, 200, &login)
			m6bHTTPRequest(t, server.URL, login.Token, http.MethodPost, "/api/v1/auth/tenant", `{"tenant_id":9401}`, 200, &login)
			before := f.snapshot(t)
			// When: authenticated POST through the actual app route.
			body := fmt.Sprintf(`{"ts":%q,"properties":{"temperature":25}}`, telemetryTimestamp().Format(time.RFC3339))
			for _, device := range []string{"v2-hidden", "v2-foreign"} {
				status, response := telemetryHTTPPostDevice(t, server.URL, login.Token, device, body)
				t.Logf("scenario=app_%s_%s post_status=%d response=%s", tc.role, device, status, response)
				if status != 404 || before != f.snapshot(t) {
					t.Fatalf("hidden resource status=%d or mutated state", status)
				}
			}
			status, response := telemetryHTTPPost(t, server.URL, login.Token, body)
			// Then: roles with write permission commit; all rejected roles leave identical state.
			after := f.snapshot(t)
			t.Logf("scenario=app_%s login_status=200 post_status=%d response=%s before=%s after=%s", tc.role, status, response, before, after)
			if status != tc.want {
				t.Errorf("POST status=%d want=%d", status, tc.want)
			}
			if tc.want == 403 && before != after {
				t.Error("denied HTTP request mutated business state")
			}
			if tc.want == 202 && before == after {
				t.Error("accepted HTTP request did not persist")
			}
			if tc.want == 202 {
				f.role(t, "viewer", true)
				status, response := telemetryHTTPPost(t, server.URL, login.Token, body)
				t.Logf("scenario=app_%s_live_downgrade post_status=%d response=%s before=%s after=%s", tc.role, status, response, after, f.snapshot(t))
				if status != 401 || after != f.snapshot(t) {
					t.Fatal("old privileged JWT survived role downgrade")
				}
			}
		})
	}
}

func TestAppTelemetryWriteDeniedWhenTokenRevoked(t *testing.T) {
	// Given: a valid admin login whose live membership is revoked afterward.
	f := newTelemetryPermissionFixture(t)
	f.role(t, "admin", true)
	previous := appapi.WechatExchanger
	appapi.WechatExchanger = func(string) (string, error) { return "v2-actor", nil }
	t.Cleanup(func() { appapi.WechatExchanger = previous })
	server := httptest.NewServer(appapi.New(appapi.Config{SecretKey: strings.Repeat("v", 32), JWT: time.Hour}, appapi.Deps{Users: f.svc, Telemetry: f.svc.Telemetry()}, slog.New(slog.NewTextHandler(io.Discard, nil))).Routes())
	defer server.Close()
	var login struct{ Token string }
	m6bHTTPRequest(t, server.URL, "", http.MethodPost, "/api/v1/auth/login", `{"code":"fixture"}`, 200, &login)
	m6bHTTPRequest(t, server.URL, login.Token, http.MethodPost, "/api/v1/auth/tenant", `{"tenant_id":9401}`, 200, &login)
	f.role(t, "admin", false)
	before := f.snapshot(t)
	// When: reusing the original JWT.
	status, response := telemetryHTTPPost(t, server.URL, login.Token, `{"ts":"2026-10-01T00:00:00Z","properties":{"temperature":25}}`)
	// Then: unauthorized and no persisted writes.
	after := f.snapshot(t)
	t.Logf("scenario=app_revoked post_status=%d response=%s before=%s after=%s", status, response, before, after)
	if status != 401 || before != after {
		t.Fatalf("status=%d state_changed=%t", status, before != after)
	}
}

func telemetryHTTPPost(t *testing.T, baseURL, token, body string) (int, string) {
	return telemetryHTTPPostDevice(t, baseURL, token, "v2-permission", body)
}

func telemetryHTTPPostDevice(t *testing.T, baseURL, token, device, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v2/devices/"+device+"/telemetry", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(raw) {
		t.Fatalf("invalid JSON response: %s", raw)
	}
	return response.StatusCode, string(raw)
}
