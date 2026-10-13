package core_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestPlatformOrganizationPermissionsAndInvalidIdentifiers(t *testing.T) {
	f := newPlatformContractMembershipFixture(t)
	for _, tc := range []struct {
		name    string
		request platformContractRequest
	}{
		{"unauthenticated_session", platformContractRequest{"GET", "/session", "/admin/v1/session", "", "", 401}},
		{"user_admin_platform_stats", platformContractRequest{"GET", "/platform/stats", "/admin/v1/platform/stats", f.owner, "", 403}},
		{"user_platform_directory", platformContractRequest{"GET", "/platform/users", "/admin/v1/platform/users", f.owner, "", 403}},
		{"platform_business_members", platformContractRequest{"GET", "/farms/{id}/members", fmt.Sprintf("/admin/v1/farms/%d/members", f.farm), f.admin, "", 403}},
		{"viewer_tenant_members", platformContractRequest{"GET", "/tenants/{id}/members", fmt.Sprintf("/user/v1/tenants/%d/members", f.tenant), f.member, "", 403}},
		{"viewer_farm_members", platformContractRequest{"GET", "/farms/{id}/members", fmt.Sprintf("/user/v1/farms/%d/members", f.farm), f.member, "", 403}},
		{"foreign_tenant_members", platformContractRequest{"GET", "/tenants/{id}/members", fmt.Sprintf("/user/v1/tenants/%d/members", f.otherTenant), f.owner, "", 404}},
		{"foreign_tenant_write", platformContractRequest{"PUT", "/tenants/{id}/members/{user_id}", fmt.Sprintf("/user/v1/tenants/%d/members/%d", f.otherTenant, f.user), f.owner, `{"role":"viewer","active":true}`, 404}},
		{"missing_farm_members", platformContractRequest{"GET", "/farms/{id}/members", "/user/v1/farms/999999/members", f.owner, "", 404}},
		{"user_create_tenant", platformContractRequest{"POST", "/tenants", "/admin/v1/tenants", f.owner, `{"name":"unauthorized"}`, 403}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := f.state(t)
			f.request(t, tc.request)
			if f.state(t) != before {
				t.Error("denied request changed persisted state")
			}
		})
	}
	for _, id := range []string{"0", "-1"} {
		for index, tc := range []platformContractRequest{
			{"PUT", "/tenants/{id}/status", "/admin/v1/tenants/" + id + "/status", f.admin, `{"active":true}`, 400},
			{"GET", "/tenants/{id}/members", "/admin/v1/tenants/" + id + "/members", f.admin, "", 400},
			{"PUT", "/tenants/{id}/members/{user_id}", fmt.Sprintf("/admin/v1/tenants/%s/members/%d", id, f.user), f.admin, `{"role":"viewer","active":true}`, 400},
			{"PUT", "/tenants/{id}/members/{user_id}", fmt.Sprintf("/admin/v1/tenants/%d/members/%s", f.tenant, id), f.admin, `{"role":"viewer","active":true}`, 400},
			{"GET", "/farms/{id}/members", "/user/v1/farms/" + id + "/members", f.owner, "", 400},
			{"PUT", "/farms/{id}/members/{user_id}", fmt.Sprintf("/user/v1/farms/%s/members/%d", id, f.user), f.owner, `{"role":"viewer","active":true}`, 400},
			{"PUT", "/farms/{id}/members/{user_id}", fmt.Sprintf("/user/v1/farms/%d/members/%s", f.farm, id), f.owner, `{"role":"viewer","active":true}`, 400},
		} {
			t.Run(fmt.Sprintf("%s_%d_%s_%s", id, index, tc.method, tc.route), func(t *testing.T) {
				before := f.state(t)
				f.request(t, tc)
				if f.state(t) != before {
					t.Error("invalid identifier changed persisted state")
				}
			})
		}
	}
}

func TestPlatformOrganizationDatabaseFailuresAreInternalAndAtomic(t *testing.T) {
	for _, tc := range []struct {
		name, table string
		request     platformContractRequest
	}{
		{"tenant_list", "tenants", platformContractRequest{"GET", "/tenants", "/admin/v1/tenants", "", "", 500}},
		{"platform_stats", "devices", platformContractRequest{"GET", "/platform/stats", "/admin/v1/platform/stats", "", "", 500}},
		{"tenant_status", "tenants", platformContractRequest{"PUT", "/tenants/{id}/status", "", "", `{"active":false}`, 500}},
		{"tenant_status_audit", "audit_events", platformContractRequest{"PUT", "/tenants/{id}/status", "", "", `{"active":false}`, 500}},
		{"tenant_members", "tenant_memberships", platformContractRequest{"GET", "/tenants/{id}/members", "", "", "", 500}},
		{"tenant_member_write", "audit_events", platformContractRequest{"PUT", "/tenants/{id}/members/{user_id}", "", "", `{"role":"member","active":true}`, 500}},
		{"tenant_create", "audit_events", platformContractRequest{"POST", "/tenants", "/admin/v1/tenants", "", `{"name":"rollback org"}`, 500}},
		{"farm_member_write", "audit_events", platformContractRequest{"PUT", "/farms/{id}/members/{user_id}", "", "", `{"role":"viewer","active":true}`, 500}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlatformContractMembershipFixture(t)
			request := tc.request
			request.token = f.admin
			switch request.route {
			case "/tenants/{id}/status":
				request.path = fmt.Sprintf("/admin/v1/tenants/%d/status", f.tenant)
			case "/tenants/{id}/members":
				request.path = fmt.Sprintf("/admin/v1/tenants/%d/members", f.tenant)
			case "/tenants/{id}/members/{user_id}":
				request.path = fmt.Sprintf("/admin/v1/tenants/%d/members/%d", f.tenant, f.user)
			case "/farms/{id}/members/{user_id}":
				request.path = fmt.Sprintf("/user/v1/farms/%d/members/%d", f.farm, f.user)
				request.token = f.owner
			}
			before := f.state(t)
			execute(t, f.pool, "ALTER TABLE "+tc.table+" RENAME TO platform_contract_relation_unavailable")
			f.request(t, request)
			execute(t, f.pool, "ALTER TABLE platform_contract_relation_unavailable RENAME TO "+tc.table)
			if f.state(t) != before {
				t.Error("database failure left partial writes")
			}
			f.assertDatabaseDiagnostic(t)
		})
	}
}

func (f *platformContractFixture) assertDatabaseDiagnostic(t *testing.T) {
	t.Helper()
	f.server.Close()
	if err := f.sink.Close(); err != nil {
		t.Fatal("debug dump close failed")
	}
	raw, err := os.ReadFile(f.dumpPath)
	if err != nil {
		t.Fatal("debug dump unavailable")
	}
	for _, secret := range []string{f.admin, f.owner, f.member, "Platform-contract-only-1234", "Platform-user-only-1234"} {
		if strings.Contains(string(raw), secret) {
			t.Error("debug dump contains credential material")
		}
	}
	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var record struct {
			Stage     string `json:"stage"`
			Level     string `json:"level"`
			RequestID string `json:"request_id"`
			Error     string `json:"err"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal("invalid debug dump record")
		}
		if record.RequestID != f.requestID || record.Level != "ERROR" {
			continue
		}
		if record.Stage == "http_provider" || record.Stage == "http_auth" {
			if record.Error != "postgres_42P01" {
				t.Errorf("database diagnostic error class=%s", record.Error)
			}
			found = true
		}
	}
	if !found {
		t.Error("database failure has no correlated safe diagnostic")
	}
}
