package core_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/openapi"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

func TestM6dIssueAndAuthenticateOpenKey(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id,authority) VALUES(8101,'m6d-user','USER'); INSERT INTO tenants(id,name,active) VALUES(8101,'m6d-tenant',true); INSERT INTO tenant_memberships(tenant_id,user_id,role,active) VALUES(8101,8101,'owner',true); INSERT INTO farms(id,owner_id,tenant_id,name) VALUES(8101,8101,8101,'m6d-farm'); INSERT INTO ponds(id,farm_id,name) VALUES(8101,8101,'m6d-pond')`); err != nil {
		t.Fatal(err)
	}
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := core.NewWithPolicy(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)), policy)
	if err != nil {
		t.Fatal(err)
	}
	licenseKey := installTestLicenseWithFeatures(t, p, svc, 100, []string{"openapi"})
	svc.SetAPIKeyRoot([]byte(strings.Repeat("r", 32)))
	ownerCtx := domain.WithTenantUserID(domain.WithTenantRole(domain.WithTenantID(ctx, 8101), "owner"), 8101)
	key, secretText, err := svc.IssueAPIKey(ownerCtx, 8101, "integration", []string{"ponds:read"}, domain.APIKeyResourceScope{}, 8101)
	if err != nil {
		t.Fatal(err)
	}
	if secretText == "" || key.KeyID == "" {
		t.Fatal("issue returned no credentials")
	}
	secret, err := base64.RawURLEncoding.DecodeString(secretText)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	body := []byte{}
	bodyHash := sha256.Sum256(body)
	nonce := "0123456789abcdef0123456789abcdef"
	signed := strings.Join([]string{"GET", "/open/v1/ponds", "", strconv.FormatInt(now.Unix(), 10), nonce, hex.EncodeToString(bodyHash[:])}, "\n")
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signed))
	req := domain.OpenRequest{KeyID: key.KeyID, Timestamp: now.Unix(), Nonce: nonce, Signature: hex.EncodeToString(mac.Sum(nil)), Method: "GET", Path: "/open/v1/ponds", Body: body}
	bad := req
	bad.Signature = strings.Repeat("0", 64)
	if _, err := svc.AuthenticateOpen(ctx, bad, now); err == nil {
		t.Fatal("invalid signature accepted")
	}
	stale := req
	stale.Timestamp = now.Add(-301 * time.Second).Unix()
	if _, err := svc.AuthenticateOpen(ctx, stale, now); !errors.Is(err, core.ErrOpenUnauthorized) {
		t.Fatalf("stale timestamp error=%v", err)
	}
	future := req
	future.Timestamp = now.Add(301 * time.Second).Unix()
	if _, err := svc.AuthenticateOpen(ctx, future, now); !errors.Is(err, core.ErrOpenUnauthorized) {
		t.Fatalf("future timestamp error=%v", err)
	}
	principal, err := svc.AuthenticateOpen(ctx, req, now)
	if err != nil {
		t.Fatal(err)
	}
	if principal.TenantID != 8101 {
		t.Fatalf("tenant = %d", principal.TenantID)
	}
	if _, err := svc.AuthenticateOpen(ctx, req, now); err == nil {
		t.Fatal("replayed nonce accepted")
	}
	httpNow := time.Now().UTC().Truncate(time.Second)
	httpNonce := "abcdef0123456789abcdef0123456789"
	httpHash := sha256.Sum256(nil)
	httpSigned := strings.Join([]string{"GET", "/open/v1/ponds", "", strconv.FormatInt(httpNow.Unix(), 10), httpNonce, hex.EncodeToString(httpHash[:])}, "\n")
	httpMAC := hmac.New(sha256.New, secret)
	_, _ = httpMAC.Write([]byte(httpSigned))
	httpReq := httptest.NewRequest(http.MethodGet, "/open/v1/ponds", nil)
	httpReq.Header.Set("X-Key-Id", key.KeyID)
	httpReq.Header.Set("X-Timestamp", strconv.FormatInt(httpNow.Unix(), 10))
	httpReq.Header.Set("X-Nonce", httpNonce)
	httpReq.Header.Set("X-Signature", hex.EncodeToString(httpMAC.Sum(nil)))
	httpRec := httptest.NewRecorder()
	openapi.New(openapi.Deps{Auth: svc, Resources: svc}).Routes().ServeHTTP(httpRec, httpReq)
	if httpRec.Code != http.StatusOK || !strings.Contains(httpRec.Body.String(), "m6d-pond") {
		t.Fatalf("open http status=%d body=%s", httpRec.Code, httpRec.Body.String())
	}
	tamperedReq := httptest.NewRequest(http.MethodGet, "/open/v1/ponds", strings.NewReader("tampered"))
	tamperedReq.Header.Set("X-Key-Id", key.KeyID)
	tamperedReq.Header.Set("X-Timestamp", strconv.FormatInt(httpNow.Unix(), 10))
	tamperedReq.Header.Set("X-Nonce", "tampered-0123456789abcdef0123456")
	tamperedReq.Header.Set("X-Signature", hex.EncodeToString(httpMAC.Sum(nil)))
	tamperedRec := httptest.NewRecorder()
	openapi.New(openapi.Deps{Auth: svc, Resources: svc}).Routes().ServeHTTP(tamperedRec, tamperedReq)
	if tamperedRec.Code != http.StatusUnauthorized {
		t.Fatalf("tampered body status=%d", tamperedRec.Code)
	}
	restarted, err := core.NewWithPolicy(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)), policy)
	if err != nil {
		t.Fatal(err)
	}
	restarted.SetLicenseRuntime(&core.LicenseRuntime{PublicKey: &licenseKey.PublicKey, KeyID: "test-key"})
	restarted.SetAPIKeyRoot([]byte(strings.Repeat("r", 32)))
	restartNow := time.Now().UTC().Truncate(time.Second)
	restartNonce := "restart-0123456789abcdef0123456789"
	restartHash := sha256.Sum256(nil)
	restartSigned := strings.Join([]string{"GET", "/open/v1/ponds", "", strconv.FormatInt(restartNow.Unix(), 10), restartNonce, hex.EncodeToString(restartHash[:])}, "\n")
	restartMAC := hmac.New(sha256.New, secret)
	_, _ = restartMAC.Write([]byte(restartSigned))
	if _, err := restarted.AuthenticateOpen(ctx, domain.OpenRequest{KeyID: key.KeyID, Timestamp: restartNow.Unix(), Nonce: restartNonce, Signature: hex.EncodeToString(restartMAC.Sum(nil)), Method: "GET", Path: "/open/v1/ponds"}, restartNow); err != nil {
		t.Fatalf("post-restart key authentication: %v", err)
	}
	keys, err := svc.ListAPIKeys(ownerCtx, 8101, 8101)
	if err != nil || len(keys) != 1 {
		t.Fatalf("keys = %d err=%v", len(keys), err)
	}
	rotated, rotatedSecretText, err := svc.RotateAPIKey(ownerCtx, 8101, 8101, key.KeyID)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.KeyID == key.KeyID || rotatedSecretText == secretText {
		t.Fatal("rotation did not create fresh credentials")
	}
	if _, err := svc.AuthenticateOpen(ctx, req, now); !errors.Is(err, core.ErrOpenUnauthorized) {
		t.Fatalf("revoked key error=%v", err)
	}
	if err := svc.RevokeAPIKey(ownerCtx, 8101, 8101, rotated.KeyID); err != nil {
		t.Fatal(err)
	}
	rotatedSecret, err := base64.RawURLEncoding.DecodeString(rotatedSecretText)
	if err != nil {
		t.Fatal(err)
	}
	rotatedHash := sha256.Sum256(nil)
	rotatedNonce := "rotated-0123456789abcdef012345678"
	rotatedSigned := strings.Join([]string{"GET", "/open/v1/ponds", "", strconv.FormatInt(now.Unix(), 10), rotatedNonce, hex.EncodeToString(rotatedHash[:])}, "\n")
	rotatedMAC := hmac.New(sha256.New, rotatedSecret)
	_, _ = rotatedMAC.Write([]byte(rotatedSigned))
	if _, err := svc.AuthenticateOpen(ctx, domain.OpenRequest{KeyID: rotated.KeyID, Timestamp: now.Unix(), Nonce: rotatedNonce, Signature: hex.EncodeToString(rotatedMAC.Sum(nil)), Method: "GET", Path: "/open/v1/ponds"}, now); !errors.Is(err, core.ErrOpenUnauthorized) {
		t.Fatalf("revoked rotated key error=%v", err)
	}
	audit, err := svc.ListAPIKeyAuditEvents(ownerCtx, 8101, 8101, 100)
	if err != nil || len(audit) < 3 {
		t.Fatalf("api key audit events=%d err=%v", len(audit), err)
	}
}

func TestM6dConcurrentNonceAndRateLimit(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id,authority) VALUES(8201,'m6d-concurrent','USER'); INSERT INTO tenants(id,name,active) VALUES(8201,'m6d-concurrent-tenant',true); INSERT INTO tenant_memberships(tenant_id,user_id,role,active) VALUES(8201,8201,'owner',true); INSERT INTO farms(id,owner_id,tenant_id,name) VALUES(8201,8201,8201,'m6d-concurrent-farm'); INSERT INTO ponds(id,farm_id,name) VALUES(8201,8201,'m6d-concurrent-pond')`); err != nil {
		t.Fatal(err)
	}
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := core.NewWithPolicy(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)), policy)
	if err != nil {
		t.Fatal(err)
	}
	installTestLicenseWithFeatures(t, p, svc, 100, []string{"openapi"})
	svc.SetAPIKeyRoot([]byte(strings.Repeat("r", 32)))
	ownerCtx := domain.WithTenantUserID(domain.WithTenantRole(domain.WithTenantID(ctx, 8201), "owner"), 8201)
	key, secretText, err := svc.IssueAPIKey(ownerCtx, 8201, "concurrent", []string{"ponds:read"}, domain.APIKeyResourceScope{}, 8201)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := base64.RawURLEncoding.DecodeString(secretText)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	makeRequest := func(nonce string) domain.OpenRequest {
		hash := sha256.Sum256(nil)
		signed := strings.Join([]string{"GET", "/open/v1/ponds", "", strconv.FormatInt(now.Unix(), 10), nonce, hex.EncodeToString(hash[:])}, "\n")
		mac := hmac.New(sha256.New, secret)
		_, _ = mac.Write([]byte(signed))
		return domain.OpenRequest{KeyID: key.KeyID, Timestamp: now.Unix(), Nonce: nonce, Signature: hex.EncodeToString(mac.Sum(nil)), Method: "GET", Path: "/open/v1/ponds"}
	}
	const nonce = "fedcba9876543210fedcba98765432"
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, callErr := svc.AuthenticateOpen(ctx, makeRequest(nonce), now)
			results <- callErr
		}()
	}
	wg.Wait()
	close(results)
	var success, replay int
	for callErr := range results {
		if callErr == nil {
			success++
		} else if errors.Is(callErr, core.ErrOpenReplay) {
			replay++
		}
	}
	if success != 1 || replay != 1 {
		t.Fatalf("concurrent nonce success=%d replay=%d", success, replay)
	}
	for i := 0; i < 9; i++ {
		if _, err := svc.AuthenticateOpen(ctx, makeRequest(fmt.Sprintf("rate-%02d-0123456789abcdef", i)), now); err != nil {
			t.Fatalf("burst request %d: %v", i, err)
		}
	}
	if _, err := svc.AuthenticateOpen(ctx, makeRequest("rate-limit-0123456789abcdef"), now); !errors.As(err, new(*core.OpenRateLimitError)) {
		t.Fatalf("rate limit error=%v", err)
	}
}
