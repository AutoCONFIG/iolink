package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	corepkg "git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPlatformConsoleOnboardingAndIsolation(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := (platform.WebBootstrap{Pool: p}).Initialize(ctx, "operator", "Operator-console-1234"); err != nil {
		t.Fatal(err)
	}
	var businessTenants, users, memberships int
	if err := p.QueryRow(ctx, `SELECT (SELECT count(*) FROM tenants WHERE name<>'__iolink_system__'),(SELECT count(*) FROM users WHERE authority='USER'),(SELECT count(*) FROM tenant_memberships)`).Scan(&businessTenants, &users, &memberships); err != nil {
		t.Fatal(err)
	}
	if businessTenants != 0 || users != 0 || memberships != 0 {
		t.Fatalf("automatic provisioning: tenants=%d users=%d memberships=%d", businessTenants, users, memberships)
	}
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc, err := corepkg.NewWithPolicy(ctx, p, logger, policy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EnsureUser(ctx, "unassigned-wechat"); err != nil {
		t.Fatal(err)
	}
	if err := p.QueryRow(ctx, `SELECT count(*) FROM tenant_memberships`).Scan(&memberships); err != nil || memberships != 0 {
		t.Fatalf("registration granted membership: %d %v", memberships, err)
	}
	ts := httptest.NewServer(adminapi.New(adminapi.Config{SecretKey: "console-test-secret", JWT: time.Hour}, adminapi.Deps{Store: svc, Policy: policy, Logger: logger}).Routes())
	defer ts.Close()
	call := func(method, path, token, body string, want int) []byte {
		t.Helper()
		request, err := http.NewRequest(method, ts.URL+"/admin/v1"+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s status=%d want=%d", method, path, response.StatusCode, want)
		}
		return raw
	}
	login := func(username, password string) string {
		t.Helper()
		raw := call("POST", "/login", "", fmt.Sprintf(`{"username":%q,"password":%q}`, username, password), 200)
		var result struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(raw, &result); err != nil || result.Token == "" {
			t.Fatal("login response invalid")
		}
		return result.Token
	}
	admin := login("operator", "Operator-console-1234")
	raw := call("GET", "/session", admin, "", 200)
	var session struct {
		PlatformAdmin bool  `json:"platform_admin"`
		TenantID      int64 `json:"tenant_id"`
	}
	if err := json.Unmarshal(raw, &session); err != nil || !session.PlatformAdmin || session.TenantID != 0 {
		t.Fatalf("platform session=%+v %v", session, err)
	}
	if raw := string(call("GET", "/tenants", admin, "", 200)); raw != "[]" {
		t.Fatalf("internal tenant visible: %s", raw)
	}
	call("GET", "/stats", admin, "", 403)
	call("POST", "/register", "", `{"username":"customer","password":"Customer-console-1234"}`, 201)
	call("POST", "/register", "", `{"username":"customer","password":"Customer-console-1234"}`, 409)
	call("POST", "/register", "", `{"username":"short","password":"short"}`, 400)
	call("POST", "/register", "", `{"username":"intruder","password":"Customer-console-1234","authority":"ADMIN"}`, 400)
	idle := login("customer", "Customer-console-1234")
	call("GET", "/stats", idle, "", 403)
	call("GET", "/platform/stats", idle, "", 403)
	call("GET", "/platform/users", idle, "", 403)
	if raw := string(call("GET", "/tenants", idle, "", 200)); raw != "[]" {
		t.Fatal("unassigned account sees tenants")
	}
	call("PUT", "/tenants/1/members/1", idle, `{"role":"owner","active":true}`, 403)
	call("POST", "/tenants", idle, `{"name":"unauthorized"}`, 403)
	var tenant domain.Tenant
	raw = call("POST", "/tenants", admin, `{"name":"customer-org"}`, 201)
	if err := json.Unmarshal(raw, &tenant); err != nil || tenant.ID == 0 {
		t.Fatal("tenant response invalid")
	}
	if raw := string(call("GET", fmt.Sprintf("/tenants/%d/members", tenant.ID), admin, "", 200)); raw != "[]" {
		t.Fatalf("empty tenant members must be an array: %s", raw)
	}
	call("POST", "/tenants", admin, `{"name":"customer-org"}`, 409)
	call("POST", "/tenants", admin, `{"name":"__iolink_system__"}`, 400)
	var customerID, adminID int64
	if err := p.QueryRow(ctx, `SELECT id FROM users WHERE username='customer'`).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if err := p.QueryRow(ctx, `SELECT id FROM users WHERE username='operator'`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	call("PUT", fmt.Sprintf("/tenants/%d/members/%d", tenant.ID, adminID), admin, `{"role":"owner","active":true}`, 400)
	call("PUT", fmt.Sprintf("/tenants/%d/members/%d", tenant.ID, customerID), admin, `{"role":"owner","active":true}`, 204)
	call("GET", "/session", idle, "", 401)
	customer := login("customer", "Customer-console-1234")
	call("GET", "/stats", customer, "", 200)
	call("GET", fmt.Sprintf("/tenants/%d/members", tenant.ID), admin, "", 200)
	call("GET", "/tenants/99999/members", customer, "", 404)
	call("GET", "/platform/stats", customer, "", 403)
	platformCtx := domain.WithPlatformActor(ctx, domain.PlatformActor{ID: adminID, TokenVersion: 0})
	other, err := svc.CreateTenant(platformCtx, "other-org", adminID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO farms(tenant_id,owner_id,name) VALUES($1,$2,'owned'),($3,NULL,'foreign')`, tenant.ID, customerID, other.ID); err != nil {
		t.Fatal(err)
	}
	var farms []domain.Farm
	if err := json.Unmarshal(call("GET", "/farms", customer, "", 200), &farms); err != nil || len(farms) != 1 || farms[0].Name != "owned" {
		t.Fatalf("tenant dashboard resources=%+v err=%v", farms, err)
	}
	var stats domain.PlatformStats
	if err := json.Unmarshal(call("GET", "/platform/stats", admin, "", 200), &stats); err != nil || stats.Tenants != 2 || stats.Users != 2 || stats.ActiveUsers != 1 {
		t.Fatalf("platform stats=%+v err=%v", stats, err)
	}
	if _, err := svc.PlatformStats(ctx); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("anonymous aggregate access=%v", err)
	}
	if _, err := p.Exec(ctx, `CREATE FUNCTION reject_console_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action IN ('tenant.created','user.registered','tenant.member_changed') THEN RAISE EXCEPTION 'audit unavailable'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_console BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION reject_console_audit()`); err != nil {
		t.Fatal(err)
	}
	call("PUT", fmt.Sprintf("/tenants/%d/members/%d", tenant.ID, customerID), admin, `{"role":"viewer","active":true}`, 500)
	call("GET", "/stats", customer, "", 200)
	call("POST", "/tenants", admin, `{"name":"rollback-org"}`, 500)
	call("POST", "/register", "", `{"username":"rollback-user","password":"Customer-console-1234"}`, 500)
	var partial int
	if err := p.QueryRow(ctx, `SELECT (SELECT count(*) FROM tenants WHERE name='rollback-org')+(SELECT count(*) FROM users WHERE username='rollback-user')`).Scan(&partial); err != nil || partial != 0 {
		t.Fatalf("partial state=%d %v", partial, err)
	}
	call("PUT", fmt.Sprintf("/tenants/%d/status", tenant.ID), admin, `{"active":false}`, 204)
	call("GET", "/stats", customer, "", 401)
	call("POST", "/login", "", `{"username":"customer","password":"Customer-console-1234"}`, 401)
	call("GET", "/stats", admin, "", 403)
}

func TestCLISetupCreatesOnlyPlatformAdmin(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := platform.SetupInit(ctx, p, platform.SetupInput{PlatformUsername: "operator", PlatformPassword: "Operator-cli-1234"}); err != nil {
		t.Fatal(err)
	}
	var adminCount int
	var passwordHash string
	if err := p.QueryRow(ctx, `SELECT count(*),coalesce(max(password_hash),'') FROM users WHERE authority='ADMIN' AND username='operator'`).Scan(&adminCount, &passwordHash); err != nil || adminCount != 1 || !platform.CheckPassword(passwordHash, "Operator-cli-1234") {
		t.Fatalf("CLI administrator creation failed: count=%d err=%v", adminCount, err)
	}
	var count int
	if err := p.QueryRow(ctx, `SELECT (SELECT count(*) FROM tenants WHERE name<>'__iolink_system__')+(SELECT count(*) FROM users WHERE authority='USER')+(SELECT count(*) FROM tenant_memberships)`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("CLI auto provisioned=%d %v", count, err)
	}
}
