package adminapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
)

// ---- fake store ----

type fakeStore struct {
	admin   *domain.User
	devices map[string]domain.Device
	rules   map[int64]domain.AlarmRule
	ruleSeq int64
	stats   domain.Stats
}

func (f *fakeStore) FindAdminByLogin(_ context.Context, login string) (*domain.User, error) {
	if f.admin != nil && *f.admin.Username == login {
		return f.admin, nil
	}
	return nil, domain.ErrUnknownMetric // any error → 401
}
func (f *fakeStore) ListFarms(_ context.Context) ([]domain.Farm, error) {
	return nil, nil
}
func (f *fakeStore) CreateFarm(_ context.Context, _ int64, name, _ string) (domain.Farm, error) {
	return domain.Farm{ID: 1, Name: name}, nil
}
func (f *fakeStore) UpdateFarm(_ context.Context, _ int64, _, _ string) error { return nil }
func (f *fakeStore) DeleteFarm(_ context.Context, _ int64) error              { return nil }
func (f *fakeStore) ListPonds(_ context.Context) ([]domain.Pond, error)       { return nil, nil }
func (f *fakeStore) CreatePond(_ context.Context, _ int64, name string, _ float64) (domain.Pond, error) {
	return domain.Pond{ID: 2, Name: name}, nil
}
func (f *fakeStore) UpdatePond(_ context.Context, _ int64, _ string, _ float64) error { return nil }
func (f *fakeStore) DeletePond(_ context.Context, _ int64) error                      { return nil }

func (f *fakeStore) RegisterDevice(_ context.Context, pondID int64, model string) (domain.Device, string, error) {
	no := "dev-abc12345"
	secret := strings.Repeat("ab", 16) // 32 hex chars, like the real generator
	f.devices[no] = domain.Device{ID: 1, PondID: pondID, DeviceNo: no, Model: model, Status: domain.DeviceOffline}
	return f.devices[no], secret, nil
}
func (f *fakeStore) ListDevices(_ context.Context) ([]domain.Device, error) {
	out := make([]domain.Device, 0, len(f.devices))
	for _, d := range f.devices {
		out = append(out, d)
	}
	return out, nil
}
func (f *fakeStore) DeleteDevice(_ context.Context, no string) error {
	delete(f.devices, no)
	return nil
}

func (f *fakeStore) ListRules(_ context.Context) ([]domain.AlarmRule, error) {
	out := make([]domain.AlarmRule, 0, len(f.rules))
	for _, r := range f.rules {
		out = append(out, r)
	}
	return out, nil
}
func (f *fakeStore) CreateRule(_ context.Context, r domain.AlarmRule) (domain.AlarmRule, error) {
	f.ruleSeq++
	r.ID = f.ruleSeq
	f.rules[r.ID] = r
	return r, nil
}
func (f *fakeStore) UpdateRule(_ context.Context, r domain.AlarmRule) error {
	f.rules[r.ID] = r
	return nil
}
func (f *fakeStore) DeleteRule(_ context.Context, id int64) error {
	delete(f.rules, id)
	return nil
}

func (f *fakeStore) FindAdminByID(_ context.Context, id int64) (*domain.User, error) {
	if f.admin != nil && f.admin.ID == id {
		return f.admin, nil
	}
	return nil, domain.ErrUnknownMetric
}
func (f *fakeStore) ChangeAdminPassword(_ context.Context, id int64, oldPW, newPW string) error {
	if f.admin == nil || f.admin.ID != id {
		return domain.ErrUnknownMetric
	}
	if f.admin.PasswordHash == nil || !platform.CheckPassword(*f.admin.PasswordHash, oldPW) {
		return domain.ErrOldPasswordMismatch
	}
	h := platform.HashPassword(newPW)
	f.admin.PasswordHash = &h
	return nil
}

func (f *fakeStore) ListAllAlarms(_ context.Context, _ int) ([]domain.Alarm, error) {
	return nil, nil
}
func (f *fakeStore) ConfirmAlarm(_ context.Context, _ int64) error            { return nil }
func (f *fakeStore) BatchConfirm(_ context.Context, _ []int64) (int64, error) { return 0, nil }
func (f *fakeStore) Stats(_ context.Context) (domain.Stats, error)            { return f.stats, nil }

// ---- helpers ----

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	hash := platform.HashPassword("admin123")
	s := New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Store: &fakeStore{
		admin:   &domain.User{ID: 9, Username: strptr("admin"), PasswordHash: &hash, Authority: "ADMIN"},
		devices: map[string]domain.Device{},
		rules:   map[int64]domain.AlarmRule{},
		stats:   domain.Stats{DevicesTotal: 3, Online: 2, Offline: 1, OpenAlarms: 4},
	}})
	return httptest.NewServer(s.Routes())
}

func strptr(s string) *string { return &s }

func adminLogin(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	resp, err := http.Post(ts.URL+"/admin/v1/login", "application/json",
		strings.NewReader(`{"username":"admin","password":"admin123"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.Token == "" {
		t.Fatal("no admin token")
	}
	return out.Token
}

func authGet(t *testing.T, ts *httptest.Server, token, path string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("GET", ts.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// ---- tests ----

func TestAdminLoginAndAuth(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	token := adminLogin(t, ts)

	resp := authGet(t, ts, token, "/admin/v1/stats")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("stats status %d", resp.StatusCode)
	}
	var st domain.Stats
	_ = json.NewDecoder(resp.Body).Decode(&st)
	if st.DevicesTotal != 3 || st.OpenAlarms != 4 {
		t.Fatalf("bad stats: %+v", st)
	}
}

func TestAdminRejectsAppJWTAndBadPassword(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	resp, err := http.Post(ts.URL+"/admin/v1/login", "application/json",
		strings.NewReader(`{"username":"admin","password":"wrong"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 for wrong password, got %d", resp.StatusCode)
	}
	if r := authGet(t, ts, "not-a-token", "/admin/v1/stats"); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 for garbage token, got %d", r.StatusCode)
	}
}

func TestRegisterDeviceSecretOnce(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	token := adminLogin(t, ts)

	req, _ := http.NewRequest("POST", ts.URL+"/admin/v1/devices",
		strings.NewReader(`{"pond_id":1,"model":"ESP32"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		DeviceNo string `json:"device_no"`
		Secret   string `json:"secret"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.DeviceNo == "" || out.Secret == "" {
		t.Fatalf("device_no/secret required: %+v", out)
	}
	if len(out.Secret) < 24 {
		t.Fatalf("secret too weak: %q", out.Secret)
	}
}

func TestRuleValidation(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	token := adminLogin(t, ts)

	// 未知 metric → 400
	req, _ := http.NewRequest("POST", ts.URL+"/admin/v1/alarm-rules",
		strings.NewReader(`{"pond_id":1,"metric":"bogus","min_value":1,"level":"critical"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("unknown metric should 400, got %d", resp.StatusCode)
	}

	// 无 min/max → 400
	req2, _ := http.NewRequest("POST", ts.URL+"/admin/v1/alarm-rules",
		strings.NewReader(`{"pond_id":1,"metric":"ph","level":"warning"}`))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req2)
	resp2.Body.Close()
	if resp2.StatusCode != 400 {
		t.Fatalf("no-threshold rule should 400, got %d", resp2.StatusCode)
	}

	// 合法规则 → 200
	req3, _ := http.NewRequest("POST", ts.URL+"/admin/v1/alarm-rules",
		strings.NewReader(`{"pond_id":1,"metric":"dissolved_oxygen","min_value":4,"level":"critical"}`))
	req3.Header.Set("Authorization", "Bearer "+token)
	req3.Header.Set("Content-Type", "application/json")
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Fatalf("valid rule should 200, got %d", resp3.StatusCode)
	}
}

func TestChangeAdminPassword(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	token := adminLogin(t, ts)

	// 错误旧密码 → 401
	req, _ := http.NewRequest("POST", ts.URL+"/admin/v1/password",
		strings.NewReader(`{"old_password":"wrong","new_password":"newpass123"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong old password should 401, got %d", resp.StatusCode)
	}

	// 正确旧密码 → 204
	req2, _ := http.NewRequest("POST", ts.URL+"/admin/v1/password",
		strings.NewReader(`{"old_password":"admin123","new_password":"newpass123"}`))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("valid change should 204, got %d", resp2.StatusCode)
	}
}
