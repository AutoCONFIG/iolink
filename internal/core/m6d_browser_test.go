package core_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	webassets "git.hyhy.fun/rsplab/iolink/internal/web"
)

func TestM6dBrowser_realManagementFlow(t *testing.T) {
	if os.Getenv("IOLINK_M6D_BROWSER") != "1" {
		t.Skip("set IOLINK_M6D_BROWSER=1 after a production frontend build")
	}
	f := newM6dHTTPFixture(t)
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	rootSecret := strings.Repeat("s", 32)
	mux := http.NewServeMux()
	mux.Handle("/admin/v1/", adminapi.New(adminapi.Config{SecretKey: rootSecret, JWT: time.Hour}, adminapi.Deps{Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), Store: f.svc, APIKeys: f.svc, Policy: policy}).Routes())
	assets, err := webassets.Admin()
	if err != nil {
		t.Fatal(err)
	}
	files := http.FileServerFS(assets)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/system" || r.URL.Path == "/api-keys/audit" {
			copy := r.Clone(r.Context())
			copy.URL.Path = "/"
			files.ServeHTTP(w, copy)
			return
		}
		files.ServeHTTP(w, r)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	version, err := f.svc.TenantMembershipVersion(t.Context(), 8301, 8301)
	if err != nil {
		t.Fatal(err)
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"aid": 8301, "ver": 0, "tenant_id": 8301, "tenant_ver": version, "tenant_role": "owner", "exp": time.Now().Add(time.Hour).Unix()}).SignedString(platform.DeriveAdminKey(rootSecret))
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "npm", "run", "e2e", "--", "--config", "playwright.m6d.config.ts", "--reporter=line")
	cmd.Dir = filepath.Join(root, "web")
	cmd.Env = append(os.Environ(), "IOLINK_M6D_URL="+server.URL, "IOLINK_M6D_TOKEN="+token)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("real management browser: %v\n%s", err, output)
	}
	t.Log(string(output))
}
