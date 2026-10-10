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

func TestFinishRejectsOldSourceLeaseWithoutStateMutation(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-old-source")
	if err != nil || !ok {
		t.Fatalf("lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if _, err = p.Exec(t.Context(), `UPDATE video_cameras SET source_version=2 WHERE id=40`); err != nil {
		t.Fatal(err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobReady, NextAt: time.Now().UTC()}, time.Now().UTC()); !errors.Is(err, media.ErrLeaseLost) {
		t.Fatalf("err=%v", err)
	}
	var streamState, sessionState string
	if err = p.QueryRow(t.Context(), `SELECT s.state,(SELECT state FROM video_sessions WHERE id=$1) FROM video_streams s`, session.ID).Scan(&streamState, &sessionState); err != nil {
		t.Fatal(err)
	}
	if streamState != "starting" || sessionState != "pending" {
		t.Fatalf("state mutated stream=%s session=%s", streamState, sessionState)
	}
}

func TestRevalidateRejectsRevokedMembership(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreatePlayback(ctx, createInput(t, target)); err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-revoked")
	if err != nil || !ok {
		t.Fatalf("lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if _, err = p.Exec(t.Context(), `UPDATE tenant_memberships SET active=false,permission_version=permission_version+1 WHERE tenant_id=10 AND user_id=1`); err != nil {
		t.Fatal(err)
	}
	auth, err := store.RevalidateJob(ctx, lease)
	if err != nil || auth.Active {
		t.Fatalf("auth=%+v err=%v", auth, err)
	}
}

func TestAuthorizeRejectsCurrentVersionRevocation(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-auth-version")
	if err != nil || !ok {
		t.Fatalf("lease=%+v ok=%v err=%v", lease, ok, err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobReady, NextAt: time.Now().UTC()}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("media-token"))
	claims := media.MediaClaims{SessionID: session.ID, TenantID: 10, CameraID: 40, UserID: 1, SourceVersion: 1, Audience: "iolink-media-v1", IssuedAt: session.CreatedAt, ExpiresAt: session.ExpiresAt, JTI: uuid.NewString()}
	for name, statement := range map[string]string{
		"user token":    `UPDATE users SET token_version=token_version+1 WHERE id=1`,
		"tenant policy": `UPDATE tenants SET permission_version=permission_version+1 WHERE id=10`,
		"member policy": `UPDATE tenant_memberships SET permission_version=permission_version+1 WHERE tenant_id=10 AND user_id=1`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err = p.Exec(t.Context(), statement); err != nil {
				t.Fatal(err)
			}
			if _, err = store.AuthorizeMedia(context.Background(), claims, hash, time.Now().UTC()); !errors.Is(err, media.ErrForbidden) {
				t.Fatalf("err=%v", err)
			}
		})
		if _, err = p.Exec(t.Context(), `UPDATE users SET token_version=2 WHERE id=1`); err != nil {
			t.Fatal(err)
		}
		if _, err = p.Exec(t.Context(), `UPDATE tenants SET permission_version=3 WHERE id=10`); err != nil {
			t.Fatal(err)
		}
		if _, err = p.Exec(t.Context(), `UPDATE tenant_memberships SET permission_version=4 WHERE tenant_id=10 AND user_id=1`); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReadAndStopRejectCurrentAuthorization(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(t.Context(), `UPDATE users SET token_version=token_version+1 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ReadOwnedPlayback(ctx, session.ID); !errors.Is(err, media.ErrNotFound) {
		t.Fatalf("read err=%v", err)
	}
	if err = store.StopOwnedPlayback(ctx, session.ID, time.Now().UTC()); !errors.Is(err, media.ErrNotFound) {
		t.Fatalf("stop err=%v", err)
	}
}

func TestPrepareRejectsUnavailableLicenseSnapshot(t *testing.T) {
	p, store, ctx := fixture(t)
	if _, err := p.Exec(t.Context(), `UPDATE license_state SET payload=NULL,signature=NULL,payload_sha256=NULL,imported_at=NULL WHERE singleton`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PreparePlayback(ctx, 40); !errors.Is(err, media.ErrLicenseUnavailable) {
		t.Fatalf("missing license err=%v", err)
	}

	p, store, ctx = fixture(t)
	if _, err := p.Exec(t.Context(), `UPDATE license_clock SET clock_error=true WHERE singleton`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PreparePlayback(ctx, 40); !errors.Is(err, media.ErrLicenseUnavailable) {
		t.Fatalf("clock error=%v", err)
	}
}

func TestFinishRejectsMalformedLeaseAndPayloadFence(t *testing.T) {
	p, store, ctx := fixture(t)
	target, err := store.PreparePlayback(ctx, 40)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePlayback(ctx, createInput(t, target))
	if err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(ctx, time.Now().UTC(), time.Minute, "worker-fence")
	if err != nil || !ok {
		t.Fatalf("lease=%+v ok=%v err=%v", lease, ok, err)
	}
	bad := lease
	bad.Kind = media.JobKind("unknown")
	if err = store.FinishJob(ctx, bad, media.JobResult{Status: media.JobReady, NextAt: time.Now().UTC()}, time.Now().UTC()); !errors.Is(err, media.ErrInvalid) {
		t.Fatalf("invalid kind err=%v", err)
	}
	if _, err = p.Exec(t.Context(), `UPDATE jobs SET payload=jsonb_set(payload,'{camera_id}',to_jsonb(999::bigint),true) WHERE id=$1`, lease.ID); err != nil {
		t.Fatal(err)
	}
	if err = store.FinishJob(ctx, lease, media.JobResult{Status: media.JobReady, NextAt: time.Now().UTC()}, time.Now().UTC()); !errors.Is(err, media.ErrLeaseLost) {
		t.Fatalf("payload fence err=%v", err)
	}
	var streamState, sessionState string
	if err = p.QueryRow(t.Context(), `SELECT s.state,(SELECT state FROM video_sessions WHERE id=$1) FROM video_streams s`, session.ID).Scan(&streamState, &sessionState); err != nil {
		t.Fatal(err)
	}
	if streamState != "starting" || sessionState != "pending" {
		t.Fatalf("state mutated stream=%s session=%s", streamState, sessionState)
	}
}
