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
	"git.hyhy.fun/rsplab/iolink/internal/appapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/camerapg"
	corepkg "git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"git.hyhy.fun/rsplab/iolink/internal/videocredential"
	"github.com/jackc/pgx/v5/pgxpool"
)

const cameraContractBody = `{"name":"Configured","pond_id":1,"source":{"kind":"rtsp","uri":"rtsp://192.168.10.99:554/live","credentials":{"username":"private-camera-user","password":"private-camera-password"}}}`

type cameraContractFixture struct {
	pool       *pgxpool.Pool
	server     *httptest.Server
	user, mini string
	dumpPath   string
	sink       *observability.Sink
}

type cameraContractRequest struct {
	method, template, path, body string
	status                       int
}

func newCameraContractFixture(t *testing.T) cameraContractFixture {
	t.Helper()
	return newCameraContractFixtureWithProvider(t, true)
}

func newCameraContractFixtureWithProvider(t *testing.T, configured bool) cameraContractFixture {
	t.Helper()
	svc, pool := setup(t)
	execute(t, pool, `UPDATE users SET username='camera-owner',password_hash=$1 WHERE id=1`, platform.HashPassword("isolated-camera-password"))
	execute(t, pool, `INSERT INTO users(id,open_id,username,password_hash,authority) VALUES(9010,'camera-actor','camera-actor',$1,'USER'),(9011,'camera-platform','camera-platform',$1,'ADMIN'),(9013,'camera-foreign',NULL,NULL,'USER')`, platform.HashPassword("isolated-camera-password"))
	execute(t, pool, `INSERT INTO tenant_memberships(tenant_id,user_id,role) SELECT id,9010,'viewer' FROM tenants WHERE name='test-tenant';
 INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role) SELECT id,1,9010,'viewer' FROM tenants WHERE name='test-tenant';
 INSERT INTO tenants(id,name) VALUES(9003,'camera-foreign');
 INSERT INTO tenant_memberships(tenant_id,user_id,role) VALUES(9003,9013,'owner');
 INSERT INTO farms(id,tenant_id,owner_id,name) SELECT 9002,id,9013,'same tenant' FROM tenants WHERE name='test-tenant';
 INSERT INTO farms(id,tenant_id,owner_id,name) VALUES(9003,9003,9013,'foreign');
 INSERT INTO ponds(id,farm_id,name) VALUES(9002,9002,'same tenant'),(9003,9003,'foreign');
 INSERT INTO video_cameras(id,tenant_id,farm_id,pond_id,name,source_kind,rtsp_uri) SELECT 101,id,1,1,'Owned','rtsp','rtsp://192.168.10.20/live' FROM tenants WHERE name='test-tenant';
 INSERT INTO video_cameras(id,tenant_id,farm_id,pond_id,name,source_kind,rtsp_uri) SELECT 102,id,9002,9002,'Unassigned','rtsp','rtsp://192.168.10.21/live' FROM tenants WHERE name='test-tenant';
 INSERT INTO video_cameras(id,tenant_id,farm_id,pond_id,name,source_kind,rtsp_uri) VALUES(103,9003,9003,9003,'Foreign','rtsp','rtsp://192.168.10.22/live');
 SELECT setval(pg_get_serial_sequence('video_cameras','id'),200,true)`)
	key := installTestLicenseWithFeatures(t, pool, svc, 100, []string{"video"})
	dumpRoot := t.TempDir()
	logger, sink, err := observability.New(observability.Config{Dir: dumpRoot, Version: "camera-contract", Level: "error", MaxMB: 1, Backups: 1}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sink.Close(); err != nil {
			t.Error(err)
		}
	})
	cipher, err := videocredential.New([]byte("isolated-camera-credential-key-32-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	cameras := camera.New(camera.Dependencies{Store: camerapg.New(pool), Cipher: cipher, License: camera.LicenseVerifier{PublicKey: &key.PublicKey, KeyID: "test-key"}, LicenseClock: corepkg.NewLicenseService(pool, logger)})
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	adminDeps := adminapi.Deps{Store: svc, Policy: policy, Logger: logger}
	appDeps := appapi.Deps{Users: svc}
	if configured {
		adminDeps.Cameras = cameras
		appDeps.Cameras = cameras
	}
	admin := adminapi.New(adminapi.Config{SecretKey: "isolated-camera-contract", JWT: time.Hour}, adminDeps)
	mini := appapi.New(appapi.Config{SecretKey: "isolated-camera-contract", JWT: time.Hour}, appDeps, logger)
	mini.SetWechatExchanger(func(code string) (string, error) {
		if code == "actor" {
			return "camera-actor", nil
		}
		return "test-user", nil
	})
	mux := http.NewServeMux()
	mux.Handle("/user/", admin.UserRoutes())
	mux.Handle("/admin/", admin.Routes())
	mux.Handle("/api/", mini.Routes())
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	f := cameraContractFixture{pool: pool, server: server, sink: sink, dumpPath: filepath.Join(dumpRoot, "camera-contract", "iolinkd.jsonl")}
	f.user, f.mini = f.login(t, "owner")
	return f
}

func (f cameraContractFixture) login(t *testing.T, actor string) (string, string) {
	t.Helper()
	username := "camera-owner"
	if actor == "actor" {
		username = "camera-actor"
	}
	var user, mini struct{ Token string }
	response := f.request(t, cameraContractRequest{"POST", "", "/user/v1/login", `{"username":"` + username + `","password":"isolated-camera-password"}`, 200}, "")
	if err := json.Unmarshal(response.raw, &user); err != nil || response.status != 200 || user.Token == "" {
		t.Fatal("camera business login failed")
	}
	response = f.request(t, cameraContractRequest{"POST", "", "/api/v1/auth/login", `{"code":"` + actor + `"}`, 200}, "")
	if err := json.Unmarshal(response.raw, &mini); err != nil || response.status != 200 || mini.Token == "" {
		t.Fatal("camera mini login failed")
	}
	return user.Token, mini.Token
}

func (f cameraContractFixture) request(t *testing.T, tc cameraContractRequest, token string) baselineAppResponse {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), tc.method, f.server.URL+tc.path, strings.NewReader(tc.body))
	if err != nil {
		t.Fatal("camera request construction failed")
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal("camera TCP transport failed")
	}
	raw, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatal("camera response read/close failed")
	}
	return baselineAppResponse{response.StatusCode, response.Header.Get("Content-Type"), raw, response.Header.Get("X-Request-ID")}
}

func assertCameraContract(t *testing.T, tc cameraContractRequest, response baselineAppResponse) {
	t.Helper()
	t.Logf("camera method=%s route=%s status=%d expected=%d", tc.method, tc.template, response.status, tc.status)
	if response.status != tc.status {
		t.Fatalf("camera status=%d want=%d", response.status, tc.status)
	}
	assertHTTPContract(t, "video", tc.method, tc.template, response.status, response.contentType, response.raw)
	if response.status >= 400 {
		codes := map[int]string{400: "invalid_request", 401: "unauthorized", 403: "forbidden", 404: "not_found", 500: "internal_error", 503: "unavailable"}
		var result struct{ Code, Message string }
		if err := json.Unmarshal(response.raw, &result); err != nil || result.Code != codes[tc.status] || result.Message != result.Code {
			t.Fatal("camera failure did not return its safe machine code")
		}
	}
}

func (f cameraContractFixture) snapshot(t *testing.T) string {
	t.Helper()
	var raw []byte
	if err := f.pool.QueryRow(t.Context(), `SELECT jsonb_build_object('cameras',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM video_cameras c),'streams',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM video_streams s),'sessions',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM video_sessions s),'jobs',(SELECT jsonb_agg(to_jsonb(j) ORDER BY id) FROM jobs j WHERE kind='video.stop'),'audit',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM audit_events a WHERE resource_type='camera'))`).Scan(&raw); err != nil {
		t.Fatal("camera snapshot query failed")
	}
	return string(raw)
}
