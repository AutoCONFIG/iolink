package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/persistence"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return testdb.New(t)
}
func TestEmbeddedMigrations(t *testing.T) {
	ms, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) < 2 {
		t.Fatal("missing baseline repair")
	}
	for _, m := range ms {
		if m.sql == "" || len(m.checksum) != 64 {
			t.Fatal("invalid embedded migration")
		}
	}
}

func TestLegacyTenantReconciliationAndRollback(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	ms, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) < 5 {
		t.Fatal("missing tenant migration")
	}
	if err := apply(ctx, pool, ms[:4], false); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,open_id) VALUES(9001,'legacy-owner'); INSERT INTO farms(id,owner_id,name) VALUES(9001,9001,'legacy-farm'); INSERT INTO ponds(id,farm_id,name) VALUES(9001,9001,'legacy-pond'); INSERT INTO devices(id,pond_id,device_no,secret_hash) VALUES(9001,9001,'legacy-device','hash'); INSERT INTO sensor_data(ts,device_no,temperature) VALUES(now(),'legacy-device',20),(now(),'orphan-device',21); INSERT INTO device_shadows(device_no,last) VALUES('legacy-device','{}'),('orphan-device','{}'); INSERT INTO audit_events(actor_id,action,resource_type,resource_id) VALUES(9001,'legacy','farm','9001')`); err != nil {
		t.Fatal(err)
	}
	bad := append([]migration(nil), ms[:5]...)
	bad[4].sql += "\nSELECT 1/0;"
	if err := apply(ctx, pool, bad, false); err == nil {
		t.Fatal("failing migration succeeded")
	}
	var present bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.tenant_migration_reconciliation') IS NOT NULL`).Scan(&present); err != nil || present {
		t.Fatalf("failed migration left reconciliation table=%t err=%v", present, err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM device_shadows WHERE device_no='orphan-device'`); err != nil {
		t.Fatal(err)
	}
	if err := apply(ctx, pool, ms, false); err != nil {
		t.Fatal(err)
	}
	var farms, ponds, devices, orphanSensors, orphanShadows, audits, owners int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM tenant_migration_reconciliation WHERE resource_type='farm'),
		(SELECT count(*) FROM tenant_migration_reconciliation WHERE resource_type='pond'),
		(SELECT count(*) FROM tenant_migration_reconciliation WHERE resource_type='device'),
		(SELECT count(*) FROM tenant_migration_reconciliation WHERE resource_type='sensor_data'),
		(SELECT count(*) FROM tenant_migration_reconciliation WHERE resource_type='device_shadow'),
		(SELECT count(*) FROM tenant_migration_reconciliation WHERE resource_type='audit_event'),
		(SELECT count(*) FROM tenant_memberships WHERE user_id=9001)`).Scan(&farms, &ponds, &devices, &orphanSensors, &orphanShadows, &audits, &owners); err != nil {
		t.Fatal(err)
	}
	if farms != 1 || ponds != 1 || devices != 1 || orphanSensors != 1 || orphanShadows != 0 || audits != 1 || owners != 1 {
		t.Fatalf("reconciliation farms=%d ponds=%d devices=%d orphanSensors=%d orphanShadows=%d audits=%d owners=%d", farms, ponds, devices, orphanSensors, orphanShadows, audits, owners)
	}
}

func TestM6bMembershipMigrationRollbackAndLegacyBackfill(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	ms, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) < 10 {
		t.Fatal("missing M6b migration")
	}
	if err := apply(ctx, pool, ms[:9], false); err != nil {
		t.Fatal(err)
	}
	var tenantID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM tenants WHERE name='__iolink_system__'`).Scan(&tenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,open_id) VALUES(9101,'m6b-legacy-owner')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO farms(id,owner_id,tenant_id,name) VALUES(9101,9101,$1,'m6b-legacy-farm')`, tenantID); err != nil {
		t.Fatal(err)
	}
	bad := append([]migration(nil), ms...)
	bad[9].sql += "\nSELECT 1/0;"
	if err := apply(ctx, pool, bad, false); err == nil {
		t.Fatal("failing M6b migration succeeded")
	}
	var farmMembershipTable bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.farm_memberships') IS NOT NULL`).Scan(&farmMembershipTable); err != nil || farmMembershipTable {
		t.Fatalf("failed M6b migration left farm membership table=%t err=%v", farmMembershipTable, err)
	}
	var ownerMemberships int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tenant_memberships WHERE tenant_id=$1 AND user_id=9101`, tenantID).Scan(&ownerMemberships); err != nil || ownerMemberships != 0 {
		t.Fatalf("failed M6b migration left owner memberships=%d err=%v", ownerMemberships, err)
	}
	if err := apply(ctx, pool, ms, false); err != nil {
		t.Fatal(err)
	}
	var farmMemberships int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tenant_memberships WHERE tenant_id=$1 AND user_id=9101 AND role='owner'`, tenantID).Scan(&ownerMemberships); err != nil || ownerMemberships != 1 {
		t.Fatalf("M6b tenant owner memberships=%d err=%v", ownerMemberships, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM farm_memberships WHERE tenant_id=$1 AND farm_id=9101 AND user_id=9101 AND role='owner'`, tenantID).Scan(&farmMemberships); err != nil || farmMemberships != 1 {
		t.Fatalf("M6b farm owner memberships=%d err=%v", farmMemberships, err)
	}
}
func TestFreshUpAndBootstrap(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	if CheckLatest(ctx, pool) == nil {
		t.Fatal("uninitialized database accepted")
	}
	for range 2 {
		if err := Up(ctx, pool); err != nil {
			t.Fatal(err)
		}
	}
	if err := CheckLatest(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if platform.CheckAdminReady(ctx, pool) == nil {
		t.Fatal("public default admin installed")
	}
	if err := platform.BootstrapAdmin(ctx, pool, "operator", "audit-Only-Str0ng-Pass!", false); err != nil {
		t.Fatal(err)
	}
	if err := platform.CheckAdminReady(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var systemTenantID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM tenants WHERE name='__iolink_system__'`).Scan(&systemTenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id) VALUES($1, NULL, '', 'user', 'operator')`, systemTenantID); err == nil {
		t.Fatal("empty audit action must be rejected")
	}
	if platform.BootstrapAdmin(ctx, pool, "another", "audit-Only-Str0ng-Pass!", false) == nil {
		t.Fatal("second bootstrap must be rejected")
	}
	var hash string
	if err := pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE username='operator'").Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") || !platform.CheckPassword(hash, "audit-Only-Str0ng-Pass!") {
		t.Fatal("invalid bootstrapped password")
	}
	if err := platform.BootstrapAdmin(ctx, pool, "operator", "new-audit-Only-Str0ng!", true); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := pool.QueryRow(ctx, "SELECT token_version FROM users WHERE username='operator'").Scan(&version); err != nil || version != 1 {
		t.Fatalf("password reset version=%d err=%v", version, err)
	}
	var tenantID, farmID int64
	err := pool.QueryRow(ctx, `WITH owner AS (
		SELECT id FROM users WHERE username='operator'
	), tenant AS (
		INSERT INTO tenants(name, active) VALUES('fresh-bootstrap-tenant', TRUE) RETURNING id
	), membership AS (
		INSERT INTO tenant_memberships(tenant_id,user_id,role)
		SELECT tenant.id,owner.id,'owner' FROM tenant CROSS JOIN owner
		RETURNING tenant_id
	)
	INSERT INTO farms(owner_id,tenant_id,name)
	SELECT owner.id,membership.tenant_id,'farm' FROM owner CROSS JOIN membership
	RETURNING id,tenant_id`).Scan(&farmID, &tenantID)
	if err != nil {
		t.Fatal(err)
	}
	var pondID int64
	if err = pool.QueryRow(ctx, `INSERT INTO ponds(farm_id,name) VALUES($1,'pond') RETURNING id`, farmID).Scan(&pondID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO devices(pond_id,device_no,secret_hash) VALUES($1,'audit-device','hash')`, pondID); err != nil {
		t.Fatal(err)
	}
	svc, err := core.New(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.HandleEvent(event.Event{Kind: event.KindProperties, DeviceNo: "audit-device", Ts: time.Now().UTC(), Properties: map[string]float64{"temperature": 26, "signal": -65}}); err != nil {
		t.Fatal(err)
	}
	var n, sig int
	var storedSensorTenant, storedShadowTenant int64
	if err := pool.QueryRow(ctx, "SELECT count(*),max(signal),max(tenant_id) FROM sensor_data").Scan(&n, &sig, &storedSensorTenant); err != nil || n != 1 || sig != -65 || storedSensorTenant != tenantID {
		t.Fatalf("fresh ingest: n=%d sig=%d tenant=%d err=%v", n, sig, storedSensorTenant, err)
	}
	if err = pool.QueryRow(ctx, "SELECT tenant_id FROM device_shadows WHERE device_no='audit-device'").Scan(&storedShadowTenant); err != nil || storedShadowTenant != tenantID {
		t.Fatalf("fresh shadow tenant=%d want=%d err=%v", storedShadowTenant, tenantID, err)
	}
	rd, err := svc.Telemetry().Latest(ctx, "audit-device")
	if err != nil || rd.Temperature == nil || *rd.Temperature != 26 {
		t.Fatalf("shadow: %+v %v", rd, err)
	}
	if err = (persistence.Store{Pool: pool}).Within(ctx, func(ctx context.Context, tx pgx.Tx) error {
		snapshot, readErr := persistence.ReadTelemetrySnapshot(ctx, tx, tenantID, "audit-device")
		if readErr != nil {
			return readErr
		}
		if snapshot.TenantID != tenantID || snapshot.PondID != pondID {
			return fmt.Errorf("tenant snapshot = %+v, want tenant=%d pond=%d", snapshot, tenantID, pondID)
		}
		var foreignTenantID int64
		if readErr = tx.QueryRow(ctx, `INSERT INTO tenants(name, active) VALUES('fresh-bootstrap-foreign-tenant', TRUE) RETURNING id`).Scan(&foreignTenantID); readErr != nil {
			return readErr
		}
		_, readErr = persistence.ReadTelemetrySnapshot(ctx, tx, foreignTenantID, "audit-device")
		if !errors.Is(readErr, pgx.ErrNoRows) {
			return fmt.Errorf("foreign tenant read error = %v, want %v", readErr, pgx.ErrNoRows)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
func TestLegacyAdoptionAndPreservation(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	raw, err := os.ReadFile("testdata/legacy.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO sensor_data(ts,device_no,temperature) VALUES(now(),'historical',21)`); err != nil {
		t.Fatal(err)
	}
	if err := Up(ctx, pool); err == nil {
		t.Fatal("unmanaged schema silently adopted")
	}
	if err := AdoptLegacy(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if platform.CheckAdminReady(ctx, pool) == nil {
		t.Fatal("legacy default password allowed to serve")
	}
	if err := platform.BootstrapAdmin(ctx, pool, "admin", "fresh-Strong-Audit-Pass", true); err != nil {
		t.Fatal(err)
	}
	if err := platform.CheckAdminReady(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var value float64
	if err := pool.QueryRow(ctx, "SELECT temperature FROM sensor_data WHERE device_no='historical'").Scan(&value); err != nil || value != 21 {
		t.Fatalf("legacy data lost: %v %v", value, err)
	}
	if err := Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
}
func TestRejectLegacyDrift(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/legacy.sql")
	if _, err := pool.Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE farms DROP CONSTRAINT farms_owner_id_fkey"); err != nil {
		t.Fatal(err)
	}
	if AdoptLegacy(ctx, pool) == nil {
		t.Fatal("missing ownership FK accepted")
	}
	ok, _ := hasHistory(ctx, pool)
	if ok {
		t.Fatal("failed adoption modified schema history")
	}
}
func TestAtomicFailureAndChecksum(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	if err := Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	ms, _ := load()
	raw := "CREATE TABLE should_rollback(id int); SELECT nonexistent_function();"
	h := sha256.Sum256([]byte(raw))
	ms = append(ms, migration{len(ms) + 1, "003_failure.sql", raw, hex.EncodeToString(h[:])})
	if apply(ctx, pool, ms, false) == nil {
		t.Fatal("bad migration accepted")
	}
	var absent bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('should_rollback') IS NULL").Scan(&absent); err != nil || !absent {
		t.Fatal("DDL not rolled back", err)
	}
	if err := CheckLatest(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE schema_migrations SET checksum='changed' WHERE version=1"); err != nil {
		t.Fatal(err)
	}
	if Up(ctx, pool) == nil || CheckLatest(ctx, pool) == nil {
		t.Fatal("checksum drift accepted")
	}
}
func TestConcurrentUp(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wg.Go(func() { errs <- Up(ctx, pool) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := CheckLatest(ctx, pool); err != nil {
		t.Fatal(err)
	}
}
func TestRejectHistoryGapAndNewerVersion(t *testing.T) {
	for _, sql := range []string{"DELETE FROM schema_migrations WHERE version=1", "INSERT INTO schema_migrations(version,name,checksum) VALUES(99,'future','future')"} {
		t.Run(sql, func(t *testing.T) {
			pool := testDB(t)
			ctx := context.Background()
			if err := Up(ctx, pool); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
			if CheckLatest(ctx, pool) == nil {
				t.Fatal("invalid history accepted")
			}
		})
	}
}
