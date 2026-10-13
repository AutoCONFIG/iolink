package core_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/appapi"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
)

func TestBaselineWechatLoginClassifiesRealUpstreamFailuresWithoutProvisioningUsers(t *testing.T) {
	for _, tc := range []struct {
		name, body, code, diagnostic string
		status, want                 int
		offline                      bool
	}{
		{"invalid code", `{"errcode":40029,"errmsg":"private-provider-message"}`, "unauthorized", "provider_error", 200, 401, false},
		{"provider busy", `{"errcode":-1,"errmsg":"private-provider-message"}`, "upstream_error", "provider_error", 200, 502, false},
		{"http unavailable", `{"openid":"private-open-id","session_key":"private-session-key"}`, "upstream_error", "upstream_status", 503, 502, false},
		{"http bad request with invalid code", `{"errcode":40029,"errmsg":"private-provider-message"}`, "upstream_error", "upstream_status", 400, 502, false},
		{"malformed response", `private-provider-body`, "upstream_error", "response_decode", 200, 502, false},
		{"missing openid", `{"session_key":"private-session-key"}`, "upstream_error", "provider_error", 200, 502, false},
		{"transport unavailable", "", "upstream_error", "network_error", 0, 502, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given: real PostgreSQL and a local HTTP provider with one failure class.
			svc, pool := setup(t)
			before := count(t, pool, `SELECT count(*) FROM users`)
			var logs bytes.Buffer
			logger, err := observability.Console(observability.Config{Level: "debug"}, &logs)
			if err != nil {
				t.Fatal(err)
			}
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				if _, err := io.WriteString(w, tc.body); err != nil {
					t.Error("local provider response write failed")
				}
			}))
			t.Cleanup(provider.Close)
			if tc.offline {
				provider.Close()
			}
			api := appapi.New(appapi.Config{SecretKey: "private-jwt-key", JWT: time.Hour, Wechat: appapi.WechatConfig{AppID: "private-app-id", Secret: "private-app-secret", HTTP: provider.Client(), SessionURL: provider.URL}}, appapi.Deps{Users: svc}, logger)
			server := httptest.NewServer(api.Routes())
			t.Cleanup(server.Close)
			// When: the production login endpoint calls its HTTP adapter over TCP.
			response := baselineAppHTTP(t, server, baselineAppRequest{method: "POST", path: "/auth/login", body: `{"code":"private-login-code"}`})
			server.Close()
			// Then: the credential/upstream distinction is public and no user is created.
			if count(t, pool, `SELECT count(*) FROM users`) != before {
				t.Fatal("failed upstream login provisioned a user")
			}
			response.contract(t, "POST", "/auth/login", tc.want)
			var public struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(response.raw, &public); err != nil || public.Error != tc.code {
				t.Fatal("login failure did not return its declared machine code")
			}
			for _, private := range []string{"private-app-id", "private-app-secret", "private-login-code", "private-open-id", "private-session-key", "private-provider-message", "private-provider-body", "private-jwt-key", provider.URL, "js_code=", "grant_type="} {
				if strings.Contains(logs.String(), private) {
					t.Fatal("private upstream data leaked into diagnostics")
				}
			}
			seen := false
			requestID := response.requestID
			if requestID == "" {
				t.Fatal("HTTP response did not expose its server-generated request ID")
			}
			for _, line := range bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte{'\n'}) {
				var record struct {
					Stage, Outcome string
					RequestID      string `json:"request_id"`
					ErrorCode      string `json:"error_code"`
					Status         int
				}
				if err := json.Unmarshal(line, &record); err != nil {
					t.Fatal("diagnostic record malformed")
				}
				if record.RequestID != requestID {
					t.Fatal("HTTP and downstream diagnostics lost request correlation")
				}
				if record.Stage == "wechat" && record.Outcome == "failed" && record.ErrorCode == tc.diagnostic && record.Status == tc.status {
					seen = true
				}
			}
			if !seen {
				t.Fatal("safe upstream completion diagnostic missing")
			}
			t.Logf("provider_status=%d login_status=%d machine_code=%s diagnostic=%s user_rows_unchanged=true", tc.status, response.status, tc.code, tc.diagnostic)
		})
	}
}
