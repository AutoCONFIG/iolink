package core_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestExtendedAppContractsPositiveWithRealPostgres(t *testing.T) {
	// Given: two active memberships and a published model with a hidden property.
	f := newExtendedAppFixture(t)
	for _, tc := range []extendedAppRequest{
		{"GET", "/auth/tenants", "/api/v1/auth/tenants", "", 200},
		{"POST", "/auth/tenant", "/api/v1/auth/tenant", `{"tenant_id":9401}`, 200},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", f.validBody, 202},
		{"GET", "/devices/{device_no}/model/latest", "/api/v2/devices/v2-permission/model/latest", "", 200},
		{"GET", "/devices/{device_no}/history", "/api/v2/devices/v2-permission/history?metric=temperature", "", 200},
	} {
		t.Run(tc.method+tc.route, func(t *testing.T) {
			// When: using the production HTTP surface and PostgreSQL adapters.
			response := f.checked(t, tc)
			// Then: the public shape preserves actual memberships and readable model data.
			switch tc.route {
			case "/auth/tenants":
				var rows []struct {
					TenantID int64 `json:"tenant_id"`
				}
				if err := json.Unmarshal(response.raw, &rows); err != nil || len(rows) != 2 || rows[0].TenantID != 9401 || rows[1].TenantID != 9402 {
					t.Fatal("tenant list lost the actual memberships")
				}
			case "/devices/{device_no}/model/latest":
				var latest struct {
					Fields []struct {
						Identifier string
						Readable   bool
					} `json:"fields"`
					Properties map[string]json.RawMessage `json:"properties"`
				}
				if err := json.Unmarshal(response.raw, &latest); err != nil || len(latest.Fields) != 2 || string(latest.Properties["mode"]) != `"auto"` || latest.Properties["secret"] != nil {
					t.Fatal("model detail failed to filter unreadable properties")
				}
				for _, field := range latest.Fields {
					if !field.Readable || field.Identifier == "secret" {
						t.Fatal("model detail exposed an unreadable field")
					}
				}
			case "/devices/{device_no}/history":
				var history struct {
					Points []struct {
						Value        float64
						ModelVersion int `json:"model_version"`
					}
				}
				if err := json.Unmarshal(response.raw, &history); err != nil || len(history.Points) != 1 || history.Points[0].Value != 25 || history.Points[0].ModelVersion != 1 {
					t.Fatal("history lost the actual model-versioned sample")
				}
			}
		})
	}
	t.Run("switched token enforces target role", func(t *testing.T) {
		response := f.checked(t, extendedAppRequest{"POST", "/auth/tenant", "/api/v1/auth/tenant", `{"tenant_id":9402}`, 200})
		var auth struct{ Token string }
		if err := json.Unmarshal(response.raw, &auth); err != nil || auth.Token == "" {
			t.Fatal("tenant switch did not return a token")
		}
		f.token = auth.Token
		f.checked(t, extendedAppRequest{"GET", "/devices/{device_no}/model/latest", "/api/v2/devices/v2-permission/model/latest", "", 404})
		f.checked(t, extendedAppRequest{"GET", "/devices/{device_no}/model/latest", "/api/v2/devices/v2-foreign/model/latest", "", 200})
		before := f.snapshot(t)
		response = f.request(t, extendedAppRequest{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-foreign/telemetry", f.validBody, 403})
		if before != f.snapshot(t) {
			t.Fatal("viewer token wrote telemetry after tenant switch")
		}
		response.contract(t, "POST", "/devices/{device_no}/telemetry", 403)
	})
}

func TestExtendedAppContractsRejectInvalidInputWithoutWrites(t *testing.T) {
	// Given: real writable resources and their complete business-state snapshot.
	f := newExtendedAppFixture(t)
	valid := f.validBody
	for _, tc := range []extendedAppRequest{
		{"POST", "/auth/tenant", "/api/v1/auth/tenant", `{"tenant_id":9401,"extra":true}`, 400},
		{"POST", "/auth/tenant", "/api/v1/auth/tenant", `{"Tenant_id":9401}`, 400},
		{"POST", "/auth/tenant", "/api/v1/auth/tenant", `{"tenant_id":9401,"tenant_id":9401}`, 400},
		{"POST", "/auth/tenant", "/api/v1/auth/tenant", `{"tenant_id":9401}{}`, 400},
		{"POST", "/auth/tenant", "/api/v1/auth/tenant", `{"tenant_id":null}`, 400},
		{"POST", "/auth/tenant", "/api/v1/auth/tenant", `{"tenant_id":"9401"}`, 400},
		{"POST", "/auth/tenant", "/api/v1/auth/tenant", `{"tenant_id":9401.5}`, 400},
		{"POST", "/auth/tenant", "/api/v1/auth/tenant", `{"tenant_id":0}`, 400},
		{"POST", "/auth/tenant", "/api/v1/auth/tenant", `{"tenant_id":9999}`, 404},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", strings.TrimSuffix(valid, "}") + `,"extra":true}`, 400},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", valid + `{}`, 400},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", strings.Replace(valid, `"ts":`, `"TS":`, 1), 400},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", strings.TrimSuffix(valid, "}") + `,"properties":{"temperature":25}}`, 400},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", strings.Replace(valid, `"temperature":25`, `"temperature":25,"unknown":1`, 1), 400},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", strings.Replace(valid, `"temperature":25`, `"temperature":51`, 1), 400},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", strings.Replace(valid, `"temperature":25`, `"temperature":25,"temperature":26`, 1), 400},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", `{"ts":null,"properties":{"temperature":25}}`, 400},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", `{"ts":"invalid","properties":{"temperature":25}}`, 400},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", `{"ts":"2026-10-14T00:00:00Z","properties":null}`, 400},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", `{"ts":"2026-10-14T00:00:00Z","properties":[]}`, 400},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-foreign/telemetry", valid, 404},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-hidden/telemetry", valid, 404},
		{"GET", "/devices/{device_no}/model/latest", "/api/v2/devices/v2-foreign/model/latest", "", 404},
		{"GET", "/devices/{device_no}/model/latest", "/api/v2/devices/v2-hidden/model/latest", "", 404},
		{"GET", "/devices/{device_no}/history", "/api/v2/devices/v2-foreign/history?metric=temperature", "", 404},
		{"GET", "/devices/{device_no}/history", "/api/v2/devices/v2-permission/history?metric=secret", "", 400},
		{"GET", "/devices/{device_no}/history", "/api/v2/devices/v2-permission/history?metric=unknown", "", 400},
		{"GET", "/devices/{device_no}/history", "/api/v2/devices/v2-permission/history?metric=temperature&limit=0", "", 400},
		{"GET", "/devices/{device_no}/history", "/api/v2/devices/v2-permission/history?metric=temperature&limit=10001", "", 400},
		{"GET", "/devices/{device_no}/history", "/api/v2/devices/v2-permission/history?metric=temperature&limit=%zz", "", 400},
		{"GET", "/devices/{device_no}/history", "/api/v2/devices/v2-permission/history?metric=temperature&from=invalid", "", 400},
		{"GET", "/devices/{device_no}/history", "/api/v2/devices/v2-permission/history?metric=temperature&from=", "", 400},
	} {
		t.Run(fmt.Sprintf("%s %s status%d", tc.method, tc.route, tc.status), func(t *testing.T) {
			before := f.snapshot(t)
			// When: sending malformed input, an unpublished model or an inaccessible resource.
			response := f.request(t, tc)
			// Then: every rejection is contractual and leaves telemetry, shadow, alarms/outbox unchanged.
			if before != f.snapshot(t) {
				t.Fatal("rejected request changed business state")
			}
			response.contract(t, tc.method, tc.route, tc.status)
		})
	}
}

func TestExtendedAppContractsExcludeExpiredTargetTenant(t *testing.T) {
	// Given: the current tenant remains active while a different membership expires.
	f := newExtendedAppFixture(t)
	execute(t, f.pool, `UPDATE tenant_memberships SET expires_at=now()-interval '1 second' WHERE user_id=9402 AND tenant_id=9402`)
	// When: listing available tenants and trying to switch to the expired grant.
	response := f.checked(t, extendedAppRequest{"GET", "/auth/tenants", "/api/v1/auth/tenants", "", 200})
	var tenants []struct {
		TenantID int64 `json:"tenant_id"`
	}
	if err := json.Unmarshal(response.raw, &tenants); err != nil || len(tenants) != 1 || tenants[0].TenantID != 9401 {
		t.Fatal("expired target tenant remained selectable")
	}
	response = f.request(t, extendedAppRequest{"POST", "/auth/tenant", "/api/v1/auth/tenant", `{"tenant_id":9402}`, 404})
	// Then: no token is issued for that unavailable target tenant.
	response.contract(t, "POST", "/auth/tenant", 404)
}

func TestExtendedAppContractsEmptyHistoryAndHiddenModel(t *testing.T) {
	// Given: an assigned published model with no samples yet.
	f := newExtendedAppFixture(t)
	response := f.checked(t, extendedAppRequest{"GET", "/devices/{device_no}/history", "/api/v2/devices/v2-permission/history?metric=mode", "", 200})
	var history struct{ Points []json.RawMessage }
	if err := json.Unmarshal(response.raw, &history); err != nil || history.Points == nil || len(history.Points) != 0 {
		t.Fatal("empty history must return a non-null empty points array")
	}
	f.checked(t, extendedAppRequest{"GET", "/devices/{device_no}/model/latest", "/api/v2/devices/v2-permission/model/latest", "", 200})
	execute(t, f.pool, `INSERT INTO product_models(product_id,version,schema,published_at) SELECT product_id,3,'{"fields":[{"identifier":"secret","type":"string","readable":false,"nullable":true}]}'::jsonb,now() FROM devices WHERE device_no='v2-permission'; UPDATE devices SET model_version=3 WHERE device_no='v2-permission'`)
	// When: all fields in the assigned published model are unreadable.
	response = f.request(t, extendedAppRequest{"GET", "/devices/{device_no}/model/latest", "/api/v2/devices/v2-permission/model/latest", "", 200})
	// Then: the public model still has an array, with no readable fields/properties.
	response.contract(t, "GET", "/devices/{device_no}/model/latest", 200)
	var latest struct {
		Fields     []json.RawMessage
		Properties map[string]json.RawMessage
	}
	if err := json.Unmarshal(response.raw, &latest); err != nil || latest.Fields == nil || len(latest.Fields) != 0 || len(latest.Properties) != 0 {
		t.Fatal("all-hidden model exposed data or returned a null collection")
	}
}

func TestExtendedAppContractsRejectExpiredMembershipAndProviderFailures(t *testing.T) {
	// Given: live membership validation and PostgreSQL-backed operations.
	for _, tc := range []extendedAppRequest{
		{"GET", "/auth/tenants", "/api/v1/auth/tenants", "", 401},
		{"POST", "/auth/tenant", "/api/v1/auth/tenant", `{"tenant_id":9402}`, 401},
		{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", "", 401},
		{"GET", "/devices/{device_no}/model/latest", "/api/v2/devices/v2-permission/model/latest", "", 401},
		{"GET", "/devices/{device_no}/history", "/api/v2/devices/v2-permission/history?metric=temperature", "", 401},
	} {
		t.Run("expired "+tc.route, func(t *testing.T) {
			f := newExtendedAppFixture(t)
			execute(t, f.pool, `UPDATE tenant_memberships SET expires_at=now()-interval '1 second' WHERE user_id=9402 AND tenant_id=9401`)
			before := f.snapshot(t)
			// When: an existing JWT is used after its membership expires.
			response := f.request(t, tc)
			// Then: authorization rejects it before any business-state mutation.
			if before != f.snapshot(t) {
				t.Fatal("expired membership mutated state")
			}
			response.contract(t, tc.method, tc.route, 401)
		})
	}
	for _, tc := range []struct {
		table   string
		request extendedAppRequest
	}{
		{"tenant_memberships", extendedAppRequest{"GET", "/auth/tenants", "/api/v1/auth/tenants", "", 500}},
		{"tenant_memberships", extendedAppRequest{"POST", "/auth/tenant", "/api/v1/auth/tenant", `{"tenant_id":9402}`, 500}},
		{"telemetry", extendedAppRequest{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", "", 500}},
		{"alarm_rules", extendedAppRequest{"POST", "/devices/{device_no}/telemetry", "/api/v2/devices/v2-permission/telemetry", "", 500}},
		{"device_shadows", extendedAppRequest{"GET", "/devices/{device_no}/model/latest", "/api/v2/devices/v2-permission/model/latest", "", 500}},
		{"telemetry", extendedAppRequest{"GET", "/devices/{device_no}/history", "/api/v2/devices/v2-permission/history?metric=temperature", "", 500}},
	} {
		t.Run("provider "+tc.request.route, func(t *testing.T) {
			f := newExtendedAppFixture(t)
			if tc.request.method == "POST" && strings.HasPrefix(tc.request.path, "/api/v2/") {
				tc.request.body = f.validBody
			}
			before := f.snapshot(t)
			execute(t, f.pool, "ALTER TABLE "+tc.table+" RENAME TO extended_fault_relation")
			// When: production SQL encounters a unavailable relation.
			response := f.request(t, tc.request)
			execute(t, f.pool, "ALTER TABLE extended_fault_relation RENAME TO "+tc.table)
			// Then: only the stable internal error reaches the client and the transaction rolled back.
			if before != f.snapshot(t) {
				t.Fatal("provider failure left partial state")
			}
			response.contract(t, tc.request.method, tc.request.route, 500)
		})
	}
}
