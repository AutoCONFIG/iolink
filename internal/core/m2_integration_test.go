package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/appapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	corepkg "git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/event"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

func TestM2OwnershipLifecycleAndTokenRevocation(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	s, err := corepkg.New(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	installTestLicense(t, p, s, 100)
	if _, err = p.Exec(ctx, `INSERT INTO users(id,open_id,nickname,authority) VALUES (10,'wx-a','A','USER'),(11,'wx-b','B','USER')`); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(ctx, `WITH tenant AS (INSERT INTO tenants(name) VALUES ('m2-tenant') RETURNING id)
		INSERT INTO tenant_memberships(tenant_id,user_id,role)
		SELECT tenant.id,10,'member' FROM tenant`); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(ctx, `WITH tenant AS (INSERT INTO tenants(name) VALUES ('m2-other-tenant') RETURNING id) INSERT INTO tenant_memberships(tenant_id,user_id,role) SELECT tenant.id,11,'member' FROM tenant`); err != nil {
		t.Fatal(err)
	}
	a := int64(10)
	f, err := s.CreateFarm(ctx, &a, "shared", "lake")
	if err != nil {
		t.Fatal(err)
	}
	pond, err := s.CreatePond(ctx, f.ID, "pond-a", 1)
	if err != nil {
		t.Fatal(err)
	}
	dev, secret, err := s.RegisterDevice(ctx, pond.ID, "device-a", "model", 60)
	if err != nil || len(secret) != 64 {
		t.Fatal(err)
	}
	if s.Authenticate(dev.DeviceNo, secret) == false {
		t.Fatal("new device credential rejected")
	}
	if err := s.HandleEvent(event.Event{Kind: event.KindProperties, DeviceNo: dev.DeviceNo, Ts: time.Now().UTC(), MessageID: "m1", Properties: map[string]float64{"temperature": 26}}); err != nil {
		t.Fatal(err)
	}

	api := appapi.New(appapi.Config{SecretKey: strings.Repeat("a", 32), JWT: time.Hour}, appapi.Deps{Ponds: s.Ponds(), Devices: s.Devices(), Telemetry: s.Telemetry(), Alarms: s.Alarms(), Users: s}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	api.SetWechatExchanger(func(code string) (string, error) {
		if code == "a" {
			return "wx-a", nil
		}
		if code == "b" {
			return "wx-b", nil
		}
		return "", domain.ErrNotFound
	})
	appServer := httptest.NewServer(api.Routes())
	defer appServer.Close()
	login := func(code string) string {
		r, e := http.Post(appServer.URL+"/api/v1/auth/login", "application/json", strings.NewReader(`{"code":"`+code+`"}`))
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		var out struct {
			Token string `json:"token"`
		}
		if json.NewDecoder(r.Body).Decode(&out) != nil || out.Token == "" {
			t.Fatal("missing app token")
		}
		return out.Token
	}
	aToken, bToken := login("a"), login("b")
	var ownerTenantID int64
	if err := p.QueryRow(ctx, `SELECT tenant_id FROM farms WHERE id=$1`, f.ID).Scan(&ownerTenantID); err != nil {
		t.Fatal(err)
	}
	var ownerSelection struct{ Token string }
	m6bHTTPRequest(t, appServer.URL, aToken, http.MethodPost, "/api/v1/auth/tenant", fmt.Sprintf(`{"tenant_id":%d}`, ownerTenantID), http.StatusOK, &ownerSelection)
	aToken = ownerSelection.Token
	get := func(token, path string) *http.Response {
		req, _ := http.NewRequest("GET", appServer.URL+path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		r, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	r := get(aToken, "/api/v1/ponds/"+itoa(pond.ID))
	if r.StatusCode != 200 {
		t.Fatal("owner cannot read pond", r.StatusCode)
	}
	r.Body.Close()
	r = get(bToken, "/api/v1/ponds/"+itoa(pond.ID))
	if r.StatusCode != 404 {
		t.Fatal("foreign user read pond", r.StatusCode)
	}
	r.Body.Close()
	r = get(aToken, "/api/v1/devices/"+dev.DeviceNo)
	if r.StatusCode != 200 {
		t.Fatal("owner cannot read device", r.StatusCode)
	}
	r.Body.Close()
	r = get(bToken, "/api/v1/devices/"+dev.DeviceNo)
	if r.StatusCode != 404 {
		t.Fatal("foreign user read device", r.StatusCode)
	}
	r.Body.Close()
	if err := s.SetFarmOwner(ctx, f.ID, ptr(11)); err != nil {
		t.Fatal(err)
	}
	var auditAction, auditMetadata string
	if err := p.QueryRow(ctx, `SELECT action,metadata::text FROM audit_events WHERE resource_type='farm' AND resource_id=$1 ORDER BY id DESC LIMIT 1`, fmt.Sprint(f.ID)).Scan(&auditAction, &auditMetadata); err != nil {
		t.Fatal("owner transfer audit", err)
	}
	if auditAction != "farm.owner_changed" || !strings.Contains(auditMetadata, `"new_owner_id": 11`) {
		t.Fatalf("unexpected owner audit: %s %s", auditAction, auditMetadata)
	}
	var farmTenantID int64
	if err := p.QueryRow(ctx, `SELECT tenant_id FROM farms WHERE id=$1`, f.ID).Scan(&farmTenantID); err != nil {
		t.Fatal(err)
	}
	switchReq, _ := http.NewRequest(http.MethodPost, appServer.URL+"/api/v1/auth/tenant", strings.NewReader(fmt.Sprintf(`{"tenant_id":%d}`, farmTenantID)))
	switchReq.Header.Set("Authorization", "Bearer "+bToken)
	switchResp, err := http.DefaultClient.Do(switchReq)
	if err != nil || switchResp.StatusCode != http.StatusOK {
		if switchResp != nil {
			switchResp.Body.Close()
		}
		t.Fatalf("new owner tenant switch status=%v err=%v", switchResp.StatusCode, err)
	}
	var switched struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(switchResp.Body).Decode(&switched); err != nil {
		switchResp.Body.Close()
		t.Fatal(err)
	}
	switchResp.Body.Close()
	bToken = switched.Token
	r = get(aToken, "/api/v1/ponds/"+itoa(pond.ID))
	if r.StatusCode != 404 {
		t.Fatal("old owner retained access", r.StatusCode)
	}
	r.Body.Close()
	r = get(bToken, "/api/v1/ponds/"+itoa(pond.ID))
	if r.StatusCode != 200 {
		t.Fatal("new owner missing access", r.StatusCode)
	}
	r.Body.Close()
	if err := s.SetFarmOwner(ctx, f.ID, nil); err != nil {
		t.Fatal(err)
	}
	r = get(bToken, "/api/v1/ponds/"+itoa(pond.ID))
	if r.StatusCode != 404 {
		t.Fatal("unassigned farm visible", r.StatusCode)
	}
	r.Body.Close()

	adminPassword := "M2-admin-password-strong!"
	if err := platform.BootstrapAdmin(ctx, p, "admin", adminPassword, false); err != nil {
		t.Fatal(err)
	}
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	admin := adminapi.New(adminapi.Config{SecretKey: strings.Repeat("b", 32), JWT: time.Hour}, adminapi.Deps{Store: s, Telemetry: s.Telemetry(), Policy: policy})
	adminServer := httptest.NewServer(admin.Routes())
	defer adminServer.Close()
	loginAdmin := func() string {
		r, e := http.Post(adminServer.URL+"/admin/v1/login", "application/json", strings.NewReader(`{"username":"admin","password":"`+adminPassword+`"}`))
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		var out struct {
			Token string `json:"token"`
		}
		if json.NewDecoder(r.Body).Decode(&out) != nil || out.Token == "" {
			t.Fatal("missing admin token")
		}
		return out.Token
	}
	adminToken := loginAdmin()
	req, _ := http.NewRequest("POST", adminServer.URL+"/admin/v1/password", strings.NewReader(`{"old_password":"M2-admin-password-strong!","new_password":"M2-admin-password-new!"}`))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	r, err = http.DefaultClient.Do(req)
	if err != nil || r.StatusCode != 204 {
		t.Fatal("password change", r.StatusCode, err)
	}
	r.Body.Close()
	req, _ = http.NewRequest("GET", adminServer.URL+"/admin/v1/stats", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	r, err = http.DefaultClient.Do(req)
	if err != nil || r.StatusCode != 401 {
		t.Fatal("old admin token survived", r.StatusCode, err)
	}
	r.Body.Close()

	if err := s.MoveDevice(ctx, dev.DeviceNo, pond.ID); err != nil {
		t.Fatal(err)
	}
	r = get(bToken, "/api/v1/water/latest?device_no="+dev.DeviceNo)
	if r.StatusCode != 404 {
		t.Fatal("latest survived move", r.StatusCode)
	}
	r.Body.Close()
	if err := s.DeleteDevice(ctx, dev.DeviceNo); err != nil {
		t.Fatal(err)
	}
	if s.Authenticate(dev.DeviceNo, secret) {
		t.Fatal("disabled device authenticated")
	}
	if err := s.DeletePond(ctx, pond.ID); !errors.Is(err, domain.ErrPondHasDevices) {
		t.Fatal("pond delete did not protect disabled device", err)
	}
}

func TestCreateFarmAssignsDefaultTenantMembership(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	s, err := corepkg.New(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	installTestLicense(t, p, s, 100)
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id,nickname,authority) VALUES (20,'wx-new','New','USER')`); err != nil {
		t.Fatal(err)
	}
	ownerID := int64(20)
	farm, err := s.CreateFarm(ctx, &ownerID, "new-owner-farm", "lake")
	if err != nil {
		t.Fatal(err)
	}
	var memberships int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM tenant_memberships tm JOIN farms f ON f.tenant_id=tm.tenant_id WHERE f.id=$1 AND tm.user_id=$2`, farm.ID, ownerID).Scan(&memberships); err != nil {
		t.Fatal(err)
	}
	if memberships != 1 {
		t.Fatalf("owner memberships = %d, want 1", memberships)
	}
	if _, err := p.Exec(ctx, `INSERT INTO tenants(name) VALUES('second-owner-tenant')`); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO tenant_memberships(tenant_id,user_id,role) SELECT id,$1,'member' FROM tenants WHERE name='second-owner-tenant'`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateFarm(ctx, &ownerID, "ambiguous-owner-farm", "lake"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("multiple memberships error = %v, want conflict", err)
	}
}

func TestHistoryAndAlarmFiltersUseSnapshotOwnership(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	s, err := corepkg.New(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	installTestLicense(t, p, s, 100)
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id,nickname,authority) VALUES (30,'wx-a','A','USER'),(31,'wx-b','B','USER')`); err != nil {
		t.Fatal(err)
	}
	a, b := int64(30), int64(31)
	fa, err := s.CreateFarm(ctx, &a, "a", "")
	if err != nil {
		t.Fatal(err)
	}
	fb, err := s.CreateFarm(ctx, &b, "b", "")
	if err != nil {
		t.Fatal(err)
	}
	pa, err := s.CreatePond(ctx, fa.ID, "pa", 1)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := s.CreatePond(ctx, fb.ID, "pb", 1)
	if err != nil {
		t.Fatal(err)
	}
	dev, _, err := s.RegisterDevice(ctx, pa.ID, "d", "m", 60)
	if err != nil {
		t.Fatal(err)
	}
	oldTS := time.Now().UTC().Add(-time.Minute)
	if _, err := p.Exec(ctx, `INSERT INTO sensor_data(ts,device_no,pond_id,temperature) VALUES($1,$2,$3,25)`, oldTS, dev.DeviceNo, pa.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveDevice(ctx, dev.DeviceNo, pb.ID); err != nil {
		t.Fatal(err)
	}
	telemetry := s.Telemetry().(domain.UserTelemetryRepo)
	points, err := telemetry.HistoryForUser(ctx, dev.DeviceNo, a, "temperature", oldTS.Add(-time.Minute), oldTS.Add(time.Minute), 20)
	if err != nil || len(points) != 1 || points[0].Value != 25 {
		t.Fatalf("snapshot history = %#v, err=%v", points, err)
	}
	foreign, err := telemetry.HistoryForUser(ctx, dev.DeviceNo, b, "temperature", oldTS.Add(-time.Minute), oldTS.Add(time.Minute), 20)
	if err != nil || len(foreign) != 0 {
		t.Fatalf("foreign history = %#v, err=%v", foreign, err)
	}
	var alarmIDs []int64
	for i, pond := range []int64{pa.ID, pa.ID, pb.ID} {
		var id int64
		metric := "temperature"
		if i == 1 {
			metric = "ph"
		}
		if err := p.QueryRow(ctx, `INSERT INTO alarms(device_no,pond_id,metric,current_value,threshold,level) VALUES($1,$2,$3,30,25,'warning') RETURNING id`, dev.DeviceNo, pond, metric).Scan(&id); err != nil {
			t.Fatal(err)
		}
		alarmIDs = append(alarmIDs, id)
	}
	alarms := s.Alarms().(domain.FilteredAlarmRepo)
	visible, err := alarms.ListByUserFiltered(ctx, a, "", false, 20, 0)
	if err != nil || len(visible) != 2 {
		t.Fatalf("owner alarms = %#v, err=%v", visible, err)
	}
	if _, err := s.BatchConfirm(ctx, []int64{alarmIDs[0], 999999}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("batch missing = %v", err)
	}
	var confirmed int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM alarms WHERE id=$1 AND confirmed_at IS NOT NULL`, alarmIDs[0]).Scan(&confirmed); err != nil {
		t.Fatal(err)
	}
	if confirmed != 0 {
		t.Fatal("failed batch partially confirmed an alarm")
	}
}

func ptr(v int64) *int64  { return &v }
func itoa(v int64) string { return fmt.Sprintf("%d", v) }
