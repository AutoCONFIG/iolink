package core

import (
	"context"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

func TestWaterBackfillInterruptionRollsBackAndResumes(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	_, err := p.Exec(ctx, `INSERT INTO tenants(name) VALUES('bf-interrupt'); INSERT INTO users(id,open_id) VALUES(41,'bf-interrupt-user'); INSERT INTO farms(id,owner_id,tenant_id,name) SELECT 41,41,id,'bf-interrupt-farm' FROM tenants WHERE name='bf-interrupt'; INSERT INTO ponds(id,farm_id,name) VALUES(41,41,'bf-interrupt-pond'); INSERT INTO devices(pond_id,device_no,secret_hash) VALUES(41,'bf-interrupt-device','hash'); INSERT INTO sensor_data(ts,device_no,pond_id,tenant_id,temperature) SELECT now()-interval '2 minutes','bf-interrupt-device',41,id,24 FROM tenants WHERE name='bf-interrupt'; INSERT INTO sensor_data(ts,device_no,pond_id,tenant_id,temperature) SELECT now()-interval '1 minutes','bf-interrupt-device',41,id,25 FROM tenants WHERE name='bf-interrupt'`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = backfillWaterTelemetry(ctx, p, "interrupt-fixture", 2, 1); err == nil {
		t.Fatal("interruption unexpectedly succeeded")
	}
	var rows, checkpoints int
	if err = p.QueryRow(ctx, `SELECT count(*) FROM telemetry WHERE device_no='bf-interrupt-device'`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("rows after rollback=%d err=%v", rows, err)
	}
	if err = p.QueryRow(ctx, `SELECT count(*) FROM telemetry_backfill_checkpoints WHERE job_name='interrupt-fixture'`).Scan(&checkpoints); err != nil || checkpoints != 0 {
		t.Fatalf("checkpoint after rollback=%d err=%v", checkpoints, err)
	}
	progress, err := BackfillWaterTelemetry(ctx, p, "interrupt-fixture", 2)
	if err != nil || progress.Rows != 2 {
		t.Fatalf("resume progress=%+v err=%v", progress, err)
	}
	if err = p.QueryRow(ctx, `SELECT count(*) FROM telemetry WHERE device_no='bf-interrupt-device'`).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("rows after resume=%d err=%v", rows, err)
	}
	progress, err = BackfillWaterTelemetry(ctx, p, "interrupt-fixture", 2)
	if err != nil || progress.Rows != 0 {
		t.Fatalf("idempotent rerun progress=%+v err=%v", progress, err)
	}
	if err = p.QueryRow(ctx, `SELECT count(*) FROM telemetry WHERE device_no='bf-interrupt-device'`).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("rows after idempotent rerun=%d err=%v", rows, err)
	}
	var minValue, maxValue float64
	if err = p.QueryRow(ctx, `SELECT min((properties->>'temperature')::double precision), max((properties->>'temperature')::double precision) FROM telemetry WHERE device_no='bf-interrupt-device'`).Scan(&minValue, &maxValue); err != nil || minValue != 24 || maxValue != 25 {
		t.Fatalf("reconciliation min=%v max=%v err=%v", minValue, maxValue, err)
	}
}

func TestWaterBackfillReconcilesEveryFixtureDevice(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	_, err := p.Exec(ctx, `INSERT INTO tenants(name) VALUES('bf-reconcile'); INSERT INTO users(id,open_id) VALUES(51,'bf-reconcile-user'); INSERT INTO farms(id,owner_id,tenant_id,name) SELECT 51,51,id,'bf-reconcile-farm' FROM tenants WHERE name='bf-reconcile'; INSERT INTO ponds(id,farm_id,name) VALUES(51,51,'bf-reconcile-pond'); INSERT INTO devices(pond_id,device_no,secret_hash) VALUES (51,'bf-a','hash'),(51,'bf-b','hash'); INSERT INTO sensor_data(ts,device_no,pond_id,tenant_id,temperature) SELECT now()-interval '3 minutes','bf-a',51,id,20 FROM tenants WHERE name='bf-reconcile'; INSERT INTO sensor_data(ts,device_no,pond_id,tenant_id,temperature) SELECT now()-interval '2 minutes','bf-a',51,id,21 FROM tenants WHERE name='bf-reconcile'; INSERT INTO sensor_data(ts,device_no,pond_id,tenant_id,temperature) SELECT now()-interval '1 minutes','bf-b',51,id,30 FROM tenants WHERE name='bf-reconcile'`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		progress, err := BackfillWaterTelemetry(ctx, p, "reconcile-fixture", 1)
		if err != nil {
			t.Fatal(err)
		}
		if i < 3 && progress.Rows != 1 || i == 3 && progress.Rows != 0 {
			t.Fatalf("batch %d progress=%+v", i, progress)
		}
	}
	rows, err := p.Query(ctx, `SELECT s.device_no,count(*),min(s.ts),max(s.ts),min(t.ts),max(t.ts),min(s.temperature),max(s.temperature),min(t.properties->>'temperature'),max(t.properties->>'temperature'),count(t.*) FROM sensor_data s LEFT JOIN telemetry t ON t.device_no=s.device_no AND t.ts=s.ts WHERE s.device_no IN ('bf-a','bf-b') GROUP BY s.device_no ORDER BY s.device_no`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	want := []struct {
		device           string
		count            int64
		min, max         float64
		minText, maxText string
	}{{"bf-a", 2, 20, 21, "20", "21"}, {"bf-b", 1, 30, 30, "30", "30"}}
	for _, expected := range want {
		if !rows.Next() {
			t.Fatalf("missing %s", expected.device)
		}
		var device, minText, maxText string
		var sourceCount, projectedCount int64
		var sourceMinTS, sourceMaxTS, projectedMinTS, projectedMaxTS time.Time
		var min, max float64
		if err := rows.Scan(&device, &sourceCount, &sourceMinTS, &sourceMaxTS, &projectedMinTS, &projectedMaxTS, &min, &max, &minText, &maxText, &projectedCount); err != nil {
			t.Fatal(err)
		}
		if device != expected.device || sourceCount != expected.count || projectedCount != expected.count || !projectedMinTS.Equal(sourceMinTS) || !projectedMaxTS.Equal(sourceMaxTS) || min != expected.min || max != expected.max || minText != expected.minText || maxText != expected.maxText {
			t.Fatalf("device=%s source=%d projected=%d range=%v..%v sample=%s..%s", device, sourceCount, projectedCount, min, max, minText, maxText)
		}
	}
	if rows.Next() {
		t.Fatal("unexpected fixture device")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
