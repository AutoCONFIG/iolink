package core_test

import (
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
	"git.hyhy.fun/rsplab/iolink/internal/openapi"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
)

type keyOpenFixture struct {
	logger *slog.Logger
	m6dHTTPFixture
	token, otherToken   string
	sink                *observability.Sink
	dumpPath, requestID string
	responseStatus      int
	responseContentType string
}

type keyOpenRequest struct {
	method, route, path, body string
	status                    int
}

type keyOpenIssued struct {
	Key    domain.APIKey `json:"key"`
	Secret string        `json:"secret"`
}

func newKeyOpenFixture(t *testing.T) *keyOpenFixture {
	t.Helper()
	f := newM6dHTTPFixture(t)
	f.server.Close()
	execute(t, f.pool, `UPDATE users SET username=CASE id WHEN 8301 THEN 'key-owner' ELSE 'key-other' END,password_hash=$1 WHERE id IN (8301,8303)`, platform.HashPassword("Key-open-only-1234"))
	root := t.TempDir()
	logger, sink, err := observability.New(observability.Config{Dir: root, Version: "key-open-contract", Level: "error", MaxMB: 1, Backups: 1}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sink.Close(); err != nil {
			t.Error(err)
		}
	})
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := core.NewWithPolicy(context.Background(), f.pool, logger, policy)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetLicenseRuntime(&core.LicenseRuntime{PublicKey: &f.licenseKey.PublicKey, KeyID: "test-key"})
	svc.SetAPIKeyRoot([]byte(strings.Repeat("r", 32)))
	if err := svc.ConfigureOpenRateLimit(60, 100); err != nil {
		t.Fatal(err)
	}
	f.svc = svc
	mux := http.NewServeMux()
	mux.Handle("/user/", adminapi.New(adminapi.Config{SecretKey: strings.Repeat("k", 32), JWT: time.Hour}, adminapi.Deps{Store: svc, APIKeys: svc, Policy: policy, Logger: logger}).UserRoutes())
	mux.Handle("/open/", openapi.New(openapi.Deps{Auth: svc, Resources: svc, Logger: logger}).Routes())
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	out := &keyOpenFixture{m6dHTTPFixture: f, sink: sink, logger: logger, dumpPath: filepath.Join(root, "key-open-contract", "iolinkd.jsonl")}
	for _, who := range []string{"key-owner", "key-other"} {
		var login struct {
			Token string `json:"token"`
		}
		m6bHTTPRequest(t, f.server.URL, "", "POST", "/user/v1/login", `{"username":"`+who+`","password":"Key-open-only-1234"}`, 200, &login)
		if login.Token == "" {
			t.Fatal("fixture login returned no token")
		}
		if who == "key-owner" {
			out.token = login.Token
		} else {
			out.otherToken = login.Token
		}
	}
	return out
}

func (f *keyOpenFixture) admin(t *testing.T, tc keyOpenRequest) []byte {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), tc.method, f.server.URL+"/user/v1"+tc.path, strings.NewReader(tc.body))
	if err != nil {
		t.Fatal("HTTP request construction failed")
	}
	req.Header.Set("Content-Type", "application/json")
	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
	return f.response(t, req, tc, "admin")
}

func (f *keyOpenFixture) response(t *testing.T, req *http.Request, tc keyOpenRequest, spec string) []byte {
	t.Helper()
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal("HTTP request failed")
	}
	raw, err := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if err != nil || closeErr != nil {
		t.Fatal("HTTP response read failed")
	}
	f.requestID = resp.Header.Get("X-Request-ID")
	f.responseStatus = resp.StatusCode
	f.responseContentType = resp.Header.Get("Content-Type")
	if resp.StatusCode != tc.status {
		t.Errorf("%s %s status=%d want=%d", tc.method, tc.route, resp.StatusCode, tc.status)
	}
	t.Run("response_contract", func(t *testing.T) {
		assertHTTPContract(t, spec, tc.method, tc.route, resp.StatusCode, resp.Header.Get("Content-Type"), raw)
	})
	return raw
}

func (f *keyOpenFixture) signed(t *testing.T, issued keyOpenIssued, tc keyOpenRequest, nonce string) []byte {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), tc.method, f.server.URL+tc.path, strings.NewReader(tc.body))
	if err != nil {
		t.Fatal("signed HTTP request construction failed")
	}
	secret, err := base64.RawURLEncoding.DecodeString(issued.Secret)
	if err != nil {
		t.Fatal("fixture secret decode failed")
	}
	stamp := time.Now().UTC().Unix()
	signature := m6dSignatureForRequest(t, secret, domain.OpenRequest{Method: tc.method, Path: req.URL.EscapedPath(), Query: req.URL.RawQuery, Timestamp: stamp, Nonce: nonce, Body: []byte(tc.body)})
	req.Header.Set("X-Key-Id", issued.Key.KeyID)
	req.Header.Set("X-Timestamp", strconv.FormatInt(stamp, 10))
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", signature)
	return f.response(t, req, tc, "open")
}

func (f *keyOpenFixture) issueHTTP(t *testing.T) keyOpenIssued {
	t.Helper()
	var result keyOpenIssued
	platformContractDecode(t, f.admin(t, keyOpenRequest{"POST", "/api-keys", "/api-keys", `{"name":"contract-key","scopes":["ponds:read","devices:read","alarms:read"],"resources":{"farm_ids":[8301]}}`, 201}), &result)
	if result.Key.KeyID == "" || result.Secret == "" {
		t.Fatal("key issuance failed")
	}
	return result
}

func (f *keyOpenFixture) state(t *testing.T) string {
	t.Helper()
	var digest string
	err := f.pool.QueryRow(t.Context(), `SELECT md5(jsonb_build_object(
		'keys',(SELECT jsonb_agg(jsonb_build_array(key_id,tenant_id,name,scopes,resources,revoked_at,rotated_from) ORDER BY key_id) FROM api_keys),
		'audit',(SELECT count(*) FROM audit_events),
		'devices',(SELECT jsonb_agg(jsonb_build_array(id,disabled_at,session_version) ORDER BY id) FROM devices)
	)::text)`).Scan(&digest)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
