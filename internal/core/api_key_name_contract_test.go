package core_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity(t *testing.T) {
	// Given: real tenant authentication, License, vault, TCP HTTP and isolated PostgreSQL.
	f := newKeyOpenFixture(t)
	var secrets []string
	for _, tc := range []struct {
		label, name string
		status      int
	}{
		{"ascii_128", strings.Repeat("a", 128), 201},
		{"unicode_128", strings.Repeat("界", 128), 201},
		{"ascii_129", strings.Repeat("a", 129), 400},
		{"unicode_129", strings.Repeat("界", 129), 400},
	} {
		t.Run(tc.label, func(t *testing.T) {
			before := f.state(t)
			keysBefore := count(t, f.pool, "SELECT count(*) FROM api_keys")
			auditBefore := count(t, f.pool, "SELECT count(*) FROM audit_events WHERE action='api_key.issued'")
			body, err := json.Marshal(struct {
				Name   string   `json:"name"`
				Scopes []string `json:"scopes"`
			}{tc.name, []string{"ponds:read"}})
			if err != nil {
				t.Fatal("request encoding failed")
			}
			// When: the owner issues a key at the character boundary.
			raw := f.admin(t, keyOpenRequest{"POST", "/api-keys", "/api-keys", string(body), tc.status})
			// Then: HTTP status and actual response satisfy the current OpenAPI contract.
			keyDelta := count(t, f.pool, "SELECT count(*) FROM api_keys") - keysBefore
			auditDelta := count(t, f.pool, "SELECT count(*) FROM audit_events WHERE action='api_key.issued'") - auditBefore
			t.Logf("POST /api-keys characters=%d bytes=%d status=%d key_delta=%d issued_audit_delta=%d", utf8.RuneCountInString(tc.name), len(tc.name), f.responseStatus, keyDelta, auditDelta)
			if tc.status == 400 {
				if f.state(t) != before || keyDelta != 0 || auditDelta != 0 {
					t.Error("invalid name changed key/audit state")
				}
				if bytes.Contains(raw, []byte(tc.name)) || bytes.Contains(raw, []byte("secret")) {
					t.Error("invalid response exposed input or secret")
				}
				return
			}
			if f.responseStatus != 201 {
				return
			}
			var issued keyOpenIssued
			platformContractDecode(t, raw, &issued)
			secret, err := base64.RawURLEncoding.DecodeString(issued.Secret)
			if err != nil || len(secret) != 32 || issued.Key.KeyID == "" || issued.Key.Name != tc.name {
				t.Fatal("issued name or one-time credential invalid")
			}
			var storedName string
			var ciphertext, nonce []byte
			if err := f.pool.QueryRow(t.Context(), `SELECT name,encrypted_secret,secret_nonce FROM api_keys WHERE key_id=$1 AND tenant_id=8301`, issued.Key.KeyID).Scan(&storedName, &ciphertext, &nonce); err != nil {
				t.Fatal("persisted issued key unavailable")
			}
			if storedName != tc.name || keyDelta != 1 || auditDelta != 1 || len(ciphertext) == 0 || len(nonce) == 0 || bytes.Equal(ciphertext, secret) {
				t.Error("issuance did not persist matching name, encrypted key and audit together")
			}
			secrets = append(secrets, issued.Secret)
		})
	}
	for _, route := range []string{"/api-keys", "/api-keys/audit"} {
		t.Run("secret_privacy"+route, func(t *testing.T) {
			// When: the owner reads key metadata or operation audit.
			raw := f.admin(t, keyOpenRequest{"GET", route, route, "", 200})
			// Then: neither response returns any one-time credential.
			for _, secret := range secrets {
				if bytes.Contains(raw, []byte(secret)) {
					t.Error("metadata response exposed secret")
				}
			}
			if bytes.Contains(raw, []byte(`"secret"`)) || bytes.Contains(raw, []byte("encrypted_secret")) || bytes.Contains(raw, []byte("secret_nonce")) {
				t.Error("metadata response exposed credential fields")
			}
			t.Logf("GET %s status=%d one_time_secret_absent=true", route, f.responseStatus)
		})
	}
	f.server.Close()
	if err := f.sink.Close(); err != nil {
		t.Fatal("diagnostic sink close failed")
	}
	dump, err := os.ReadFile(f.dumpPath)
	if err != nil {
		t.Fatal("diagnostic dump unavailable")
	}
	for _, private := range append(secrets, f.token, f.otherToken, "Key-open-only-1234", strings.Repeat("界", 128), strings.Repeat("a", 128)) {
		if bytes.Contains(dump, []byte(private)) {
			t.Error("diagnostic dump exposed credentials or request name")
		}
	}
	t.Log("diagnostic_dump_credentials_and_request_name_absent=true")
}
