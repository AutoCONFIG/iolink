package appapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
)

var poisonProviderError = errors.New("postgres://private-user:private-password@private-host/private-database?token=private-token")

type failingPondList struct{ fakeRepos }

func (failingPondList) ListByUser(context.Context, int64) ([]domain.Pond, error) {
	return nil, poisonProviderError
}

type failingDeviceList struct{ fakeDevices }

func (failingDeviceList) ListByPond(context.Context, int64) ([]domain.Device, error) {
	return nil, poisonProviderError
}

type failingAlarmList struct{ fakeAlarms }

func (failingAlarmList) ListByUser(context.Context, int64, int) ([]domain.Alarm, error) {
	return nil, poisonProviderError
}

type failingFilteredAlarms struct{ failingAlarmList }

func (failingFilteredAlarms) ListByUserFiltered(context.Context, int64, domain.AlarmLevel, bool, int, int) ([]domain.Alarm, error) {
	return nil, poisonProviderError
}

type failingAlarmConfirm struct{ fakeAlarms }

func (failingAlarmConfirm) ConfirmByUser(context.Context, int64, int64) error {
	return poisonProviderError
}

type failingHistory struct{ fakeTelemetry }

func (failingHistory) HistoryForUser(context.Context, string, int64, string, time.Time, time.Time, int) ([]domain.MetricPoint, error) {
	return nil, poisonProviderError
}

type missingScopedHistory struct{ domain.TelemetryRepo }

type failingEnsureUser struct{ fakeTenantUsers }

func (failingEnsureUser) EnsureUser(context.Context, string) (*domain.User, error) {
	return nil, poisonProviderError
}

type failingDefaultTenant struct{ fakeTenantUsers }

func (failingDefaultTenant) DefaultTenantForUser(context.Context, int64) (int64, error) {
	return 0, poisonProviderError
}

type failingTokenVersion struct{ fakeTenantUsers }

func (failingTokenVersion) UserTokenVersion(context.Context, int64) (int, error) {
	return 0, poisonProviderError
}

type providerHTTPCase struct {
	name, method, path string
	configure          func(*Deps)
}

func TestBaselineHTTPProviderFailuresUseSafeInternalError(t *testing.T) {
	for _, tc := range []providerHTTPCase{
		{"pond_list", "GET", "/ponds", func(d *Deps) { d.Ponds = &failingPondList{} }},
		{"pond_list_alarms", "GET", "/ponds", func(d *Deps) { d.Alarms = &failingAlarmList{} }},
		{"pond_list_devices", "GET", "/ponds", func(d *Deps) { d.Devices = failingDeviceList{} }},
		{"pond_detail_devices", "GET", "/ponds/1", func(d *Deps) { d.Devices = failingDeviceList{} }},
		{"pond_detail_alarms", "GET", "/ponds/1", func(d *Deps) { d.Alarms = &failingAlarmList{} }},
		{"device_list_ponds", "GET", "/devices?pond_id=1", func(d *Deps) { d.Ponds = &failingPondDetail{} }},
		{"device_list_devices", "GET", "/devices", func(d *Deps) { d.Devices = failingDeviceList{} }},
		{"history_query", "GET", "/water/history?device_no=private-device&metric=ph", func(d *Deps) { d.Telemetry = failingHistory{} }},
		{"history_capability", "GET", "/water/history?device_no=private-device&metric=ph", func(d *Deps) { d.Telemetry = missingScopedHistory{fakeTelemetry{}} }},
		{"alarms_list", "GET", "/alarms", func(d *Deps) { d.Alarms = &failingAlarmList{} }},
		{"alarms_filtered", "GET", "/alarms", func(d *Deps) { d.Alarms = &failingFilteredAlarms{} }},
		{"alarm_confirm", "POST", "/alarms/1/confirm", func(d *Deps) { d.Alarms = &failingAlarmConfirm{} }},
		{"summary_ponds", "GET", "/stats/summary", func(d *Deps) { d.Ponds = &failingPondList{} }},
		{"summary_devices", "GET", "/stats/summary", func(d *Deps) { d.Devices = failingDeviceList{} }},
		{"summary_alarms", "GET", "/stats/summary", func(d *Deps) { d.Alarms = &failingAlarmList{} }},
		{"login_user", "POST", "/auth/login", func(d *Deps) { d.Users = failingEnsureUser{} }},
		{"login_tenant", "POST", "/auth/login", func(d *Deps) { d.Users = failingDefaultTenant{} }},
		{"login_version", "POST", "/auth/login", func(d *Deps) { d.Users = failingTokenVersion{} }},
	} {
		t.Run(tc.name, func(t *testing.T) { exerciseProviderFailure(t, tc) })
	}
}

func exerciseProviderFailure(t *testing.T, tc providerHTTPCase) {
	t.Helper()
	// Given: a successful auth provider, a safe dump logger and one failed dependency.
	dumpRoot := t.TempDir()
	logger, sink, err := observability.New(observability.Config{Dir: dumpRoot, Level: "error", MaxMB: 1, Backups: 1, Version: "test"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sink.Close(); err != nil {
			t.Error(err)
		}
	})
	deps := Deps{Ponds: &fakeRepos{ponds: []domain.Pond{{ID: 1}}}, Devices: fakeDevices{}, Telemetry: fakeTelemetry{}, Alarms: &fakeAlarms{}, Users: fakeTenantUsers{role: "owner"}}
	tc.configure(&deps)
	s := New(Config{SecretKey: "test-key", JWT: time.Hour}, deps, logger)
	s.SetWechatExchanger(func(string) (string, error) { return "private-openid", nil })
	ts := httptest.NewServer(s.Routes())
	t.Cleanup(ts.Close)
	var body io.Reader
	var token string
	if tc.path == "/auth/login" {
		body = strings.NewReader(`{"code":"private-wx-code"}`)
	} else {
		token = login(t, ts)
	}
	// When: an authenticated client reaches the real HTTP handler's failed provider.
	req, err := http.NewRequest(tc.method, ts.URL+"/api/v1"+tc.path, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "private-inbound-id")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatal(err)
	}
	ts.Close()
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	// Then: the public contract is stable and the dump contains a correlated safe class.
	if resp.StatusCode != http.StatusInternalServerError || string(response) != `{"error":"internal_error"}` {
		t.Fatalf("status=%d body=%s", resp.StatusCode, response)
	}
	logs, err := os.ReadFile(filepath.Join(dumpRoot, "test", "iolinkd.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(logs, []byte("private-")) || token != "" && bytes.Contains(logs, []byte(token)) {
		t.Fatal("sensitive provider/request data leaked into dump")
	}
	requestID := resp.Header.Get("X-Request-ID")
	for _, line := range bytes.Split(bytes.TrimSpace(logs), []byte{'\n'}) {
		var record struct {
			Stage     string          `json:"stage"`
			Level     string          `json:"level"`
			RequestID string          `json:"request_id"`
			Error     json.RawMessage `json:"err"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		if record.Stage == "http_provider" && record.Level == "ERROR" {
			expectedError := `"*errors.errorString"`
			if tc.name == "history_capability" || tc.name == "device_page_capability" {
				expectedError = ""
			}
			if requestID == "" || record.RequestID != requestID || string(record.Error) != expectedError {
				t.Fatalf("invalid diagnostic=%+v", record)
			}
			t.Logf("status=500 response=internal_error correlated_diagnostic=%s", line)
			return
		}
	}
	t.Fatal("missing safe provider error diagnostic")
}
