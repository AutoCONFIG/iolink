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
	if _, err = p.Exec(ctx, `INSERT INTO users(id,open_id,nickname,authority) VALUES (10,'wx-a','A','USER'),(11,'wx-b','B','USER')`); err != nil {
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
	admin := adminapi.New(adminapi.Config{SecretKey: strings.Repeat("b", 32), JWT: time.Hour}, adminapi.Deps{Store: s, Telemetry: s.Telemetry()})
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

func ptr(v int64) *int64  { return &v }
func itoa(v int64) string { return fmt.Sprintf("%d", v) }
