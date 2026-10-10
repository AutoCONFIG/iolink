package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/appapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/license"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const cameraHTTPRoot = "camera-http-disposable-root-key-32-bytes"
const cameraHTTPPassword = "camera-fixture-password"
const cameraHTTPConfig = `{"name":"camera","pond_id":1701,"source":{"kind":"rtsp","uri":"rtsp://192.168.10.20:554/live","credentials":{"username":"secret-username","password":"secret-password"}}}`

type cameraHTTPFixture struct {
	pool   *pgxpool.Pool
	server *httptest.Server
	key    *rsa.PrivateKey
}

func newCameraHTTPFixture(t *testing.T, verification ...bool) cameraHTTPFixture {
	t.Helper()
	p := testdb.New(t)
	if err := migrate.Up(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	hash := platform.HashPassword(cameraHTTPPassword)
	_, err = p.Exec(t.Context(), `INSERT INTO tenants(id,name) VALUES(1701,'camera-a'),(1702,'camera-b');
 INSERT INTO users(id,open_id,username,password_hash,authority) SELECT id,'camera-'||id,'camera-'||id,NULL,CASE WHEN id=1707 THEN 'ADMIN' ELSE 'USER' END FROM generate_series(1701,1708) id;
 INSERT INTO tenant_memberships(tenant_id,user_id,role,expires_at) VALUES(1701,1701,'owner',NULL),(1701,1702,'admin',NULL),(1701,1703,'member',NULL),(1701,1704,'viewer',NULL),(1701,1705,'support',now()+interval '1 hour'),(1701,1707,'support',now()+interval '1 hour'),(1702,1706,'owner',NULL);
 INSERT INTO farms(id,tenant_id,owner_id,name) VALUES(1701,1701,1701,'allowed'),(1702,1701,1701,'hidden'),(1703,1702,1706,'foreign');
 INSERT INTO ponds(id,farm_id,name) VALUES(1701,1701,'allowed'),(1702,1702,'hidden'),(1703,1703,'foreign');
 INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role,expires_at) SELECT 1701,1701,id,CASE WHEN id=1705 THEN 'support' ELSE 'viewer' END,CASE WHEN id=1705 THEN now()+interval '1 hour' END FROM generate_series(1703,1705) id;
 INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role,expires_at) VALUES(1701,1701,1707,'support',now()+interval '1 hour');
 INSERT INTO video_cameras(id,tenant_id,farm_id,pond_id,name,source_kind,rtsp_uri,credential_cipher) VALUES(1701,1701,1701,1701,'allowed','rtsp','rtsp://192.168.10.20:554/live',decode(repeat('aa',29),'hex')),(1702,1701,1702,1702,'hidden','rtsp','rtsp://192.168.10.21:554/live',NULL),(1703,1702,1703,1703,'foreign','rtsp','rtsp://192.168.10.22:554/live',NULL);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(t.Context(), `UPDATE users SET password_hash=$1 WHERE id BETWEEN 1701 AND 1708`, hash); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := core.NewWithPolicy(t.Context(), p, logger, policy)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := &key.PublicKey
	if len(verification) > 0 && !verification[0] {
		publicKey = nil
	}
	cameras, err := cameraService(p, cameraHTTPRoot, camera.Dependencies{License: camera.LicenseVerifier{PublicKey: publicKey, KeyID: "camera-http-key"}, LicenseClock: svc})
	if err != nil {
		t.Fatal(err)
	}
	admin := adminapi.New(adminapi.Config{SecretKey: cameraHTTPRoot, JWT: time.Hour}, adminapi.Deps{Store: svc, Policy: policy, Cameras: cameras, Logger: logger})
	mini := appapi.New(appapi.Config{SecretKey: cameraHTTPRoot, JWT: time.Hour}, appapi.Deps{Users: svc, Cameras: cameras}, logger)
	mini.SetWechatExchanger(func(code string) (string, error) { return code, nil })
	mux := http.NewServeMux()
	mux.Handle("/user/v1/", admin.UserRoutes())
	mux.Handle("/admin/v1/", admin.Routes())
	mux.Handle("/api/v1/", mini.Routes())
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return cameraHTTPFixture{p, server, key}
}
func (f cameraHTTPFixture) request(t *testing.T, token, method, path, body string, want int) []byte {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), method, f.server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := f.server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != want {
		t.Fatalf("method=%s path=%s status=%d want=%d response=%s", method, path, response.StatusCode, want, raw)
	}
	if strings.Contains(path, "cameras") && len(raw) > 0 {
		for _, secret := range []string{"rtsp://", "192.168.", "credential", "secret-username", "secret-password", "tenant_id", "farm_id"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("camera response leaked %s", secret)
			}
		}
		t.Logf("method=%s path=%s status=%d redacted_response=%s", method, path, response.StatusCode, raw)
	}
	return raw
}
func (f cameraHTTPFixture) login(t *testing.T, id int, mini bool) string {
	t.Helper()
	path := "/user/v1/login"
	body := fmt.Sprintf(`{"username":"camera-%d","password":%q}`, id, cameraHTTPPassword)
	if mini {
		path = "/api/v1/auth/login"
		body = fmt.Sprintf(`{"code":"camera-%d"}`, id)
	}
	raw := f.request(t, "", http.MethodPost, path, body, 200)
	var result struct{ Token string }
	if err := json.Unmarshal(raw, &result); err != nil || result.Token == "" {
		t.Fatal("login failed")
	}
	return result.Token
}

func (f cameraHTTPFixture) adminMiniToken(t *testing.T) string {
	t.Helper()
	var version int
	var membershipVersion int64
	var role string
	if err := f.pool.QueryRow(t.Context(), `SELECT u.token_version,tm.permission_version,tm.role FROM users u JOIN tenant_memberships tm ON tm.user_id=u.id WHERE u.id=1707 AND tm.tenant_id=1701`).Scan(&version, &membershipVersion, &role); err != nil {
		t.Fatal(err)
	}
	claims := jwt.MapClaims{"uid": int64(1707), "ver": version, "exp": time.Now().Add(time.Hour).Unix(), "tenant_id": int64(1701), "tenant_ver": membershipVersion, "tenant_role": role}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(platform.DeriveAppKey(cameraHTTPRoot))
	if err != nil {
		t.Fatal(err)
	}
	return token
}
func (f cameraHTTPFixture) installLicense(t *testing.T, features []string, expired bool) {
	t.Helper()
	var deployment string
	if err := f.pool.QueryRow(t.Context(), `SELECT deployment_id FROM deployment_config`).Scan(&deployment); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	expires := now.Add(time.Hour)
	if expired {
		expires = now.Add(-time.Second)
	}
	payload, err := json.Marshal(license.Payload{LicenseID: "camera-http", DeploymentID: deployment, IssuedAt: now.Add(-time.Hour), NotBefore: now.Add(-time.Hour), ExpiresAt: &expires, MaxDevices: 100, Features: features, KeyID: "camera-http-key"})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	signature, err := rsa.SignPSS(rand.Reader, f.key, crypto.SHA256, digest[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.pool.Exec(t.Context(), `UPDATE license_state SET payload=$1,signature=$2,payload_sha256=$3,imported_at=now(),imported_by=1707`, payload, signature, fmt.Sprintf("%x", digest))
	if err != nil {
		t.Fatal(err)
	}
}
