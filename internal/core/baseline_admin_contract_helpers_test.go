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

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"github.com/jackc/pgx/v5/pgxpool"
)

type baselineAdminFixture struct {
	svc    *core.Service
	pool   *pgxpool.Pool
	server *httptest.Server
	token  string
}

type baselineAdminRequest struct {
	method, route, path, body string
	status                    int
}

type baselineAdminResponse struct {
	ID        int64  `json:"id"`
	Token     string `json:"token"`
	DeviceNo  string `json:"device_no"`
	Confirmed int64  `json:"confirmed"`
}

func newBaselineAdminFixture(t *testing.T) *baselineAdminFixture {
	t.Helper()
	_, pool := setup(t)
	execute(t, pool, `UPDATE users SET username='contract-owner',password_hash=$1,authority='USER' WHERE id=1`, platform.HashPassword("Contract-test-only-1234"))
	execute(t, pool, `SELECT setval(pg_get_serial_sequence('farms','id'),100); SELECT setval(pg_get_serial_sequence('ponds','id'),100); SELECT setval(pg_get_serial_sequence('users','id'),100)`)
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := core.NewWithPolicy(context.Background(), pool, slog.New(slog.NewJSONHandler(io.Discard, nil)), policy)
	if err != nil {
		t.Fatal(err)
	}
	installTestLicense(t, pool, svc, 100)
	server := httptest.NewServer(adminapi.New(adminapi.Config{SecretKey: strings.Repeat("c", 32), JWT: time.Hour}, adminapi.Deps{Store: svc, Telemetry: svc.Telemetry(), Catalog: svc.Products(), Policy: policy, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}).UserRoutes())
	t.Cleanup(server.Close)
	f := &baselineAdminFixture{svc: svc, pool: pool, server: server}
	// Given: credentials are fixture data; tokens stay in memory only.
	var login baselineAdminResponse
	m6bHTTPRequest(t, server.URL, "", http.MethodPost, "/user/v1/login", `{"username":"contract-owner","password":"Contract-test-only-1234"}`, 200, &login)
	f.token = login.Token
	if f.token == "" {
		t.Fatal("fixture login returned no token")
	}
	return f
}

func (f *baselineAdminFixture) request(t *testing.T, tc baselineAdminRequest) baselineAdminResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, tc.method, f.server.URL+"/user/v1"+tc.path, strings.NewReader(tc.body))
	if err != nil {
		t.Fatal("HTTP request construction failed")
	}
	req.Header.Set("Content-Type", "application/json")
	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal("HTTP request failed")
	}
	raw, err := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if err != nil || closeErr != nil {
		t.Fatal("HTTP response read failed")
	}
	if resp.StatusCode != tc.status {
		t.Errorf("%s %s status=%d want=%d", tc.method, tc.route, resp.StatusCode, tc.status)
	}
	var result baselineAdminResponse
	if len(raw) > 0 && raw[0] == '{' {
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal("HTTP response decode failed")
		}
	}
	// Then: check the current schema without putting bodies or credentials in output.
	t.Run("response_contract", func(t *testing.T) {
		assertHTTPContract(t, "admin", tc.method, tc.route, resp.StatusCode, resp.Header.Get("Content-Type"), raw)
	})
	return result
}

func (f *baselineAdminFixture) state(t *testing.T) string {
	t.Helper()
	var digest string
	err := f.pool.QueryRow(context.Background(), `SELECT md5(jsonb_build_object(
		'farms',(SELECT jsonb_agg(jsonb_build_array(id,owner_id,name,location) ORDER BY id) FROM farms),
		'ponds',(SELECT jsonb_agg(jsonb_build_array(id,farm_id,name,area_mu) ORDER BY id) FROM ponds),
		'devices',(SELECT jsonb_agg(jsonb_build_array(id,pond_id,name,model,report_interval,disabled_at) ORDER BY id) FROM devices),
		'rules',(SELECT jsonb_agg(jsonb_build_array(id,pond_id,metric,min_value,max_value,level,enabled) ORDER BY id) FROM alarm_rules),
		'alarms',(SELECT jsonb_agg(jsonb_build_array(id,confirmed_at) ORDER BY id) FROM alarms),
		'users',(SELECT jsonb_agg(jsonb_build_array(id,token_version,password_hash) ORDER BY id) FROM users),
		'audits',(SELECT count(*) FROM audit_events)
	)::text)`).Scan(&digest)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func TestBaselineAdminRejectsNonpositiveResourceIDsAndInvalidQueries(t *testing.T) {
	f := newBaselineAdminFixture(t)
	for _, id := range []string{"0", "-1"} {
		for _, tc := range []baselineAdminRequest{
			{"PUT", "/farms/{id}", "/farms/" + id, `{"name":"must reject"}`, 400},
			{"DELETE", "/farms/{id}", "/farms/" + id, "", 400},
			{"PUT", "/ponds/{id}", "/ponds/" + id, `{"farm_id":1,"name":"must reject"}`, 400},
			{"DELETE", "/ponds/{id}", "/ponds/" + id, "", 400},
			{"PUT", "/alarm-rules/{id}", "/alarm-rules/" + id, `{"pond_id":1,"metric":"temperature","max_value":25,"level":"warning"}`, 400},
			{"DELETE", "/alarm-rules/{id}", "/alarm-rules/" + id, "", 400},
			{"POST", "/alarms/{id}/confirm", "/alarms/" + id + "/confirm", "", 400},
		} {
			t.Run(id+"_"+tc.method+"_"+tc.route, func(t *testing.T) {
				before := f.state(t)
				f.request(t, tc)
				if f.state(t) != before {
					t.Error("invalid identifier changed persisted state")
				}
			})
		}
	}
	for index, tc := range []baselineAdminRequest{
		{"GET", "/devices", "/devices?pond_id=", "", 400},
		{"GET", "/devices", fmt.Sprintf("/devices?page=%d&page_size=200", int(^uint(0)>>1)), "", 400},
	} {
		t.Run([]string{"empty_pond_filter", "overflow_page"}[index], func(t *testing.T) { f.request(t, tc) })
	}
}

func TestBaselineAdminDeviceDetailPreservesDatabaseFailure(t *testing.T) {
	f := newBaselineAdminFixture(t)
	execute(t, f.pool, "ALTER TABLE devices RENAME TO baseline_contract_devices_unavailable")
	defer execute(t, f.pool, "ALTER TABLE baseline_contract_devices_unavailable RENAME TO devices")
	f.request(t, baselineAdminRequest{"GET", "/devices/{device_no}", "/devices/one", "", 500})
}
