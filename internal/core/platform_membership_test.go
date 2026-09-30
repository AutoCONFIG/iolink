package core_test

import (
	"context"
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
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	corepkg "git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
	"github.com/golang-jwt/jwt/v5"
)

func TestM6bStoredPlatformMembershipFailsClosed(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id,username,password_hash,authority) VALUES(9301,'platform-stored','platform-stored',$1,'ADMIN')`, platform.HashPassword("M6b-http-password!")); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id) VALUES(9302,'user-stored'); INSERT INTO tenants(id,name) VALUES(9301,'stored'); INSERT INTO farms(id,owner_id,tenant_id,name) VALUES(9301,9302,9301,'farm'); INSERT INTO tenant_memberships(tenant_id,user_id,role) VALUES(9301,9301,'owner'),(9301,9302,'owner')`); err != nil {
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
	key := strings.Repeat("s", 32)
	server := httptest.NewServer(adminapi.New(adminapi.Config{SecretKey: key, JWT: time.Hour}, adminapi.Deps{Store: svc, Policy: policy}).Routes())
	defer server.Close()
	for _, role := range []string{"owner", "admin", "member", "viewer"} {
		t.Run(role, func(t *testing.T) {
			// Given: an invalid legacy ADMIN ordinary membership and matching old JWT.
			if _, err := p.Exec(ctx, `UPDATE tenant_memberships SET role=$1 WHERE user_id=9301`, role); err != nil {
				t.Fatal(err)
			}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"aid": 9301, "ver": 0, "tenant_id": 9301, "tenant_ver": 0, "tenant_role": role, "exp": time.Now().Add(time.Hour).Unix()}).SignedString(platform.DeriveAdminKey(key))
			if err != nil {
				t.Fatal(err)
			}
			// When/Then: every auth read fails closed, listing/switching cannot expose this tenant.
			if _, err := svc.DefaultTenantForUser(ctx, 9301); !errors.Is(err, domain.ErrNotFound) {
				t.Errorf("ADMIN default tenant role=%s err=%v", role, err)
			}
			if _, err := svc.TenantMembershipVersion(ctx, 9301, 9301); err == nil {
				t.Error("ADMIN ordinary membership version accepted")
			}
			if _, err := svc.TenantRole(ctx, 9301, 9301); err == nil {
				t.Error("ADMIN ordinary tenant role accepted")
			}
			tenants, err := svc.ListUserTenants(ctx, 9301)
			if err != nil || len(tenants) != 0 {
				t.Errorf("ADMIN selectable tenants=%+v err=%v", tenants, err)
			}
			m6bHTTPRequest(t, server.URL, token, http.MethodGet, "/admin/v1/farms", "", http.StatusUnauthorized, nil)
			var login struct{ Token string }
			m6bHTTPRequest(t, server.URL, "", http.MethodPost, "/admin/v1/login", `{"username":"platform-stored","password":"M6b-http-password!"}`, http.StatusOK, &login)
			m6bHTTPRequest(t, server.URL, login.Token, http.MethodGet, "/admin/v1/farms", "", http.StatusForbidden, nil)
			t.Logf("ADMIN stored role=%s old token=401 fresh login business read=403", role)
		})
	}
	for _, scoped := range []bool{false, true} {
		t.Run(fmt.Sprintf("owner_assignment_scoped_%t", scoped), func(t *testing.T) {
			actorCtx := ctx
			if scoped {
				actorCtx = domain.WithTenantID(ctx, 9301)
			}
			if err := svc.SetFarmOwnerByActor(actorCtx, 9301, ptr(9301), 9302); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("platform farm owner assignment err=%v", err)
			}
			var owner int64
			if err := p.QueryRow(ctx, `SELECT owner_id FROM farms WHERE id=9301`).Scan(&owner); err != nil || owner != 9302 {
				t.Fatalf("failed assignment changed owner=%d err=%v", owner, err)
			}
		})
	}
}

func TestM6bEnsurePlatformUserDoesNotCreateOrdinaryMembership(t *testing.T) {
	// Given: a global platform identity without tenant membership.
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id,authority) VALUES(9401,'platform-ensure','ADMIN')`); err != nil {
		t.Fatal(err)
	}
	svc, err := corepkg.New(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	// When: existing identity passes through WeChat user initialization.
	if _, err := svc.EnsureUser(ctx, "platform-ensure"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("EnsureUser platform identity err=%v", err)
	}
	// Then: no ordinary membership is created for the platform identity.
	var count int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM tenant_memberships WHERE user_id=9401`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("platform membership count=%d err=%v", count, err)
	}
}
