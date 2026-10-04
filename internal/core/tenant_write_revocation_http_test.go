package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	corepkg "git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
)

type reviewerRevokeFarmPolicy struct {
	policy  domain.PermissionPolicy
	revoke  func() error
	invoked bool
}

type reviewerRevokingMemberStore struct {
	*corepkg.Service
	revoke  func() error
	invoked bool
}

func (s *reviewerRevokingMemberStore) SetTenantMember(ctx context.Context, tenantID, userID int64, role string, active bool, expiresAt *time.Time, actorID int64) error {
	s.invoked = true
	if err := s.revoke(); err != nil {
		return err
	}
	return s.Service.SetTenantMember(ctx, tenantID, userID, role, active, expiresAt, actorID)
}

func TestReviewerMemberGrantRejectsCommittedActorRevocation(t *testing.T) {
	for _, change := range []string{"role", "membership", "tenant", "platform_authority"} {
		t.Run(change, func(t *testing.T) {
			f := newTelemetryPermissionFixture(t)
			f.role(t, "admin", true)
			if _, err := f.pool.Exec(context.Background(), `UPDATE users SET username='review-member-manager',password_hash=$1 WHERE id=9402`, platform.HashPassword("Reviewer-local-test-pass!")); err != nil {
				t.Fatal(err)
			}
			queries := map[string]string{
				"role":               `UPDATE tenant_memberships SET role='viewer',permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`,
				"membership":         `UPDATE tenant_memberships SET active=false,permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`,
				"tenant":             `UPDATE tenants SET active=false WHERE id=9401`,
				"platform_authority": `UPDATE users SET authority='ADMIN' WHERE id=9402`,
			}
			policy, err := authorization.New()
			if err != nil {
				t.Fatal(err)
			}
			store := &reviewerRevokingMemberStore{Service: f.svc, revoke: func() error {
				_, err := f.pool.Exec(context.Background(), queries[change])
				return err
			}}
			server := httptest.NewServer(adminapi.New(adminapi.Config{SecretKey: strings.Repeat("y", 32), JWT: time.Hour}, adminapi.Deps{Store: store, Policy: policy}).Routes())
			defer server.Close()
			var login struct{ Token string }
			m6bHTTPRequest(t, server.URL, "", http.MethodPost, "/admin/v1/login", `{"username":"review-member-manager","password":"Reviewer-local-test-pass!"}`, 200, &login)
			var before string
			if err := f.pool.QueryRow(context.Background(), `SELECT role FROM tenant_memberships WHERE tenant_id=9401 AND user_id=9401`).Scan(&before); err != nil {
				t.Fatal(err)
			}
			status, body := appScopeHTTP(t, server, appScopeRequest{http.MethodPut, "/admin/v1/tenants/9401/members/9401", `{"role":"viewer","active":true}`, login.Token})
			var after string
			if err := f.pool.QueryRow(context.Background(), `SELECT role FROM tenant_memberships WHERE tenant_id=9401 AND user_id=9401`).Scan(&after); err != nil {
				t.Fatal(err)
			}
			t.Logf("source=9120469 member_change=%s injected=%t status=%d before=%q after=%q response=%s", change, store.invoked, status, before, after, body)
			if !store.invoked || status < 400 || before != after {
				t.Fatalf("committed actor revocation did not prevent member grant: change=%s status=%d changed=%t", change, status, before != after)
			}
		})
	}
}

func (p *reviewerRevokeFarmPolicy) Allow(role, resource, action string) (bool, error) {
	allowed, err := p.policy.Allow(role, resource, action)
	if err != nil || !allowed {
		return allowed, err
	}
	if resource == "farms" && action == "write" && !p.invoked {
		p.invoked = true
		if err := p.revoke(); err != nil {
			return false, err
		}
	}
	return allowed, nil
}

func TestReviewerFarmUpdateRejectsCommittedRevocation(t *testing.T) {
	for _, change := range []string{"role", "membership", "tenant", "platform_authority"} {
		t.Run(change, func(t *testing.T) {
			// Given: an authenticated manager followed by revocation at the permission boundary.
			f := newTelemetryPermissionFixture(t)
			f.role(t, "admin", true)
			if _, err := f.pool.Exec(context.Background(), `UPDATE users SET username='review-farm-manager',password_hash=$1 WHERE id=9402`, platform.HashPassword("Reviewer-local-test-pass!")); err != nil {
				t.Fatal(err)
			}
			queries := map[string]string{
				"role":               `UPDATE tenant_memberships SET role='viewer',permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`,
				"membership":         `UPDATE tenant_memberships SET active=false,permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`,
				"tenant":             `UPDATE tenants SET active=false WHERE id=9401`,
				"platform_authority": `UPDATE users SET authority='ADMIN' WHERE id=9402`,
			}
			policy, err := authorization.New()
			if err != nil {
				t.Fatal(err)
			}
			boundary := &reviewerRevokeFarmPolicy{policy: policy, revoke: func() error {
				_, err := f.pool.Exec(context.Background(), queries[change])
				return err
			}}
			server := httptest.NewServer(adminapi.New(adminapi.Config{SecretKey: strings.Repeat("x", 32), JWT: time.Hour}, adminapi.Deps{Store: f.svc, Policy: boundary}).Routes())
			defer server.Close()
			var login struct{ Token string }
			m6bHTTPRequest(t, server.URL, "", http.MethodPost, "/admin/v1/login", `{"username":"review-farm-manager","password":"Reviewer-local-test-pass!"}`, 200, &login)
			var before string
			if err := f.pool.QueryRow(context.Background(), `SELECT name FROM farms WHERE id=9401`).Scan(&before); err != nil {
				t.Fatal(err)
			}
			// When: the real HTTP write proceeds after the database revocation committed.
			status, body := appScopeHTTP(t, server, appScopeRequest{http.MethodPut, "/admin/v1/farms/9401", `{"name":"revoked-manager-write","location":"review"}`, login.Token})
			var after string
			if err := f.pool.QueryRow(context.Background(), `SELECT name FROM farms WHERE id=9401`).Scan(&after); err != nil {
				t.Fatal(err)
			}
			// Then: rejection must leave the farm unchanged.
			t.Logf("source=9120469 change=%s injected=%t status=%d before=%q after=%q response=%s", change, boundary.invoked, status, before, after, body)
			if !boundary.invoked || status < 400 || before != after {
				t.Fatalf("committed revocation did not prevent mutation: change=%s status=%d changed=%t", change, status, before != after)
			}
		})
	}
}
