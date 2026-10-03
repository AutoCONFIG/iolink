package core_test

import (
	"context"
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

func appScopeServer(t *testing.T, f telemetryPermissionFixture) (*httptest.Server, string) {
	t.Helper()
	previous := appapi.WechatExchanger
	appapi.WechatExchanger = func(string) (string, error) { return "v2-actor", nil }
	t.Cleanup(func() { appapi.WechatExchanger = previous })
	server := httptest.NewServer(appapi.New(appapi.Config{SecretKey: strings.Repeat("v", 32), JWT: time.Hour}, appapi.Deps{Users: f.svc, Ponds: f.svc.Ponds(), Devices: f.svc.Devices(), Telemetry: f.svc.Telemetry(), Alarms: f.svc.Alarms()}, slog.New(slog.NewTextHandler(io.Discard, nil))).Routes())
	t.Cleanup(server.Close)
	var login struct{ Token string }
	m6bHTTPRequest(t, server.URL, "", http.MethodPost, "/api/v1/auth/login", `{"code":"fixture"}`, 200, &login)
	m6bHTTPRequest(t, server.URL, login.Token, http.MethodPost, "/api/v1/auth/tenant", `{"tenant_id":9401}`, 200, &login)
	return server, login.Token
}

type appScopeRequest struct {
	method, path, body, token string
}

func appScopeHTTP(t *testing.T, server *httptest.Server, input appScopeRequest) (int, json.RawMessage) {
	t.Helper()
	req, err := http.NewRequest(input.method, server.URL+input.path, strings.NewReader(input.body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+input.token)
	req.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("http method=%s path=%s status=%d response=%s", input.method, input.path, response.StatusCode, raw)
	return response.StatusCode, raw
}

func seedAppScopeReadings(t *testing.T, f telemetryPermissionFixture) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO alarm_rules(pond_id,metric,max_value,level,enabled) VALUES(9403,'temperature',20,'warning',true)`); err != nil {
		t.Fatal(err)
	}
	for _, device := range []string{"v2-permission", "v2-hidden"} {
		if _, err := f.repo.SubmitTelemetry(telemetryActor("owner", 9401), device, 9401, time.Now().UTC().Add(-time.Minute), telemetryProperties()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAppResourceHTTPWhenRoleDeterminesFarmScope(t *testing.T) {
	for _, role := range []string{"owner", "admin", "member", "viewer", "support"} {
		t.Run(role, func(t *testing.T) {
			// Given: two same-tenant farms; managers have no assignments and lower roles have one.
			f := newTelemetryPermissionFixture(t)
			seedAppScopeReadings(t, f)
			f.role(t, role, true)
			manager := role == "owner" || role == "admin"
			if manager {
				if _, err := f.pool.Exec(context.Background(), `DELETE FROM farm_memberships WHERE user_id=9402`); err != nil {
					t.Fatal(err)
				}
			}
			server, token := appScopeServer(t, f)
			wantCount := 1
			if manager {
				wantCount = 2
			}
			for _, path := range []string{"/api/v1/ponds", "/api/v1/devices", "/api/v1/alarms"} {
				t.Run(path, func(t *testing.T) {
					// When: listing through authenticated production HTTP routes.
					status, raw := appScopeHTTP(t, server, appScopeRequest{http.MethodGet, path, "", token})
					// Then: the response contains exactly the farms permitted by the live tenant role.
					var rows []json.RawMessage
					if err := json.Unmarshal(raw, &rows); err != nil || status != 200 || len(rows) != wantCount {
						t.Fatalf("role=%s status=%d rows=%d want=%d error=%v", role, status, len(rows), wantCount, err)
					}
				})
			}
			t.Run("stats", func(t *testing.T) {
				status, raw := appScopeHTTP(t, server, appScopeRequest{http.MethodGet, "/api/v1/stats/summary", "", token})
				var stats struct {
					Ponds          []json.RawMessage `json:"ponds"`
					OfflineDevices int               `json:"offline_devices"`
					AlarmDevices   int               `json:"alarm_devices"`
				}
				if err := json.Unmarshal(raw, &stats); err != nil || status != 200 || len(stats.Ponds) != wantCount || stats.OfflineDevices != wantCount || stats.AlarmDevices != wantCount {
					t.Fatalf("status=%d stats=%+v want_count=%d error=%v", status, stats, wantCount, err)
				}
			})
			for _, resource := range []struct {
				pond, device string
				foreign      bool
			}{{"9401", "v2-permission", false}, {"9403", "v2-hidden", false}, {"9402", "v2-foreign", true}} {
				want := 200
				if resource.foreign || (!manager && resource.device == "v2-hidden") {
					want = 404
				}
				for _, path := range []string{
					"/api/v1/ponds/" + resource.pond,
					"/api/v1/devices?pond_id=" + resource.pond,
					"/api/v1/devices/" + resource.device,
					"/api/v1/water/latest?device_no=" + resource.device,
					"/api/v1/water/history?device_no=" + resource.device + "&metric=temperature&range=30d",
					"/api/v2/devices/" + resource.device + "/model/latest",
					"/api/v2/devices/" + resource.device + "/history?metric=temperature",
				} {
					t.Run(path, func(t *testing.T) {
						// When: accessing a resource through its public read route.
						status, raw := appScopeHTTP(t, server, appScopeRequest{http.MethodGet, path, "", token})
						// Then: allowed resources return data and hidden or foreign resources return 404.
						if status != want || !json.Valid(raw) {
							t.Fatalf("role=%s status=%d want=%d response=%s", role, status, want, raw)
						}
						if want == 200 && strings.Contains(path, "history") {
							var history struct {
								Points []struct{ Value float64 } `json:"points"`
							}
							if err := json.Unmarshal(raw, &history); err != nil || len(history.Points) != 1 || history.Points[0].Value != 25 {
								t.Fatalf("history=%+v error=%v", history, err)
							}
						}
					})
				}
			}
			var alarmID int64
			if err := f.pool.QueryRow(context.Background(), `SELECT id FROM alarms WHERE device_no='v2-hidden'`).Scan(&alarmID); err != nil {
				t.Fatal(err)
			}
			before := f.snapshot(t)
			// When: confirming an alarm in the unassigned same-tenant farm.
			status, _ := appScopeHTTP(t, server, appScopeRequest{http.MethodPost, fmt.Sprintf("/api/v1/alarms/%d/confirm", alarmID), "", token})
			// Then: managers may confirm, limited roles cannot mutate the hidden farm.
			want := 404
			if manager {
				want = 204
			} else if role == "viewer" {
				want = 403
			}
			after := f.snapshot(t)
			t.Logf("scenario=%s_hidden_alarm_confirm before=%s after=%s", role, before, after)
			if status != want || (!manager && before != after) {
				t.Fatalf("status=%d want=%d state_changed=%t", status, want, before != after)
			}
		})
	}
}

func TestAppResourceHTTPWhenManagerTokenLosesAuthority(t *testing.T) {
	for _, change := range []string{"role", "membership", "platform_authority"} {
		t.Run(change, func(t *testing.T) {
			// Given: an admin JWT followed by a live revocation or platform identity change.
			f := newTelemetryPermissionFixture(t)
			f.role(t, "admin", true)
			server, token := appScopeServer(t, f)
			query := map[string]string{
				"role":               `UPDATE tenant_memberships SET role='viewer',permission_version=permission_version+1 WHERE user_id=9402`,
				"membership":         `UPDATE tenant_memberships SET active=false,permission_version=permission_version+1 WHERE user_id=9402`,
				"platform_authority": `UPDATE users SET authority='ADMIN' WHERE id=9402`,
			}[change]
			if _, err := f.pool.Exec(context.Background(), query); err != nil {
				t.Fatal(err)
			}
			before := f.snapshot(t)
			// When: the original token attempts a same-tenant read and write.
			for _, input := range []appScopeRequest{
				{http.MethodGet, "/api/v2/devices/v2-hidden/model/latest", "", token},
				{http.MethodPost, "/api/v2/devices/v2-hidden/telemetry", `{"ts":"2026-10-01T00:00:00Z","properties":{"temperature":25}}`, token},
			} {
				status, _ := appScopeHTTP(t, server, input)
				// Then: middleware rejects the stale privilege and storage remains identical.
				after := f.snapshot(t)
				t.Logf("scenario=%s_revocation status=%d before=%s after=%s", change, status, before, after)
				if status != 401 || before != after {
					t.Fatalf("status=%d state_changed=%t", status, before != after)
				}
			}
		})
	}
}
