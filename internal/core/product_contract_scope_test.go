package core_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestProductContractHTTPCrossTenantAndMissingTargetsLeaveStateUnchanged(t *testing.T) {
	// Given: another tenant owns a published model and device.
	f := newProductContractFixture(t)
	p, _ := f.create(t)
	f.call(t, baselineAdminRequest{"POST", "/products/{product_id}/models/{version}/publish", productPublishPath(p.ID, 1), "", 200})
	f.call(t, baselineAdminRequest{"POST", "/products/{product_id}/models", productModelPath(p.ID), productContractFields, 201})
	execute(t, f.pool, `INSERT INTO tenants(id,name) VALUES(9902,'product-foreign'); INSERT INTO products(id,tenant_id,name) VALUES(9902,9902,'foreign'); INSERT INTO farms(id,tenant_id,name) VALUES(9902,9902,'foreign'); INSERT INTO ponds(id,farm_id,name) VALUES(9902,9902,'foreign'); INSERT INTO devices(pond_id,device_no,secret_hash) VALUES(9902,'product-foreign-device','fixture')`)
	execute(t, f.pool, `INSERT INTO product_models(product_id,version,schema,published_at) VALUES(9902,1,$1::jsonb,now())`, productContractFields)
	for index, tc := range []baselineAdminRequest{
		{"GET", "/products/{product_id}/models", productModelPath(9902), "", 404},
		{"POST", "/products/{product_id}/models", productModelPath(9902), productContractFields, 404},
		{"POST", "/products/{product_id}/models/{version}/publish", productPublishPath(9902, 1), "", 404},
		{"GET", "/devices/{device_no}/product", "/devices/product-foreign-device/product", "", 404},
		{"PUT", "/devices/{device_no}/product", "/devices/one/product", productAssignmentBody(9902, 1), 404},
		{"PUT", "/devices/{device_no}/product", "/devices/product-foreign-device/product", productAssignmentBody(p.ID, 1), 404},
		{"GET", "/products/{product_id}/models", productModelPath(999999), "", 404},
		{"GET", "/devices/{device_no}/product", "/devices/product-missing/product", "", 404},
		{"PUT", "/devices/{device_no}/product", "/devices/one/product", productAssignmentBody(p.ID, 2), 404},
		{"POST", "/products", "/products", `{"name":"product-contract"}`, 409},
	} {
		t.Run(fmt.Sprintf("hidden_%02d", index), func(t *testing.T) {
			before := f.snapshot(t)
			// When: a foreign, missing, unpublished or conflicting target is requested.
			f.call(t, tc)
			// Then: all persisted product, device, shadow and audit state remains intact.
			if f.snapshot(t) != before {
				t.Error("rejected target changed persisted state")
			}
		})
	}
	products := productDecode[[]struct {
		TenantID int64 `json:"tenant_id"`
	}](t, f.call(t, baselineAdminRequest{"GET", "/products", "/products", "", 200}).raw)
	for _, product := range products {
		if product.TenantID == 9902 {
			t.Error("foreign product leaked into list")
		}
	}
}

func TestProductContractHTTPViewerWritesAndRevokedTokenLeaveStateUnchanged(t *testing.T) {
	// Given: a real owner is demoted after its device has a published assignment.
	f := newProductContractFixture(t)
	p, _ := f.create(t)
	f.call(t, baselineAdminRequest{"POST", "/products/{product_id}/models/{version}/publish", productPublishPath(p.ID, 1), "", 200})
	f.call(t, baselineAdminRequest{"PUT", "/devices/{device_no}/product", "/devices/one/product", productAssignmentBody(p.ID, 1), 200})
	execute(t, f.pool, `UPDATE tenant_memberships SET role='viewer',permission_version=permission_version+1 WHERE user_id=1`)
	execute(t, f.pool, `INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role) SELECT id,1,1,'viewer' FROM tenants WHERE name='test-tenant'`)
	f.call(t, baselineAdminRequest{"GET", "/products", "/products", "", 401})
	var login baselineAdminResponse
	m6bHTTPRequest(t, f.server.URL, "", http.MethodPost, "/user/v1/login", `{"username":"contract-owner","password":"Contract-test-only-1234"}`, 200, &login)
	f.token = login.Token
	for index, tc := range []baselineAdminRequest{
		{"POST", "/products", "/products", `{"name":"viewer-must-not-create"}`, 403},
		{"POST", "/products/{product_id}/models", productModelPath(p.ID), productContractFields, 403},
		{"POST", "/products/{product_id}/models/{version}/publish", productPublishPath(p.ID, 1), "", 403},
		{"PUT", "/devices/{device_no}/product", "/devices/one/product", productAssignmentBody(p.ID, 1), 403},
	} {
		t.Run(fmt.Sprintf("viewer_%02d", index), func(t *testing.T) {
			before := f.snapshot(t)
			// When: a current read-only member invokes each write operation.
			f.call(t, tc)
			// Then: authorization fails and no persisted state changes.
			if f.snapshot(t) != before {
				t.Error("viewer write changed persisted state")
			}
		})
	}
	for _, tc := range []baselineAdminRequest{
		{"GET", "/products", "/products", "", 200},
		{"GET", "/products/{product_id}/models", productModelPath(p.ID), "", 200},
		{"GET", "/devices/{device_no}/product", "/devices/one/product", "", 200},
	} {
		f.call(t, tc)
	}
}
