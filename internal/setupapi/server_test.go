package setupapi_test

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"git.hyhy.fun/rsplab/iolink/internal/setupapi"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testKey = "isolated-bootstrap-installation-key-123456"
const credentials = `{"username":"operator","password":"Operator-test-1234"}`

func fixture(t *testing.T) (*pgxpool.Pool, *setupapi.Server) {
	t.Helper()
	p := testdb.New(t)
	if err := migrate.Up(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	return p, setupapi.New(platform.WebBootstrap{Pool: p}, testKey, slog.New(slog.NewJSONHandler(io.Discard, nil)))
}

func request(h http.Handler, method, path, body, key, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-IoLink-Setup-Key", key)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestBootstrapWhenFreshThenInitialized(t *testing.T) {
	p, s := fixture(t)
	h := s.Routes()
	if w := request(h, "GET", "/setup/v1/status", "", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"required":true`) {
		t.Fatal(w.Code, w.Body)
	}
	called := false
	protected := s.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(204) }))
	if w := request(protected, "GET", "/admin/v1/stats", "", "", ""); w.Code != 503 || called {
		t.Fatal(w.Code, called)
	}
	if w := request(h, "POST", "/setup/v1/initialize", credentials, testKey, "http://example.com"); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	if err := platform.CheckAdminReady(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	if w := request(h, "GET", "/setup/v1/status", "", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"required":false`) {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(protected, "GET", "/admin/v1/stats", "", "", ""); w.Code != 204 || !called {
		t.Fatal(w.Code, called)
	}
	if w := request(h, "POST", "/setup/v1/initialize", credentials, testKey, ""); w.Code != 409 {
		t.Fatal(w.Code, w.Body)
	}
	var count int
	if err := p.QueryRow(t.Context(), `SELECT count(*) FROM users WHERE authority='ADMIN'`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func TestBootstrapWhenInvalidThenNoAccounts(t *testing.T) {
	p, s := fixture(t)
	for _, tc := range []struct {
		name, body, key, origin string
		status                  int
	}{
		{"missing_key", credentials, "", "", 401},
		{"wrong_key", credentials, "wrong-key", "", 401},
		{"cross_origin", credentials, testKey, "https://attacker.example", 403},
		{"origin_port", credentials, testKey, "http://example.com:1234", 403},
		{"origin_userinfo", credentials, testKey, "http://user@example.com", 403},
		{"origin_path", credentials, testKey, "http://example.com/path", 403},
		{"origin_query", credentials, testKey, "http://example.com?query=1", 403},
		{"origin_fragment", credentials, testKey, "http://example.com#fragment", 403},
		{"invalid_json", "{", testKey, "", 400},
		{"unknown_field", `{"username":"operator","password":"Operator-test-1234","extra":1}`, testKey, "", 400},
		{"trailing_json", credentials + `{}`, testKey, "", 400},
		{"empty_fields", `{}`, testKey, "", 400},
		{"short_password", `{"username":"operator","password":"short"}`, testKey, "", 400},
		{"oversize", strings.Repeat(" ", 4097) + credentials, testKey, "", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := request(s.Routes(), "POST", "/setup/v1/initialize", tc.body, tc.key, tc.origin)
			if w.Code != tc.status {
				t.Fatal(w.Code, w.Body)
			}
			var count int
			if err := p.QueryRow(t.Context(), `SELECT count(*) FROM users`).Scan(&count); err != nil || count != 0 {
				t.Fatal(count, err)
			}
		})
	}
}

func TestBootstrapWhenHTTPSProxyOriginThenInitialized(t *testing.T) {
	_, s := fixture(t)
	w := request(s.Routes(), "POST", "/setup/v1/initialize", credentials, testKey, "https://example.com")
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
}

func TestBootstrapWhenConcurrentThenSingleAdministrator(t *testing.T) {
	p, s := fixture(t)
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() { results <- request(s.Routes(), "POST", "/setup/v1/initialize", credentials, testKey, "").Code })
	}
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for status := range results {
		counts[status]++
	}
	if counts[204] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
	var audits int
	if err := p.QueryRow(t.Context(), `SELECT count(*) FROM audit_events WHERE action='admin.bootstrap'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatal(audits, err)
	}
}

func TestBootstrapWhenAuditFailsThenRollsBack(t *testing.T) {
	p, s := fixture(t)
	if _, err := p.Exec(t.Context(), `CREATE FUNCTION deny_bootstrap() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$; CREATE TRIGGER deny_bootstrap BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION deny_bootstrap()`); err != nil {
		t.Fatal(err)
	}
	w := request(s.Routes(), "POST", "/setup/v1/initialize", credentials, testKey, "")
	if w.Code != 503 {
		t.Fatal(w.Code, w.Body)
	}
	var count int
	if err := p.QueryRow(t.Context(), `SELECT count(*) FROM users`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestBootstrapWhenDatabaseUnavailableThenFailsClosed(t *testing.T) {
	p, _ := fixture(t)
	p.Close()
	var output bytes.Buffer
	s := setupapi.New(platform.WebBootstrap{Pool: p}, testKey, slog.New(slog.NewJSONHandler(&output, nil)))
	w := request(s.Routes(), "POST", "/setup/v1/initialize", credentials, testKey, "")
	if w.Code != 503 {
		t.Fatal(w.Code, w.Body)
	}
	if strings.Contains(output.String(), testKey) || strings.Contains(output.String(), "Operator-test-1234") {
		t.Fatal("credentials logged")
	}
}
