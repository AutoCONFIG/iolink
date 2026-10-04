package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"github.com/golang-jwt/jwt/v5"
)

// ---- fake store ----

type fakeStore struct {
	admin            *domain.User
	tenantRole       string
	defaultTenantErr error
	devices          map[string]domain.Device
	rules            map[int64]domain.AlarmRule
	ruleSeq          int64
	stats            domain.Stats
	licenseStatus    license.Status
	licenseErr       error
	importErr        error
	imports          []license.Envelope
	rejections       [][]byte
	registerErr      error
	restoreErr       error
}

func (f *fakeStore) FindAdminByLogin(_ context.Context, login string) (*domain.User, error) {
	if f.admin != nil && *f.admin.Username == login {
		return f.admin, nil
	}
	return nil, domain.ErrUnknownMetric // any error → 401
}

func (f *fakeStore) AdminTokenVersion(_ context.Context, _ int64) (int, error)       { return 0, nil }
func (f *fakeStore) UpgradeAdminPassword(_ context.Context, _ int64, _ string) error { return nil }
func (f *fakeStore) ListFarms(_ context.Context) ([]domain.Farm, error) {
	return nil, nil
}

func (f *fakeStore) CreateFarm(_ context.Context, _ *int64, name, _ string) (domain.Farm, error) {
	return domain.Farm{ID: 1, Name: name}, nil
}
func (f *fakeStore) SetFarmOwner(_ context.Context, _ int64, _ *int64) error { return nil }
func (f *fakeStore) SearchUsers(_ context.Context, _ string, _, _ int) ([]domain.User, error) {
	return nil, nil
}
func (f *fakeStore) UpdateFarm(_ context.Context, _ int64, _, _ string) error { return nil }
func (f *fakeStore) DeleteFarm(_ context.Context, _ int64) error              { return nil }
func (f *fakeStore) ListPonds(_ context.Context) ([]domain.Pond, error)       { return nil, nil }
func (f *fakeStore) CreatePond(_ context.Context, _ int64, name string, _ float64) (domain.Pond, error) {
	return domain.Pond{ID: 2, Name: name}, nil
}
func (f *fakeStore) UpdatePond(_ context.Context, _ int64, _ string, _ float64) error { return nil }
func (f *fakeStore) DeletePond(_ context.Context, _ int64) error                      { return nil }

func (f *fakeStore) RegisterDevice(_ context.Context, pondID int64, _ string, model string, _ int) (domain.Device, string, error) {
	if f.registerErr != nil {
		return domain.Device{}, "", f.registerErr
	}
	no := "dev-abc12345"
	secret := strings.Repeat("ab", 32) // 64 hex chars, like the real generator
	f.devices[no] = domain.Device{ID: 1, PondID: pondID, DeviceNo: no, Model: model, Status: domain.DeviceOffline}
	return f.devices[no], secret, nil
}

func (f *fakeStore) ListDevices(_ context.Context, _ bool, _ int64, _, _ int) ([]domain.Device, error) {
	out := make([]domain.Device, 0, len(f.devices))
	for _, d := range f.devices {
		out = append(out, d)
	}
	return out, nil
}

func (f *fakeStore) GetDevice(_ context.Context, no string) (domain.Device, error) {
	if d, ok := f.devices[no]; ok {
		return d, nil
	}
	return domain.Device{}, domain.ErrNotFound
}
func (f *fakeStore) MoveDevice(_ context.Context, _ string, _ int64) error { return nil }
func (f *fakeStore) DeleteDevice(_ context.Context, no string) error {
	delete(f.devices, no)
	return nil
}
func (f *fakeStore) RestoreDevice(_ context.Context, no string) error {
	if f.restoreErr != nil {
		return f.restoreErr
	}
	if _, ok := f.devices[no]; !ok {
		return domain.ErrNotFound
	}
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
func (f *fakeStore) ConfirmAlarmByActor(_ context.Context, _, _ int64) error  { return nil }
func (f *fakeStore) BatchConfirm(_ context.Context, _ []int64) (int64, error) { return 0, nil }
func (f *fakeStore) BatchConfirmByActor(_ context.Context, _ []int64, _ int64) (int64, error) {
	return 0, nil
}
func (f *fakeStore) Stats(_ context.Context) (domain.Stats, error) { return f.stats, nil }
func (f *fakeStore) LicenseStatus(context.Context) (license.Status, error) {
	return f.licenseStatus, f.licenseErr
}
func (f *fakeStore) ImportLicenseRaw(_ context.Context, _ []byte, envelope license.Envelope, _ int64) error {
	if f.importErr != nil {
		return f.importErr
	}
	f.imports = append(f.imports, envelope)
	return nil
}
func (f *fakeStore) RecordLicenseRejection(_ context.Context, raw []byte, _ int64, _ string) error {
	f.rejections = append(f.rejections, append([]byte(nil), raw...))
	return nil
}
func (f *fakeStore) RecordLicenseRejectionDigest(context.Context, string, int64, string) error {
	return nil
}

func (f *fakeStore) DefaultTenantForUser(context.Context, int64) (int64, error) {
	if f.defaultTenantErr != nil {
		return 0, f.defaultTenantErr
	}
	return 7, nil
}

func (f *fakeStore) TenantMembershipVersion(context.Context, int64, int64) (int64, error) {
	return 0, nil
}

func (f *fakeStore) TenantRole(context.Context, int64, int64) (string, error) {
	if f.tenantRole != "" {
		return f.tenantRole, nil
	}
	return "admin", nil
}

func (f *fakeStore) ListTenants(context.Context) ([]domain.Tenant, error) {
	return []domain.Tenant{{ID: 7, Name: "tenant", Active: true}}, nil
}
func (f *fakeStore) SetTenantActive(context.Context, int64, bool, int64) error { return nil }
func (f *fakeStore) ListTenantMembers(context.Context, int64) ([]domain.TenantMembership, error) {
	return []domain.TenantMembership{{TenantID: 7, UserID: 9, Role: "admin", Active: true}}, nil
}

func (f *fakeStore) SetTenantMember(context.Context, int64, int64, string, bool, *time.Time, int64) error {
	return nil
}

// ---- helpers ----

func newTestServer(t *testing.T) *httptest.Server {
	return newTestServerWithRole(t, "ADMIN", "admin")
}

func newTestServerWithRole(t *testing.T, authority, tenantRole string) *httptest.Server {
	t.Helper()
	hash := platform.HashPassword("admin123")
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	s := New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Store: &fakeStore{
		admin:      &domain.User{ID: 9, Username: strptr("admin"), PasswordHash: &hash, Authority: authority},
		tenantRole: tenantRole,
		devices:    map[string]domain.Device{},
		rules:      map[int64]domain.AlarmRule{},
		stats:      domain.Stats{DevicesTotal: 3, Online: 2, Offline: 1, OpenAlarms: 4},
	}, Policy: policy})
	return httptest.NewServer(s.Routes())
}

func strptr(s string) *string { return &s }

func adminLogin(t *testing.T, ts *httptest.Server) string {
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

func TestPlatformAdminCanReadAndImportLicense(t *testing.T) {
	hash := platform.HashPassword("admin123")
	store := &fakeStore{
		admin:   &domain.User{ID: 9, Username: strptr("admin"), PasswordHash: &hash, Authority: "ADMIN"},
		devices: map[string]domain.Device{}, rules: map[int64]domain.AlarmRule{},
		licenseStatus: license.Status{State: license.StatePermanent, Features: []string{}},
	}
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Store: store, Policy: policy}).Routes())
	defer ts.Close()
	token := adminLogin(t, ts)
	resp := authGet(t, ts, token, "/admin/v1/license")
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("license status=%d", resp.StatusCode)
	}
	resp.Body.Close()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/admin/v1/license", strings.NewReader(`{"payload_b64":"YQ==","signature_b64":"Yg=="}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent || len(store.imports) != 1 {
		t.Fatalf("license import status=%d imports=%d", resp.StatusCode, len(store.imports))
	}
}

func TestTenantAdminCannotReadOrImportLicense(t *testing.T) {
	ts := newTestServerWithRole(t, "USER", "admin")
	defer ts.Close()
	token := adminLogin(t, ts)
	resp := authGet(t, ts, token, "/admin/v1/license")
	if resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Fatalf("license GET status=%d", resp.StatusCode)
	}
	resp.Body.Close()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/admin/v1/license", strings.NewReader(`{"payload_b64":"YQ==","signature_b64":"Yg=="}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("license POST status=%d", resp.StatusCode)
	}
}

func TestLicenseImportMapsClockErrorAndRejectsDuplicateJSON(t *testing.T) {
	hash := platform.HashPassword("admin123")
	store := &fakeStore{admin: &domain.User{ID: 9, Username: strptr("admin"), PasswordHash: &hash, Authority: "ADMIN"}, devices: map[string]domain.Device{}, rules: map[int64]domain.AlarmRule{}, importErr: license.ErrClockError}
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Store: store, Policy: policy}).Routes())
	defer ts.Close()
	token := adminLogin(t, ts)
	for body, want := range map[string]int{
		`{"payload_b64":"YQ==","signature_b64":"Yg==","payload_b64":"Yw=="}`: http.StatusBadRequest,
		`{"payload_b64":"YQ==","signature_b64":"Yg=="}`:                      http.StatusConflict,
	} {
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/admin/v1/license", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("body=%s status=%d want=%d", body, resp.StatusCode, want)
		}
	}
	if len(store.rejections) != 1 || string(store.rejections[0]) != `{"payload_b64":"YQ==","signature_b64":"Yg==","payload_b64":"Yw=="}` {
		t.Fatalf("malformed import rejection audit=%q", store.rejections)
	}
}

func TestPlatformAdminCannotGrantTenantMembership(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	token := adminLogin(t, ts)
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/admin/v1/tenants/7/members/9", strings.NewReader(`{"role":"owner","active":true}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("platform membership mutation status=%d, want 403", resp.StatusCode)
	}
}

func TestPlatformAdminCanLoginAfterTenantMembershipExpires(t *testing.T) {
	hash := platform.HashPassword("admin123")
	s := New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Store: &fakeStore{
		admin:            &domain.User{ID: 9, Username: strptr("admin"), PasswordHash: &hash, Authority: "ADMIN"},
		defaultTenantErr: domain.ErrInactiveTenant,
		devices:          map[string]domain.Device{},
		rules:            map[int64]domain.AlarmRule{},
	}})
	ts := httptest.NewServer(s.Routes())
	defer ts.Close()
	token := adminLogin(t, ts)
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/admin/v1/tenants/7/status", strings.NewReader(`{"active":true}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("platform restore status=%d, want 204", resp.StatusCode)
	}
}

func TestSupportCanReadTenantBusinessRoutes(t *testing.T) {
	ts := newTestServerWithRole(t, "USER", "support")
	defer ts.Close()
	token := adminLogin(t, ts)
	resp := authGet(t, ts, token, "/admin/v1/farms")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("support farms status=%d, want 200", resp.StatusCode)
	}
}

func TestTenantRoleActionMatrix(t *testing.T) {
	for _, tc := range []struct {
		role             string
		confirmStatus    int
		batchStatus      int
		memberStatus     int
		memberListStatus int
	}{
		{"owner", http.StatusNoContent, http.StatusOK, http.StatusNoContent, http.StatusOK},
		{"admin", http.StatusNoContent, http.StatusOK, http.StatusNoContent, http.StatusOK},
		{"member", http.StatusNoContent, http.StatusOK, http.StatusForbidden, http.StatusForbidden},
		{"viewer", http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden},
		{"support", http.StatusNoContent, http.StatusOK, http.StatusForbidden, http.StatusForbidden},
	} {
		t.Run(tc.role, func(t *testing.T) {
			ts := newTestServerWithRole(t, "USER", tc.role)
			defer ts.Close()
			token := adminLogin(t, ts)
			for _, path := range []string{"/admin/v1/farms", "/admin/v1/ponds", "/admin/v1/devices", "/admin/v1/alarms"} {
				resp := authGet(t, ts, token, path)
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("%s GET status=%d", path, resp.StatusCode)
				}
			}
			resp := authGet(t, ts, token, "/admin/v1/tenants/7/members")
			resp.Body.Close()
			if resp.StatusCode != tc.memberListStatus {
				t.Fatalf("tenant members GET status=%d want=%d", resp.StatusCode, tc.memberListStatus)
			}
			for _, action := range []struct {
				method, path, body string
				want               int
			}{
				{http.MethodPost, "/admin/v1/alarms/1/confirm", "", tc.confirmStatus},
				{http.MethodPost, "/admin/v1/alarms/batch-confirm", `{"ids":[1]}`, tc.batchStatus},
				{http.MethodPut, "/admin/v1/tenants/7/members/9", `{"role":"viewer","active":true}`, tc.memberStatus},
			} {
				req, _ := http.NewRequest(action.method, ts.URL+action.path, strings.NewReader(action.body))
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
				if resp.StatusCode != action.want {
					t.Fatalf("%s %s status=%d want=%d", action.method, action.path, resp.StatusCode, action.want)
				}
			}
		})
	}
}

func TestAdminAuthRejectsMissingExpiry(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"aid": 9, "ver": 0}).SignedString(platform.DeriveAdminKey("test-key"))
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", ts.URL+"/admin/v1/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing-exp status = %d", resp.StatusCode)
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
	if len(out.Secret) != 64 {
		t.Fatalf("secret too weak: %q", out.Secret)
	}
}

func TestRegisterDeviceMapsLicenseErrors(t *testing.T) {
	hash := platform.HashPassword("admin123")
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		err  error
		code int
		body string
	}{
		{name: "required", err: license.ErrRequired, code: http.StatusForbidden, body: `{"error":"license_required"}`},
		{name: "quota", err: license.ErrQuotaExceeded, code: http.StatusForbidden, body: `{"error":"device_quota_exceeded"}`},
		{name: "clock", err: license.ErrClockError, code: http.StatusConflict, body: `{"error":"license_clock_error"}`},
		{name: "internal", err: errors.New("database password=hidden"), code: http.StatusInternalServerError, body: `{"error":"internal_error"}`},
		{name: "unavailable", err: license.ErrUnavailable, code: http.StatusServiceUnavailable, body: `{"error":"license_unavailable"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{admin: &domain.User{ID: 9, Username: strptr("admin"), PasswordHash: &hash, Authority: "ADMIN"}, devices: map[string]domain.Device{}, rules: map[int64]domain.AlarmRule{}, registerErr: tt.err}
			ts := httptest.NewServer(New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Store: store, Policy: policy}).Routes())
			defer ts.Close()
			token := adminLogin(t, ts)
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/admin/v1/devices", strings.NewReader(`{"pond_id":1,"model":"ESP32"}`))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			var got map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tt.code || got["error"] != strings.TrimSuffix(strings.TrimPrefix(tt.body, `{"error":"`), `"}`) {
				t.Fatalf("status=%d body=%v want status=%d body=%s", resp.StatusCode, got, tt.code, tt.body)
			}
		})
	}
}

func TestRestoreDeviceMapsLicenseErrors(t *testing.T) {
	hash := platform.HashPassword("admin123")
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		err  error
		code int
		body string
	}{
		{name: "success", code: http.StatusNoContent},
		{name: "not found", err: domain.ErrNotFound, code: http.StatusNotFound, body: `{"error":"not_found"}`},
		{name: "forbidden", err: domain.ErrForbidden, code: http.StatusForbidden, body: `{"error":"forbidden"}`},
		{name: "required", err: license.ErrRequired, code: http.StatusForbidden, body: `{"error":"license_required"}`},
		{name: "quota", err: license.ErrQuotaExceeded, code: http.StatusForbidden, body: `{"error":"device_quota_exceeded"}`},
		{name: "clock", err: license.ErrClockError, code: http.StatusConflict, body: `{"error":"license_clock_error"}`},
		{name: "conflict", err: domain.ErrConflict, code: http.StatusConflict, body: `{"error":"conflict"}`},
		{name: "unavailable", err: license.ErrUnavailable, code: http.StatusServiceUnavailable, body: `{"error":"license_unavailable"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{admin: &domain.User{ID: 9, Username: strptr("admin"), PasswordHash: &hash, Authority: "ADMIN"}, devices: map[string]domain.Device{"dev-restore": {DeviceNo: "dev-restore"}}, rules: map[int64]domain.AlarmRule{}, restoreErr: tt.err}
			ts := httptest.NewServer(New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Store: store, Policy: policy}).Routes())
			defer ts.Close()
			token := adminLogin(t, ts)
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/admin/v1/devices/dev-restore/restore", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.code {
				t.Fatalf("status=%d want=%d", resp.StatusCode, tt.code)
			}
			if tt.body != "" {
				var got map[string]string
				if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
					t.Fatal(err)
				}
				if got["error"] != strings.TrimSuffix(strings.TrimPrefix(tt.body, `{"error":"`), `"}`) {
					t.Fatalf("body=%v want=%s", got, tt.body)
				}
			}
		})
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

func TestRejectInvertedRuleBounds(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	token := adminLogin(t, ts)
	for _, method := range []string{"POST", "PUT"} {
		path := "/admin/v1/alarm-rules"
		if method == "PUT" {
			path += "/1"
		}
		req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(`{"pond_id":1,"metric":"ph","min_value":8,"max_value":4,"level":"warning"}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 400 {
			t.Fatal("inverted range accepted", method, response.StatusCode)
		}
	}
}
