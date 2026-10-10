package core_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	corepkg "git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

func TestM7aVideoHistoryProtectsResourceDeletionThroughUserHTTP(t *testing.T) {
	for _, item := range []struct {
		name, role, path string
		enabled          bool
		status           int
	}{
		{"active camera pond", "owner", "/ponds/741", true, http.StatusConflict},
		{"disabled camera pond", "owner", "/ponds/741", false, http.StatusConflict},
		{"tenant admin camera pond", "admin", "/ponds/741", true, http.StatusConflict},
		{"camera farm", "owner", "/farms/741", false, http.StatusConflict},
		{"unreferenced pond", "owner", "/ponds/742", true, http.StatusNoContent},
		{"foreign pond", "owner", "/ponds/743", true, http.StatusNotFound},
		{"foreign farm", "owner", "/farms/742", true, http.StatusNotFound},
		{"missing pond", "owner", "/ponds/799", true, http.StatusNotFound},
		{"member role", "member", "/ponds/741", true, http.StatusForbidden},
		{"read role", "viewer", "/ponds/741", true, http.StatusForbidden},
		{"unauthenticated", "owner", "/ponds/741", true, http.StatusUnauthorized},
	} {
		t.Run(item.name, func(t *testing.T) {
			pool := testdb.New(t)
			if err := migrate.Up(t.Context(), pool); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(t.Context(), `INSERT INTO users(id,open_id,username,password_hash,authority)
				VALUES (741,'video-history-owner','video-owner',$1,'USER'),(742,'video-history-other',NULL,NULL,'USER')`, platform.HashPassword("Video-history-test-password!")); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(t.Context(), `INSERT INTO tenants(id,name) VALUES (741,'video-history'),(742,'video-foreign');
				INSERT INTO farms(id,owner_id,tenant_id,name) VALUES(741,741,741,'Video'),(742,742,742,'Foreign');
				INSERT INTO ponds(id,farm_id,name) VALUES(741,741,'Video'),(742,741,'Empty'),(743,742,'Foreign');
				INSERT INTO video_cameras(id,tenant_id,farm_id,pond_id,name,source_kind,rtsp_uri)
				VALUES(741,741,741,741,'Camera','rtsp','rtsp://192.168.10.20:554/live');
				INSERT INTO video_streams(id,camera_id,tenant_id,source_version)
				VALUES ('00000000-0000-4000-8000-000000000741',741,741,1);
				INSERT INTO video_sessions(id,stream_id,camera_id,tenant_id,source_version,user_id,user_token_version,
				tenant_permission_version,member_permission_version,token_hash,created_at,expires_at)
				VALUES ('00000000-0000-4000-8000-000000000742','00000000-0000-4000-8000-000000000741',741,741,1,741,
				0,0,0,decode(repeat('aa',32),'hex'),now(),now()+interval '5 minutes')`); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(t.Context(), `INSERT INTO tenant_memberships(tenant_id,user_id,role) VALUES(741,741,$1)`, item.role); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(t.Context(), `UPDATE video_cameras SET enabled=$1`, item.enabled); err != nil {
				t.Fatal(err)
			}
			policy, err := authorization.New()
			if err != nil {
				t.Fatal(err)
			}
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			svc, err := corepkg.NewWithPolicy(t.Context(), pool, logger, policy)
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(adminapi.New(adminapi.Config{SecretKey: strings.Repeat("v", 32), JWT: time.Hour}, adminapi.Deps{Store: svc, Policy: policy, Logger: logger}).UserRoutes())
			defer server.Close()
			var login struct {
				Token string `json:"token"`
			}
			m6bHTTPRequest(t, server.URL, "", http.MethodPost, "/user/v1/login", `{"username":"video-owner","password":"Video-history-test-password!"}`, http.StatusOK, &login)
			if item.status == http.StatusUnauthorized {
				login.Token = ""
			}
			m6bHTTPRequest(t, server.URL, login.Token, http.MethodDelete, "/user/v1"+item.path, "", item.status, nil)
			if item.status != http.StatusNoContent {
				var cameras, ponds, farms, sessions int
				if err := pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM video_cameras),(SELECT count(*) FROM ponds),
					(SELECT count(*) FROM farms),(SELECT count(*) FROM video_sessions)`).Scan(&cameras, &ponds, &farms, &sessions); err != nil {
					t.Fatal(err)
				}
				if cameras != 1 || ponds != 3 || farms != 2 || sessions != 1 {
					t.Fatal("rejected delete removed video history or resource")
				}
			}
		})
	}
}
