package core_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
)

type revokeBeforeConfirm struct {
	policy  domain.PermissionPolicy
	revoke  func() error
	invoked bool
}

func (p *revokeBeforeConfirm) Allow(role, resource, action string) (bool, error) {
	allowed, err := p.policy.Allow(role, resource, action)
	if err != nil || !allowed {
		return allowed, err
	}
	if resource == "alarms" && action == "confirm" && !p.invoked {
		p.invoked = true
		if err := p.revoke(); err != nil {
			return false, err
		}
	}
	return allowed, nil
}

func TestM6bAlarmHTTPRejectsRevocationAfterAuthentication(t *testing.T) {
	for _, operation := range []string{"batch", "singular"} {
		for _, change := range []string{"role", "membership", "tenant", "platform_authority", "permission_version"} {
			t.Run(operation+"_"+change, func(t *testing.T) {
				f := newTelemetryPermissionFixture(t)
				seedAppScopeReadings(t, f)
				f.role(t, "admin", true)
				if _, err := f.pool.Exec(context.Background(), `UPDATE users SET username='revocation-manager',password_hash=$1 WHERE id=9402`, platform.HashPassword("Revocation-manager-pass!")); err != nil {
					t.Fatal(err)
				}
				var id int64
				if err := f.pool.QueryRow(context.Background(), `SELECT id FROM alarms WHERE device_no='v2-hidden'`).Scan(&id); err != nil {
					t.Fatal(err)
				}
				query := map[string]string{
					"role":               `UPDATE tenant_memberships SET role='viewer',permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`,
					"membership":         `UPDATE tenant_memberships SET active=false,permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`,
					"tenant":             `UPDATE tenants SET active=false WHERE id=9401`,
					"platform_authority": `UPDATE users SET authority='ADMIN' WHERE id=9402`,
					"permission_version": `UPDATE tenant_memberships SET permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`,
				}[change]
				policy, err := authorization.New()
				if err != nil {
					t.Fatal(err)
				}
				boundary := &revokeBeforeConfirm{policy: policy, revoke: func() error {
					_, err := f.pool.Exec(context.Background(), query)
					return err
				}}
				server := httptest.NewServer(adminapi.New(adminapi.Config{SecretKey: strings.Repeat("r", 32), JWT: time.Hour}, adminapi.Deps{Store: f.svc, Policy: boundary}).Routes())
				defer server.Close()
				var login struct{ Token string }
				m6bHTTPRequest(t, server.URL, "", http.MethodPost, "/admin/v1/login", `{"username":"revocation-manager","password":"Revocation-manager-pass!"}`, 200, &login)
				path := fmt.Sprintf("/admin/v1/alarms/%d/confirm", id)
				body := ""
				if operation == "batch" {
					path = "/admin/v1/alarms/batch-confirm"
					body = fmt.Sprintf(`{"ids":[%d]}`, id)
				}
				before := f.snapshot(t)
				req, err := http.NewRequest(http.MethodPost, server.URL+path, strings.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer "+login.Token)
				req.Header.Set("Content-Type", "application/json")
				response, err := server.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				raw, err := io.ReadAll(response.Body)
				if err != nil {
					t.Fatal(err)
				}
				after := f.snapshot(t)
				t.Logf("operation=%s revocation=%s injected=%t status=%d response=%s unchanged=%t", operation, change, boundary.invoked, response.StatusCode, raw, before == after)
				if !boundary.invoked || response.StatusCode != http.StatusNotFound || before != after {
					t.Fatalf("write after revocation: status=%d changed=%t", response.StatusCode, before != after)
				}
			})
		}
	}
}
