package core_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
)

type platformContractLimitedStore struct{ adminapi.AdminStore }

func TestPlatformOrganizationMissingCapabilitiesRejectAndRecordDiagnostics(t *testing.T) {
	f := newPlatformContractFixture(t)
	server := httptest.NewServer(adminapi.New(adminapi.Config{SecretKey: strings.Repeat("p", 32), JWT: time.Hour}, adminapi.Deps{Store: platformContractLimitedStore{f.svc}, Logger: f.logger}).Routes())
	t.Cleanup(server.Close)
	f.server = server
	ids := make(map[string]bool)
	for _, tc := range []struct {
		name    string
		request platformContractRequest
	}{
		{"registration", platformContractRequest{"POST", "/register", "/admin/v1/register", "", `{"username":"unavailable-user","password":"Platform-user-only-1234"}`, 503}},
		{"tenant_create", platformContractRequest{"POST", "/tenants", "/admin/v1/tenants", f.admin, `{"name":"unavailable-org"}`, 503}},
		{"platform_stats", platformContractRequest{"GET", "/platform/stats", "/admin/v1/platform/stats", f.admin, "", 503}},
		{"platform_users", platformContractRequest{"GET", "/platform/users", "/admin/v1/platform/users", f.admin, "", 503}},
		{"tenant_list", platformContractRequest{"GET", "/tenants", "/admin/v1/tenants", f.admin, "", 500}},
		{"tenant_status", platformContractRequest{"PUT", "/tenants/{id}/status", "/admin/v1/tenants/999999/status", f.admin, `{"active":true}`, 500}},
		{"tenant_members", platformContractRequest{"GET", "/tenants/{id}/members", "/admin/v1/tenants/999999/members", f.admin, "", 500}},
		{"tenant_member_write", platformContractRequest{"PUT", "/tenants/{id}/members/{user_id}", "/admin/v1/tenants/999999/members/999999", f.admin, `{"role":"member","active":true}`, 500}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := f.state(t)
			var response struct {
				Error string `json:"error"`
			}
			platformContractDecode(t, f.request(t, tc.request), &response)
			code := "internal_error"
			if tc.request.status == 503 {
				code = "unavailable"
			}
			if response.Error != code {
				t.Error("missing capability returned an unsafe or unstable error code")
			}
			ids[f.requestID] = false
			if f.state(t) != before {
				t.Error("missing capability changed persisted state")
			}
		})
	}
	server.Close()
	if err := f.sink.Close(); err != nil {
		t.Fatal("debug dump close failed")
	}
	raw, err := os.ReadFile(f.dumpPath)
	if err != nil {
		t.Fatal("debug dump unavailable")
	}
	for _, secret := range []string{f.admin, "Platform-contract-only-1234", "Platform-user-only-1234"} {
		if strings.Contains(string(raw), secret) {
			t.Error("debug dump contains credential material")
		}
	}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var record struct {
			Stage     string `json:"stage"`
			Level     string `json:"level"`
			RequestID string `json:"request_id"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal("invalid debug dump record")
		}
		if _, ok := ids[record.RequestID]; ok && record.Stage == "http_provider" && record.Level == "ERROR" {
			ids[record.RequestID] = true
		}
	}
	if len(ids) != 8 {
		t.Error("capability request IDs were missing or reused")
	}
	for _, recorded := range ids {
		if !recorded {
			t.Error("missing capability has no correlated diagnostic")
		}
	}
}
