package core_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSetupContractsAcrossRealHTTPInitialization(t *testing.T) {
	f := newSetupContractFixture(t)
	status := func(want bool) {
		raw := f.request(t, setupContractRequest{method: "GET", route: "/status", status: 200})
		var state struct{ Required bool }
		if err := json.Unmarshal(raw, &state); err != nil || state.Required != want {
			t.Error("setup required state differs")
		}
	}
	status(true)
	f.request(t, setupContractRequest{"POST", "/initialize", setupContractCredentials, setupContractKey, "application/json", f.server.URL, 204})
	status(false)
	f.request(t, setupContractRequest{"POST", "/initialize", setupContractCredentials, setupContractKey, "application/json", "", 409})
	if count(t, f.pool, "SELECT count(*) FROM users WHERE authority='ADMIN'") != 1 || count(t, f.pool, "SELECT count(*) FROM tenant_memberships") != 0 || count(t, f.pool, "SELECT count(*) FROM tenants WHERE name<>'__iolink_system__'") != 0 {
		t.Error("setup did not create exactly one platform administrator without business memberships")
	}
}

func TestSetupContractsRejectInvalidRequestsWithoutProvisioning(t *testing.T) {
	for _, tc := range []struct {
		name, body, key, media, origin string
		status                         int
	}{
		{"missing key", setupContractCredentials, "", "application/json", "", 401},
		{"cross origin", setupContractCredentials, setupContractKey, "application/json", "https://foreign.invalid", 403},
		{"wrong media", setupContractCredentials, setupContractKey, "text/plain", "", 415},
		{"unknown field", `{"username":"operator","password":"Contract-operator-only-1234","extra":true}`, setupContractKey, "application/json", "", 400},
		{"wrong field case", `{"Username":"operator","password":"Contract-operator-only-1234"}`, setupContractKey, "application/json", "", 400},
		{"duplicate field", `{"username":"operator","username":"other","password":"Contract-operator-only-1234"}`, setupContractKey, "application/json", "", 400},
		{"trailing JSON", setupContractCredentials + `{}`, setupContractKey, "application/json", "", 400},
		{"null password", `{"username":"operator","password":null}`, setupContractKey, "application/json", "", 400},
		{"missing password", `{"username":"operator"}`, setupContractKey, "application/json", "", 400},
		{"wrong value type", `{"username":10,"password":"Contract-operator-only-1234"}`, setupContractKey, "application/json", "", 400},
		{"invalid UTF8", "{\"username\":\"operat\xffor\",\"password\":\"Contract-operator-only-1234\"}", setupContractKey, "application/json", "", 400},
		{"array root", `[]`, setupContractKey, "application/json", "", 400},
		{"null root", `null`, setupContractKey, "application/json", "", 400},
		{"short password", `{"username":"operator","password":"short"}`, setupContractKey, "application/json", "", 400},
		{"username whitespace", `{"username":" operator","password":"Contract-operator-only-1234"}`, setupContractKey, "application/json", "", 400},
		{"oversize", strings.Repeat(" ", 4097) + setupContractCredentials, setupContractKey, "application/json", "", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSetupContractFixture(t)
			f.request(t, setupContractRequest{"POST", "/initialize", tc.body, tc.key, tc.media, tc.origin, tc.status})
			if count(t, f.pool, "SELECT count(*) FROM users") != 0 || count(t, f.pool, "SELECT count(*) FROM tenant_memberships") != 0 {
				t.Error("rejected setup request provisioned identity")
			}
		})
	}
}

func TestSetupContractsStorageFailuresStayClosed(t *testing.T) {
	f := newSetupContractFixture(t)
	execute(t, f.pool, "ALTER TABLE users RENAME TO setup_fault_relation")
	f.request(t, setupContractRequest{method: "GET", route: "/status", status: 503})
	f.request(t, setupContractRequest{"POST", "/initialize", setupContractCredentials, setupContractKey, "application/json", "", 503})
	execute(t, f.pool, "ALTER TABLE setup_fault_relation RENAME TO users")
	execute(t, f.pool, "ALTER TABLE audit_events RENAME TO setup_fault_audit")
	f.request(t, setupContractRequest{"POST", "/initialize", setupContractCredentials, setupContractKey, "application/json", "", 503})
	execute(t, f.pool, "ALTER TABLE setup_fault_audit RENAME TO audit_events")
	if count(t, f.pool, "SELECT count(*) FROM users") != 0 {
		t.Error("failed setup audit persisted administrator")
	}
}
