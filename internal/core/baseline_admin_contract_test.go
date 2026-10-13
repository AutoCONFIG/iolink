package core_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestBaselineAdminContractPositiveMatrix(t *testing.T) {
	// Given: a real tenant owner, current schemas, TCP HTTP and isolated PostgreSQL.
	f := newBaselineAdminFixture(t)
	var farm, pond, rule int64
	var device string
	run := func(name string, tc baselineAdminRequest) baselineAdminResponse {
		var result baselineAdminResponse
		t.Run(name, func(t *testing.T) { result = f.request(t, tc) })
		return result
	}
	// When: drive the existing resource workflow through its authenticated interface.
	run("POST_login", baselineAdminRequest{"POST", "/login", "/login", `{"username":"contract-owner","password":"Contract-test-only-1234"}`, 200})
	run("GET_users", baselineAdminRequest{"GET", "/users", "/users?query=1", "", 200})
	farm = run("POST_farms", baselineAdminRequest{"POST", "/farms", "/farms", `{"name":"contract farm","location":"local","owner_id":1}`, 200}).ID
	run("GET_farms", baselineAdminRequest{"GET", "/farms", "/farms", "", 200})
	run("PUT_farm", baselineAdminRequest{"PUT", "/farms/{id}", fmt.Sprintf("/farms/%d", farm), `{"name":"updated farm","location":"updated"}`, 204})
	run("PUT_farm_owner", baselineAdminRequest{"PUT", "/farms/{id}/owner", fmt.Sprintf("/farms/%d/owner", farm), `{"owner_id":1}`, 204})
	pond = run("POST_ponds", baselineAdminRequest{"POST", "/ponds", "/ponds", fmt.Sprintf(`{"farm_id":%d,"name":"contract pond","area_mu":3}`, farm), 200}).ID
	run("PUT_pond", baselineAdminRequest{"PUT", "/ponds/{id}", fmt.Sprintf("/ponds/%d", pond), fmt.Sprintf(`{"farm_id":%d,"name":"updated pond","area_mu":4}`, farm), 204})
	run("GET_ponds", baselineAdminRequest{"GET", "/ponds", fmt.Sprintf("/ponds?farm_id=%d", farm), "", 200})
	device = run("POST_devices", baselineAdminRequest{"POST", "/devices", "/devices", fmt.Sprintf(`{"pond_id":%d,"name":"contract device","model":"water","report_interval":60}`, pond), 200}).DeviceNo
	run("GET_devices", baselineAdminRequest{"GET", "/devices", "/devices?page=1&page_size=20", "", 200})
	run("GET_device", baselineAdminRequest{"GET", "/devices/{device_no}", "/devices/" + device, "", 200})
	run("PUT_device_pond", baselineAdminRequest{"PUT", "/devices/{device_no}/pond", "/devices/" + device + "/pond", `{"pond_id":2}`, 204})
	rule = run("POST_alarm_rules", baselineAdminRequest{"POST", "/alarm-rules", "/alarm-rules", `{"pond_id":1,"metric":"temperature","max_value":25,"level":"warning"}`, 200}).ID
	run("GET_alarm_rules", baselineAdminRequest{"GET", "/alarm-rules", "/alarm-rules", "", 200})
	run("PUT_alarm_rule", baselineAdminRequest{"PUT", "/alarm-rules/{id}", fmt.Sprintf("/alarm-rules/%d", rule), `{"pond_id":1,"metric":"temperature","max_value":24,"level":"critical","enabled":true}`, 204})
	if err := f.svc.HandleEvent(sample(time.Now().UTC(), "contract-alarm", map[string]float64{"temperature": 30})); err != nil {
		t.Fatal(err)
	}
	var alarm int64
	if err := f.pool.QueryRow(context.Background(), `SELECT id FROM alarms WHERE device_no='one'`).Scan(&alarm); err != nil {
		t.Fatal(err)
	}
	run("GET_alarms", baselineAdminRequest{"GET", "/alarms", "/alarms", "", 200})
	run("POST_alarm_confirm", baselineAdminRequest{"POST", "/alarms/{id}/confirm", fmt.Sprintf("/alarms/%d/confirm", alarm), "", 204})
	batch := run("POST_batch_confirm", baselineAdminRequest{"POST", "/alarms/batch-confirm", "/alarms/batch-confirm", fmt.Sprintf(`{"ids":[%d]}`, alarm), 200})
	if batch.Confirmed != 0 {
		t.Error("repeat confirmation changed the alarm")
	}
	run("GET_stats", baselineAdminRequest{"GET", "/stats", "/stats", "", 200})
	run("DELETE_alarm_rule", baselineAdminRequest{"DELETE", "/alarm-rules/{id}", fmt.Sprintf("/alarm-rules/%d", rule), "", 204})
	run("DELETE_device", baselineAdminRequest{"DELETE", "/devices/{device_no}", "/devices/" + device, "", 204})
	run("DELETE_pond", baselineAdminRequest{"DELETE", "/ponds/{id}", fmt.Sprintf("/ponds/%d", pond), "", 204})
	run("DELETE_farm", baselineAdminRequest{"DELETE", "/farms/{id}", fmt.Sprintf("/farms/%d", farm), "", 204})
	run("POST_password", baselineAdminRequest{"POST", "/password", "/password", `{"old_password":"Contract-test-only-1234","new_password":"Contract-test-new-1234"}`, 204})
	// Then: successful deletes and password revocation are persisted.
	if count(t, f.pool, fmt.Sprintf("SELECT count(*) FROM farms WHERE id=%d", farm)) != 0 {
		t.Error("deleted farm persisted")
	}
	if count(t, f.pool, fmt.Sprintf("SELECT count(*) FROM ponds WHERE id=%d", pond)) != 0 {
		t.Error("deleted pond persisted")
	}
	run("GET_stats_revoked_token", baselineAdminRequest{"GET", "/stats", "/stats", "", 401})
}

func TestBaselineAdminContractRejectedRequests(t *testing.T) {
	// Given: no credentials, then a tenant owner with existing bound resources.
	f := newBaselineAdminFixture(t)
	token := f.token
	f.token = ""
	t.Run("401_missing_token", func(t *testing.T) {
		f.request(t, baselineAdminRequest{"GET", "/stats", "/stats", "", http.StatusUnauthorized})
	})
	f.token = token
	for _, tc := range []struct {
		name    string
		request baselineAdminRequest
	}{
		{"400_malformed", baselineAdminRequest{"POST", "/farms", "/farms", `{`, 400}},
		{"400_unknown_field", baselineAdminRequest{"POST", "/farms", "/farms", `{"name":"must reject","unexpected":"value"}`, 400}},
		{"400_trailing_json", baselineAdminRequest{"POST", "/farms", "/farms", `{"name":"must reject"} {}`, 400}},
		{"400_owner_omitted", baselineAdminRequest{"PUT", "/farms/{id}/owner", "/farms/1/owner", `{}`, 400}},
		{"400_owner_zero", baselineAdminRequest{"POST", "/farms", "/farms", `{"name":"must reject","owner_id":0}`, 400}},
		{"400_farm_update_unknown", baselineAdminRequest{"PUT", "/farms/{id}", "/farms/1", `{"name":"must reject","unknown":1}`, 400}},
		{"400_owner_unknown", baselineAdminRequest{"PUT", "/farms/{id}/owner", "/farms/1/owner", `{"owner_id":1,"unknown":1}`, 400}},
		{"400_pond_unknown", baselineAdminRequest{"POST", "/ponds", "/ponds", `{"farm_id":1,"name":"must reject","unknown":1}`, 400}},
		{"400_pond_area_negative", baselineAdminRequest{"POST", "/ponds", "/ponds", `{"farm_id":1,"name":"must reject","area_mu":-1}`, 400}},
		{"400_pond_update_unknown", baselineAdminRequest{"PUT", "/ponds/{id}", "/ponds/1", `{"farm_id":1,"name":"must reject","unknown":1}`, 400}},
		{"400_device_unknown", baselineAdminRequest{"POST", "/devices", "/devices", `{"pond_id":1,"unknown":1}`, 400}},
		{"400_device_interval_zero", baselineAdminRequest{"POST", "/devices", "/devices", `{"pond_id":1,"report_interval":0}`, 400}},
		{"400_device_interval_null", baselineAdminRequest{"POST", "/devices", "/devices", `{"pond_id":1,"report_interval":null}`, 400}},
		{"400_move_unknown", baselineAdminRequest{"PUT", "/devices/{device_no}/pond", "/devices/one/pond", `{"pond_id":2,"unknown":1}`, 400}},
		{"400_rule_unknown", baselineAdminRequest{"POST", "/alarm-rules", "/alarm-rules", `{"pond_id":1,"metric":"temperature","max_value":25,"level":"warning","unknown":1}`, 400}},
		{"400_rule_enabled_null", baselineAdminRequest{"POST", "/alarm-rules", "/alarm-rules", `{"pond_id":1,"metric":"temperature","max_value":25,"level":"warning","enabled":null}`, 400}},
		{"400_rule_update_unknown", baselineAdminRequest{"PUT", "/alarm-rules/{id}", "/alarm-rules/1", `{"pond_id":1,"metric":"temperature","max_value":25,"level":"warning","unknown":1}`, 400}},
		{"400_password_unknown", baselineAdminRequest{"POST", "/password", "/password", `{"old_password":"Contract-test-only-1234","new_password":"Contract-test-new-1234","unknown":1}`, 400}},
		{"400_batch_unknown", baselineAdminRequest{"POST", "/alarms/batch-confirm", "/alarms/batch-confirm", `{"ids":[1],"unknown":1}`, 400}},
		{"400_batch_zero_id", baselineAdminRequest{"POST", "/alarms/batch-confirm", "/alarms/batch-confirm", `{"ids":[0]}`, 400}},
		{"400_pond_transfer", baselineAdminRequest{"PUT", "/ponds/{id}", "/ponds/1", `{"farm_id":999999,"name":"must reject"}`, 400}},
		{"404_missing_farm", baselineAdminRequest{"PUT", "/farms/{id}", "/farms/999999", `{"name":"missing"}`, 404}},
		{"404_missing_device", baselineAdminRequest{"GET", "/devices/{device_no}", "/devices/missing", "", 404}},
		{"409_bound_farm", baselineAdminRequest{"DELETE", "/farms/{id}", "/farms/1", "", 409}},
		{"409_bound_pond", baselineAdminRequest{"DELETE", "/ponds/{id}", "/ponds/1", "", 409}},
		{"400_duplicate_batch", baselineAdminRequest{"POST", "/alarms/batch-confirm", "/alarms/batch-confirm", `{"ids":[1,1]}`, 400}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := f.state(t)
			f.request(t, tc.request)
			if f.state(t) != before {
				t.Error("rejected request modified persisted state")
			}
		})
	}
	execute(t, f.pool, `INSERT INTO users(id,open_id) VALUES(201,'contract-foreign'); INSERT INTO tenants(id,name) VALUES(201,'contract-foreign'); INSERT INTO tenant_memberships(tenant_id,user_id,role) VALUES(201,201,'owner'); INSERT INTO farms(id,tenant_id,owner_id,name) VALUES(201,201,201,'foreign'); INSERT INTO ponds(id,farm_id,name) VALUES(201,201,'foreign'); INSERT INTO devices(pond_id,device_no,secret_hash) VALUES(201,'contract-foreign','fixture-hash')`)
	for _, tc := range []baselineAdminRequest{
		{"GET", "/devices/{device_no}", "/devices/contract-foreign", "", 404},
		{"PUT", "/farms/{id}", "/farms/201", `{"name":"unauthorized"}`, 404},
	} {
		t.Run("404_foreign_"+tc.method, func(t *testing.T) { f.request(t, tc) })
	}
	if count(t, f.pool, "SELECT count(*) FROM farms WHERE id=201 AND name='foreign'") != 1 {
		t.Error("cross-tenant write changed foreign resource")
	}
	var viewerRule baselineAdminResponse
	t.Run("viewer_fixture_rule", func(t *testing.T) {
		viewerRule = f.request(t, baselineAdminRequest{"POST", "/alarm-rules", "/alarm-rules", `{"pond_id":1,"metric":"temperature","max_value":25,"level":"warning"}`, 200})
	})
	if err := f.svc.HandleEvent(sample(time.Now().UTC(), "viewer-permission", map[string]float64{"temperature": 30})); err != nil {
		t.Fatal(err)
	}
	var viewerAlarm int64
	if err := f.pool.QueryRow(context.Background(), `SELECT id FROM alarms WHERE device_no='one'`).Scan(&viewerAlarm); err != nil {
		t.Fatal(err)
	}
	// Given: a viewer token freshly issued after the membership version changes.
	execute(t, f.pool, `UPDATE tenant_memberships SET role='viewer',permission_version=permission_version+1 WHERE user_id=1`)
	var login baselineAdminResponse
	m6bHTTPRequest(t, f.server.URL, "", "POST", "/user/v1/login", `{"username":"contract-owner","password":"Contract-test-only-1234"}`, 200, &login)
	f.token = login.Token
	for _, tc := range []baselineAdminRequest{
		{"POST", "/farms", "/farms", `{"name":"viewer denied"}`, 403},
		{"PUT", "/farms/{id}", "/farms/1", `{"name":"viewer denied"}`, 403},
		{"DELETE", "/farms/{id}", "/farms/1", "", 403},
		{"PUT", "/farms/{id}/owner", "/farms/1/owner", `{"owner_id":1}`, 403},
		{"POST", "/ponds", "/ponds", `{"farm_id":1,"name":"viewer denied"}`, 403},
		{"PUT", "/ponds/{id}", "/ponds/1", `{"farm_id":1,"name":"viewer denied"}`, 403},
		{"DELETE", "/ponds/{id}", "/ponds/1", "", 403},
		{"POST", "/devices", "/devices", `{"pond_id":1}`, 403},
		{"PUT", "/devices/{device_no}/pond", "/devices/one/pond", `{"pond_id":2}`, 403},
		{"DELETE", "/devices/{device_no}", "/devices/one", "", 403},
		{"POST", "/alarm-rules", "/alarm-rules", `{"pond_id":1,"metric":"ph","min_value":6,"level":"warning"}`, 403},
		{"PUT", "/alarm-rules/{id}", fmt.Sprintf("/alarm-rules/%d", viewerRule.ID), `{"pond_id":1,"metric":"temperature","max_value":26,"level":"warning"}`, 403},
		{"DELETE", "/alarm-rules/{id}", fmt.Sprintf("/alarm-rules/%d", viewerRule.ID), "", 403},
		{"POST", "/alarms/{id}/confirm", fmt.Sprintf("/alarms/%d/confirm", viewerAlarm), "", 403},
		{"POST", "/alarms/batch-confirm", "/alarms/batch-confirm", fmt.Sprintf(`{"ids":[%d]}`, viewerAlarm), 403},
	} {
		t.Run("403_viewer_"+tc.method+"_"+tc.route, func(t *testing.T) {
			before := f.state(t)
			f.request(t, tc)
			if f.state(t) != before {
				t.Error("viewer request changed persisted state")
			}
		})
	}
}

func TestBaselineAdminDisabledRuleRemainsInactiveUntilEnabled(t *testing.T) {
	f := newBaselineAdminFixture(t)
	var rule baselineAdminResponse
	t.Run("create_disabled", func(t *testing.T) {
		rule = f.request(t, baselineAdminRequest{"POST", "/alarm-rules", "/alarm-rules", `{"pond_id":1,"metric":"temperature","max_value":25,"level":"warning","enabled":false}`, 200})
	})
	if count(t, f.pool, fmt.Sprintf("SELECT count(*) FROM alarm_rules WHERE id=%d AND enabled=false", rule.ID)) != 1 {
		t.Error("disabled rule was persisted enabled")
	}
	if err := f.svc.HandleEvent(sample(time.Now().UTC(), "disabled-rule", map[string]float64{"temperature": 30})); err != nil {
		t.Fatal(err)
	}
	if count(t, f.pool, "SELECT count(*) FROM alarms") != 0 {
		t.Error("disabled rule triggered an alarm")
	}
	t.Run("enable_by_default", func(t *testing.T) {
		f.request(t, baselineAdminRequest{"PUT", "/alarm-rules/{id}", fmt.Sprintf("/alarm-rules/%d", rule.ID), `{"pond_id":1,"metric":"temperature","max_value":25,"level":"warning"}`, 204})
	})
	if err := f.svc.HandleEvent(sample(time.Now().UTC(), "enabled-rule", map[string]float64{"temperature": 30})); err != nil {
		t.Fatal(err)
	}
	if count(t, f.pool, "SELECT count(*) FROM alarms") != 1 {
		t.Error("enabled rule did not trigger exactly one alarm")
	}
}

func TestBaselineAdminFarmOwnerOmissionAndNullCreateUnassigned(t *testing.T) {
	f := newBaselineAdminFixture(t)
	for _, tc := range []struct{ name, body string }{
		{"omitted", `{"name":"unassigned farm"}`},
		{"null", `{"name":"unassigned farm","owner_id":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := f.request(t, baselineAdminRequest{"POST", "/farms", "/farms", tc.body, 200})
			if count(t, f.pool, fmt.Sprintf("SELECT count(*) FROM farms WHERE id=%d AND owner_id IS NULL", result.ID)) != 1 {
				t.Error("creator was silently assigned as owner")
			}
		})
	}
}

func TestBaselineAdminBatchConfirmationRollsBackMissingResource(t *testing.T) {
	// Given: an unconfirmed alarm created through the actual ingestion pipeline.
	f := newBaselineAdminFixture(t)
	var rule baselineAdminResponse
	t.Run("create_rule", func(t *testing.T) {
		rule = f.request(t, baselineAdminRequest{"POST", "/alarm-rules", "/alarm-rules", `{"pond_id":1,"metric":"temperature","max_value":25,"level":"warning"}`, 200})
	})
	if rule.ID == 0 {
		t.Fatal("alarm rule fixture did not persist")
	}
	if err := f.svc.HandleEvent(sample(time.Now().UTC(), "contract-batch", map[string]float64{"temperature": 30})); err != nil {
		t.Fatal(err)
	}
	var alarm int64
	if err := f.pool.QueryRow(context.Background(), "SELECT id FROM alarms WHERE device_no='one'").Scan(&alarm); err != nil {
		t.Fatal(err)
	}
	// When: one valid alarm is followed by an unavailable resource in the same batch.
	t.Run("missing_resource", func(t *testing.T) {
		f.request(t, baselineAdminRequest{"POST", "/alarms/batch-confirm", "/alarms/batch-confirm", fmt.Sprintf(`{"ids":[%d,999999]}`, alarm), 404})
	})
	// Then: the first alarm remains unconfirmed after the rejected transaction.
	if count(t, f.pool, fmt.Sprintf("SELECT count(*) FROM alarms WHERE id=%d AND confirmed_at IS NULL", alarm)) != 1 {
		t.Error("failed batch partially confirmed alarm")
	}
	var batch baselineAdminResponse
	t.Run("valid_batch", func(t *testing.T) {
		batch = f.request(t, baselineAdminRequest{"POST", "/alarms/batch-confirm", "/alarms/batch-confirm", fmt.Sprintf(`{"ids":[%d]}`, alarm), 200})
	})
	if batch.Confirmed != 1 || count(t, f.pool, fmt.Sprintf("SELECT count(*) FROM alarms WHERE id=%d AND confirmed_at IS NOT NULL", alarm)) != 1 {
		t.Error("valid batch did not confirm exactly one alarm")
	}
}
