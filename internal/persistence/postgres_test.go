package persistence

import (
	"strings"
	"testing"
)

func TestTelemetrySnapshotQueryRequiresActiveOwnerMembership(t *testing.T) {
	for _, fragment := range []string{
		"EXISTS (SELECT 1 FROM tenant_memberships",
		"JOIN tenants t ON t.id=tm.tenant_id",
		"tm.user_id=f.owner_id",
		"tm.tenant_id=f.tenant_id",
		"AND t.active",
		"f.tenant_id=$1",
		"s.tenant_id IS NOT NULL AND s.tenant_id=f.tenant_id",
	} {
		if !strings.Contains(telemetrySnapshotQuery, fragment) {
			t.Fatalf("authorization query lost required fail-closed predicate %q", fragment)
		}
	}
	if strings.Contains(telemetrySnapshotQuery, "COALESCE(s.tenant_id") || strings.Contains(telemetrySnapshotQuery, "LEFT JOIN device_shadows") {
		t.Fatal("shadow tenant must not be an authorization alternative")
	}
}

func TestTelemetrySnapshotQueryCannotCrossTenantFarmChain(t *testing.T) {
	for _, fragment := range []string{
		"JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id",
		"AND f.tenant_id=$1",
		"tm.tenant_id=f.tenant_id AND tm.tenant_id=$1",
		"s.tenant_id IS NOT NULL AND s.tenant_id=f.tenant_id",
	} {
		if !strings.Contains(telemetrySnapshotQuery, fragment) {
			t.Fatalf("two-tenant authorization chain missing %q", fragment)
		}
	}
}

func TestTenantPredicateRejectsSQLInjectionIdentifiers(t *testing.T) {
	if got, err := TenantPredicate("devices"); err != nil || got != "devices.tenant_id = $1" {
		t.Fatalf("valid alias: got %q, err %v", got, err)
	}
	for _, alias := range []string{"devices;DROP TABLE users", "devices--", "1devices", "devices space"} {
		if got, err := TenantPredicate(alias); err == nil || got != "" {
			t.Fatalf("malformed alias %q was accepted as %q (err=%v)", alias, got, err)
		}
	}
}
