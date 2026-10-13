package core_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"git.hyhy.fun/rsplab/iolink/internal/setupapi"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	setupContractKey         = "isolated-setup-contract-only-key"
	setupContractCredentials = `{"username":"contract-operator","password":"Contract-operator-only-1234"}`
)

type setupContractFixture struct {
	pool   *pgxpool.Pool
	server *httptest.Server
	dump   string
}

func newSetupContractFixture(t *testing.T) setupContractFixture {
	t.Helper()
	pool := testdb.New(t)
	if err := migrate.Up(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	logger, sink, err := observability.New(observability.Config{Dir: dir, Version: "setup-contract", Level: "error", MaxMB: 1, Backups: 1}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sink.Close(); err != nil {
			t.Error(err)
		}
	})
	api := setupapi.New(platform.WebBootstrap{Pool: pool}, setupContractKey, logger)
	server := httptest.NewServer(api.Routes())
	t.Cleanup(server.Close)
	return setupContractFixture{pool: pool, server: server, dump: filepath.Join(dir, "setup-contract", "iolinkd.jsonl")}
}

type setupContractRequest struct {
	method, route, body, key, contentType, origin string
	status                                        int
}

func (f setupContractFixture) request(t *testing.T, input setupContractRequest) []byte {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), input.method, f.server.URL+"/setup/v1"+input.route, strings.NewReader(input.body))
	if err != nil {
		t.Fatal("setup request construction failed")
	}
	req.Header.Set("X-IoLink-Setup-Key", input.key)
	req.Header.Set("Content-Type", input.contentType)
	if input.origin != "" {
		req.Header.Set("Origin", input.origin)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal("setup HTTP transport failed")
	}
	raw, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatal("setup response read failed")
	}
	if resp.StatusCode != input.status {
		t.Errorf("setup %s %s status=%d want=%d", input.method, input.route, resp.StatusCode, input.status)
	}
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Error("setup response may be cached")
	}
	assertHTTPContract(t, "setup", input.method, input.route, resp.StatusCode, resp.Header.Get("Content-Type"), raw)
	if input.status == 503 {
		f.assertStorageDiagnostic(t, resp.Header.Get("X-Request-ID"))
	}
	return raw
}

func (f setupContractFixture) assertStorageDiagnostic(t *testing.T, requestID string) {
	t.Helper()
	raw, err := os.ReadFile(f.dump)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{setupContractKey, "Contract-operator-only-1234", "contract-operator", "setup_fault_relation", "setup_fault_audit"} {
		if strings.Contains(string(raw), secret) {
			t.Error("setup dump contains sensitive input or SQL detail")
		}
	}
	found := false
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal("invalid setup dump JSON")
		}
		if record["request_id"] == requestID && record["stage"] == "setup" && record["err"] == "postgres_42P01" {
			found = true
		}
	}
	if requestID == "" || !found {
		t.Error("setup storage failure lacks correlated safe SQLSTATE diagnostic")
	}
}
