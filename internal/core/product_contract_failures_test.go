package core_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestProductContractHTTPActualSQLFailuresAreSafeAndCorrelated(t *testing.T) {
	// Given: every product operation uses the real PostgreSQL catalog and dump sink.
	f := newProductContractFixture(t)
	p, _ := f.create(t)
	f.call(t, baselineAdminRequest{"POST", "/products/{product_id}/models/{version}/publish", productPublishPath(p.ID, 1), "", 200})
	ids := make(map[string]bool)
	for index, tc := range []struct {
		table   string
		request baselineAdminRequest
	}{
		{"products", baselineAdminRequest{"GET", "/products", "/products", "", 500}},
		{"products", baselineAdminRequest{"POST", "/products", "/products", `{"name":"sql-failure"}`, 500}},
		{"product_models", baselineAdminRequest{"GET", "/products/{product_id}/models", productModelPath(p.ID), "", 500}},
		{"product_models", baselineAdminRequest{"POST", "/products/{product_id}/models", productModelPath(p.ID), productContractFields, 500}},
		{"product_models", baselineAdminRequest{"POST", "/products/{product_id}/models/{version}/publish", productPublishPath(p.ID, 1), "", 500}},
		{"product_models", baselineAdminRequest{"GET", "/devices/{device_no}/product", "/devices/one/product", "", 500}},
		{"product_models", baselineAdminRequest{"PUT", "/devices/{device_no}/product", "/devices/one/product", productAssignmentBody(p.ID, 1), 500}},
	} {
		t.Run(fmt.Sprintf("sql_%02d", index), func(t *testing.T) {
			before := f.snapshot(t)
			func() {
				execute(t, f.pool, "ALTER TABLE "+tc.table+" RENAME TO product_contract_unavailable")
				defer execute(t, f.pool, "ALTER TABLE product_contract_unavailable RENAME TO "+tc.table)
				// When: a genuine relation failure reaches the authenticated production route.
				response := f.call(t, tc.request)
				// Then: the public body is canonical and the server generated an ID.
				if string(response.raw) != `{"error":"internal_error"}` || response.requestID == "" {
					t.Error("SQL failure did not produce a safe correlated error")
				}
				if _, exists := ids[response.requestID]; exists {
					t.Error("provider requests reused a correlation identifier")
				}
				ids[response.requestID] = false
			}()
			if f.snapshot(t) != before {
				t.Error("SQL failure left partial persisted state")
			}
		})
	}
	if err := f.sink.Close(); err != nil {
		t.Fatal(err)
	}
	dump, err := os.ReadFile(f.dump)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{f.token, "product_contract_unavailable", "Contract-test-only-1234", "sql-failure"} {
		if strings.Contains(string(dump), value) {
			t.Error("product diagnostics contain private request or SQL data")
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(string(dump)), "\n") {
		var record struct {
			Stage, Level, Err string
			RequestID         string `json:"request_id"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal("dump JSON decode failed")
		}
		if record.Stage == "http_provider" && record.Level == "ERROR" && record.Err == "postgres_42P01" {
			if _, ok := ids[record.RequestID]; ok {
				ids[record.RequestID] = true
			}
		}
	}
	for _, found := range ids {
		if !found {
			t.Error("missing safely classified SQLSTATE diagnostic for request")
		}
	}
	if destination := os.Getenv("IOLINK_PRODUCT_DUMP_EVIDENCE"); destination != "" {
		if err := os.WriteFile(destination, dump, 0o600); err != nil {
			t.Fatal("verified product dump evidence could not be saved")
		}
	}
}

func TestProductContractHTTPAssignmentAuditFailureRollsBack(t *testing.T) {
	// Given: a device has a shadow and the assignment audit insert cannot commit.
	f := newProductContractFixture(t)
	p, _ := f.create(t)
	f.call(t, baselineAdminRequest{"POST", "/products/{product_id}/models/{version}/publish", productPublishPath(p.ID, 1), "", 200})
	if err := f.svc.HandleEvent(sample(time.Now().UTC(), "product-before-failed-assignment", map[string]float64{"temperature": 25})); err != nil {
		t.Fatal(err)
	}
	execute(t, f.pool, `CREATE FUNCTION product_contract_reject_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='device.product_assigned' THEN RAISE EXCEPTION 'private provider audit failure' USING ERRCODE='23514'; END IF; RETURN NEW; END $$; CREATE TRIGGER product_contract_reject BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION product_contract_reject_audit()`)
	before := f.snapshot(t)
	// When: the real assignment transaction encounters a failing audit trigger.
	response := f.call(t, baselineAdminRequest{"PUT", "/devices/{device_no}/product", "/devices/one/product", productAssignmentBody(p.ID, 1), 500})
	// Then: assignment, shadow deletion and audit all roll back together.
	if f.snapshot(t) != before || string(response.raw) != `{"error":"internal_error"}` {
		t.Error("failed assignment left partial state or unsafe error")
	}
}

func TestProductContractHTTPMissingTokenRejectsAllOperations(t *testing.T) {
	// Given: an existing draft and no bearer token.
	f := newProductContractFixture(t)
	p, _ := f.create(t)
	f.token = ""
	for index, tc := range []baselineAdminRequest{
		{"GET", "/products", "/products", "", 401},
		{"POST", "/products", "/products", `{"name":"unauthorized"}`, 401},
		{"GET", "/products/{product_id}/models", productModelPath(p.ID), "", 401},
		{"POST", "/products/{product_id}/models", productModelPath(p.ID), productContractFields, 401},
		{"POST", "/products/{product_id}/models/{version}/publish", productPublishPath(p.ID, 1), "", 401},
		{"GET", "/devices/{device_no}/product", "/devices/one/product", "", 401},
		{"PUT", "/devices/{device_no}/product", "/devices/one/product", productAssignmentBody(p.ID, 1), 401},
	} {
		t.Run(fmt.Sprintf("unauthorized_%02d", index), func(t *testing.T) {
			before := f.snapshot(t)
			// When: an unauthenticated client invokes each operation.
			f.call(t, tc)
			// Then: authentication rejects the request before any persistent mutation.
			if f.snapshot(t) != before {
				t.Error("unauthorized request changed persisted state")
			}
		})
	}
}
