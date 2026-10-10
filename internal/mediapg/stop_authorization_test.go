package mediapg_test

import (
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
