package appapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"github.com/golang-jwt/jwt/v5"

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
func (f *fakeRepos) GetByUser(ctx context.Context, id, _ int64) (iolinkcontractsdomain.Pond, error) {
	return f.Get(ctx, id)
}

type fakeDevices struct{}

func (fakeDevices) ListByPond(_ context.Context, _ int64) ([]iolinkcontractsdomain.Device, error) {
	return []iolinkcontractsdomain.Device{{ID: 1, PondID: 1, DeviceNo: "dev-001", Status: iolinkcontractsdomain.DeviceOnline}}, nil
}
func (fakeDevices) GetByDeviceNo(_ context.Context, no string) (iolinkcontractsdomain.Device, error) {
	return iolinkcontractsdomain.Device{ID: 1, PondID: 1, DeviceNo: no, Status: iolinkcontractsdomain.DeviceOnline}, nil
}
func (f fakeDevices) GetByDeviceNoForUser(ctx context.Context, no string, _ int64) (iolinkcontractsdomain.Device, error) {
	return f.GetByDeviceNo(ctx, no)
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
func (fakeTelemetry) HistoryForUser(ctx context.Context, _ string, _ int64, metric string, from, to time.Time, maxPoints int) ([]iolinkcontractsdomain.MetricPoint, error) {
	return fakeTelemetry{}.History(ctx, "", metric, from, to, maxPoints)
}

type fakeAlarms struct{ list []iolinkcontractsdomain.Alarm }

func (f *fakeAlarms) ListByUser(_ context.Context, _ int64, _ int) ([]iolinkcontractsdomain.Alarm, error) {
	return f.list, nil
}
func (f *fakeAlarms) Confirm(_ context.Context, _ int64) error          { return nil }
func (f *fakeAlarms) ConfirmByUser(_ context.Context, _, _ int64) error { return nil }

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
func (fakeUsers) UserTokenVersion(_ context.Context, _ int64) (int, error) { return 0, nil }

type fakeTenantUsers struct {
	fakeUsers
	role string
}

func (fakeTenantUsers) DefaultTenantForUser(_ context.Context, _ int64) (int64, error) { return 7, nil }
func (fakeTenantUsers) TenantMembershipVersion(_ context.Context, _, _ int64) (int64, error) {
	return 0, nil
}
func (f fakeTenantUsers) TenantRole(_ context.Context, _, _ int64) (string, error) {
	if f.role != "" {
		return f.role, nil
	}
	return "viewer", nil
}
func (fakeTenantUsers) ListUserTenants(_ context.Context, _ int64) ([]iolinkcontractsdomain.TenantMembership, error) {
	return []iolinkcontractsdomain.TenantMembership{{TenantID: 7, Name: "tenant", Role: "viewer"}}, nil
}

type revocableTenantUsers struct {
	fakeTenantUsers
	version int64
}

type noTenantUsers struct{ fakeTenantUsers }

func (noTenantUsers) DefaultTenantForUser(context.Context, int64) (int64, error) {
	return 0, iolinkcontractsdomain.ErrNotFound
}

func (f *revocableTenantUsers) TenantMembershipVersion(_ context.Context, _, _ int64) (int64, error) {
	return f.version, nil
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	f := &fakeRepos{ponds: []iolinkcontractsdomain.Pond{{ID: 1, FarmID: 1, Name: "1号池塘"}}}
	s := New(Config{Addr: ":0", SecretKey: "test-key", JWT: time.Hour}, Deps{
		Ponds: f, Devices: fakeDevices{}, Telemetry: fakeTelemetry{},
		Alarms: &fakeAlarms{}, Users: fakeTenantUsers{},
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

func TestTenantViewerCannotConfirmAlarm(t *testing.T) {
	f := &fakeRepos{ponds: []iolinkcontractsdomain.Pond{{ID: 1, FarmID: 1, Name: "1号池塘"}}}
	s := New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Ponds: f, Devices: fakeDevices{}, Telemetry: fakeTelemetry{}, Alarms: &fakeAlarms{}, Users: fakeTenantUsers{}}, testLogger())
	s.SetWechatExchanger(func(string) (string, error) { return "openid-tenant", nil })
	ts := httptest.NewServer(s.Routes())
	defer ts.Close()
	token := login(t, ts)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/alarms/1/confirm", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer confirm status=%d", resp.StatusCode)
	}
}

func TestTenantRoleConfirmationMatrix(t *testing.T) {
	for _, tc := range []struct {
		role string
		want int
	}{
		{role: "owner", want: http.StatusNoContent},
		{role: "admin", want: http.StatusNoContent},
		{role: "member", want: http.StatusNoContent},
		{role: "support", want: http.StatusNoContent},
		{role: "viewer", want: http.StatusForbidden},
	} {
		t.Run(tc.role, func(t *testing.T) {
			users := fakeTenantUsers{role: tc.role}
			s := New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Ponds: &fakeRepos{}, Devices: fakeDevices{}, Telemetry: fakeTelemetry{}, Alarms: &fakeAlarms{}, Users: users}, testLogger())
			s.SetWechatExchanger(func(string) (string, error) { return "openid-" + tc.role, nil })
			ts := httptest.NewServer(s.Routes())
			defer ts.Close()
			token := login(t, ts)
			req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/alarms/1/confirm", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("role %s status=%d want=%d", tc.role, resp.StatusCode, tc.want)
			}
		})
	}
}

func TestTenantTokenRevocationAndSwitchBoundary(t *testing.T) {
	f := &fakeRepos{ponds: []iolinkcontractsdomain.Pond{{ID: 1, FarmID: 1, Name: "1号池塘"}}}
	users := &revocableTenantUsers{}
	s := New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Ponds: f, Devices: fakeDevices{}, Telemetry: fakeTelemetry{}, Alarms: &fakeAlarms{}, Users: users}, testLogger())
	s.SetWechatExchanger(func(string) (string, error) { return "openid-revocable", nil })
	ts := httptest.NewServer(s.Routes())
	defer ts.Close()
	token := login(t, ts)
	badSwitch, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/auth/tenant", bytesReader(`{"tenant_id":99}`))
	badSwitch.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(badSwitch)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		resp.Body.Close()
		t.Fatalf("cross-tenant switch status=%d", resp.StatusCode)
	}
	resp.Body.Close()
	users.version = 1
	revoked, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/auth/tenants", nil)
	revoked.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(revoked)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked tenant token status=%d", resp.StatusCode)
	}
}

func TestTenantCapableTokenWithoutClaimsRejected(t *testing.T) {
	s := New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Ponds: &fakeRepos{}, Devices: fakeDevices{}, Telemetry: fakeTelemetry{}, Alarms: &fakeAlarms{}, Users: &revocableTenantUsers{}}, testLogger())
	claims := jwt.MapClaims{"uid": float64(1), "ver": float64(0), "exp": time.Now().Add(time.Hour).Unix()}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(platform.DeriveAppKey("test-key"))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/tenants", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing tenant claims status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTenantTokenWithoutRoleRejected(t *testing.T) {
	s := New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Ponds: &fakeRepos{}, Devices: fakeDevices{}, Telemetry: fakeTelemetry{}, Alarms: &fakeAlarms{}, Users: &revocableTenantUsers{}}, testLogger())
	claims := jwt.MapClaims{"uid": float64(1), "ver": float64(0), "exp": time.Now().Add(time.Hour).Unix(), "tenant_id": float64(7), "tenant_ver": float64(0)}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(platform.DeriveAppKey("test-key"))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/tenants", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing tenant role status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLoginWithoutTenantMembershipFailsClosed(t *testing.T) {
	s := New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Ponds: &fakeRepos{}, Devices: fakeDevices{}, Telemetry: fakeTelemetry{}, Alarms: &fakeAlarms{}, Users: noTenantUsers{}}, testLogger())
	if _, err := s.signToken(context.Background(), 1); !errors.Is(err, iolinkcontractsdomain.ErrInactiveTenant) {
		t.Fatalf("unscoped token fallback err=%v", err)
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

func TestAuthRejectsMissingExpiry(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"uid": 1, "ver": 0}).SignedString(platform.DeriveAppKey("test-key"))
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/ponds", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || strings.Contains(resp.Header.Get("Content-Type"), "text/plain") {
		t.Fatalf("missing-exp status/content type = %d/%s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}
