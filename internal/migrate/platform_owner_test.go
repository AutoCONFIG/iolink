package migrate

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
)

func TestM6bLegacyPlatformOwnerBusinessBoundary(t *testing.T) {
	// Given: pre-M6b farms owned by an ADMIN and a USER in the same tenant.
	p := testDB(t)
	ctx := context.Background()
	ms, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(ctx, p, ms[:9], false); err != nil {
		t.Fatal(err)
	}
	passwordHash := platform.HashPassword("M6b-owner-password!")
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id,username,password_hash,authority) VALUES(9201,'legacy-platform','legacy-platform',$1,'ADMIN'),(9202,'legacy-user','legacy-user',$1,'USER')`, passwordHash); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO farms(id,tenant_id,owner_id,name) SELECT 9201,id,9201,'platform-farm' FROM tenants WHERE name='__iolink_system__'`); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO farms(id,tenant_id,owner_id,name) SELECT 9202,id,9202,'user-farm' FROM tenants WHERE name='__iolink_system__'`); err != nil {
		t.Fatal(err)
	}
	// When: apply the real migration then log in through the admin HTTP surface.
	if err := apply(ctx, p, ms, false); err != nil {
		t.Fatal(err)
	}
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := core.NewWithPolicy(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)), policy)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(adminapi.New(adminapi.Config{SecretKey: strings.Repeat("s", 32), JWT: time.Hour}, adminapi.Deps{Store: svc, Policy: policy}).Routes())
	defer server.Close()
	// Then: platform lifecycle login is usable, business reads are forbidden.
	adminToken := ownerLogin(t, server.URL, "legacy-platform")
	ownerHTTP(t, server.URL, adminToken, http.MethodGet, "/admin/v1/farms", "", http.StatusForbidden, nil)
	var counts struct{ Tenant, Farm, Reconciled, Preserved int }
	if err := p.QueryRow(ctx, `SELECT (SELECT count(*) FROM tenant_memberships WHERE user_id=9201),(SELECT count(*) FROM farm_memberships WHERE user_id=9201),(SELECT count(*) FROM tenant_migration_reconciliation WHERE migration_name='010_tenant_rbac' AND resource_type='farm' AND resource_id='9201' AND resolution='platform_admin_owner_preserved_without_membership'),(SELECT count(*) FROM farms WHERE id=9201 AND owner_id=9201)`).Scan(&counts.Tenant, &counts.Farm, &counts.Reconciled, &counts.Preserved); err != nil {
		t.Fatal(err)
	}
	if counts.Tenant != 0 || counts.Farm != 0 || counts.Reconciled != 1 || counts.Preserved != 1 {
		t.Fatalf("platform migration counts=%+v", counts)
	}
	t.Logf("migration platform memberships=0 reconciliation=1 legacy farm/owner preserved=1")
	userToken := ownerLogin(t, server.URL, "legacy-user")
	var farms []domain.Farm
	ownerHTTP(t, server.URL, userToken, http.MethodGet, "/admin/v1/farms", "", http.StatusOK, &farms)
	if len(farms) != 2 {
		t.Fatalf("USER owner tenant farms=%+v", farms)
	}
	var tenantID int64
	if err := p.QueryRow(ctx, `SELECT tenant_id FROM farms WHERE id=9202`).Scan(&tenantID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"tenant_memberships", "farm_memberships"} {
		var count int
		if err := p.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE user_id=9202 AND role='owner'`).Scan(&count); err != nil || count != 1 {
			t.Fatalf("USER %s owners=%d err=%v", table, count, err)
		}
	}
	// Explicit support only sees assigned farms, and revocation/expiry rejects its old token.
	expiry := time.Now().Add(time.Hour)
	if err := svc.SetTenantMember(ctx, tenantID, 9201, "support", true, &expiry, 9202); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetFarmMember(domain.WithTenantID(ctx, tenantID), 9202, 9201, "support", true, &expiry, 9202); err != nil {
		t.Fatal(err)
	}
	token := ownerLogin(t, server.URL, "legacy-platform")
	ownerHTTP(t, server.URL, token, http.MethodGet, "/admin/v1/farms", "", http.StatusOK, &farms)
	if len(farms) != 1 || farms[0].ID != 9202 {
		t.Fatalf("support farms=%+v; legacy owner must not confer assignment", farms)
	}
	if _, err := p.Exec(ctx, `UPDATE tenant_memberships SET expires_at=now()-interval '1 minute' WHERE user_id=9201`); err != nil {
		t.Fatal(err)
	}
	ownerHTTP(t, server.URL, token, http.MethodGet, "/admin/v1/farms", "", http.StatusUnauthorized, nil)
	token = ownerLogin(t, server.URL, "legacy-platform")
	ownerHTTP(t, server.URL, token, http.MethodGet, "/admin/v1/farms", "", http.StatusForbidden, nil)
	if err := svc.SetTenantMember(ctx, tenantID, 9201, "support", true, &expiry, 9202); err != nil {
		t.Fatal(err)
	}
	token = ownerLogin(t, server.URL, "legacy-platform")
	if err := svc.SetTenantMember(ctx, tenantID, 9201, "support", false, &expiry, 9202); err != nil {
		t.Fatal(err)
	}
	ownerHTTP(t, server.URL, token, http.MethodGet, "/admin/v1/farms", "", http.StatusUnauthorized, nil)
	var audits int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action='tenant.member_changed' AND resource_id='9201'`).Scan(&audits); err != nil || audits != 3 {
		t.Fatalf("support audit count=%d err=%v", audits, err)
	}
}

func ownerLogin(t *testing.T, baseURL, username string) string {
	t.Helper()
	var output struct{ Token string }
	ownerHTTP(t, baseURL, "", http.MethodPost, "/admin/v1/login", `{"username":"`+username+`","password":"M6b-owner-password!"}`, http.StatusOK, &output)
	if output.Token == "" {
		t.Fatal("login returned no token")
	}
	return output.Token
}
func ownerHTTP(t *testing.T, baseURL, token, method, path, body string, want int, output any) {
	t.Helper()
	req, err := http.NewRequest(method, baseURL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	t.Logf("%s %s status=%d expected=%d", method, path, response.StatusCode, want)
	if response.StatusCode != want {
		t.Fatalf("%s %s status=%d want=%d", method, path, response.StatusCode, want)
	}
	if output != nil {
		if err := json.NewDecoder(response.Body).Decode(output); err != nil {
			t.Fatal(err)
		}
	}
}
