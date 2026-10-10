package mediapg_test

import (
	"errors"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/media"
)

func TestStopRevalidationAllowsCleanupAfterSourceChange(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-stop-source")
	if err != nil || !ok {
		t.Fatalf("start lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobReady, NextAt: time.Now().UTC()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err = store.StopOwnedPlayback(ctx, session.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(t.Context(), `UPDATE video_cameras SET source_version=2 WHERE id=40`); err != nil {
		t.Fatal(err)
	}
	lease, ok, err = store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-stop-source")
	if err != nil || !ok || lease.Kind != media.JobStop {
		t.Fatalf("stop lease=%+v ok=%v err=%v", lease, ok, err)
	}
	auth, err := store.RevalidateJob(ctx, lease)
	if err != nil {
		t.Fatal(err)
	}
	if !auth.Active || auth.SourceCurrent || auth.StopSuperseded {
		t.Fatalf("auth=%+v", auth)
	}
}

func TestStopRevalidationAllowsCleanupAfterCameraDisable(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-stop-disabled")
	if err != nil || !ok {
		t.Fatalf("start lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobReady, NextAt: time.Now().UTC()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err = store.StopOwnedPlayback(ctx, session.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(t.Context(), `UPDATE video_cameras SET enabled=false WHERE id=40`); err != nil {
		t.Fatal(err)
	}
	lease, ok, err = store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-stop-disabled")
	if err != nil || !ok || lease.Kind != media.JobStop {
		t.Fatalf("stop lease=%+v ok=%v err=%v", lease, ok, err)
	}
	auth, err := store.RevalidateJob(ctx, lease)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Active || !auth.SourceCurrent || auth.StopSuperseded {
		t.Fatalf("auth=%+v", auth)
	}
}

func TestStopRevalidationDoesNotHideProviderCleanupOnRevokedMembership(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-stop-membership")
	if err != nil || !ok {
		t.Fatalf("start lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobReady, NextAt: time.Now().UTC()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err = store.StopOwnedPlayback(ctx, session.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(t.Context(), `UPDATE tenant_memberships SET active=false WHERE tenant_id=10 AND user_id=1`); err != nil {
		t.Fatal(err)
	}
	lease, ok, err = store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-stop-membership")
	if err != nil || !ok || lease.Kind != media.JobStop {
		t.Fatalf("stop lease=%+v ok=%v err=%v", lease, ok, err)
	}
	auth, err := store.RevalidateJob(ctx, lease)
	if auth.Active || !auth.SourceCurrent || auth.StopSuperseded {
		t.Fatalf("auth=%+v err=%v", auth, err)
	}
}

func TestStopJobFencesCreateUntilFinish(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-stop-fence-start")
	if err != nil || !ok {
		t.Fatalf("start lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobReady, NextAt: time.Now().UTC()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err = store.StopOwnedPlayback(ctx, first.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreatePlayback(ctx, createInputToken(t, target, "media-token-new-session")); !errors.Is(err, media.ErrConflict) {
		t.Fatalf("create while stopping err=%v", err)
	}
	var sessions int
	if err = p.QueryRow(t.Context(), `SELECT count(*) FROM video_sessions WHERE stream_id=$1`, first.StreamID).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 {
		t.Fatalf("sessions=%d", sessions)
	}
	lease, ok, err = store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-stop-fence-stop")
	if err != nil || !ok || lease.Kind != media.JobStop {
		t.Fatalf("stop lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobStopped, NextAt: time.Now().UTC()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	target, err = store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreatePlayback(ctx, createInputToken(t, target, "media-token-restart-after-stop"))
	if err != nil || second.State != media.SessionPending {
		t.Fatalf("restart=%+v err=%v", second, err)
	}
}

func TestStopRetryKeepsStoppingAndRejectsCreate(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-stop-retry-start")
	if err != nil || !ok {
		t.Fatalf("start lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobReady, NextAt: time.Now().UTC()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err = store.StopOwnedPlayback(ctx, first.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	lease, ok, err = store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-stop-retry-stop")
	if err != nil || !ok || lease.Kind != media.JobStop {
		t.Fatalf("stop lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobRetryable, NextAt: time.Now().UTC()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var state string
	if err = p.QueryRow(t.Context(), `SELECT state FROM video_streams WHERE id=$1`, first.StreamID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "stopping" {
		t.Fatalf("state=%s", state)
	}
	if _, err = store.CreatePlayback(ctx, createInputToken(t, target, "media-token-during-stop-retry")); !errors.Is(err, media.ErrConflict) {
		t.Fatalf("create during retry err=%v", err)
	}
	var starts int
	if err = p.QueryRow(t.Context(), `SELECT count(*) FROM jobs WHERE kind='video.start'`).Scan(&starts); err != nil {
		t.Fatal(err)
	}
	if starts != 1 {
		t.Fatalf("start jobs=%d", starts)
	}
	lease, ok, err = store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-stop-retry-again")
	if err != nil || !ok || lease.Kind != media.JobStop {
		t.Fatalf("retry lease=%+v ok=%v err=%v", lease, ok, err)
	}
}
