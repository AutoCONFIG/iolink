package core_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestPlatformOrganizationContractSuccessAndNoAutomaticProvisioning(t *testing.T) {
	// Given: initialization created only a platform administrator.
	f := newPlatformContractFixture(t)
	var session struct {
		PlatformAdmin bool  `json:"platform_admin"`
		TenantID      int64 `json:"tenant_id"`
	}
	t.Run("GET_session", func(t *testing.T) {
		platformContractDecode(t, f.request(t, platformContractRequest{"GET", "/session", "/admin/v1/session", f.admin, "", 200}), &session)
	})
	if !session.PlatformAdmin || session.TenantID != 0 {
		t.Error("platform session acquired tenant context")
	}
	t.Run("GET_tenants_empty", func(t *testing.T) {
		var tenants []any
		platformContractDecode(t, f.request(t, platformContractRequest{"GET", "/tenants", "/admin/v1/tenants", f.admin, "", 200}), &tenants)
		if len(tenants) != 0 {
			t.Error("internal or automatic tenant exposed")
		}
	})
	var user int64
	t.Run("POST_register", func(t *testing.T) { user = f.register(t, "platform-customer") })
	if count(t, f.pool, "SELECT count(*) FROM tenant_memberships") != 0 || count(t, f.pool, "SELECT count(*) FROM tenants WHERE name<>'__iolink_system__'") != 0 {
		t.Error("registration automatically provisioned tenant membership")
	}
	t.Run("GET_platform_users", func(t *testing.T) {
		var users []struct {
			ID int64 `json:"id"`
		}
		platformContractDecode(t, f.request(t, platformContractRequest{"GET", "/platform/users", "/admin/v1/platform/users", f.admin, "", 200}), &users)
		if len(users) != 1 || users[0].ID != user {
			t.Error("platform directory did not return registered user")
		}
	})
	t.Run("POST_tenants", func(t *testing.T) { f.tenant = f.createTenant(t, "platform-contract-org") })
	t.Run("PUT_tenant_member", func(t *testing.T) { f.assign(t, user, "owner") })
	t.Run("GET_tenant_members", func(t *testing.T) {
		var members []struct {
			UserID int64 `json:"user_id"`
		}
		platformContractDecode(t, f.request(t, platformContractRequest{"GET", "/tenants/{id}/members", fmt.Sprintf("/admin/v1/tenants/%d/members", f.tenant), f.admin, "", 200}), &members)
		if len(members) != 1 || members[0].UserID != user {
			t.Error("tenant member update did not persist")
		}
	})
	t.Run("GET_tenants", func(t *testing.T) {
		f.request(t, platformContractRequest{"GET", "/tenants", "/admin/v1/tenants", f.admin, "", 200})
	})
	t.Run("GET_platform_stats", func(t *testing.T) {
		f.request(t, platformContractRequest{"GET", "/platform/stats", "/admin/v1/platform/stats", f.admin, "", 200})
	})
	t.Run("PUT_tenant_status", func(t *testing.T) {
		f.request(t, platformContractRequest{"PUT", "/tenants/{id}/status", fmt.Sprintf("/admin/v1/tenants/%d/status", f.tenant), f.admin, `{"active":false}`, 204})
	})
	if count(t, f.pool, fmt.Sprintf("SELECT count(*) FROM tenants WHERE id=%d AND active=false", f.tenant)) != 1 {
		t.Error("tenant status did not persist")
	}
	// Then: platform login and explicit tenant configuration never grant business membership to ADMIN.
	if count(t, f.pool, "SELECT count(*) FROM tenant_memberships m JOIN users u ON u.id=m.user_id WHERE u.authority='ADMIN'") != 0 {
		t.Error("platform account acquired ordinary tenant membership")
	}
}

func TestPlatformFarmMemberContractAndImmediateRevocation(t *testing.T) {
	// Given: tenant owner and viewer, with no viewer farm access initially.
	f := newPlatformContractMembershipFixture(t)
	memberPath := fmt.Sprintf("/user/v1/farms/%d/members/%d", f.farm, f.user)
	t.Run("PUT_farm_member", func(t *testing.T) {
		f.request(t, platformContractRequest{"PUT", "/farms/{id}/members/{user_id}", memberPath, f.owner, `{"role":"viewer","active":true}`, 204})
	})
	t.Run("GET_farm_members", func(t *testing.T) {
		f.request(t, platformContractRequest{"GET", "/farms/{id}/members", fmt.Sprintf("/user/v1/farms/%d/members", f.farm), f.owner, "", 200})
	})
	visible := func(t *testing.T, want int) {
		t.Helper()
		var farms []struct {
			ID int64 `json:"id"`
		}
		platformContractDecode(t, f.request(t, platformContractRequest{"GET", "/farms", "/user/v1/farms", f.member, "", 200}), &farms)
		if len(farms) != want {
			t.Errorf("visible farm count=%d want=%d", len(farms), want)
		}
	}
	t.Run("viewer_visible", func(t *testing.T) { visible(t, 1) })
	t.Run("revoke_farm_member", func(t *testing.T) {
		f.request(t, platformContractRequest{"PUT", "/farms/{id}/members/{user_id}", memberPath, f.owner, `{"role":"viewer","active":false,"expires_at":null}`, 204})
	})
	// Then: the same token cannot see the revoked farm on its next request.
	t.Run("viewer_revoked", func(t *testing.T) { visible(t, 0) })
	t.Run("revoke_tenant_member", func(t *testing.T) {
		f.request(t, platformContractRequest{"PUT", "/tenants/{id}/members/{user_id}", fmt.Sprintf("/admin/v1/tenants/%d/members/%d", f.tenant, f.user), f.admin, `{"role":"viewer","active":false}`, 204})
	})
	t.Run("tenant_token_revoked", func(t *testing.T) {
		f.request(t, platformContractRequest{"GET", "/session", "/user/v1/session", f.member, "", 401})
	})
}

func TestPlatformOrganizationContractInvalidRequestsLeaveNoWrites(t *testing.T) {
	f := newPlatformContractMembershipFixture(t)
	tenantPath := fmt.Sprintf("/admin/v1/tenants/%d", f.tenant)
	tenantMember := fmt.Sprintf("%s/members/%d", tenantPath, f.user)
	farmMember := fmt.Sprintf("/user/v1/farms/%d/members/%d", f.farm, f.user)
	for _, tc := range []struct {
		name    string
		request platformContractRequest
	}{
		{"register_unknown", platformContractRequest{"POST", "/register", "/admin/v1/register", "", `{"username":"invalid-user","password":"Platform-user-only-1234","unknown":1}`, 400}},
		{"register_trailing", platformContractRequest{"POST", "/register", "/admin/v1/register", "", `{"username":"invalid-user","password":"Platform-user-only-1234"} {}`, 400}},
		{"register_null", platformContractRequest{"POST", "/register", "/admin/v1/register", "", `{"username":null,"password":"Platform-user-only-1234"}`, 400}},
		{"tenant_unknown", platformContractRequest{"POST", "/tenants", "/admin/v1/tenants", f.admin, `{"name":"invalid-org","unknown":1}`, 400}},
		{"tenant_trailing", platformContractRequest{"POST", "/tenants", "/admin/v1/tenants", f.admin, `{"name":"invalid-org"} {}`, 400}},
		{"tenant_null", platformContractRequest{"POST", "/tenants", "/admin/v1/tenants", f.admin, `{"name":null}`, 400}},
		{"status_unknown", platformContractRequest{"PUT", "/tenants/{id}/status", tenantPath + "/status", f.admin, `{"active":false,"unknown":1}`, 400}},
		{"status_trailing", platformContractRequest{"PUT", "/tenants/{id}/status", tenantPath + "/status", f.admin, `{"active":false} {}`, 400}},
		{"status_null", platformContractRequest{"PUT", "/tenants/{id}/status", tenantPath + "/status", f.admin, `{"active":null}`, 400}},
		{"tenant_member_unknown", platformContractRequest{"PUT", "/tenants/{id}/members/{user_id}", tenantMember, f.admin, `{"role":"viewer","active":true,"unknown":1}`, 400}},
		{"tenant_member_trailing", platformContractRequest{"PUT", "/tenants/{id}/members/{user_id}", tenantMember, f.admin, `{"role":"viewer","active":true} {}`, 400}},
		{"tenant_member_null", platformContractRequest{"PUT", "/tenants/{id}/members/{user_id}", tenantMember, f.admin, `{"role":"viewer","active":null}`, 400}},
		{"tenant_member_omitted", platformContractRequest{"PUT", "/tenants/{id}/members/{user_id}", tenantMember, f.admin, `{"role":"viewer"}`, 400}},
		{"farm_member_unknown", platformContractRequest{"PUT", "/farms/{id}/members/{user_id}", farmMember, f.owner, `{"role":"viewer","active":true,"unknown":1}`, 400}},
		{"farm_member_trailing", platformContractRequest{"PUT", "/farms/{id}/members/{user_id}", farmMember, f.owner, `{"role":"viewer","active":true} {}`, 400}},
		{"farm_member_null", platformContractRequest{"PUT", "/farms/{id}/members/{user_id}", farmMember, f.owner, `{"role":"viewer","active":null}`, 400}},
		{"farm_member_omitted", platformContractRequest{"PUT", "/farms/{id}/members/{user_id}", farmMember, f.owner, `{"role":"viewer"}`, 400}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolated := newPlatformContractMembershipFixture(t)
			request := tc.request
			request.path = strings.ReplaceAll(request.path, tenantPath, fmt.Sprintf("/admin/v1/tenants/%d", isolated.tenant))
			if request.route == "/tenants/{id}/members/{user_id}" {
				request.path = fmt.Sprintf("/admin/v1/tenants/%d/members/%d", isolated.tenant, isolated.user)
			}
			if request.route == "/farms/{id}/members/{user_id}" {
				request.path = fmt.Sprintf("/user/v1/farms/%d/members/%d", isolated.farm, isolated.user)
			}
			if request.token == f.admin {
				request.token = isolated.admin
			}
			if request.token == f.owner {
				request.token = isolated.owner
			}
			before := isolated.state(t)
			isolated.request(t, request)
			if isolated.state(t) != before {
				t.Error("rejected request changed persisted state")
			}
		})
	}
}
