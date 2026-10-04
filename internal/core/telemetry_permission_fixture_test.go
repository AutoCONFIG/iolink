package core_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	corepkg "git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

type telemetryPermissionFixture struct {
	pool *pgxpool.Pool
	svc  *corepkg.Service
	repo domain.GenericTelemetryRepo
}

func newTelemetryPermissionFixture(t *testing.T) telemetryPermissionFixture {
	t.Helper()
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	_, err := p.Exec(ctx, `INSERT INTO users(id,open_id) VALUES(9401,'v2-owner'),(9402,'v2-actor');
 INSERT INTO tenants(id,name) VALUES(9401,'v2-permission'),(9402,'v2-other');
 INSERT INTO tenant_memberships(tenant_id,user_id,role) VALUES(9401,9401,'owner'),(9401,9402,'viewer');
 INSERT INTO farms(id,tenant_id,owner_id,name) VALUES(9401,9401,9401,'v2-farm'),(9402,9402,9402,'v2-foreign'),(9403,9401,9401,'hidden');
 INSERT INTO ponds(id,farm_id,name) VALUES(9401,9401,'v2-pond'),(9402,9402,'foreign'),(9403,9403,'hidden');
 INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role) VALUES(9401,9401,9402,'viewer');
 INSERT INTO devices(pond_id,device_no,secret_hash) VALUES(9401,'v2-permission','hash'),(9402,'v2-foreign','hash'),(9403,'v2-hidden','hash');
 INSERT INTO alarm_rules(pond_id,metric,max_value,level,enabled) VALUES(9401,'temperature',20,'warning',true)`)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := corepkg.NewWithPolicy(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)), policy)
	if err != nil {
		t.Fatal(err)
	}
	var version string
	if err := p.QueryRow(ctx, `SELECT extversion FROM pg_extension WHERE extname='timescaledb'`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	t.Logf("environment real_timescaledb=%s", version)
	return telemetryPermissionFixture{p, svc, svc.Telemetry().(domain.GenericTelemetryRepo)}
}

func (f telemetryPermissionFixture) role(t *testing.T, role string, active bool) {
	t.Helper()
	_, err := f.pool.Exec(context.Background(), `UPDATE tenant_memberships SET role=$1,active=$2,expires_at=CASE WHEN $1='support' THEN now()+interval '1 hour' ELSE NULL END,permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`, role, active)
	if err != nil {
		t.Fatal(err)
	}
}

func (f telemetryPermissionFixture) snapshot(t *testing.T) string {
	t.Helper()
	var data []byte
	err := f.pool.QueryRow(context.Background(), `SELECT jsonb_build_object(
 'telemetry',(SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY ts,device_no),'[]') FROM telemetry t),
 'sensor_data',(SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY ts,device_no),'[]') FROM sensor_data t),
 'shadows',(SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY device_no),'[]') FROM device_shadows t),
 'alarms',(SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id),'[]') FROM alarms t),
 'outbox',(SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id),'[]') FROM notification_outbox t))`).Scan(&data)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func telemetryActor(role string, actor int64) context.Context {
	return domain.WithTenantUserID(domain.WithTenantRole(domain.WithTenantID(context.Background(), 9401), role), actor)
}

func telemetryProperties() map[string]json.RawMessage {
	return map[string]json.RawMessage{"temperature": json.RawMessage(`25`)}
}

func telemetryTimestamp() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }
