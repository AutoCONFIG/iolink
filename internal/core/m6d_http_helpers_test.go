package core_test

import (
	"context"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/openapi"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

type m6dHTTPFixture struct {
	pool       *pgxpool.Pool
	svc        *core.Service
	owner      context.Context
	server     *httptest.Server
	licenseKey *rsa.PrivateKey
}

func m6dSignatureForRequest(t *testing.T, secret []byte, req domain.OpenRequest) string {
	t.Helper()
	query, err := core.CanonicalOpenQuery(req.Query)
	if err != nil {
		t.Fatal(err)
	}
	bodyHash := sha256.Sum256(req.Body)
	canonical := strings.Join([]string{req.Method, req.Path, query, strconv.FormatInt(req.Timestamp, 10), req.Nonce, hex.EncodeToString(bodyHash[:])}, "\n")
	mac := hmac.New(sha256.New, secret)
	if _, err := mac.Write([]byte(canonical)); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(mac.Sum(nil))
}

func newM6dHTTPFixture(t *testing.T) m6dHTTPFixture {
	t.Helper()
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `
		INSERT INTO users(id,open_id,authority) VALUES(8301,'m6d-http-owner','USER'),(8303,'m6d-http-other','USER');
		INSERT INTO tenants(id,name) VALUES(8301,'m6d-http'),(8303,'m6d-http-other');
		INSERT INTO tenant_memberships(tenant_id,user_id,role) VALUES(8301,8301,'owner'),(8303,8303,'owner');
		INSERT INTO farms(id,tenant_id,owner_id,name) VALUES(8301,8301,8301,'allowed'),(8302,8301,8301,'hidden'),(8303,8303,8303,'foreign');
		INSERT INTO ponds(id,farm_id,name) VALUES(8301,8301,'allowed'),(8302,8302,'hidden'),(8303,8303,'foreign');
		INSERT INTO devices(pond_id,device_no,secret_hash) VALUES(8301,'m6d-allowed','hash'),(8302,'m6d-hidden','hash'),(8303,'m6d-foreign','hash');
		INSERT INTO alarms(device_no,pond_id,metric,current_value,threshold,level) VALUES
		('m6d-allowed',8301,'temperature',30,25,'warning'),('m6d-hidden',8302,'temperature',30,25,'warning'),('m6d-foreign',8303,'temperature',30,25,'warning');`); err != nil {
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
	owner := domain.WithTenantUserID(domain.WithTenantRole(domain.WithTenantID(ctx, 8301), "owner"), 8301)
	server := httptest.NewServer(openapi.New(openapi.Deps{Auth: svc, Resources: svc}).Routes())
	t.Cleanup(server.Close)
	return m6dHTTPFixture{pool: p, svc: svc, owner: owner, server: server, licenseKey: licenseKey}
}

func (f m6dHTTPFixture) issue(t *testing.T, scopes []string, resources domain.APIKeyResourceScope) (domain.APIKey, string) {
	t.Helper()
	key, secret, err := f.svc.IssueAPIKey(f.owner, 8301, "http-test", scopes, resources, 8301)
	if err != nil {
		t.Fatal(err)
	}
	return key, secret
}

func m6dSignedHTTP(t *testing.T, server *httptest.Server, key domain.APIKey, secretText, path, nonce string) (int, http.Header, []byte) {
	t.Helper()
	return m6dSignedHTTPBody(t, server, key, secretText, path, nonce, "", "")
}

func m6dSignedHTTPBody(t *testing.T, server *httptest.Server, key domain.APIKey, secretText, path, nonce, signedBody, actualBody string) (int, http.Header, []byte) {
	t.Helper()
	secret, err := base64.RawURLEncoding.DecodeString(secretText)
	if err != nil {
		t.Fatal(err)
	}
	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	hash := sha256.Sum256([]byte(signedBody))
	signed := strings.Join([]string{"GET", path, "", timestamp, nonce, hex.EncodeToString(hash[:])}, "\n")
	mac := hmac.New(sha256.New, secret)
	if _, err := mac.Write([]byte(signed)); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+path, strings.NewReader(actualBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Key-Id", key.KeyID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", hex.EncodeToString(mac.Sum(nil)))
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, body
}

func m6dNonce(t *testing.T) string {
	t.Helper()
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s/%d", t.Name(), time.Now().UnixNano())))
	return hex.EncodeToString(hash[:])
}
