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
