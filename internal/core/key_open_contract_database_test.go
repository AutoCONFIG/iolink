package core_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestKeyOpenContractDatabaseFailuresAreInternalAtomicAndDiagnosed(t *testing.T) {
	for _, tc := range []struct {
		name, table string
		request     keyOpenRequest
		open        bool
	}{
		{"key_list", "api_keys", keyOpenRequest{"GET", "/api-keys", "/api-keys", "", 500}, false},
		{"key_audit", "audit_events", keyOpenRequest{"GET", "/api-keys/audit", "/api-keys/audit", "", 500}, false},
		{"key_issue_audit", "audit_events", keyOpenRequest{"POST", "/api-keys", "/api-keys", `{"name":"rollback-key","scopes":["ponds:read"]}`, 500}, false},
		{"key_rotate_audit", "audit_events", keyOpenRequest{"POST", "/api-keys/{key_id}/rotate", "", "", 500}, false},
		{"key_revoke_audit", "audit_events", keyOpenRequest{"POST", "/api-keys/{key_id}/revoke", "", "", 500}, false},
		{"open_key_auth", "api_keys", keyOpenRequest{"GET", "/open/v1/ponds", "/open/v1/ponds", "", 500}, true},
		{"open_tenant_auth", "tenants", keyOpenRequest{"GET", "/open/v1/ponds", "/open/v1/ponds", "", 500}, true},
		{"open_nonce_auth", "api_key_nonces", keyOpenRequest{"GET", "/open/v1/ponds", "/open/v1/ponds", "", 500}, true},
		{"open_pond_query", "ponds", keyOpenRequest{"GET", "/open/v1/ponds", "/open/v1/ponds", "", 500}, true},
		{"open_pond_detail", "ponds", keyOpenRequest{"GET", "/open/v1/ponds/{id}", "/open/v1/ponds/8301", "", 500}, true},
		{"open_device_list", "devices", keyOpenRequest{"GET", "/open/v1/devices", "/open/v1/devices", "", 500}, true},
		{"open_device_query", "devices", keyOpenRequest{"GET", "/open/v1/devices/{device_no}", "/open/v1/devices/m6d-allowed", "", 500}, true},
		{"open_alarm_query", "alarms", keyOpenRequest{"GET", "/open/v1/alarms", "/open/v1/alarms", "", 500}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newKeyOpenFixture(t)
			issued := f.issueHTTP(t)
			request := tc.request
			if request.route == "/api-keys/{key_id}/rotate" {
				request.path = "/api-keys/" + issued.Key.KeyID + "/rotate"
			}
			if request.route == "/api-keys/{key_id}/revoke" {
				request.path = "/api-keys/" + issued.Key.KeyID + "/revoke"
			}
			before := f.state(t)
			execute(t, f.pool, "ALTER TABLE "+tc.table+" RENAME TO key_contract_relation_unavailable")
			var probe int
			probeErr := f.pool.QueryRow(t.Context(), "SELECT 1 FROM "+tc.table+" LIMIT 1").Scan(&probe)
			var state interface{ SQLState() string }
			if !errors.As(probeErr, &state) || state.SQLState() != "42P01" {
				t.Fatal("relation fault did not produce expected SQLSTATE")
			}
			t.Log("injected relation failure SQLSTATE=42P01")
			if tc.open {
				f.signed(t, issued, request, m6dNonce(t))
			} else {
				f.admin(t, request)
			}
			execute(t, f.pool, "ALTER TABLE key_contract_relation_unavailable RENAME TO "+tc.table)
			if f.state(t) != before {
				t.Error("database failure left key/audit/device writes")
			}
			f.databaseDiagnostic(t, issued.Secret)
		})
	}
}

func (f *keyOpenFixture) databaseDiagnostic(t *testing.T, secret string) {
	t.Helper()
	f.server.Close()
	if err := f.sink.Close(); err != nil {
		t.Fatal("debug dump close failed")
	}
	raw, err := os.ReadFile(f.dumpPath)
	if err != nil {
		t.Fatal("debug dump unavailable")
	}
	for _, private := range []string{f.token, f.otherToken, secret, "Key-open-only-1234"} {
		if strings.Contains(string(raw), private) {
			t.Error("debug dump contains credentials")
		}
	}
	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var record struct {
			RequestID string `json:"request_id"`
			Stage     string `json:"stage"`
			Level     string `json:"level"`
			Error     string `json:"err"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal("debug dump JSON invalid")
		}
		if record.RequestID == f.requestID && record.Level == "ERROR" && record.Stage != "" && record.Error == "postgres_42P01" {
			found = true
		}
	}
	if !found {
		t.Error("SQL failure has no correlated safe diagnostic")
	}
}
