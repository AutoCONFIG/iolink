package core_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	corepkg "git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

func TestGenericTelemetryV2PersistsValidatedPropertiesAndScopedHistory(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id) VALUES (21,'m6a-owner'),(22,'m6a-foreign'); INSERT INTO tenants(name) VALUES ('m6a-tenant'); INSERT INTO tenant_memberships(tenant_id,user_id,role) SELECT id,21,'owner' FROM tenants WHERE name='m6a-tenant'; INSERT INTO farms(id,owner_id,tenant_id,name) SELECT 21,21,id,'m6a-farm' FROM tenants WHERE name='m6a-tenant'; INSERT INTO ponds(id,farm_id,name) VALUES (21,21,'m6a-pond'); INSERT INTO devices(pond_id,device_no,secret_hash) VALUES (21,'m6a-device','hash')`); err != nil {
		t.Fatal(err)
	}
	var tenantID, productID int64
	if err := p.QueryRow(ctx, `SELECT id FROM tenants WHERE name='m6a-tenant'`).Scan(&tenantID); err != nil {
		t.Fatal(err)
	}
	if err := p.QueryRow(ctx, `INSERT INTO products(tenant_id,name) VALUES($1,'m6a-product') RETURNING id`, tenantID).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	schema := `{"fields":[{"identifier":"temperature","type":"number","unit":"℃","minimum":0,"maximum":50,"readable":true,"nullable":false},{"identifier":"mode","type":"string","enum_values":["auto","manual"],"readable":true,"nullable":false},{"identifier":"secret","type":"string","readable":false,"nullable":true}]}`
	if _, err := p.Exec(ctx, `INSERT INTO product_models(product_id,version,schema,published_at) VALUES($1,1,$2::jsonb,now())`, productID, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `UPDATE devices SET product_id=$1,model_version=1 WHERE device_no='m6a-device'`, productID); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO alarm_rules(pond_id,metric,max_value,level,enabled) VALUES(21,'temperature',20,'warning',true)`); err != nil {
		t.Fatal(err)
	}
	s, err := corepkg.New(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	repo := s.Telemetry().(domain.GenericTelemetryRepo)
	ts := time.Now().UTC().Truncate(time.Microsecond)
	if err := s.HandleEvent(event.Event{Kind: event.KindProperties, DeviceNo: "m6a-device", MessageID: "generic-1", Ts: ts.Add(-time.Second), GenericProperties: map[string]json.RawMessage{"temperature": json.RawMessage(`25`), "mode": json.RawMessage(`"auto"`)}}); err != nil {
		t.Fatal(err)
	}
	if err := s.HandleEvent(event.Event{Kind: event.KindProperties, DeviceNo: "m6a-device", MessageID: "generic-1", Ts: ts, GenericProperties: map[string]json.RawMessage{"temperature": json.RawMessage(`25`), "mode": json.RawMessage(`"auto"`)}}); err != nil {
		t.Fatal(err)
	}
	var alarmCount int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM alarms WHERE device_no='m6a-device' AND metric='temperature'`).Scan(&alarmCount); err != nil || alarmCount != 1 {
		t.Fatalf("generic alarm count=%d err=%v", alarmCount, err)
	}
	result, err := repo.SubmitTelemetry(ctx, "m6a-device", 21, ts, map[string]json.RawMessage{
		"temperature": json.RawMessage(`25`), "mode": json.RawMessage(`"auto"`), "secret": json.RawMessage(`"hidden"`),
	})
	if err != nil || len(result.Accepted) != 3 || len(result.Rejected) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	invalid, err := repo.SubmitTelemetry(ctx, "m6a-device", 21, ts.Add(2*time.Second), map[string]json.RawMessage{"unknown": json.RawMessage(`1`)})
	if err != nil || len(invalid.Rejected) != 1 {
		t.Fatalf("invalid result=%+v err=%v", invalid, err)
	}
	var beforeMixed int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM telemetry WHERE device_no='m6a-device'`).Scan(&beforeMixed); err != nil {
		t.Fatal(err)
	}
	mixed, err := repo.SubmitTelemetry(ctx, "m6a-device", 21, ts.Add(3*time.Second), map[string]json.RawMessage{"temperature": json.RawMessage(`25`), "unknown": json.RawMessage(`1`)})
	if err != nil || len(mixed.Rejected) != 1 {
		t.Fatalf("mixed result=%+v err=%v", mixed, err)
	}
	var afterMixed int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM telemetry WHERE device_no='m6a-device'`).Scan(&afterMixed); err != nil || afterMixed != beforeMixed {
		t.Fatalf("mixed write count before=%d after=%d err=%v", beforeMixed, afterMixed, err)
	}
	if _, err := repo.SubmitTelemetry(ctx, "m6a-device", 22, ts.Add(time.Second), map[string]json.RawMessage{"temperature": json.RawMessage(`20`)}); err == nil {
		t.Fatal("foreign owner was accepted")
	}
	scopedCtx := domain.WithTenantID(ctx, tenantID)
	points, unit, err := repo.HistoryV2ForUser(scopedCtx, "m6a-device", 21, "temperature", ts.Add(-time.Minute), ts.Add(time.Minute), 10)
	if err != nil || unit != "℃" || len(points) != 2 || points[0].ModelVersion != 1 {
		t.Fatalf("points=%+v unit=%q err=%v", points, unit, err)
	}
	value, ok := points[0].Value.(float64)
	if !ok || value != 25 {
		t.Fatalf("temperature point=%+v", points[0])
	}
	modePoints, modeUnit, err := repo.HistoryV2ForUser(scopedCtx, "m6a-device", 21, "mode", ts.Add(-time.Minute), ts.Add(time.Minute), 10)
	if err != nil || modeUnit != "" || len(modePoints) != 2 {
		t.Fatalf("mode points=%+v unit=%q err=%v", modePoints, modeUnit, err)
	}
	if mode, ok := modePoints[0].Value.(string); !ok || mode != "auto" {
		t.Fatalf("mode point=%+v", modePoints[0])
	}
	detail := s.Telemetry().(domain.DeviceModelLatestRepo)
	latest, err := detail.ModelLatestForUser(scopedCtx, "m6a-device", 21)
	if err != nil || len(latest.Fields) != 2 || string(latest.Properties["mode"]) != `"auto"` || latest.Properties["secret"] != nil {
		t.Fatalf("dynamic detail=%+v err=%v", latest, err)
	}
}

func TestGenericTelemetryBackfillResumesFromCheckpoint(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO tenants(name) VALUES('bf-tenant'); INSERT INTO users(id,open_id) VALUES(31,'bf-user'); INSERT INTO farms(id,owner_id,tenant_id,name) SELECT 31,31,id,'bf-farm' FROM tenants WHERE name='bf-tenant'; INSERT INTO ponds(id,farm_id,name) VALUES(31,31,'bf-pond'); INSERT INTO devices(pond_id,device_no,secret_hash) VALUES(31,'bf-device','hash'); INSERT INTO sensor_data(ts,device_no,pond_id,tenant_id,temperature) SELECT now()-interval '2 minutes','bf-device',31,id,24 FROM tenants WHERE name='bf-tenant'; INSERT INTO sensor_data(ts,device_no,pond_id,tenant_id,temperature) SELECT now()-interval '1 minutes','bf-device',31,id,25 FROM tenants WHERE name='bf-tenant'`); err != nil {
		t.Fatal(err)
	}
	var n int
	progress, err := corepkg.BackfillWaterTelemetry(ctx, p, "fixture-water", 1)
	if err != nil || progress.Rows != 1 {
		t.Fatalf("first progress=%+v err=%v", progress, err)
	}
	progress, err = corepkg.BackfillWaterTelemetry(ctx, p, "fixture-water", 1)
	if err != nil || progress.Rows != 1 {
		t.Fatalf("resume progress=%+v err=%v", progress, err)
	}
	if err = p.QueryRow(ctx, `SELECT count(*) FROM telemetry WHERE device_no='bf-device'`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("rows=%d err=%v", n, err)
	}
}
