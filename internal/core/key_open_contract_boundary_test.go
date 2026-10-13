package core_test

import (
	"fmt"
	"testing"
)

func TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites(t *testing.T) {
	f := newKeyOpenFixture(t)
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"unknown", `{"name":"invalid-key","scopes":["ponds:read"],"unknown":1}`, 400},
		{"trailing", `{"name":"invalid-key","scopes":["ponds:read"]} {}`, 400},
		{"null_name", `{"name":null,"scopes":["ponds:read"]}`, 400},
		{"null_scopes", `{"name":"invalid-key","scopes":null}`, 400},
		{"null_resources", `{"name":"invalid-key","scopes":["ponds:read"],"resources":null}`, 400},
		{"null_resource_array", `{"name":"invalid-key","scopes":["ponds:read"],"resources":{"farm_ids":null}}`, 400},
		{"zero_resource", `{"name":"invalid-key","scopes":["ponds:read"],"resources":{"farm_ids":[0]}}`, 400},
		{"foreign_resource", `{"name":"invalid-key","scopes":["ponds:read"],"resources":{"farm_ids":[8303]}}`, 404},
		{"unknown_nested", `{"name":"invalid-key","scopes":["ponds:read"],"resources":{"unknown":1}}`, 400},
		{"duplicate_name", `{"name":"first","name":"second","scopes":["ponds:read"]}`, 400},
		{"duplicate_scopes", `{"name":"invalid-key","scopes":["ponds:read","ponds:read"]}`, 400},
		{"wrong_case", `{"Name":"invalid-key","scopes":["ponds:read"]}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := f.state(t)
			f.admin(t, keyOpenRequest{"POST", "/api-keys", "/api-keys", tc.body, tc.status})
			if f.state(t) != before {
				t.Error("invalid issuance left key/audit state")
			}
		})
	}
}

func TestKeyOpenContractForeignManagementAndMembershipRevocation(t *testing.T) {
	f := newKeyOpenFixture(t)
	issued := f.issueHTTP(t)
	owner := f.token
	f.token = f.otherToken
	for _, operation := range []string{"rotate", "revoke"} {
		t.Run("foreign_"+operation, func(t *testing.T) {
			before := f.state(t)
			f.admin(t, keyOpenRequest{"POST", "/api-keys/{key_id}/" + operation, "/api-keys/" + issued.Key.KeyID + "/" + operation, "", 404})
			if f.state(t) != before {
				t.Error("foreign management mutated key")
			}
		})
	}
	f.token = owner
	execute(t, f.pool, `UPDATE tenant_memberships SET active=false,permission_version=permission_version+1 WHERE user_id=8301`)
	t.Run("revoked_owner", func(t *testing.T) {
		before := f.state(t)
		f.admin(t, keyOpenRequest{"GET", "/api-keys", "/api-keys", "", 401})
		if f.state(t) != before {
			t.Error("revoked membership mutated keys")
		}
	})
}

func TestKeyOpenContractSignedInputBoundsAndNonceState(t *testing.T) {
	f := newKeyOpenFixture(t)
	issued := f.issueHTTP(t)
	for _, id := range []string{"0", "-1", "not-an-integer"} {
		t.Run("invalid_pond_id_"+id, func(t *testing.T) {
			f.signed(t, issued, keyOpenRequest{"GET", "/open/v1/ponds/{id}", "/open/v1/ponds/" + id, "", 400}, m6dNonce(t))
		})
	}
	for _, query := range []string{"name=%ZZ", "name=%FF"} {
		t.Run(fmt.Sprintf("invalid_query_%d", len(query)), func(t *testing.T) {
			before := count(t, f.pool, "SELECT count(*) FROM api_key_nonces")
			status, headers, raw := m6dSignedHTTP(t, f.server, issued.Key, issued.Secret, "/open/v1/ponds?"+query, m6dNonce(t))
			if status != 401 {
				t.Errorf("GET /open/v1/ponds status=%d want=401", status)
			}
			assertHTTPContract(t, "open", "GET", "/open/v1/ponds", status, headers.Get("Content-Type"), raw)
			if count(t, f.pool, "SELECT count(*) FROM api_key_nonces") != before {
				t.Error("invalid signed query consumed nonce")
			}
		})
	}
}
