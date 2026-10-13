package adminapi

import (
	"bytes"
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

	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
)

type adminProviderHTTPCase struct {
	name, method, path, body string
	store                    func(*fakeStore) AdminStore
}

func TestBaselineAdminHTTPProviderFailuresUseSafeInternalError(t *testing.T) {
	for _, tc := range []adminProviderHTTPCase{
		{"farm_list", "GET", "/farms", ``, func(f *fakeStore) AdminStore { return failfarmList{f} }},
		{"farm_create", "POST", "/farms", `{"name":"private-farm"}`, func(f *fakeStore) AdminStore { return failfarmCreate{f} }},
		{"farm_update", "PUT", "/farms/1", `{"name":"private-farm"}`, func(f *fakeStore) AdminStore { return failfarmUpdate{f} }},
		{"farm_delete", "DELETE", "/farms/1", ``, func(f *fakeStore) AdminStore { return failfarmDelete{f} }},
		{"farm_owner", "PUT", "/farms/1/owner", `{"owner_id":9}`, func(f *fakeStore) AdminStore { return failfarmOwner{f} }},
		{"farm_owner_actor", "PUT", "/farms/1/owner", `{"owner_id":9}`, func(f *fakeStore) AdminStore { return failfarmOwnerActor{f} }},
		{"user_search", "GET", "/users?query=private-query", ``, func(f *fakeStore) AdminStore { return failuserSearch{f} }},
		{"pond_list", "GET", "/ponds", ``, func(f *fakeStore) AdminStore { return failpondList{f} }},
		{"pond_farm_check", "GET", "/ponds?farm_id=1", ``, func(f *fakeStore) AdminStore { return failpondFarm{f} }},
		{"pond_alarm_enrichment", "GET", "/ponds", ``, func(f *fakeStore) AdminStore { return failalarmList{f} }},
		{"pond_create", "POST", "/ponds", `{"farm_id":1,"name":"private-pond"}`, func(f *fakeStore) AdminStore { return failpondCreate{f} }},
		{"pond_update", "PUT", "/ponds/1", `{"farm_id":1,"name":"private-pond"}`, func(f *fakeStore) AdminStore { return failpondUpdate{f} }},
		{"pond_delete", "DELETE", "/ponds/1", ``, func(f *fakeStore) AdminStore { return failpondDelete{f} }},
		{"device_register", "POST", "/devices", `{"pond_id":1,"name":"private-device"}`, func(f *fakeStore) AdminStore { return faildeviceRegister{f} }},
		{"device_list", "GET", "/devices", ``, func(f *fakeStore) AdminStore { return faildeviceList{f} }},
		{"device_move", "PUT", "/devices/private-device/pond", `{"pond_id":1}`, func(f *fakeStore) AdminStore { return faildeviceMove{f} }},
		{"device_delete", "DELETE", "/devices/private-device", ``, func(f *fakeStore) AdminStore { return faildeviceDelete{f} }},
		{"device_restore", "POST", "/devices/private-device/restore", ``, func(f *fakeStore) AdminStore { return faildeviceRestore{f} }},
		{"rule_list", "GET", "/alarm-rules", ``, func(f *fakeStore) AdminStore { return failruleList{f} }},
		{"rule_create", "POST", "/alarm-rules", `{"pond_id":1,"metric":"ph","max_value":8,"level":"warning"}`, func(f *fakeStore) AdminStore { return failruleCreate{f} }},
		{"rule_update", "PUT", "/alarm-rules/1", `{"pond_id":1,"metric":"ph","max_value":8,"level":"warning"}`, func(f *fakeStore) AdminStore { return failruleUpdate{f} }},
		{"rule_delete", "DELETE", "/alarm-rules/1", ``, func(f *fakeStore) AdminStore { return failruleDelete{f} }},
		{"alarm_list", "GET", "/alarms", ``, func(f *fakeStore) AdminStore { return failalarmList{f} }},
		{"alarm_confirm", "POST", "/alarms/1/confirm", ``, func(f *fakeStore) AdminStore { return failalarmConfirm{f} }},
		{"alarm_confirm_actor", "POST", "/alarms/1/confirm", ``, func(f *fakeStore) AdminStore { return failalarmConfirmActor{f} }},
		{"alarm_batch", "POST", "/alarms/batch-confirm", `{"ids":[1]}`, func(f *fakeStore) AdminStore { return failalarmBatch{f} }},
		{"alarm_batch_actor", "POST", "/alarms/batch-confirm", `{"ids":[1]}`, func(f *fakeStore) AdminStore { return failalarmBatchActor{f} }},
		{"stats", "GET", "/stats", ``, func(f *fakeStore) AdminStore { return failstats{f} }},
		{"password", "POST", "/password", `{"old_password":"private-old-password","new_password":"private-new-password"}`, func(f *fakeStore) AdminStore { return failpassword{f} }},
		{"login_version", "POST", "/login", `{"username":"admin","password":"admin123"}`, func(f *fakeStore) AdminStore { return failloginVersion{f} }},
		{"login_tenant", "POST", "/login", `{"username":"admin","password":"admin123"}`, func(f *fakeStore) AdminStore { return failloginTenant{f} }},
		{"login_membership", "POST", "/login", `{"username":"admin","password":"admin123"}`, func(f *fakeStore) AdminStore { return failloginMembership{f} }},
		{"login_role", "POST", "/login", `{"username":"admin","password":"admin123"}`, func(f *fakeStore) AdminStore { return failloginRole{f} }},
		{"member_list", "GET", "/farms/1/members", ``, func(f *fakeStore) AdminStore { return failfarmMemberList{f} }},
		{"member_set", "PUT", "/farms/1/members/9", `{"role":"member","active":true}`, func(f *fakeStore) AdminStore { return failfarmMemberSet{f} }},
		{"missing_member_list", "GET", "/farms/1/members", ``, func(f *fakeStore) AdminStore { return f }},
		{"missing_member_set", "PUT", "/farms/1/members/9", `{"role":"member","active":true}`, func(f *fakeStore) AdminStore { return f }},
	} {
		t.Run(tc.name, func(t *testing.T) { exerciseAdminProviderFailure(t, tc) })
	}
}

func exerciseAdminProviderFailure(t *testing.T, tc adminProviderHTTPCase) {
	t.Helper()
	// Given: real authentication, a safe dump and one dependency that fails.
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
	hash := platform.HashPassword("admin123")
	f := &fakeStore{admin: &domain.User{ID: 9, Username: strptr("admin"), PasswordHash: &hash, Authority: "USER"}, tenantRole: "owner"}
	if strings.HasPrefix(tc.name, "alarm_") && strings.HasSuffix(tc.name, "_actor") {
		f.tenantRole = "support"
	}
	s := New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Logger: logger, Store: tc.store(f), Policy: policy})
	ts := httptest.NewServer(s.Routes())
	t.Cleanup(ts.Close)
	token := ""
	if tc.path != "/login" {
		token = adminLogin(t, ts)
	}
	// When: an authenticated TCP request reaches the failed provider.
	req, err := http.NewRequest(tc.method, ts.URL+"/admin/v1"+tc.path, strings.NewReader(tc.body))
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
	// Then: only the stable contract leaves HTTP and one correlated safe diagnostic is kept.
	if resp.StatusCode != 500 || string(response) != `{"error":"internal_error"}` {
		t.Fatalf("unexpected provider failure response: status=%d", resp.StatusCode)
	}
	logs, err := os.ReadFile(filepath.Join(dir, "test", "iolinkd.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(logs, []byte("private-")) || bytes.Contains(logs, []byte("admin123")) || token != "" && bytes.Contains(logs, []byte(token)) {
		t.Fatal("provider or request secret leaked into dump")
	}
	found := 0
	for _, line := range bytes.Split(bytes.TrimSpace(logs), []byte{'\n'}) {
		var record struct {
			Stage     string `json:"stage"`
			Level     string `json:"level"`
			RequestID string `json:"request_id"`
			Error     string `json:"err"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		if record.Stage != "http_provider" || record.RequestID != resp.Header.Get("X-Request-ID") {
			continue
		}
		found++
		wantError := "*errors.errorString"
		if strings.HasPrefix(tc.name, "missing_") {
			wantError = ""
		}
		if record.Level != "ERROR" || record.RequestID == "" || record.Error != wantError {
			t.Fatalf("invalid diagnostic=%+v", record)
		}
		t.Logf("status=500 response=internal_error correlated_diagnostic=%s", line)
	}
	if found != 1 {
		t.Fatalf("provider diagnostic count=%d want=1", found)
	}
}
