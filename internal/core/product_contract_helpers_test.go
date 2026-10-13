package core_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
)

const productContractFields = `{"fields":[{"identifier":"temperature","type":"number","unit":"C","readable":true,"writable":false,"nullable":false},{"identifier":"private_value","type":"string","unit":"","readable":false,"writable":false,"nullable":true}]}`

type productContractFixture struct {
	*baselineAdminFixture
	dump string
	sink *observability.Sink
}

func newProductContractFixture(t *testing.T) productContractFixture {
	t.Helper()
	f := newBaselineAdminFixture(t)
	f.server.Close()
	dir := t.TempDir()
	logger, sink, err := observability.New(observability.Config{Dir: dir, Version: "product", Level: "error", MaxMB: 1, Backups: 1}, io.Discard)
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
	f.server = httptest.NewServer(adminapi.New(adminapi.Config{SecretKey: strings.Repeat("c", 32), JWT: time.Hour}, adminapi.Deps{Store: f.svc, Catalog: f.svc.Products(), Policy: policy, Logger: logger}).UserRoutes())
	t.Cleanup(f.server.Close)
	return productContractFixture{f, filepath.Join(dir, "product", "iolinkd.jsonl"), sink}
}

func (f productContractFixture) call(t *testing.T, input baselineAdminRequest) baselineAppResponse {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), input.method, f.server.URL+"/user/v1"+input.path, strings.NewReader(input.body))
	if err != nil {
		t.Fatal("product request construction failed")
	}
	req.Header.Set("Content-Type", "application/json")
	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal("product HTTP request failed")
	}
	raw, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatal("product response read failed")
	}
	response := baselineAppResponse{resp.StatusCode, resp.Header.Get("Content-Type"), raw, resp.Header.Get("X-Request-ID")}
	t.Logf("product contract method=%s route=%s status=%d expected=%d", input.method, input.route, response.status, input.status)
	if response.status != input.status {
		t.Errorf("product status got=%d want=%d", response.status, input.status)
	}
	t.Run("response_contract", func(t *testing.T) {
		assertHTTPContract(t, "admin", input.method, input.route, response.status, response.contentType, response.raw)
	})
	return response
}

func productDecode[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal("product response JSON decode failed")
	}
	return out
}

func (f productContractFixture) create(t *testing.T) (domain.Product, domain.ProductModel) {
	t.Helper()
	p := productDecode[domain.Product](t, f.call(t, baselineAdminRequest{"POST", "/products", "/products", `{"name":"product-contract"}`, 201}).raw)
	path := productModelPath(p.ID)
	m := productDecode[domain.ProductModel](t, f.call(t, baselineAdminRequest{"POST", "/products/{product_id}/models", path, productContractFields, 201}).raw)
	return p, m
}

func (f productContractFixture) snapshot(t *testing.T) string {
	t.Helper()
	var digest string
	if err := f.pool.QueryRow(t.Context(), `SELECT md5(jsonb_build_object(
 'products',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM products p),
 'models',(SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM product_models m),
 'devices',(SELECT jsonb_agg(to_jsonb(d) ORDER BY id) FROM devices d),
 'shadows',(SELECT jsonb_agg(to_jsonb(s) ORDER BY device_no) FROM device_shadows s),
 'audit',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM audit_events a))::text)`).Scan(&digest); err != nil {
		t.Fatal("product state snapshot failed")
	}
	return digest
}
