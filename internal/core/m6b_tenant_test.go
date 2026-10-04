package core_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	corepkg "git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

func TestM6bTenantMembershipVersionAndRevocation(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id,authority) VALUES (601,'m6b-user','USER'),(603,'m6b-assigned','USER'),(604,'m6b-foreign','USER'),(606,'m6b-expired','USER'),(607,'m6b-platform','ADMIN'); INSERT INTO tenants(id,name) VALUES (601,'m6b-tenant'); INSERT INTO tenant_memberships(tenant_id,user_id,role) VALUES (601,601,'admin'),(601,603,'member'),(601,606,'member')`); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO tenants(id,name) VALUES (602,'m6b-other'); INSERT INTO tenant_memberships(tenant_id,user_id,role) VALUES (602,604,'member'); INSERT INTO farms(id,owner_id,tenant_id,name) VALUES (601,601,601,'farm-a'),(602,601,602,'farm-b'); INSERT INTO ponds(id,farm_id,name) VALUES (601,601,'pond-a'),(602,602,'pond-b')`); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role) VALUES (601,601,603,'member')`); err != nil {
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
	installTestLicense(t, p, svc, 100)
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id) VALUES (605,'m6b-revoked'); INSERT INTO tenant_memberships(tenant_id,user_id,role,active) SELECT id,605,'member',false FROM tenants WHERE name='__iolink_system__'`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateFarm(ctx, ptr(605), "revoked-owner-farm", ""); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("revoked membership farm creation err=%v", err)
	}
	tenantID, err := svc.DefaultTenantForUser(ctx, 601)
	if err != nil || tenantID != 601 {
		t.Fatalf("default tenant=%d err=%v", tenantID, err)
	}
	version, err := svc.TenantMembershipVersion(ctx, 601, 601)
	if err != nil || version != 0 {
		t.Fatalf("membership version=%d err=%v", version, err)
	}
	if _, err := p.Exec(ctx, `UPDATE tenant_memberships SET permission_version=permission_version+1,active=false WHERE tenant_id=601 AND user_id=601`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DefaultTenantForUser(ctx, 601); !errors.Is(err, domain.ErrInactiveTenant) {
		t.Fatalf("inactive membership error=%v", err)
	}
	if _, err := svc.TenantMembershipVersion(ctx, 601, 601); err == nil {
		t.Fatal("inactive membership accepted")
	}
	if _, err := p.Exec(ctx, `UPDATE tenant_memberships SET active=true WHERE tenant_id=601 AND user_id=601`); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetTenantMember(ctx, 602, 601, "viewer", true, nil, 601); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetTenantMember(ctx, 601, 607, "owner", true, nil, 601); !errors.Is(err, domain.ErrInvalidProductModel) {
		t.Fatalf("platform owner grant err=%v", err)
	}
	if err := svc.SetTenantMember(ctx, 602, 601, "support", true, nil, 601); !errors.Is(err, domain.ErrInvalidProductModel) {
		t.Fatalf("support without expiry err=%v", err)
	}
	expires := time.Now().Add(time.Hour)
	if err := svc.SetTenantMember(ctx, 602, 601, "support", true, &expires, 601); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetTenantMember(ctx, 601, 607, "support", true, &expires, 601); err != nil {
		t.Fatalf("platform support grant err=%v", err)
	}
	if err := svc.SetFarmOwner(domain.WithTenantID(ctx, 601), 601, ptr(604)); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant owner assignment err=%v", err)
	}
	adminCtx := domain.WithTenantUserID(domain.WithTenantRole(domain.WithTenantID(ctx, 601), "admin"), 601)
	if err := svc.SetFarmOwner(adminCtx, 601, ptr(603)); err != nil {
		t.Fatal(err)
	}
	supportCtx := domain.WithTenantUserID(domain.WithTenantRole(domain.WithTenantID(ctx, 601), "support"), 603)
	supportFarms, err := svc.ListFarms(supportCtx)
	if err != nil || len(supportFarms) != 1 || supportFarms[0].ID != 601 {
		t.Fatalf("support farm scope=%+v err=%v", supportFarms, err)
	}
	supportPonds, err := svc.ListPonds(supportCtx)
	if err != nil || len(supportPonds) != 1 || supportPonds[0].ID != 601 {
		t.Fatalf("support pond scope=%+v err=%v", supportPonds, err)
	}
	if err := svc.SetFarmMember(domain.WithTenantID(ctx, 601), 601, 604, "member", true, nil, 601); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant farm member assignment err=%v", err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role) VALUES(602,601,604,'member')`); err == nil {
		t.Fatal("database accepted farm membership with mismatched tenant")
	}
	var invalidCount int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM farm_memberships WHERE farm_id=601 AND user_id=604`).Scan(&invalidCount); err != nil || invalidCount != 0 {
		t.Fatalf("failed assignment left membership count=%d err=%v", invalidCount, err)
	}
	expired := time.Now().Add(-time.Minute)
	if _, err := p.Exec(ctx, `INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role,active,expires_at) VALUES (601,601,606,'member',true,$1)`, expired); err != nil {
		t.Fatal(err)
	}
	expiredPonds, err := svc.Ponds().ListByUser(domain.WithTenantID(ctx, 601), 606)
	if err != nil || len(expiredPonds) != 0 {
		t.Fatalf("expired farm membership still sees ponds=%+v err=%v", expiredPonds, err)
	}
	oldOwnerPonds, err := svc.Ponds().ListByUser(domain.WithTenantID(ctx, 601), 601)
	if err != nil || len(oldOwnerPonds) != 0 {
		t.Fatalf("previous farm owner still sees ponds=%+v err=%v", oldOwnerPonds, err)
	}
	members, err := svc.ListTenantMembers(ctx, 602)
	if err != nil || len(members) != 2 || members[0].Role != "support" || members[0].ExpiresAt == nil {
		t.Fatalf("members=%+v err=%v", members, err)
	}
	if err := svc.SetTenantActive(ctx, 602, false, 601); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TenantMembershipVersion(ctx, 601, 602); err == nil {
		t.Fatal("inactive tenant membership accepted")
	}
	if _, err := p.Exec(ctx, `UPDATE tenant_memberships SET active=true WHERE tenant_id=601 AND user_id=601`); err != nil {
		t.Fatal(err)
	}
	var confirmedID int64
	if err := p.QueryRow(ctx, `INSERT INTO alarms(device_no,pond_id,metric,current_value,threshold,level,confirmed_at) VALUES('foreign',602,'temperature',20,25,'warning',now()) RETURNING id`).Scan(&confirmedID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Alarms().ConfirmByUser(domain.WithTenantID(ctx, 601), confirmedID, 601); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant confirmed alarm err=%v", err)
	}
	var assignedAlarmID int64
	if err := p.QueryRow(ctx, `INSERT INTO alarms(device_no,pond_id,metric,current_value,threshold,level) VALUES('assigned',601,'temperature',20,25,'warning') RETURNING id`).Scan(&assignedAlarmID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Alarms().ConfirmByUser(domain.WithTenantID(ctx, 601), assignedAlarmID, 603); err != nil {
		t.Fatalf("assigned member confirm err=%v", err)
	}
	ponds, err := svc.Ponds().ListByUser(domain.WithTenantID(ctx, 601), 601)
	if err != nil || len(ponds) != 0 {
		t.Fatalf("tenant-scoped ponds=%+v err=%v", ponds, err)
	}
	farms, err := svc.ListFarms(domain.WithTenantID(ctx, 601))
	if err != nil || len(farms) != 1 || farms[0].ID != 601 {
		t.Fatalf("tenant-scoped admin farms=%+v err=%v", farms, err)
	}
	farms, err = svc.ListFarms(domain.WithTenantID(ctx, 602))
	if err != nil || len(farms) != 1 || farms[0].ID != 602 {
		t.Fatalf("second tenant admin farms=%+v err=%v", farms, err)
	}
	assignedPonds, err := svc.Ponds().ListByUser(domain.WithTenantID(ctx, 601), 603)
	if err != nil || len(assignedPonds) != 1 || assignedPonds[0].ID != 601 {
		t.Fatalf("assigned farm ponds=%+v err=%v", assignedPonds, err)
	}
	device, secret, err := svc.RegisterDevice(adminCtx, 601, "m6b-device", "water", 60)
	if err != nil {
		t.Fatal(err)
	}
	if !svc.Authenticate(device.DeviceNo, secret) {
		t.Fatal("active tenant device authentication rejected")
	}
	supportDevices, err := svc.ListDevices(supportCtx, false, 0, 50, 0)
	if err != nil || len(supportDevices) != 1 || supportDevices[0].DeviceNo != device.DeviceNo {
		t.Fatalf("support device scope=%+v err=%v", supportDevices, err)
	}
	if _, err := p.Exec(ctx, `UPDATE tenants SET active=false WHERE id=601; UPDATE tenant_memberships SET permission_version=permission_version+1 WHERE tenant_id=601`); err != nil {
		t.Fatal(err)
	}
	if svc.Authenticate(device.DeviceNo, secret) {
		t.Fatal("inactive tenant device authenticated")
	}
	err = svc.HandleEvent(event.Event{Kind: event.KindProperties, DeviceNo: device.DeviceNo, Ts: time.Now().UTC(), MessageID: "inactive", Properties: map[string]float64{"temperature": 24}})
	if err == nil {
		t.Fatal("inactive tenant accepted telemetry")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Logf("inactive tenant telemetry error=%v", err)
	}
}

func TestM6bScopedReadsAndExistsQueries(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	_, err := p.Exec(ctx, `INSERT INTO users(id,open_id) VALUES (701,'owner'),(702,'support');
		INSERT INTO tenants(id,name) VALUES (701,'read-scope');
		INSERT INTO tenant_memberships(tenant_id,user_id,role,expires_at) VALUES (701,701,'owner',NULL),(701,702,'support',now()+interval '1 hour');
		INSERT INTO farms(id,owner_id,tenant_id,name) VALUES (701,701,701,'assigned'),(702,701,701,'hidden');
		INSERT INTO ponds(id,farm_id,name) VALUES (701,701,'assigned'),(702,702,'hidden');
		INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role,expires_at) VALUES (701,701,702,'support',now()+interval '1 hour');
		INSERT INTO devices(pond_id,device_no,secret_hash) VALUES (701,'visible-device','hash'),(702,'hidden-device','hash');
		INSERT INTO alarms(device_no,pond_id,metric,current_value,threshold,level,confirmed_at) VALUES
		('visible-device',701,'temperature',20,25,'warning',now()),
		('hidden-device',702,'temperature',20,25,'warning',now())`)
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
	scoped := domain.WithTenantID(ctx, 701)
	support := domain.WithTenantUserID(domain.WithTenantRole(scoped, "support"), 702)
	if farms, err := svc.ListFarms(domain.WithTenantRole(scoped, "support")); err != nil || len(farms) != 0 {
		t.Fatalf("support without actor scope=%+v err=%v", farms, err)
	}
	owner := domain.WithTenantUserID(domain.WithTenantRole(scoped, "owner"), 701)
	if members, err := svc.ListFarmMembers(owner, 701); err != nil || len(members) != 1 {
		t.Fatalf("farm members=%+v err=%v", members, err)
	}
	if _, err := svc.ListFarmMembers(domain.WithTenantRole(scoped, "owner"), 701); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("missing actor listed farm members: %v", err)
	}
	if _, err := svc.ListFarmMembers(domain.WithTenantUserID(domain.WithTenantRole(scoped, "owner"), 702), 701); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("forged role listed farm members: %v", err)
	}
	if _, err := svc.ListFarmMembers(support, 701); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("support listed farm members: %v", err)
	}
	if _, err := svc.ListFarmMembers(support, 702); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("support listed unassigned farm members: %v", err)
	}
	if exists, err := svc.FarmExists(scoped, 701); err != nil || !exists {
		t.Fatalf("farm exists=%t err=%v", exists, err)
	}
	if farms, err := svc.ListFarms(support); err != nil || len(farms) != 1 || farms[0].ID != 701 {
		t.Fatalf("support farms=%+v err=%v", farms, err)
	}
	if ponds, err := svc.ListPonds(support); err != nil || len(ponds) != 1 || ponds[0].ID != 701 {
		t.Fatalf("support ponds=%+v err=%v", ponds, err)
	}
	if devices, err := svc.ListDevices(support, false, 0, 50, 0); err != nil || len(devices) != 1 || devices[0].DeviceNo != "visible-device" {
		t.Fatalf("support devices=%+v err=%v", devices, err)
	}
	if _, err := svc.GetDevice(support, "hidden-device"); err == nil {
		t.Fatal("support read device in unassigned farm")
	}
	if alarms, err := svc.ListAllAlarms(support, 0); err != nil || len(alarms) != 1 || alarms[0].DeviceNo != "visible-device" {
		t.Fatalf("support alarms=%+v err=%v", alarms, err)
	}
	var visibleID, hiddenID int64
	if err := p.QueryRow(ctx, `INSERT INTO alarms(device_no,pond_id,metric,current_value,threshold,level) VALUES('visible-batch',701,'ph',6,7,'warning') RETURNING id`).Scan(&visibleID); err != nil {
		t.Fatal(err)
	}
	if err := p.QueryRow(ctx, `INSERT INTO alarms(device_no,pond_id,metric,current_value,threshold,level) VALUES('hidden-batch',702,'ph',6,7,'warning') RETURNING id`).Scan(&hiddenID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BatchConfirmByActor(support, []int64{visibleID, hiddenID}, 702); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-farm batch confirmation error=%v", err)
	}
	if _, err := svc.BatchConfirm(support, []int64{visibleID, hiddenID}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("direct batch bypassed farm permissions: %v", err)
	}
	var batchConfirmed int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM alarms WHERE id IN ($1,$2) AND confirmed_at IS NOT NULL`, visibleID, hiddenID).Scan(&batchConfirmed); err != nil || batchConfirmed != 0 {
		t.Fatalf("failed batch changed alarms count=%d err=%v", batchConfirmed, err)
	}
	if n, err := svc.BatchConfirmByActor(support, []int64{visibleID}, 702); err != nil || n != 1 {
		t.Fatalf("assigned batch confirmation count=%d err=%v", n, err)
	}
	if n, err := svc.BatchConfirmByActor(support, []int64{visibleID}, 702); err != nil || n != 0 {
		t.Fatalf("idempotent batch confirmation count=%d err=%v", n, err)
	}
	if stats, err := svc.Stats(support); err != nil || stats.DevicesTotal != 1 || stats.OpenAlarms != 0 {
		t.Fatalf("support statistics=%+v err=%v", stats, err)
	}
	if _, err := svc.BatchConfirmByActor(support, []int64{hiddenID}, 701); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("mismatched actor batch confirmation error=%v", err)
	}
	var alarmID int64
	if err := p.QueryRow(ctx, `SELECT id FROM alarms WHERE device_no='visible-device'`).Scan(&alarmID); err != nil {
		t.Fatal(err)
	}
	if err := svc.ConfirmAlarm(support, alarmID); err != nil {
		t.Fatalf("confirm already-confirmed alarm: %v", err)
	}
}
