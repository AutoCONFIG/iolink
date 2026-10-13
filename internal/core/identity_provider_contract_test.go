package core_test

import "testing"

func TestIdentityStorageFailuresAreNotReportedAsRevokedCredentials(t *testing.T) {
	for _, table := range []string{"users", "tenants", "tenant_memberships"} {
		t.Run("admin_"+table, func(t *testing.T) {
			f := newBaselineAdminFixture(t)
			execute(t, f.pool, "ALTER TABLE "+table+" RENAME TO identity_fault_relation")
			defer execute(t, f.pool, "ALTER TABLE identity_fault_relation RENAME TO "+table)
			f.request(t, baselineAdminRequest{"GET", "/farms", "/farms", "", 500})
		})
		t.Run("app_"+table, func(t *testing.T) {
			f := newBaselineAppFixture(t)
			execute(t, f.pool, "ALTER TABLE "+table+" RENAME TO identity_fault_relation")
			defer execute(t, f.pool, "ALTER TABLE identity_fault_relation RENAME TO "+table)
			response := baselineAppHTTP(t, f.server, baselineAppRequest{method: "GET", path: "/ponds", token: f.owner})
			response.contract(t, "GET", "/ponds", 500)
		})
	}
}
