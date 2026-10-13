package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

type platformContractFixture struct {
	svc                             *core.Service
	logger                          *slog.Logger
	pool                            *pgxpool.Pool
	server                          *httptest.Server
	admin, owner, member            string
	tenant, user, otherTenant, farm int64
	sink                            *observability.Sink
	dumpPath, requestID             string
}

type platformContractRequest struct {
	method, route, path, token, body string
	status                           int
}

func newPlatformContractFixture(t *testing.T) *platformContractFixture {
	t.Helper()
	p := testdb.New(t)
	if err := migrate.Up(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if err := (platform.WebBootstrap{Pool: p}).Initialize(context.Background(), "platform-contract-admin", "Platform-contract-only-1234"); err != nil {
		t.Fatal(err)
	}
	if count(t, p, "SELECT count(*) FROM tenants WHERE name<>'__iolink_system__'") != 0 || count(t, p, "SELECT count(*) FROM tenant_memberships") != 0 {
		t.Fatal("initialization created a business tenant or membership")
	}
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	dumpRoot := t.TempDir()
	logger, sink, err := observability.New(observability.Config{Dir: dumpRoot, Version: "platform-contract", Level: "error", MaxMB: 1, Backups: 1}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sink.Close(); err != nil {
			t.Error(err)
		}
	})
	svc, err := core.NewWithPolicy(context.Background(), p, logger, policy)
	if err != nil {
		t.Fatal(err)
	}
	admin := adminapi.New(adminapi.Config{SecretKey: strings.Repeat("p", 32), JWT: time.Hour}, adminapi.Deps{Store: svc, Policy: policy, Logger: logger})
	mux := http.NewServeMux()
	mux.Handle("/admin/", admin.Routes())
	mux.Handle("/user/", admin.UserRoutes())
	f := &platformContractFixture{pool: p, svc: svc, logger: logger, server: httptest.NewServer(mux), sink: sink, dumpPath: filepath.Join(dumpRoot, "platform-contract", "iolinkd.jsonl")}
	t.Cleanup(f.server.Close)
	f.admin = f.login(t, "platform-contract-admin", "Platform-contract-only-1234", true)
	return f
}

func (f *platformContractFixture) request(t *testing.T, tc platformContractRequest) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, tc.method, f.server.URL+tc.path, strings.NewReader(tc.body))
	if err != nil {
		t.Fatal("HTTP request construction failed")
	}
	req.Header.Set("Content-Type", "application/json")
	if tc.token != "" {
		req.Header.Set("Authorization", "Bearer "+tc.token)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal("HTTP request failed")
	}
	raw, err := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if err != nil || closeErr != nil {
		t.Fatal("HTTP response read failed")
	}
	if resp.StatusCode != tc.status {
		t.Errorf("%s %s status=%d want=%d", tc.method, tc.route, resp.StatusCode, tc.status)
	}
	f.requestID = resp.Header.Get("X-Request-ID")
	t.Run("response_contract", func(t *testing.T) {
		assertHTTPContract(t, "admin", tc.method, tc.route, resp.StatusCode, resp.Header.Get("Content-Type"), raw)
	})
	return raw
}

func platformContractDecode(t *testing.T, raw []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal("HTTP response decode failed")
	}
}

func (f *platformContractFixture) login(t *testing.T, username, password string, platformAdmin bool) string {
	t.Helper()
	base := "/user/v1"
	if platformAdmin {
		base = "/admin/v1"
	}
	var result struct {
		Token string `json:"token"`
	}
	platformContractDecode(t, f.request(t, platformContractRequest{"POST", "/login", base + "/login", "", fmt.Sprintf(`{"username":%q,"password":%q}`, username, password), 200}), &result)
	if result.Token == "" {
		t.Fatal("fixture login returned no token")
	}
	return result.Token
}

func (f *platformContractFixture) register(t *testing.T, username string) int64 {
	t.Helper()
	f.request(t, platformContractRequest{"POST", "/register", "/admin/v1/register", "", fmt.Sprintf(`{"username":%q,"password":"Platform-user-only-1234"}`, username), 201})
	var id int64
	if err := f.pool.QueryRow(context.Background(), "SELECT id FROM users WHERE username=$1", username).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f *platformContractFixture) createTenant(t *testing.T, name string) int64 {
	t.Helper()
	var result struct {
		ID int64 `json:"id"`
	}
	platformContractDecode(t, f.request(t, platformContractRequest{"POST", "/tenants", "/admin/v1/tenants", f.admin, fmt.Sprintf(`{"name":%q}`, name), 201}), &result)
	if result.ID == 0 {
		t.Fatal("fixture tenant creation failed")
	}
	return result.ID
}

func (f *platformContractFixture) assign(t *testing.T, user int64, role string) {
	t.Helper()
	f.request(t, platformContractRequest{"PUT", "/tenants/{id}/members/{user_id}", fmt.Sprintf("/admin/v1/tenants/%d/members/%d", f.tenant, user), f.admin, fmt.Sprintf(`{"role":%q,"active":true,"expires_at":null}`, role), 204})
}

func newPlatformContractMembershipFixture(t *testing.T) *platformContractFixture {
	t.Helper()
	f := newPlatformContractFixture(t)
	owner := f.register(t, "platform-owner")
	f.user = f.register(t, "platform-member")
	f.tenant = f.createTenant(t, "platform-owned-org")
	f.otherTenant = f.createTenant(t, "platform-foreign-org")
	f.assign(t, owner, "owner")
	f.assign(t, f.user, "viewer")
	f.owner = f.login(t, "platform-owner", "Platform-user-only-1234", false)
	f.member = f.login(t, "platform-member", "Platform-user-only-1234", false)
	var farm struct {
		ID int64 `json:"id"`
	}
	platformContractDecode(t, f.request(t, platformContractRequest{"POST", "/farms", "/user/v1/farms", f.owner, fmt.Sprintf(`{"name":"platform-member-farm","owner_id":%d}`, owner), 200}), &farm)
	f.farm = farm.ID
	return f
}

func (f *platformContractFixture) state(t *testing.T) string {
	t.Helper()
	var digest string
	err := f.pool.QueryRow(context.Background(), `SELECT md5(jsonb_build_object(
		'users',(SELECT jsonb_agg(jsonb_build_array(id,username,authority,token_version) ORDER BY id) FROM users),
		'tenants',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM tenants t),
		'tenant_members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY tenant_id,user_id) FROM tenant_memberships m),
		'farm_members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY farm_id,user_id) FROM farm_memberships m),
		'audits',(SELECT count(*) FROM audit_events)
	)::text)`).Scan(&digest)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
