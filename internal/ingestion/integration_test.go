package ingestion_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

func TestIngestionPersistsNormalizedEventWithMessageIDDeduplication(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,open_id) VALUES(1,'ingestion-test')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tenants(name) VALUES ('ingestion-test-tenant')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tenant_memberships(tenant_id,user_id,role) SELECT id,1,'owner' FROM tenants WHERE name='ingestion-test-tenant'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO farms(id,owner_id,tenant_id,name) SELECT 1,1,id,'ingestion-test-farm' FROM tenants WHERE name='ingestion-test-tenant'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO ponds(id,farm_id,name) VALUES(1,1,'pond')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO devices(pond_id,device_no,secret_hash) VALUES(1,'ingestion-device',$1)`, core.HashDeviceSecret("secret")); err != nil {
		t.Fatal(err)
	}
	service, err := core.New(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	e := event.Event{
		Kind: event.KindProperties, DeviceNo: "ingestion-device", Ts: time.Unix(10, 0).UTC(),
		MessageID: "sample-1", Properties: map[string]float64{"temperature": 26, "signal": -65},
	}
	if err := service.HandleEvent(e); err != nil {
		t.Fatal(err)
	}
	if err := service.HandleEvent(e); err != nil {
		t.Fatal(err)
	}
	var samples int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sensor_data WHERE device_no='ingestion-device'").Scan(&samples); err != nil {
		t.Fatal(err)
	}
	if samples != 1 {
		t.Fatalf("samples=%d want 1", samples)
	}
	var tenantID, sampleTenantID, shadowTenantID int64
	if err := pool.QueryRow(ctx, "SELECT id FROM tenants WHERE name='ingestion-test-tenant'").Scan(&tenantID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT tenant_id FROM sensor_data WHERE device_no='ingestion-device'").Scan(&sampleTenantID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT tenant_id FROM device_shadows WHERE device_no='ingestion-device'").Scan(&shadowTenantID); err != nil {
		t.Fatal(err)
	}
	if sampleTenantID != tenantID || shadowTenantID != tenantID {
		t.Fatalf("tenant attribution sample=%d shadow=%d want=%d", sampleTenantID, shadowTenantID, tenantID)
	}
}
