package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/appapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
)

func TestLatestReadingHTTPWithLargePondAndDatabaseFailure(t *testing.T) {
	// Given: 201 active pond devices and both APIs wired to real PostgreSQL.
	svc, pool := setup(t)
	ctx := context.Background()
	if _, err := svc.Telemetry().Latest(ctx, "one"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("absent shadow classification: %v", err)
	}
	execute(t, pool, `UPDATE users SET username='latest-owner',password_hash=$1,authority='USER' WHERE id=1`, platform.HashPassword("Latest-test-only-1234"))
	execute(t, pool, `INSERT INTO devices(pond_id,device_no,mqtt_username,secret_hash) SELECT 1,'bulk-'||n,'bulk-user-'||n,'test-fixture' FROM generate_series(1,200) n`)
	if err := svc.HandleEvent(event.Event{Kind: event.KindProperties, DeviceNo: "bulk-200", MessageID: "latest-201", Ts: time.Now().UTC().Truncate(time.Microsecond), Properties: map[string]float64{"temperature": 26}}); err != nil {
		t.Fatal(err)
	}
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	admin := adminapi.New(adminapi.Config{SecretKey: "latest-http-test", JWT: time.Hour}, adminapi.Deps{Store: svc, Telemetry: svc.Telemetry(), Policy: policy, Logger: logger})
	app := appapi.New(appapi.Config{SecretKey: "latest-http-test", JWT: time.Hour}, appapi.Deps{Users: svc, Ponds: svc.Ponds(), Devices: svc.Devices(), Telemetry: svc.Telemetry(), Alarms: svc.Alarms()}, logger)
	app.SetWechatExchanger(func(string) (string, error) { return "test-user", nil })
	mux := http.NewServeMux()
	mux.Handle("/user/", admin.UserRoutes())
	mux.Handle("/api/", app.Routes())
	server := httptest.NewServer(mux)
	defer server.Close()
	call := func(method, path, token, body string, want int) []byte {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != want {
			t.Fatalf("%s status=%d want=%d body=%s", path, resp.StatusCode, want, raw)
		}
		return raw
	}
	token := func(raw []byte) string {
		t.Helper()
		var out struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(raw, &out); err != nil || out.Token == "" {
			t.Fatalf("invalid login: %v", err)
		}
		return out.Token
	}
	userToken := token(call("POST", "/user/v1/login", "", `{"username":"latest-owner","password":"Latest-test-only-1234"}`, 200))
	appToken := token(call("POST", "/api/v1/auth/login", "", `{"code":"local-provider-fixture"}`, 200))
	// When: both authenticated status-wall routes read the newest shadow.
	for _, tc := range []struct{ path, token string }{{"/user/v1/ponds", userToken}, {"/api/v1/ponds", appToken}} {
		raw := call("GET", tc.path, tc.token, "", 200)
		var ponds []struct {
			ID          int64 `json:"id"`
			PondID      int64 `json:"pond_id"`
			DeviceCount int   `json:"device_count"`
			Latest      *struct {
				DeviceNo string `json:"device_no"`
			} `json:"latest"`
		}
		if err := json.Unmarshal(raw, &ponds); err != nil {
			t.Fatal(err)
		}
		// Then: both include the last page and agree on the latest device.
		if len(ponds) != 2 || ponds[0].DeviceCount != 201 || ponds[0].Latest == nil || ponds[0].Latest.DeviceNo != "bulk-200" || ponds[1].DeviceCount != 0 || ponds[1].Latest != nil {
			t.Fatalf("incomplete pond status on %s: %s", tc.path, raw)
		}
	}
	call("GET", "/api/v1/water/latest?device_no=one", appToken, "", 200)
	// Given: the shadow storage is unavailable while authentication still works.
	execute(t, pool, `ALTER TABLE device_shadows RENAME TO unavailable_test_shadows`)
	t.Cleanup(func() { execute(t, pool, `ALTER TABLE unavailable_test_shadows RENAME TO device_shadows`) })
	// When: every existing latest-reading HTTP surface queries that provider.
	for _, tc := range []struct{ path, token string }{
		{"/user/v1/ponds", userToken},
		{"/user/v1/devices/bulk-200", userToken},
		{"/api/v1/ponds", appToken},
		{"/api/v1/ponds/1", appToken},
		{"/api/v1/devices/bulk-200", appToken},
		{"/api/v1/water/latest?device_no=bulk-200", appToken},
	} {
		// Then: an actual PostgreSQL failure is not a successful empty reading.
		call("GET", tc.path, tc.token, "", 500)
	}
}
