package mediapg_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/media"
	"git.hyhy.fun/rsplab/iolink/internal/mediapg"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func fixture(t *testing.T) (*pgxpool.Pool, *mediapg.Store, context.Context) {
	t.Helper()
	p := testdb.New(t)
	if err := migrate.Up(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(t.Context(), `INSERT INTO users(id,open_id,authority,token_version) VALUES (1,'media-user','USER',2);
INSERT INTO tenants(id,name,permission_version) VALUES (10,'media-tenant',3);
INSERT INTO farms(id,tenant_id,owner_id,name) VALUES (20,10,1,'media-farm');
INSERT INTO ponds(id,farm_id,name) VALUES (30,20,'media-pond');
INSERT INTO tenant_memberships(tenant_id,user_id,role,permission_version) VALUES (10,1,'owner',4);
INSERT INTO video_cameras(id,tenant_id,farm_id,pond_id,name,source_kind,source_version,rtsp_uri) VALUES (40,10,20,30,'camera','rtsp',1,'rtsp://192.0.2.1/live')`); err != nil {
		t.Fatal(err)
	}
	ctx := domain.WithTenantID(domain.WithTenantUserID(domain.WithTenantRole(context.Background(), "owner"), 1), 10)
	ctx = domain.WithTenantPermissionVersion(ctx, 3)
	return p, mediapg.New(p), ctx
}

func createInput(t *testing.T, target media.PlaybackTarget) media.CreateSession {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	id := uuid.NewString()
	hash := sha256.Sum256([]byte("media-token"))
	return media.CreateSession{Target: target, TokenHash: hash, StartJobKey: "start:40:1", Session: media.Session{
		ID: id, StreamID: target.StreamID, TenantID: target.TenantID, CameraID: target.CameraID, SourceVersion: target.SourceVersion,
		UserID: target.UserID, UserTokenVersion: target.UserTokenVersion, TenantPermissionVersion: target.TenantPermissionVersion,
		MemberPermissionVersion: target.MemberPermissionVersion, State: media.SessionPending, CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute),
	}}
}

func TestPrepareCreateAndFinishPlayback(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil || target.StreamID == "" || target.UserTokenVersion != 2 || target.TenantPermissionVersion != 3 || target.MemberPermissionVersion != 4 {
		t.Fatalf("target=%+v err=%v", target, err)
	}
	in := createInput(t, target)
	session, err := store.CreatePlayback(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if session.StreamID == "" || session.ID != in.Session.ID {
		t.Fatalf("session=%+v", session)
	}
	var streams, sessions, jobs int
	if err = p.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM video_streams),(SELECT count(*) FROM video_sessions),(SELECT count(*) FROM jobs WHERE kind='video.start')`).Scan(&streams, &sessions, &jobs); err != nil {
		t.Fatal(err)
	}
	if streams != 1 || sessions != 1 || jobs != 1 {
		t.Fatalf("streams=%d sessions=%d jobs=%d", streams, sessions, jobs)
	}
	hash := sha256.Sum256([]byte("media-token"))
	claims := media.MediaClaims{SessionID: session.ID, TenantID: 10, CameraID: 40, UserID: 1, SourceVersion: 1, Audience: "iolink-media-v1", IssuedAt: session.CreatedAt, ExpiresAt: session.ExpiresAt, JTI: uuid.NewString()}
	pending, err := store.AuthorizeMedia(context.Background(), claims, hash, time.Now().UTC())
	if err != nil || pending.State != media.SessionPending {
		t.Fatalf("pending access=%+v err=%v", pending, err)
	}
	lease, ok, err := store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "test-worker")
	if err != nil || !ok {
		t.Fatalf("lease=%+v ok=%v err=%v", lease, ok, err)
	}
	auth, err := store.RevalidateJob(ctx, lease)
	if err != nil || !auth.Active || !auth.SourceCurrent {
		t.Fatalf("auth=%+v err=%v", auth, err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobReady, NextAt: time.Now().UTC()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	access, err := store.AuthorizeMedia(context.Background(), claims, hash, time.Now().UTC())
	if err != nil || access.State != media.SessionReady {
		t.Fatalf("access=%+v err=%v", access, err)
	}
	owned, err := store.ReadOwnedPlayback(ctx, session.ID)
	if err != nil || owned.State != media.SessionReady {
		t.Fatalf("owned=%+v err=%v", owned, err)
	}
	if err = store.StopOwnedPlayback(ctx, session.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	owned, err = store.ReadOwnedPlayback(ctx, session.ID)
	if err != nil || owned.State != media.SessionRevoked {
		t.Fatalf("stopped=%+v err=%v", owned, err)
	}
}

func TestCreateRejectsSourceVersionChangeWithoutPartialRows(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(t.Context(), `UPDATE video_cameras SET source_version=2 WHERE id=40`); err != nil {
		t.Fatal(err)
	}
	_, err = store.CreatePlayback(ctx, createInput(t, target))
	if !errors.Is(err, media.ErrConflict) {
		t.Fatalf("err=%v", err)
	}
	var sessions, jobs int
	if err = p.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM video_sessions),(SELECT count(*) FROM jobs WHERE kind='video.start')`).Scan(&sessions, &jobs); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 || jobs != 0 {
		t.Fatalf("partial rows sessions=%d jobs=%d", sessions, jobs)
	}
}

func TestCreateDuplicateStartJobRollsBackSession(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreatePlayback(ctx, createInput(t, target)); err != nil {
		t.Fatal(err)
	}
	_, err = store.CreatePlayback(ctx, createInput(t, target))
	if !errors.Is(err, media.ErrConflict) {
		t.Fatalf("err=%v", err)
	}
	var sessions, jobs int
	if err = p.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM video_sessions),(SELECT count(*) FROM jobs WHERE kind='video.start')`).Scan(&sessions, &jobs); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 || jobs != 1 {
		t.Fatalf("duplicate left partial rows sessions=%d jobs=%d", sessions, jobs)
	}
}

func TestAuthorizeRejectsWrongHash(t *testing.T) {
	_, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	claims := media.MediaClaims{SessionID: session.ID, TenantID: 10, CameraID: 40, UserID: 1, SourceVersion: 1, Audience: "iolink-media-v1", IssuedAt: session.CreatedAt, ExpiresAt: session.ExpiresAt, JTI: uuid.NewString()}
	wrong := sha256.Sum256([]byte("wrong"))
	if _, err = store.AuthorizeMedia(context.Background(), claims, wrong, time.Now().UTC()); !errors.Is(err, media.ErrUnauthorized) {
		t.Fatalf("err=%v", err)
	}
	validHash := sha256.Sum256([]byte("media-token"))
	claims.Audience = "other-audience"
	if _, err = store.AuthorizeMedia(context.Background(), claims, validHash, time.Now().UTC()); !errors.Is(err, media.ErrUnauthorized) {
		t.Fatalf("audience err=%v", err)
	}
	claims.Audience = "iolink-media-v1"
	claims.JTI = ""
	if _, err = store.AuthorizeMedia(context.Background(), claims, validHash, time.Now().UTC()); !errors.Is(err, media.ErrUnauthorized) {
		t.Fatalf("jti err=%v", err)
	}
	claims.JTI = uuid.NewString()
	claims.IssuedAt = time.Now().UTC().Add(time.Second)
	if _, err = store.AuthorizeMedia(context.Background(), claims, validHash, time.Now().UTC()); !errors.Is(err, media.ErrUnauthorized) {
		t.Fatalf("future issued err=%v", err)
	}
}
