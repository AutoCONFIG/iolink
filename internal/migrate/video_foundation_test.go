package migrate

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/videocredential"
)

func TestM7aVideoFoundationMigrationConstraints(t *testing.T) {
	pool := testDB(t)
	if err := Up(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"../../docs/evidence/M7a/2026-10-09/proposal-check.sql",
		"../../docs/evidence/M7a/2026-10-11-design/port-pairs-check.sql",
		"../../docs/evidence/M7a/2026-10-11-design/hls-path-check.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(t.Context(), string(raw)); err != nil {
			t.Fatalf("video constraint suite %s: %v", path, err)
		}
	}
}

func TestM7aVideoFoundationMigrationRollsBackAtomically(t *testing.T) {
	pool := testDB(t)
	ms, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(t.Context(), pool, ms[:12], false); err != nil {
		t.Fatal(err)
	}
	bad := append([]migration(nil), ms...)
	bad[12].sql += "\nSELECT 1/0;"
	err = apply(t.Context(), pool, bad, false)
	if err == nil {
		t.Fatal("failing video migration succeeded")
	}
	var partial bool
	if err := pool.QueryRow(t.Context(), `SELECT EXISTS
		(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name LIKE 'video_%')
		OR EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ponds_id_farm_video_unique')
		OR EXISTS (SELECT 1 FROM schema_migrations WHERE version=13)`).Scan(&partial); err != nil {
		t.Fatal(err)
	}
	if partial {
		t.Fatal("failed migration left video state")
	}
	if err := Up(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	if err := CheckLatest(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
}

func TestM7aVideoCredentialsPersistAsBoundCiphertext(t *testing.T) {
	pool := testDB(t)
	if err := Up(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO tenants(id,name) VALUES (731,'video-crypto');
		INSERT INTO users(id,open_id,authority) VALUES (731,'video-crypto','USER');
		INSERT INTO farms(id,owner_id,tenant_id,name) VALUES (731,731,731,'Video crypto');
		INSERT INTO ponds(id,farm_id,name) VALUES (731,731,'Video crypto');
		INSERT INTO video_cameras(id,tenant_id,farm_id,pond_id,name,source_kind,rtsp_uri)
		VALUES (731,731,731,731,'Video crypto','rtsp','rtsp://192.168.10.20:554/live');
		INSERT INTO video_gb_devices(id,tenant_id,device_id,name,credential_cipher)
		VALUES (731,731,'34020000001320000731','Video crypto',decode(repeat('aa',29),'hex'))`); err != nil {
		t.Fatal(err)
	}
	c, err := videocredential.New(bytes.Repeat([]byte{0x61}, 32))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name, table string
		kind        domain.VideoCredentialPurpose
	}{
		{"GB device", "video_gb_devices", domain.GBDeviceCredential},
		{"RTSP camera", "video_cameras", domain.CameraCredential},
	} {
		t.Run(item.name, func(t *testing.T) {
			binding, err := domain.NewVideoCredentialBinding(item.kind, 731, 731, 1)
			if err != nil {
				t.Fatal(err)
			}
			plaintext := []byte("disposable-video-password")
			sealed, err := c.Seal(t.Context(), binding, plaintext)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(t.Context(), `UPDATE `+item.table+` SET credential_cipher=$1 WHERE id=731`, sealed); err != nil {
				t.Fatal(err)
			}
			var stored []byte
			if err := pool.QueryRow(t.Context(), `SELECT credential_cipher FROM `+item.table+` WHERE id=731`).Scan(&stored); err != nil {
				t.Fatal(err)
			}
			got, err := c.Open(t.Context(), binding, stored)
			if err != nil || !bytes.Equal(got, plaintext) || bytes.Contains(stored, plaintext) {
				t.Fatalf("stored credential did not retain encrypted binding: %v", err)
			}
		})
	}
}

func TestM7aVideoFoundationRejectsInvalidStorage(t *testing.T) {
	pool := testDB(t)
	if err := Up(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO users(id,open_id,authority) VALUES (751,'video-storage','USER');
		INSERT INTO tenants(id,name) VALUES (751,'video-storage'),(752,'video-other');
		INSERT INTO farms(id,owner_id,tenant_id,name) VALUES (751,751,751,'Video');
		INSERT INTO ponds(id,farm_id,name) VALUES (751,751,'Video');
		INSERT INTO video_cameras(id,tenant_id,farm_id,pond_id,name,source_kind,rtsp_uri)
		VALUES (751,751,751,751,'Video','rtsp','rtsp://192.168.10.20:554/live');
		INSERT INTO video_gb_devices(id,tenant_id,device_id,name,credential_cipher)
		VALUES (751,751,'34020000001320000751','Video',decode(repeat('aa',29),'hex'));
		INSERT INTO video_streams(id,camera_id,tenant_id,source_version)
		VALUES ('00000000-0000-4000-8000-000000000751',751,751,1);
		INSERT INTO video_sessions(id,stream_id,camera_id,tenant_id,source_version,user_id,user_token_version,
		tenant_permission_version,member_permission_version,token_hash,created_at,expires_at)
		VALUES ('00000000-0000-4000-8000-000000000752','00000000-0000-4000-8000-000000000751',751,751,1,751,
		0,0,0,decode(repeat('aa',32),'hex'),now(),now()+interval '5 minutes')`); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ name, sql, sqlstate string }{
		{"global GB device identity", `INSERT INTO video_gb_devices(tenant_id,device_id,name,credential_cipher) VALUES (752,'34020000001320000751','Duplicate',decode(repeat('aa',29),'hex'))`, "23505"},
		{"unique camera source stream", `INSERT INTO video_streams(id,camera_id,tenant_id,source_version) VALUES ('00000000-0000-4000-8000-000000000753',751,751,1)`, "23505"},
		{"session source version binding", `UPDATE video_sessions SET source_version=2`, "23503"},
		{"stream tenant binding", `UPDATE video_streams SET tenant_id=752`, "23503"},
		{"camera credential envelope", `UPDATE video_cameras SET credential_cipher=decode(repeat('aa',28),'hex')`, "23514"},
		{"device credential envelope", `UPDATE video_gb_devices SET credential_cipher=decode(repeat('aa',28),'hex')`, "23514"},
		{"positive source version", `UPDATE video_cameras SET source_version=0`, "23514"},
		{"session state enum", `UPDATE video_sessions SET state='playing'`, "23514"},
		{"positive session lifetime", `UPDATE video_sessions SET expires_at=created_at`, "23514"},
	} {
		t.Run(item.name, func(t *testing.T) {
			tx, err := pool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(t.Context())
			_, err = tx.Exec(t.Context(), item.sql)
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != item.sqlstate {
				t.Fatalf("expected SQLSTATE %s, got %v", item.sqlstate, err)
			}
		})
	}
}
