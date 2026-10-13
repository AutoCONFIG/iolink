package appapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/observability"
)

func TestWechatLoginTracesSafeHTTPOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		errorCode string
		timeout   bool
	}{
		{name: "success", status: 200, body: `{"openid":"private-open-id","session_key":"private-session-key"}`},
		{name: "provider failure", status: 400, body: `{"errcode":40029,"errmsg":"private-provider-message"}`, errorCode: "upstream_status"},
		{name: "malformed response", status: 503, body: `private-body`, errorCode: "upstream_status"},
		{name: "invalid credential", status: 200, body: `{"errcode":40029,"errmsg":"private-provider-message"}`, errorCode: "provider_error"},
		{name: "malformed success response", status: 200, body: `private-body`, errorCode: "response_decode"},
		{name: "timeout", status: 0, errorCode: "deadline_exceeded", timeout: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			var logs bytes.Buffer
			logger, err := observability.Console(observability.Config{Level: "debug"}, &logs)
			if err != nil {
				t.Fatal(err)
			}
			blocked := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.timeout {
					<-blocked
					return
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			defer close(blocked)
			client := server.Client()
			if tc.timeout {
				client.Timeout = 100 * time.Millisecond
			}
			exchange := RealWechatExchangerContext(WechatConfig{AppID: "private-app-id", Secret: "private-app-secret", HTTP: client, SessionURL: server.URL}, logger)

			// When
			openID, err := exchange(observability.ContextWithRequestID(context.Background(), "login-correlation"), "private-login-code")

			// Then
			if (err != nil) != (tc.errorCode != "") || tc.errorCode == "" && openID != "private-open-id" || tc.timeout && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("exchange openID=%q error=%v", openID, err)
			}
			for _, secret := range []string{"private-app-id", "private-app-secret", "private-login-code", "private-open-id", "private-session-key", "private-provider-message", "private-body", server.URL, "js_code=", "grant_type="} {
				if strings.Contains(logs.String(), secret) {
					t.Fatalf("diagnostics exposed forbidden fixture %q", secret)
				}
			}
			var completed bool
			decoder := json.NewDecoder(&logs)
			for decoder.More() {
				var record map[string]any
				if err := decoder.Decode(&record); err != nil {
					t.Fatal(err)
				}
				if record["request_id"] != "login-correlation" || record["operation"] != "code2session" || record["stage"] != "wechat" {
					t.Fatalf("uncorrelated login record: %v", record)
				}
				if record["outcome"] != "started" {
					completed = true
					outcome := "success"
					if tc.errorCode != "" {
						outcome = "failed"
					}
					if record["status"] != float64(tc.status) || record["outcome"] != outcome || record["error_code"] != tc.errorCode || record["duration_ms"] == nil {
						t.Fatalf("login outcome mismatch: %v", record)
					}
				}
				raw, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				t.Log(string(raw))
			}
			if !completed {
				t.Fatal("missing terminal login record")
			}
		})
	}
}

func TestWechatLoginHTTPPropagatesGeneratedRequestIDToDownstream(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{name: "success", body: `{"openid":"private-open-id","session_key":"private-session-key"}`, want: http.StatusOK},
		{name: "provider rejected", body: `{"errcode":40029,"errmsg":"private-provider-message"}`, want: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			var logs bytes.Buffer
			logger, err := observability.Console(observability.Config{Level: "debug"}, &logs)
			if err != nil {
				t.Fatal(err)
			}
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tc.body)
			}))
			defer provider.Close()
			server := New(Config{SecretKey: "private-jwt-secret", JWT: time.Hour, Wechat: WechatConfig{AppID: "private-app-id", Secret: "private-app-secret", HTTP: provider.Client(), SessionURL: provider.URL}}, Deps{Users: fakeTenantUsers{}}, logger)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login?private-query=value", strings.NewReader(`{"code":"private-login-code"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Request-ID", "private-caller-id")
			response := httptest.NewRecorder()

			// When
			server.Routes().ServeHTTP(response, request)

			// Then
			id := response.Header().Get("X-Request-ID")
			if response.Code != tc.want || id == "" || id == "private-caller-id" {
				t.Fatalf("status=%d request ID=%q", response.Code, id)
			}
			for _, secret := range []string{"private-app-id", "private-app-secret", "private-login-code", "private-open-id", "private-session-key", "private-provider-message", "private-jwt-secret", "private-caller-id", "private-query", provider.URL} {
				if strings.Contains(logs.String(), secret) {
					t.Fatalf("diagnostics exposed forbidden fixture %q", secret)
				}
			}
			seen := map[string]bool{}
			decoder := json.NewDecoder(&logs)
			for decoder.More() {
				var record map[string]any
				if err := decoder.Decode(&record); err != nil {
					t.Fatal(err)
				}
				if record["request_id"] != id {
					t.Fatalf("HTTP/downstream correlation changed: %v", record)
				}
				if record["operation"] == "code2session" {
					if record["outcome"] == "started" {
						seen["downstream_start"] = true
					} else {
						seen["downstream_completion"] = true
					}
				}
				if record["route"] == "/api/v1/auth/login" && record["status"] == float64(tc.want) {
					seen["http_completion"] = true
				}
				raw, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				t.Log(string(raw))
			}
			for _, stage := range []string{"downstream_start", "downstream_completion", "http_completion"} {
				if !seen[stage] {
					t.Fatalf("missing correlated stage %s", stage)
				}
			}
		})
	}
}
