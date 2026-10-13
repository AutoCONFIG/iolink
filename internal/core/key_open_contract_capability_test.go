package core_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
)

func TestKeyOpenContractMissingVaultFailsUnavailableWithoutWrites(t *testing.T) {
	f := newKeyOpenFixture(t)
	issued := f.issueHTTP(t)
	f.svc.SetAPIKeyRoot(nil)
	ids := make(map[string]bool)
	t.Run("issue", func(t *testing.T) {
		before := f.state(t)
		f.admin(t, keyOpenRequest{"POST", "/api-keys", "/api-keys", `{"name":"unavailable-key","scopes":["ponds:read"]}`, 503})
		ids[f.requestID] = false
		if f.state(t) != before {
			t.Error("missing vault mutated keys or audits")
		}
	})
	for _, tc := range []keyOpenRequest{
		{"GET", "/open/v1/ponds", "/open/v1/ponds", "", 503},
		{"GET", "/open/v1/ponds/{id}", "/open/v1/ponds/8301", "", 503},
		{"GET", "/open/v1/devices", "/open/v1/devices", "", 503},
		{"GET", "/open/v1/devices/{device_no}", "/open/v1/devices/m6d-allowed", "", 503},
		{"GET", "/open/v1/alarms", "/open/v1/alarms", "", 503},
	} {
		t.Run(tc.route, func(t *testing.T) {
			before := f.state(t)
			f.signed(t, issued, tc, m6dNonce(t))
			ids[f.requestID] = false
			if f.state(t) != before {
				t.Error("missing vault mutated keys or audits")
			}
		})
	}
	f.capabilityDiagnostics(t, ids)
}

func TestKeyOpenContractMissingKeyManagementPortRejectsWithoutWrites(t *testing.T) {
	f := newKeyOpenFixture(t)
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(adminapi.New(adminapi.Config{SecretKey: strings.Repeat("k", 32), JWT: time.Hour}, adminapi.Deps{Store: f.svc, Policy: policy, Logger: f.logger}).UserRoutes())
	t.Cleanup(server.Close)
	f.server = server
	ids := make(map[string]bool)
	for _, tc := range []keyOpenRequest{
		{"GET", "/api-keys", "/api-keys", "", 503},
		{"POST", "/api-keys", "/api-keys", `{"name":"unavailable","scopes":["ponds:read"]}`, 503},
		{"POST", "/api-keys/{key_id}/rotate", "/api-keys/unavailable/rotate", "", 503},
		{"POST", "/api-keys/{key_id}/revoke", "/api-keys/unavailable/revoke", "", 503},
		{"GET", "/api-keys/audit", "/api-keys/audit", "", 503},
	} {
		t.Run(tc.method+tc.route, func(t *testing.T) {
			before := f.state(t)
			f.admin(t, tc)
			ids[f.requestID] = false
			if f.state(t) != before {
				t.Error("missing management port left writes")
			}
		})
	}
	f.capabilityDiagnostics(t, ids)
}

func (f *keyOpenFixture) capabilityDiagnostics(t *testing.T, ids map[string]bool) {
	t.Helper()
	f.server.Close()
	if err := f.sink.Close(); err != nil {
		t.Fatal("dump close failed")
	}
	raw, err := os.ReadFile(f.dumpPath)
	if err != nil {
		t.Fatal("dump unavailable")
	}
	for _, secret := range []string{f.token, f.otherToken, "Key-open-only-1234"} {
		if strings.Contains(string(raw), secret) {
			t.Error("dump contains credentials")
		}
	}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var record struct {
			RequestID string `json:"request_id"`
			Stage     string `json:"stage"`
			Level     string `json:"level"`
			Error     string `json:"err"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal("invalid dump JSON")
		}
		if _, exists := ids[record.RequestID]; exists && record.Level == "ERROR" && record.Stage != "" {
			ids[record.RequestID] = true
			if record.Error != "" {
				t.Error("capability diagnostic manufactured an error")
			}
		}
	}
	for _, recorded := range ids {
		if !recorded {
			t.Error("capability lacks correlated diagnostic")
		}
	}
}
