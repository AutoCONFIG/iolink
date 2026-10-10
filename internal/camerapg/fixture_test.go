package camerapg_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/camerapg"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
	"git.hyhy.fun/rsplab/iolink/internal/videocredential"
	"github.com/jackc/pgx/v5/pgxpool"
)

type available struct{}

func (available) RequireConfiguration(context.Context, domain.VideoSourceKind) error { return nil }

type fixture struct {
	pool    *pgxpool.Pool
	deps    camera.Dependencies
	service *camera.Service
}

var testKey = sync.OnceValues(func() (*rsa.PrivateKey, error) { return rsa.GenerateKey(rand.Reader, 2048) })

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := testdb.New(t)
	if err := migrate.Up(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	exec(t, pool, `INSERT INTO users(id,open_id,authority) VALUES (1,'cam-owner','USER'),(2,'cam-admin','USER'),(3,'cam-member','USER'),(4,'cam-viewer','USER'),(5,'cam-support','USER'),(6,'cam-platform','ADMIN'),(7,'cam-other','USER');
 INSERT INTO tenants(id,name) VALUES (101,'cam-tenant'),(102,'cam-foreign');
 INSERT INTO farms(id,tenant_id,owner_id,name) VALUES(101,101,1,'one'),(102,101,7,'same tenant'),(103,102,7,'foreign');
 INSERT INTO ponds(id,farm_id,name) VALUES(101,101,'one'),(102,102,'two'),(103,103,'foreign');
 INSERT INTO tenant_memberships(tenant_id,user_id,role,expires_at) VALUES(101,1,'owner',NULL),(101,2,'admin',NULL),(101,3,'member',NULL),(101,4,'viewer',NULL),(101,5,'support',now()+interval '1 hour'),(101,6,'member',NULL),(101,7,'member',NULL),(102,7,'owner',NULL);
 INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role,expires_at) VALUES(101,101,3,'member',NULL),(101,101,4,'viewer',NULL),(101,101,5,'support',now()+interval '1 hour');
 INSERT INTO video_cameras(id,tenant_id,farm_id,pond_id,name,source_kind,rtsp_uri) VALUES(101,101,101,101,'one','rtsp','rtsp://192.168.10.20/live'),(102,101,102,102,'two','rtsp','rtsp://192.168.10.21/live'),(103,102,103,103,'foreign','rtsp','rtsp://192.168.10.22/live');
 SELECT setval(pg_get_serial_sequence('video_cameras','id'),200,true);
 INSERT INTO video_gb_devices(id,tenant_id,device_id,name,credential_cipher,enabled) VALUES(101,101,'34020000001320000101','one',decode(repeat('aa',29),'hex'),true),(102,101,'34020000001320000102','disabled',decode(repeat('aa',29),'hex'),false),(103,102,'34020000001320000103','foreign',decode(repeat('aa',29),'hex'),true);
 INSERT INTO video_gb_channels(device_id,tenant_id,channel_id,name,catalog_sn,present) VALUES(101,101,'34020000001310000101','current',1,true),(101,101,'34020000001310000102','absent',1,false),(102,101,'34020000001310000101','disabled device',1,true),(103,102,'34020000001310000101','foreign',1,true)`)
	key, err := testKey()
	if err != nil {
		t.Fatal(err)
	}
	var deployment string
	if err = pool.QueryRow(t.Context(), `SELECT deployment_id FROM deployment_config`).Scan(&deployment); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	payload := license.Payload{LicenseID: "camera-test", DeploymentID: deployment, IssuedAt: now.Add(-time.Hour), NotBefore: now.Add(-time.Minute), MaxDevices: 10, Features: []string{"video"}, KeyID: "camera-test-key"}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	signature, err := rsa.SignPSS(rand.Reader, key, crypto.SHA256, digest[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256})
	if err != nil {
		t.Fatal(err)
	}
	exec(t, pool, `UPDATE license_state SET payload=$1,signature=$2,payload_sha256=$3,imported_at=now()`, raw, signature, fmt.Sprintf("%x", digest))
	cipher, err := videocredential.New([]byte("camera-tests-disposable-root-key-32-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	deps := camera.Dependencies{Store: camerapg.New(pool), License: camera.LicenseVerifier{PublicKey: &key.PublicKey, KeyID: payload.KeyID}, Cipher: cipher, Availability: available{}, LicenseClock: core.NewLicenseService(pool, slog.New(slog.NewTextHandler(io.Discard, nil)))}
	return fixture{pool: pool, deps: deps, service: camera.New(deps)}
}

func exec(t *testing.T, p *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := p.Exec(t.Context(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func actor(id int64, role string) context.Context {
	ctx := domain.WithTenantID(context.Background(), 101)
	ctx = domain.WithTenantUserID(ctx, id)
	ctx = domain.WithTenantRole(ctx, role)
	return domain.WithTenantPermissionVersion(ctx, 0)
}

func configuration(t *testing.T, pond int64, credentials bool) camera.Configuration {
	t.Helper()
	input := camera.SourceInput{Kind: domain.VideoRTSP, URI: "rtsp://192.168.10.30:554/live"}
	if credentials {
		input.Credentials = &camera.Credentials{Username: "camera-login", Password: "camera-password"}
	}
	cfg, err := camera.ParseConfiguration(pond, "Configured", input)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func addSession(t *testing.T, p *pgxpool.Pool) {
	t.Helper()
	exec(t, p, `INSERT INTO video_streams(id,camera_id,tenant_id,source_version,state) VALUES('00000000-0000-4000-8000-000000000101',101,101,1,'ready');
 INSERT INTO video_sessions(id,stream_id,camera_id,tenant_id,source_version,user_id,user_token_version,tenant_permission_version,member_permission_version,token_hash,state,created_at,expires_at)
 VALUES('00000000-0000-4000-8000-000000000102','00000000-0000-4000-8000-000000000101',101,101,1,1,0,0,0,decode(repeat('ab',32),'hex'),'ready',now(),now()+interval '5 minutes')`)
}
