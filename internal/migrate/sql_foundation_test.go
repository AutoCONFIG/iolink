package migrate

import (
	"os"
	"strings"
	"testing"
)

func TestPersistenceFoundationDeclaresFailClosedIdempotency(t *testing.T) {
	raw, err := os.ReadFile("sql/005_persistence_foundation.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, fragment := range []string{
		"tenant_id BIGINT NOT NULL REFERENCES tenants(id)",
		"idempotency_key TEXT NOT NULL",
		"CHECK (length(trim(kind)) > 0 AND length(trim(idempotency_key)) > 0)",
		"UNIQUE (tenant_id, topic, idempotency_key)",
		"ALTER TABLE audit_events ALTER COLUMN tenant_id SET NOT NULL",
		"SELECT count(*) INTO null_count FROM audit_events WHERE tenant_id IS NULL",
		"RAISE EXCEPTION 'audit_events contains % NULL tenant_id rows; operator must backfill fixture ownership and retry'",
		"operator must backfill fixture ownership and retry",
		"SELECT count(*) INTO null_count FROM farms WHERE tenant_id IS NULL",
		"ALTER TABLE farms ALTER COLUMN tenant_id SET NOT NULL",
		"CREATE UNIQUE INDEX IF NOT EXISTS tenants_name_key ON tenants(name)",
		"INSERT INTO tenants(name, active) VALUES ('__iolink_system__', TRUE)",
		"audit_events_action_nonempty",
		"audit_events_resource_type_nonempty",
		"audit_events_resource_id_nonempty",
		"audit_events_actor_fk",
		"CREATE TABLE IF NOT EXISTS tenant_migration_reconciliation",
		"'audit_event',ae.id::text,'assigned_default_tenant_orphan'",
		"'sensor_data',sd.device_no || ':' || sd.ts::text,'assigned_default_tenant_orphan'",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("foundation migration missing required constraint: %s", fragment)
		}
	}
	if strings.Index(sql, "SELECT count(*) INTO null_count FROM audit_events WHERE tenant_id IS NULL") > strings.Index(sql, "ALTER TABLE audit_events ALTER COLUMN tenant_id SET NOT NULL") {
		t.Fatal("audit tenant NULL guard must precede NOT NULL enforcement")
	}
	if strings.Index(sql, "SELECT count(*) INTO null_count FROM farms WHERE tenant_id IS NULL") > strings.Index(sql, "ALTER TABLE farms ALTER COLUMN tenant_id SET NOT NULL") {
		t.Fatal("farm tenant NULL guard must precede NOT NULL enforcement")
	}
}
