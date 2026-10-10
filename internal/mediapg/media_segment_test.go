package mediapg_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/media"
	"github.com/google/uuid"
)

func TestAuthorizeUsesDatabaseClockForExpiry(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("media-token"))
	claims := media.MediaClaims{SessionID: session.ID, TenantID: 10, CameraID: 40, UserID: 1, SourceVersion: 1, Audience: "iolink-media-v1", IssuedAt: session.CreatedAt, ExpiresAt: session.ExpiresAt, JTI: uuid.NewString()}
	if _, err = p.Exec(t.Context(), `UPDATE video_sessions SET created_at=clock_timestamp()-interval '4 minutes',expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.AuthorizeMedia(context.Background(), claims, hash, time.Unix(1, 0)); !errors.Is(err, media.ErrForbidden) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveSegmentMappingRechecksCurrentAuthorization(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-segment")
	if err != nil || !ok {
		t.Fatalf("lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobReady, NextAt: time.Now().UTC()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	mapping := media.SegmentMapping{ID: uuid.NewString(), SessionID: session.ID, StreamID: session.StreamID, SourceVersion: 1, Path: "2026-10-11/12/34-56_0.ts"}
	if _, err = store.CreateSegmentMappings(ctx, []media.SegmentMapping{mapping}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ResolveSegmentMapping(context.Background(), session.ID, mapping.ID); err != nil {
		t.Fatalf("resolve err=%v", err)
	}
	if _, err = p.Exec(t.Context(), `UPDATE tenant_memberships SET active=false WHERE tenant_id=10 AND user_id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ResolveSegmentMapping(context.Background(), session.ID, mapping.ID); !errors.Is(err, media.ErrNotFound) {
		t.Fatalf("revoked resolve err=%v", err)
	}
	if _, err = p.Exec(t.Context(), `UPDATE tenant_memberships SET active=true WHERE tenant_id=10 AND user_id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(t.Context(), `UPDATE video_sessions SET state='revoked',revoked_at=clock_timestamp() WHERE id=$1`, session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ResolveSegmentMapping(context.Background(), session.ID, mapping.ID); !errors.Is(err, media.ErrNotFound) {
		t.Fatalf("revoked session err=%v", err)
	}
}

func TestCreateRestartsAfterStoppedSharedStream(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-restart-start")
	if err != nil || !ok {
		t.Fatalf("lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobReady, NextAt: time.Now().UTC()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err = store.StopOwnedPlayback(ctx, first.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	lease, ok, err = store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-restart-stop")
	if err != nil || !ok || lease.Kind != media.JobStop {
		t.Fatalf("stop lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobStopped, NextAt: time.Now().UTC()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	second, err := store.CreatePlayback(ctx, createInputToken(t, target, "media-token-restart"))
	if err != nil {
		t.Fatal(err)
	}
	if second.State != media.SessionPending || second.StreamID != first.StreamID {
		t.Fatalf("session=%+v", second)
	}
	var streamState string
	var startJobs int
	if err = p.QueryRow(t.Context(), `SELECT state,(SELECT count(*) FROM jobs WHERE kind='video.start') FROM video_streams WHERE id=$1`, first.StreamID).Scan(&streamState, &startJobs); err != nil {
		t.Fatal(err)
	}
	if streamState != "pending" || startJobs != 2 {
		t.Fatalf("stream=%s startJobs=%d", streamState, startJobs)
	}
	lease, ok, err = store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-restart-again")
	if err != nil || !ok || lease.Kind != media.JobStart {
		t.Fatalf("restart lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobReady, NextAt: time.Now().UTC()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var readyState string
	if err = p.QueryRow(t.Context(), `SELECT state FROM video_sessions WHERE id=$1`, second.ID).Scan(&readyState); err != nil {
		t.Fatal(err)
	}
	if readyState != "ready" {
		t.Fatalf("restart session state=%s", readyState)
	}
}

func TestSharedStartSurvivesRevokedFirstSession(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreatePlayback(ctx, createInputToken(t, target, "media-token-second"))
	if err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-revoked-first")
	if err != nil || !ok {
		t.Fatalf("lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if _, err = p.Exec(t.Context(), `UPDATE video_sessions SET state='revoked',revoked_at=clock_timestamp() WHERE id=$1`, first.ID); err != nil {
		t.Fatal(err)
	}
	auth, err := store.RevalidateJob(ctx, lease)
	if err != nil || !auth.Active || auth.Target.UserID != 1 {
		t.Fatalf("auth=%+v err=%v", auth, err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobReady, NextAt: time.Now().UTC()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var firstState, secondState string
	if err = p.QueryRow(t.Context(), `SELECT (SELECT state FROM video_sessions WHERE id=$1),(SELECT state FROM video_sessions WHERE id=$2)`, first.ID, second.ID).Scan(&firstState, &secondState); err != nil {
		t.Fatal(err)
	}
	if firstState != "revoked" || secondState != "ready" {
		t.Fatalf("first=%s second=%s", firstState, secondState)
	}
}

func TestExpiredSiblingDoesNotBlockLastSessionStop(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreatePlayback(ctx, createInputToken(t, target, "media-token-expired"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(t.Context(), `UPDATE video_sessions SET created_at=clock_timestamp()-interval '4 minutes',expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, first.ID); err != nil {
		t.Fatal(err)
	}
	if err = store.StopOwnedPlayback(ctx, second.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var stopJobs int
	var streamState string
	if err = p.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM jobs WHERE kind='video.stop'),(SELECT state FROM video_streams LIMIT 1)`).Scan(&stopJobs, &streamState); err != nil {
		t.Fatal(err)
	}
	if stopJobs != 1 || streamState != "stopping" {
		t.Fatalf("stopJobs=%d streamState=%s", stopJobs, streamState)
	}
}
