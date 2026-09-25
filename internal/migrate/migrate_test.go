package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("IOLINK_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("integration: set IOLINK_TEST_PG_DSN to a disposable PostgreSQL/Timescale instance")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("iolink_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, e := admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
		admin.Close()
		if e != nil {
			t.Errorf("cleanup %s: %v", name, e)
		}
	})
	return pool
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
	// Actual core SQL must work on a brand-new migrated database (regression: signal).
	_, err := pool.Exec(ctx, `INSERT INTO farms(id,owner_id,name) VALUES(1,1,'farm'); INSERT INTO ponds(id,farm_id,name) VALUES(1,1,'pond'); INSERT INTO devices(pond_id,device_no,secret_hash) VALUES(1,'audit-device','hash')`)
	if err != nil {
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
	if err := pool.QueryRow(ctx, "SELECT count(*),max(signal) FROM sensor_data").Scan(&n, &sig); err != nil || n != 1 || sig != -65 {
		t.Fatalf("fresh ingest: n=%d sig=%d err=%v", n, sig, err)
	}
	rd, err := svc.Telemetry().Latest(ctx, "audit-device")
	if err != nil || rd.Temperature == nil || *rd.Temperature != 26 {
		t.Fatalf("shadow: %+v %v", rd, err)
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
