package core_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestBaselineAppContractPositiveMatrixWithRealPostgres(t *testing.T) {
	// Given: authenticated production handlers backed by migrated PostgreSQL,
	// including real ingested shadow/history and a rule-generated alarm.
	f := newBaselineAppFixture(t)
	t.Run("login", func(t *testing.T) { f.login.contract(t, http.MethodPost, "/auth/login", 200) })
	for _, tc := range []struct {
		name, method, route, path string
		status                    int
	}{
		{"ponds", "GET", "/ponds", "/ponds", 200},
		{"pond detail", "GET", "/ponds/{id}", "/ponds/1", 200},
		{"devices", "GET", "/devices", "/devices", 200},
		{"device detail", "GET", "/devices/{device_no}", "/devices/one", 200},
		{"latest", "GET", "/water/latest", "/water/latest?device_no=one", 200},
		{"history", "GET", "/water/history", "/water/history?device_no=one&metric=temperature&range=7d", 200},
		{"alarms", "GET", "/alarms", "/alarms", 200},
		{"summary", "GET", "/stats/summary", "/stats/summary", 200},
		{"confirm", "POST", "/alarms/{id}/confirm", fmt.Sprintf("/alarms/%d/confirm", f.alarmID), 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// When: the actual TCP endpoint returns its database result.
			response := baselineAppHTTP(t, f.server, baselineAppRequest{method: tc.method, path: tc.path, token: f.owner})
			// Then: its status, content type and every returned field satisfy OpenAPI.
			response.contract(t, tc.method, tc.route, tc.status)
			switch tc.name {
			case "history":
				var history struct {
					Points []struct{ Value float64 } `json:"points"`
				}
				if err := json.Unmarshal(response.raw, &history); err != nil || len(history.Points) != 1 || history.Points[0].Value != 25 {
					t.Fatal("history did not return the actual ingested sample")
				}
			case "latest":
				var reading struct {
					Temperature float64  `json:"temperature"`
					PH          *float64 `json:"ph"`
				}
				if err := json.Unmarshal(response.raw, &reading); err != nil || reading.Temperature != 25 || reading.PH != nil {
					t.Fatal("shadow must contain reported temperature and nullable unreported metric")
				}
			case "alarms":
				var alarms []struct{ ID int64 }
				if err := json.Unmarshal(response.raw, &alarms); err != nil || len(alarms) != 1 || alarms[0].ID != f.alarmID {
					t.Fatal("alarm list did not isolate the actual rule-generated alarm")
				}
			case "confirm":
				if count(t, f.pool, `SELECT count(*) FROM alarms WHERE confirmed_at IS NOT NULL`) != 1 {
					t.Fatal("confirmation did not persist")
				}
			}
		})
	}
}

func TestBaselineAppContractEmptyAndNullableResponsesWithRealPostgres(t *testing.T) {
	// Given: an assigned owner before collecting any data and an unassigned member.
	f := newBaselineAppFixture(t)
	execute(t, f.pool, `DELETE FROM device_shadows WHERE device_no='one'`)
	for _, tc := range []struct{ name, route, path, token string }{
		{"empty ponds", "/ponds", "/ponds", f.empty},
		{"empty devices", "/devices", "/devices", f.empty},
		{"empty alarms", "/alarms", "/alarms", f.empty},
		{"zero summary", "/stats/summary", "/stats/summary", f.empty},
		{"null shadow", "/water/latest", "/water/latest?device_no=one", f.owner},
		{"null device latest", "/devices/{device_no}", "/devices/one", f.owner},
		{"empty pond", "/ponds/{id}", "/ponds/2", f.owner},
		{"empty history", "/water/history", "/water/history?device_no=one&metric=ph&range=7d", f.owner},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// When: requesting an empty collection or a resource without its shadow.
			response := baselineAppHTTP(t, f.server, baselineAppRequest{method: "GET", path: tc.path, token: tc.token})
			// Then: nullability and collection shapes are validated without synthetic responses.
			response.contract(t, "GET", tc.route, 200)
			if tc.name == "null shadow" && string(response.raw) != "null" {
				t.Fatal("missing shadow must return JSON null")
			}
			if tc.name == "empty ponds" || tc.name == "empty alarms" {
				var rows []json.RawMessage
				if err := json.Unmarshal(response.raw, &rows); err != nil || rows == nil || len(rows) != 0 {
					t.Fatal("empty collection must be a non-null empty array")
				}
			}
			if tc.name == "empty devices" {
				var page struct {
					List  []json.RawMessage `json:"list"`
					Total int               `json:"total"`
				}
				if err := json.Unmarshal(response.raw, &page); err != nil || page.List == nil || len(page.List) != 0 || page.Total != 0 {
					t.Fatal("empty device page must contain a non-null empty list and zero total")
				}
			}
		})
	}
}

func TestBaselineAppContractRejectionsWithRealPostgres(t *testing.T) {
	// Given: an owner, a read-only member and foreign resources in another tenant.
	f := newBaselineAppFixture(t)
	for _, tc := range []struct {
		name, method, route, path, body, token string
		status                                 int
	}{
		{"missing credentials", "GET", "/ponds", "/ponds", "", "", 401},
		{"invalid credentials", "GET", "/devices", "/devices", "", "invalid", 401},
		{"unauthorized pond", "GET", "/ponds/{id}", "/ponds/1", "", "", 401},
		{"unauthorized device", "GET", "/devices/{device_no}", "/devices/one", "", "", 401},
		{"unauthorized latest", "GET", "/water/latest", "/water/latest?device_no=one", "", "", 401},
		{"unauthorized history", "GET", "/water/history", "/water/history?device_no=one&metric=temperature", "", "", 401},
		{"unauthorized alarms", "GET", "/alarms", "/alarms", "", "", 401},
		{"unauthorized summary", "GET", "/stats/summary", "/stats/summary", "", "", 401},
		{"unauthorized confirm", "POST", "/alarms/{id}/confirm", fmt.Sprintf("/alarms/%d/confirm", f.alarmID), "", "", 401},
		{"malformed login", "POST", "/auth/login", "/auth/login", "{", "", 400},
		{"malformed pond", "GET", "/ponds/{id}", "/ponds/not-an-id", "", f.owner, 400},
		{"zero pond", "GET", "/ponds/{id}", "/ponds/0", "", f.owner, 400},
		{"missing pond", "GET", "/ponds/{id}", "/ponds/9999", "", f.owner, 404},
		{"foreign pond", "GET", "/ponds/{id}", "/ponds/9003", "", f.owner, 404},
		{"foreign device", "GET", "/devices/{device_no}", "/devices/contract-foreign-device", "", f.owner, 404},
		{"missing device", "GET", "/devices/{device_no}", "/devices/missing", "", f.owner, 404},
		{"foreign latest", "GET", "/water/latest", "/water/latest?device_no=contract-foreign-device", "", f.owner, 404},
		{"foreign history", "GET", "/water/history", "/water/history?device_no=contract-foreign-device&metric=temperature&range=7d", "", f.owner, 404},
		{"missing latest parameter", "GET", "/water/latest", "/water/latest", "", f.owner, 400},
		{"history metric", "GET", "/water/history", "/water/history?device_no=one&metric=invalid", "", f.owner, 400},
		{"history range", "GET", "/water/history", "/water/history?device_no=one&metric=temperature&range=invalid", "", f.owner, 400},
		{"history bound", "GET", "/water/history", "/water/history?device_no=one&metric=temperature&max_points=201", "", f.owner, 400},
		{"device pond parameter", "GET", "/devices", "/devices?pond_id=invalid", "", f.owner, 400},
		{"device page lower bound", "GET", "/devices", "/devices?page=0", "", f.owner, 400},
		{"device page size upper bound", "GET", "/devices", "/devices?page_size=201", "", f.owner, 400},
		{"device status", "GET", "/devices", "/devices?status=invalid", "", f.owner, 400},
		{"device malformed query escape", "GET", "/devices", "/devices?page=%zz", "", f.owner, 400},
		{"history malformed query escape", "GET", "/water/history", "/water/history?device_no=one&metric=temperature&range=%zz", "", f.owner, 400},
		{"alarm malformed query escape", "GET", "/alarms", "/alarms?limit=%zz", "", f.owner, 400},
		{"latest malformed query escape", "GET", "/water/latest", "/water/latest?device_no=one&extra=%zz", "", f.owner, 400},
		{"alarm limit", "GET", "/alarms", "/alarms?limit=201", "", f.owner, 400},
		{"alarm offset", "GET", "/alarms", "/alarms?offset=-1", "", f.owner, 400},
		{"alarm level", "GET", "/alarms", "/alarms?level=invalid", "", f.owner, 400},
		{"malformed confirm", "POST", "/alarms/{id}/confirm", "/alarms/invalid/confirm", "", f.owner, 400},
		{"zero confirm", "POST", "/alarms/{id}/confirm", "/alarms/0/confirm", "", f.owner, 400},
		{"missing confirm", "POST", "/alarms/{id}/confirm", "/alarms/9999/confirm", "", f.owner, 404},
		{"foreign confirm", "POST", "/alarms/{id}/confirm", "/alarms/9003/confirm", "", f.owner, 404},
		{"viewer confirm", "POST", "/alarms/{id}/confirm", fmt.Sprintf("/alarms/%d/confirm", f.alarmID), "", f.viewer, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := count(t, f.pool, `SELECT count(*) FROM alarms WHERE confirmed_at IS NOT NULL`)
			// When: an invalid or unauthorized operation reaches the production route.
			response := baselineAppHTTP(t, f.server, baselineAppRequest{tc.method, tc.path, tc.body, tc.token})
			// Then: it rejects the operation without changing alarm confirmation state.
			if after := count(t, f.pool, `SELECT count(*) FROM alarms WHERE confirmed_at IS NOT NULL`); before != after {
				t.Fatal("rejected request changed alarm state")
			}
			response.contract(t, tc.method, tc.route, tc.status)
		})
	}
}

func TestBaselineAppContractProviderFailureWithRealPostgres(t *testing.T) {
	// Given: authenticated requests and real relation failures scoped to this test database.
	f := newBaselineAppFixture(t)
	for _, tc := range []struct{ table, method, route, path string }{
		{"alarms", "GET", "/alarms", "/alarms"},
		{"alarms", "POST", "/alarms/{id}/confirm", fmt.Sprintf("/alarms/%d/confirm", f.alarmID)},
		{"sensor_data", "GET", "/water/history", "/water/history?device_no=one&metric=temperature&range=7d"},
		{"device_shadows", "GET", "/water/latest", "/water/latest?device_no=one"},
		{"devices", "GET", "/devices", "/devices"},
		{"devices", "GET", "/devices/{device_no}", "/devices/one"},
		{"devices", "GET", "/water/latest", "/water/latest?device_no=one"},
		{"devices", "GET", "/water/history", "/water/history?device_no=one&metric=temperature&range=7d"},
		{"farms", "GET", "/ponds", "/ponds"},
		{"farms", "GET", "/ponds/{id}", "/ponds/1"},
		{"farms", "GET", "/stats/summary", "/stats/summary"},
	} {
		t.Run(tc.method+" "+tc.route, func(t *testing.T) {
			execute(t, f.pool, "ALTER TABLE "+tc.table+" RENAME TO contract_fault_relation")
			defer execute(t, f.pool, "ALTER TABLE contract_fault_relation RENAME TO "+tc.table)
			// When: production SQL encounters an unavailable provider relation.
			response := baselineAppHTTP(t, f.server, baselineAppRequest{method: tc.method, path: tc.path, token: f.owner})
			// Then: the declared safe internal error response reaches the real client.
			response.contract(t, tc.method, tc.route, 500)
			if string(response.raw) != `{"error":"internal_error"}` {
				t.Fatal("provider failure did not use safe internal error")
			}
		})
	}
}

func TestBaselineAppContractDeviceFiltersWithRealPostgres(t *testing.T) {
	// Given: one active and one disabled visible device, plus a foreign device.
	f := newBaselineAppFixture(t)
	execute(t, f.pool, `UPDATE devices SET status='offline' WHERE device_no='one';
 INSERT INTO devices(pond_id,device_no,mqtt_username,secret_hash,disabled_at) SELECT 1,'contract-disabled','iolink-contract-disabled',secret_hash,now() FROM devices WHERE device_no='one'`)
	for _, tc := range []struct {
		name, query string
		total, size int
		page        int
	}{
		{"default hides disabled", "", 1, 1, 1},
		{"search excludes nonmatch", "?name=absent-search", 0, 0, 1},
		{"online excludes offline", "?status=online", 0, 0, 1},
		{"disabled filter", "?status=disabled", 1, 1, 1},
		{"all includes disabled", "?status=all", 2, 2, 1},
		{"page beyond result", "?page=2&page_size=1", 1, 0, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// When: querying actual filtered and paginated storage through TCP.
			response := baselineAppHTTP(t, f.server, baselineAppRequest{method: "GET", path: "/devices" + tc.query, token: f.owner})
			// Then: the page contains only matching visible rows and the correct total.
			response.contract(t, "GET", "/devices", 200)
			var page struct {
				List  []json.RawMessage `json:"list"`
				Total int               `json:"total"`
				Page  int               `json:"page"`
			}
			if err := json.Unmarshal(response.raw, &page); err != nil || page.Total != tc.total || len(page.List) != tc.size || page.Page != tc.page {
				t.Fatal("device query did not enforce declared search, status or pagination")
			}
		})
	}
}

func TestBaselineAppContractLoginRejectsInvalidJSONWithoutCreatingUsers(t *testing.T) {
	// Given: an exchange that would provision a new openid if invalid input reached it.
	f := newBaselineAppFixture(t)
	for _, tc := range []struct{ name, body string }{
		{"extra field", `{"code":"unregistered","extra":true}`},
		{"wrong field case", `{"Code":"unregistered"}`},
		{"duplicate field", `{"code":"unregistered","code":"unregistered"}`},
		{"trailing object", `{"code":"unregistered"}{}`},
		{"trailing garbage", `{"code":"unregistered"}invalid`},
		{"missing code", `{}`},
		{"empty code", `{"code":""}`},
		{"null code", `{"code":null}`},
		{"number code", `{"code":42}`},
		{"array code", `{"code":[]}`},
		{"null body", `null`},
		{"array body", `[]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := count(t, f.pool, `SELECT count(*) FROM users`)
			// When: malformed or schema-invalid login JSON reaches the production TCP route.
			response := baselineAppHTTP(t, f.server, baselineAppRequest{method: "POST", path: "/auth/login", body: tc.body})
			// Then: no user is provisioned and the public invalid-request response is stable.
			if count(t, f.pool, `SELECT count(*) FROM users`) != before {
				t.Fatal("invalid login created a user")
			}
			response.contract(t, "POST", "/auth/login", 400)
			if string(response.raw) != `{"error":"invalid_request"}` {
				t.Fatal("invalid login did not return the safe error code")
			}
		})
	}
}
