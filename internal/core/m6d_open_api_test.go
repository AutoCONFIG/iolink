package core_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
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
	keys, err := svc.ListAPIKeys(ownerCtx, 8101, 8101)
	if err != nil || len(keys) != 1 {
		t.Fatalf("keys = %d err=%v", len(keys), err)
	}
}
