package platform

import "testing"

func TestBootstrapAuditUsesSystemTenantScope(t *testing.T) {
	if systemTenantQuery != "SELECT id FROM tenants WHERE name='__iolink_system__' AND active FOR UPDATE" {
		t.Fatal("bootstrap must lock the active reserved system tenant")
	}
	if auditAdminQuery != "INSERT INTO audit_events(tenant_id,action,resource_type,resource_id) VALUES($1,$2,'user',$3)" {
		t.Fatal("bootstrap audit insert must include tenant_id")
	}
}
