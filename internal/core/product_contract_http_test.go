package core_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func productModelPath(product int64) string { return fmt.Sprintf("/products/%d/models", product) }
func productPublishPath(product int64, version int) string {
	return fmt.Sprintf("/products/%d/models/%d/publish", product, version)
}

func productAssignmentBody(product int64, version int) string {
	return fmt.Sprintf(`{"product_id":%d,"model_version":%d}`, product, version)
}

func TestProductContractHTTPPositiveMatrix(t *testing.T) {
	// Given: real authenticated UserRoutes, production catalog and isolated PostgreSQL.
	f := newProductContractFixture(t)
	empty := productDecode[[]domain.Product](t, f.call(t, baselineAdminRequest{"GET", "/products", "/products", "", 200}).raw)
	if len(empty) != 0 {
		t.Fatal("new tenant product collection is not empty")
	}
	// When: create, publish and assign through all seven existing product operations.
	p, draft := f.create(t)
	if p.ID < 1 || draft.Version != 1 || draft.PublishedAt != nil {
		t.Fatal("new draft identity/publication is invalid")
	}
	models := productDecode[[]domain.ProductModel](t, f.call(t, baselineAdminRequest{"GET", "/products/{product_id}/models", productModelPath(p.ID), "", 200}).raw)
	if len(models) != 1 || len(models[0].Fields) != 2 || models[0].Fields[1].Readable {
		t.Fatal("model fields lost their defined visibility")
	}
	published := productDecode[domain.ProductModel](t, f.call(t, baselineAdminRequest{"POST", "/products/{product_id}/models/{version}/publish", productPublishPath(p.ID, 1), "", 200}).raw)
	repeated := productDecode[domain.ProductModel](t, f.call(t, baselineAdminRequest{"POST", "/products/{product_id}/models/{version}/publish", productPublishPath(p.ID, 1), "", 200}).raw)
	if published.PublishedAt == nil || repeated.PublishedAt == nil || !published.PublishedAt.Equal(*repeated.PublishedAt) {
		t.Fatal("publication is not idempotent")
	}
	assigned := f.call(t, baselineAdminRequest{"PUT", "/devices/{device_no}/product", "/devices/one/product", productAssignmentBody(p.ID, 1), 200})
	var assignment struct {
		ProductID  int64  `json:"product_id"`
		Version    int    `json:"model_version"`
		AssignedAt string `json:"assigned_at"`
	}
	assignment = productDecode[struct {
		ProductID  int64  `json:"product_id"`
		Version    int    `json:"model_version"`
		AssignedAt string `json:"assigned_at"`
	}](t, assigned.raw)
	if assignment.ProductID != p.ID || assignment.Version != 1 || assignment.AssignedAt == "" {
		t.Fatal("assignment response does not identify the committed model")
	}
	f.call(t, baselineAdminRequest{"GET", "/devices/{device_no}/product", "/devices/one/product", "", 200})
	ctx := domain.WithTenantUserID(domain.WithTenantRole(domain.WithTenantID(t.Context(), p.TenantID), "owner"), 1)
	repo := f.svc.Telemetry().(domain.GenericTelemetryRepo)
	if _, err := repo.SubmitTelemetry(ctx, "one", 1, time.Now().UTC(), map[string]json.RawMessage{"temperature": json.RawMessage(`24`), "private_value": json.RawMessage(`"private-telemetry-value"`)}); err != nil {
		t.Fatal("assigned-model telemetry fixture failed")
	}
	latest, err := f.svc.Telemetry().(domain.DeviceModelLatestRepo).ModelLatestForUser(ctx, "one", 1)
	if err != nil || len(latest.Fields) != 1 || latest.Properties["private_value"] != nil || latest.Properties["temperature"] == nil {
		t.Fatal("unreadable model values leaked from the actual repository")
	}
	beforeRepeat := f.snapshot(t)
	f.call(t, baselineAdminRequest{"PUT", "/devices/{device_no}/product", "/devices/one/product", productAssignmentBody(p.ID, 1), 200})
	if f.snapshot(t) != beforeRepeat {
		t.Fatal("repeated assignment changed shadow, audit or assignment time")
	}
	products := productDecode[[]domain.Product](t, f.call(t, baselineAdminRequest{"GET", "/products", "/products", "", 200}).raw)
	if len(products) != 1 || products[0].CurrentVersion == nil || *products[0].CurrentVersion != 1 {
		t.Fatal("published current version not reported")
	}
	second := productDecode[domain.ProductModel](t, f.call(t, baselineAdminRequest{"POST", "/products/{product_id}/models", productModelPath(p.ID), productContractFields, 201}).raw)
	if second.Version != 2 {
		t.Fatal("new model version did not increment")
	}
	f.call(t, baselineAdminRequest{"POST", "/products/{product_id}/models/{version}/publish", productPublishPath(p.ID, 2), "", 200})
	// Then: publishing another version does not move an already assigned device.
	var version int
	if err := f.pool.QueryRow(t.Context(), `SELECT model_version FROM devices WHERE device_no='one'`).Scan(&version); err != nil || version != 1 {
		t.Fatal("device version changed without explicit assignment")
	}
}

func TestProductContractHTTPInvalidRequestsLeaveStateUnchanged(t *testing.T) {
	// Given: a product with a draft and a device that retains its existing model.
	f := newProductContractFixture(t)
	p, _ := f.create(t)
	f.call(t, baselineAdminRequest{"POST", "/products/{product_id}/models/{version}/publish", productPublishPath(p.ID, 1), "", 200})
	for index, tc := range []baselineAdminRequest{
		{"POST", "/products", "/products", `{"name":"unknown","extra":true}`, 400},
		{"POST", "/products", "/products", `{"name":"first","name":"duplicate"}`, 400},
		{"POST", "/products", "/products", `{"NAME":"wrong case"}`, 400},
		{"POST", "/products", "/products", `{"name":"trailing"}{}`, 400},
		{"POST", "/products", "/products", `{"name":null}`, 400},
		{"POST", "/products/{product_id}/models", productModelPath(p.ID), `{"fields":null}`, 400},
		{"POST", "/products/{product_id}/models", productModelPath(p.ID), `{"fields":[{"identifier":"missing","type":"number"}]}`, 400},
		{"POST", "/products/{product_id}/models", productModelPath(p.ID), `{"fields":[{"identifier":"bad","type":"number","unit":"","readable":null,"writable":false,"nullable":false}]}`, 400},
		{"POST", "/products/{product_id}/models", productModelPath(p.ID), `{"fields":[{"identifier":"bad","type":"number","unit":"","readable":true,"readable":false,"writable":false,"nullable":false}]}`, 400},
		{"POST", "/products/{product_id}/models", productModelPath(p.ID), `{"fields":[{"identifier":"bad","type":"number","unit":"","readable":true,"writable":false,"nullable":false,"extra":1}]}`, 400},
		{"POST", "/products/{product_id}/models", productModelPath(p.ID), productContractFields + `{}`, 400},
		{"GET", "/products/{product_id}/models", "/products/0/models", "", 400},
		{"POST", "/products/{product_id}/models/{version}/publish", productPublishPath(p.ID, 0), "", 400},
		{"PUT", "/devices/{device_no}/product", "/devices/one/product", `{"product_id":0,"model_version":1}`, 400},
		{"PUT", "/devices/{device_no}/product", "/devices/one/product", productAssignmentBody(p.ID, 1) + `{}`, 400},
		{"PUT", "/devices/{device_no}/product", "/devices/one/product", fmt.Sprintf(`{"product_id":%d,"model_version":1,"extra":true}`, p.ID), 400},
		{"PUT", "/devices/{device_no}/product", "/devices/one/product", fmt.Sprintf(`{"product_id":%d,"product_id":%d,"model_version":1}`, p.ID, p.ID), 400},
		{"PUT", "/devices/{device_no}/product", "/devices/one/product", fmt.Sprintf(`{"product_id":%d,"model_version":null}`, p.ID), 400},
		{"POST", "/products/{product_id}/models", productModelPath(p.ID), strings.Replace(productContractFields, `"unit":"C"`, `"unit":"F"`, 1), 400},
	} {
		t.Run(fmt.Sprintf("invalid_%02d", index), func(t *testing.T) {
			before := f.snapshot(t)
			// When: invalid input reaches the actual authenticated TCP handler.
			f.call(t, tc)
			// Then: no product/model/assignment/shadow/audit change is committed.
			if f.snapshot(t) != before {
				t.Error("invalid product request changed persisted state")
			}
		})
	}
}
