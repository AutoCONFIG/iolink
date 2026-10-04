package core_test

import (
	"context"
	"fmt"
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

type revokingProductPolicy struct {
	policy   domain.PermissionPolicy
	revoke   func() error
	resource string
	invoked  bool
}

func (p *revokingProductPolicy) Allow(role, resource, action string) (bool, error) {
	allowed, err := p.policy.Allow(role, resource, action)
	if err == nil && allowed && resource == p.resource && action == "write" && !p.invoked {
		p.invoked = true
		err = p.revoke()
	}
	return allowed, err
}

func TestM6bProductWritesRejectRevocationAfterMiddleware(t *testing.T) {
	for _, operation := range []string{"create_product", "create_model", "publish_model", "assign_device"} {
		for _, change := range []string{"role", "membership", "tenant", "platform_authority", "permission_version"} {
			t.Run(operation+"/"+change, func(t *testing.T) {
				// Given: valid catalog fixtures and an actor revoked after middleware allows the request.
				f := newTelemetryPermissionFixture(t)
				f.role(t, "admin", true)
				if _, err := f.pool.Exec(context.Background(), `UPDATE users SET username='product-manager',password_hash=$1 WHERE id=9402`, platform.HashPassword("Local-product-test-pass!")); err != nil {
					t.Fatal(err)
				}
				catalog := f.svc.Products()
				product, err := catalog.CreateProduct(context.Background(), 9401, "revocation-product")
				if err != nil {
					t.Fatal(err)
				}
				fields := []domain.ModelField{{Identifier: "temperature", Type: "number", Readable: true}}
				if _, err := catalog.CreateProductModel(context.Background(), 9401, product.ID, 1, fields); err != nil {
					t.Fatal(err)
				}
				if err := catalog.PublishProductModel(context.Background(), 9401, product.ID, 1); err != nil {
					t.Fatal(err)
				}
				if _, err := catalog.CreateProductModel(context.Background(), 9401, product.ID, 2, fields); err != nil {
					t.Fatal(err)
				}
				queries := map[string]string{
					"role":               `UPDATE tenant_memberships SET role='viewer',permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`,
					"membership":         `UPDATE tenant_memberships SET active=false,permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`,
					"tenant":             `UPDATE tenants SET active=false WHERE id=9401`,
					"platform_authority": `UPDATE users SET authority='ADMIN' WHERE id=9402`,
					"permission_version": `UPDATE tenant_memberships SET permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`,
				}
				policy, err := authorization.New()
				if err != nil {
					t.Fatal(err)
				}
				boundary := &revokingProductPolicy{policy: policy, resource: "products", revoke: func() error { _, err := f.pool.Exec(context.Background(), queries[change]); return err }}
				method, path, body := http.MethodPost, "/admin/v1/products", `{"name":"after-revocation"}`
				switch operation {
				case "create_model":
					path = fmt.Sprintf("/admin/v1/products/%d/models", product.ID)
					body = `{"fields":[{"identifier":"temperature","type":"number","readable":true}]}`
				case "publish_model":
					path = fmt.Sprintf("/admin/v1/products/%d/models/2/publish", product.ID)
					body = ""
				case "assign_device":
					method = http.MethodPut
					path = "/admin/v1/devices/v2-permission/product"
					body = fmt.Sprintf(`{"product_id":%d,"model_version":1}`, product.ID)
					boundary.resource = "devices"
				}
				server := httptest.NewServer(adminapi.New(adminapi.Config{SecretKey: strings.Repeat("p", 32), JWT: time.Hour}, adminapi.Deps{Store: f.svc, Catalog: catalog, Policy: boundary}).Routes())
				defer server.Close()
				var login struct{ Token string }
				m6bHTTPRequest(t, server.URL, "", http.MethodPost, "/admin/v1/login", `{"username":"product-manager","password":"Local-product-test-pass!"}`, 200, &login)
				before := productWriteSnapshot(t, f)
				// When: the handler attempts the mutation using the already authenticated context.
				status, response := appScopeHTTP(t, server, appScopeRequest{method, path, body, login.Token})
				// Then: forbidden, with catalog, device, shadow and audit state unchanged.
				if !boundary.invoked || status != http.StatusForbidden || before != productWriteSnapshot(t, f) {
					t.Fatalf("operation=%s revocation=%s injected=%t status=%d response=%s", operation, change, boundary.invoked, status, response)
				}
			})
		}
	}
}

func productWriteSnapshot(t *testing.T, f telemetryPermissionFixture) string {
	t.Helper()
	var raw []byte
	if err := f.pool.QueryRow(context.Background(), `SELECT jsonb_build_object('products',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM products p),'models',(SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM product_models m),'devices',(SELECT jsonb_agg(to_jsonb(d) ORDER BY id) FROM devices d),'shadows',(SELECT jsonb_agg(to_jsonb(s) ORDER BY device_no) FROM device_shadows s),'audit',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM audit_events a))`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
