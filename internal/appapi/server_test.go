package appapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	iolinkcontractsdomain "git.hyhy.fun/rsplab/iolink/internal/domain"
)

// ---- fakes (consumer-side; core not needed to test appapi) ----

type fakeRepos struct {
	ponds  []iolinkcontractsdomain.Pond
	alarms []iolinkcontractsdomain.Alarm
}

func (f *fakeRepos) ListByUser(_ context.Context, _ int64) ([]iolinkcontractsdomain.Pond, error) {
	return f.ponds, nil
}
func (f *fakeRepos) Get(_ context.Context, id int64) (iolinkcontractsdomain.Pond, error) {
	for _, p := range f.ponds {
		if p.ID == id {
			return p, nil
		}
	}
	return iolinkcontractsdomain.Pond{}, iolinkcontractsdomain.ErrUnknownMetric
}

type fakeDevices struct{}

func (fakeDevices) ListByPond(_ context.Context, _ int64) ([]iolinkcontractsdomain.Device, error) {
	return []iolinkcontractsdomain.Device{{ID: 1, PondID: 1, DeviceNo: "dev-001", Status: iolinkcontractsdomain.DeviceOnline}}, nil
}
func (fakeDevices) GetByDeviceNo(_ context.Context, no string) (iolinkcontractsdomain.Device, error) {
	return iolinkcontractsdomain.Device{ID: 1, PondID: 1, DeviceNo: no, Status: iolinkcontractsdomain.DeviceOnline}, nil
}
func (fakeDevices) UpdateStatus(_ context.Context, _ string, _ iolinkcontractsdomain.DeviceStatus) error {
	return nil
}

type fakeTelemetry struct{}

func (fakeTelemetry) Latest(_ context.Context, no string) (iolinkcontractsdomain.Reading, error) {
	v := 6.8
	return iolinkcontractsdomain.Reading{DeviceNo: no, Timestamp: time.Now(), DO: &v}, nil
}
func (fakeTelemetry) History(_ context.Context, _, _ string, _, _ time.Time, _ int) ([]iolinkcontractsdomain.MetricPoint, error) {
	return []iolinkcontractsdomain.MetricPoint{{Ts: time.Now(), Value: 6.8}}, nil
}

type fakeAlarms struct{ list []iolinkcontractsdomain.Alarm }

func (f *fakeAlarms) ListByUser(_ context.Context, _ int64, _ int) ([]iolinkcontractsdomain.Alarm, error) {
	return f.list, nil
}
func (f *fakeAlarms) Confirm(_ context.Context, _ int64) error { return nil }

type fakeRules struct{}

func (fakeRules) RulesForDevice(_ context.Context, _ string) ([]iolinkcontractsdomain.AlarmRule, error) {
	return nil, nil
}

type fakeUsers struct{}

func (fakeUsers) FindByOpenID(_ context.Context, _ string) (*iolinkcontractsdomain.User, error) {
	return nil, nil
}
func (fakeUsers) EnsureUser(_ context.Context, openID string) (*iolinkcontractsdomain.User, error) {
	return &iolinkcontractsdomain.User{ID: 1, OpenID: openID}, nil
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	f := &fakeRepos{ponds: []iolinkcontractsdomain.Pond{{ID: 1, FarmID: 1, Name: "1号池塘"}}}
	s := New(Config{Addr: ":0", SecretKey: "test-key", JWT: time.Hour}, Deps{
		Ponds: f, Devices: fakeDevices{}, Telemetry: fakeTelemetry{},
		Alarms: &fakeAlarms{}, Users: fakeUsers{},
	}, testLogger())
	s.SetWechatExchanger(func(string) (string, error) { return "openid-123", nil })
	return httptest.NewServer(s.Routes())
}

func login(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json",
		bytesReader(`{"code":"wx-code"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct{ Token string }
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.Token == "" {
		t.Fatal("no token in login response")
	}
	return out.Token
}

func TestLoginAndListPonds(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	token := login(t, ts)

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/ponds", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var ponds []map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&ponds)
	if len(ponds) != 1 || ponds[0]["pond_name"] != "1号池塘" {
		t.Fatalf("bad ponds: %+v", ponds)
	}
}

func TestWaterLatest(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	token := login(t, ts)

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/water/latest?device_no=dev-001", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out["dissolved_oxygen"] != 6.8 {
		t.Fatalf("bad latest: %+v", out)
	}
}

func TestAuthRequired(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/api/v1/ponds")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}
