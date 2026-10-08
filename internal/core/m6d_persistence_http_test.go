package core_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/openapi"
)

func TestM6dBodyDigest_rejectsOnlyChangedBodyAndPreservesNonce(t *testing.T) {
	f := newM6dHTTPFixture(t)
	key, secret := f.issue(t, []string{"ponds:read"}, domain.APIKeyResourceScope{})
	nonce := m6dNonce(t)
	status, _, _ := m6dSignedHTTPBody(t, f.server, key, secret, "/open/v1/ponds", nonce, "original", "modified")
	if status != http.StatusUnauthorized {
		t.Fatalf("changed body status=%d", status)
	}
	status, _, _ = m6dSignedHTTPBody(t, f.server, key, secret, "/open/v1/ponds", nonce, "original", "original")
	if status != http.StatusOK {
		t.Fatalf("rejected signature consumed nonce: status=%d", status)
	}
}

func TestM6dRestart_preservesConsumedNonceAndRateState(t *testing.T) {
	f := newM6dHTTPFixture(t)
	key, secret := f.issue(t, []string{"ponds:read"}, domain.APIKeyResourceScope{})
	nonce := m6dNonce(t)
	status, _, _ := m6dSignedHTTP(t, f.server, key, secret, "/open/v1/ponds", nonce)
	if status != http.StatusOK {
		t.Fatalf("initial status=%d", status)
	}
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := core.NewWithPolicy(t.Context(), f.pool, slog.New(slog.NewTextHandler(io.Discard, nil)), policy)
	if err != nil {
		t.Fatal(err)
	}
	restarted.SetAPIKeyRoot([]byte(strings.Repeat("r", 32)))
	restarted.SetLicenseRuntime(&core.LicenseRuntime{PublicKey: &f.licenseKey.PublicKey, KeyID: "test-key"})
	server := httptest.NewServer(openapi.New(openapi.Deps{Auth: restarted, Resources: restarted}).Routes())
	defer server.Close()
	status, _, _ = m6dSignedHTTP(t, server, key, secret, "/open/v1/ponds", nonce)
	if status != http.StatusUnauthorized {
		t.Fatalf("restart replay status=%d", status)
	}
	if _, err := f.pool.Exec(t.Context(), `UPDATE api_keys SET rate_tokens=0,rate_last_refill=now()+interval '1 hour' WHERE key_id=$1`, key.KeyID); err != nil {
		t.Fatal(err)
	}
	status, _, _ = m6dSignedHTTP(t, server, key, secret, "/open/v1/ponds", m6dNonce(t))
	if status != http.StatusTooManyRequests {
		t.Fatalf("restart rate status=%d", status)
	}
}

func TestM6dThrottle_preservesNonceAndLeavesTokenUnconsumed(t *testing.T) {
	f := newM6dHTTPFixture(t)
	key, secret := f.issue(t, []string{"ponds:read"}, domain.APIKeyResourceScope{})
	if _, err := f.pool.Exec(t.Context(), `UPDATE api_keys SET rate_tokens=0,rate_last_refill=now()+interval '1 hour' WHERE key_id=$1`, key.KeyID); err != nil {
		t.Fatal(err)
	}
	nonce := m6dNonce(t)
	status, _, _ := m6dSignedHTTP(t, f.server, key, secret, "/open/v1/ponds", nonce)
	if status != http.StatusTooManyRequests {
		t.Fatalf("initial throttle status=%d", status)
	}
	var tokens float64
	if err := f.pool.QueryRow(t.Context(), `SELECT rate_tokens FROM api_keys WHERE key_id=$1`, key.KeyID).Scan(&tokens); err != nil || tokens != 0 {
		t.Fatalf("throttle token state=%v error=%v", tokens, err)
	}
	if _, err := f.pool.Exec(t.Context(), `UPDATE api_keys SET rate_tokens=10 WHERE key_id=$1`, key.KeyID); err != nil {
		t.Fatal(err)
	}
	status, _, _ = m6dSignedHTTP(t, f.server, key, secret, "/open/v1/ponds", nonce)
	if status != http.StatusUnauthorized {
		t.Fatalf("throttled nonce reusable: status=%d", status)
	}
	status, _, _ = m6dSignedHTTP(t, f.server, key, secret, "/open/v1/ponds", m6dNonce(t))
	if status != http.StatusOK {
		t.Fatalf("fresh nonce rejected after refill: status=%d", status)
	}
}

func TestM6dPublishedVector_authenticatesWithProductionVerifier(t *testing.T) {
	f := newM6dHTTPFixture(t)
	key, _ := f.issue(t, []string{"ponds:read"}, domain.APIKeyResourceScope{})
	rootHash := sha256.Sum256([]byte(strings.Repeat("r", 32) + ":api-key"))
	block, err := aes.NewCipher(rootHash[:])
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	sealed := gcm.Seal(nil, nonce, []byte("documented-test-secret"), nil)
	if _, err := f.pool.Exec(t.Context(), `UPDATE api_keys SET encrypted_secret=$2,secret_nonce=$3 WHERE key_id=$1`, key.KeyID, sealed, nonce); err != nil {
		t.Fatal(err)
	}
	request := domain.OpenRequest{KeyID: key.KeyID, Method: "GET", Path: "/open/v1/ponds", Query: "b=two&a=hello%20world&a=&b=one", Timestamp: 1735689600, Nonce: "0123456789abcdef0123456789abcdef", Signature: "bcf8386ae0789327c5a6597a81eaa2f3121565d045012796e7e22bdba2a43149"}
	principal, err := f.svc.AuthenticateOpen(context.Background(), request, time.Unix(request.Timestamp, 0))
	if err != nil {
		t.Fatalf("published vector rejected: %v", err)
	}
	if principal.KeyID != key.KeyID || principal.TenantID != 8301 {
		t.Fatal("wrong principal")
	}
}
