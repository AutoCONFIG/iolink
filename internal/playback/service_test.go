package playback_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/media"
	"git.hyhy.fun/rsplab/iolink/internal/playback"
)

type fakeStore struct {
	target        media.PlaybackTarget
	create        media.CreateSession
	created       bool
	session       media.Session
	prepareErr    error
	createErr     error
	readErr       error
	stopErr       error
	claim         media.Lease
	claimOK       bool
	claimErr      error
	authorize     media.MediaAccess
	authorizeHash [32]byte
	authorizeErr  error
	revalidate    media.JobAuthorization
	revalidateErr error
	finish        media.JobResult
	finishLease   media.Lease
	finishErr     error
}

func (f *fakeStore) PreparePlayback(context.Context, int64) (media.PlaybackTarget, error) {
	return f.target, f.prepareErr
}

func (f *fakeStore) CreatePlayback(_ context.Context, in media.CreateSession) (media.Session, error) {
	f.create = in
	f.created = true
	if f.createErr != nil {
		return media.Session{}, f.createErr
	}
	return in.Session, nil
}

func (f *fakeStore) ReadOwnedPlayback(context.Context, string) (media.Session, error) {
	return f.session, f.readErr
}
func (f *fakeStore) StopOwnedPlayback(context.Context, string, time.Time) error { return f.stopErr }
func (f *fakeStore) ClaimJob(context.Context, time.Time, time.Duration, string) (media.Lease, bool, error) {
	return f.claim, f.claimOK, f.claimErr
}

func (f *fakeStore) RevalidateJob(context.Context, media.Lease) (media.JobAuthorization, error) {
	return f.revalidate, f.revalidateErr
}

func (f *fakeStore) FinishJob(_ context.Context, lease media.Lease, result media.JobResult, _ time.Time) error {
	f.finishLease = lease
	f.finish = result
	return f.finishErr
}

func (f *fakeStore) AuthorizeMedia(_ context.Context, _ media.MediaClaims, hash [32]byte, _ time.Time) (media.MediaAccess, error) {
	f.authorizeHash = hash
	return f.authorize, f.authorizeErr
}

type fakeLicense struct{ err error }

func (f fakeLicense) RequireVideo(context.Context, media.LicenseSnapshot) error { return f.err }

type fakeTokens struct {
	raw       string
	issueErr  error
	verifyErr error
	claims    media.MediaClaims
}

func (f fakeTokens) Issue(_ context.Context, claims media.MediaClaims) (media.IssuedToken, error) {
	if f.issueErr != nil {
		return media.IssuedToken{}, f.issueErr
	}
	if f.claims.SessionID == "" {
		f.claims = claims
	}
	return media.IssuedToken{Raw: f.raw, Claims: f.claims}, nil
}

func (f fakeTokens) Verify(context.Context, string) (media.MediaClaims, error) {
	return f.claims, f.verifyErr
}

type fakeProvider struct {
	startErr, stopErr     error
	startCalls, stopCalls int
}

func (f *fakeProvider) Start(context.Context, media.StartRequest) error {
	f.startCalls++
	return f.startErr
}

func (f *fakeProvider) Stop(context.Context, media.StopRequest) error {
	f.stopCalls++
	return f.stopErr
}

func actor() context.Context {
	ctx := domain.WithTenantID(context.Background(), 21)
	ctx = domain.WithTenantRole(ctx, "member")
	return domain.WithTenantUserID(ctx, 34)
}

func target() media.PlaybackTarget {
	return media.PlaybackTarget{
		TenantID: 21, CameraID: 9, SourceVersion: 7, StreamID: "stream-7", UserID: 34,
		UserTokenVersion: 2, TenantPermissionVersion: 3, MemberPermissionVersion: 4,
	}
}

func service(t *testing.T, store *fakeStore, enabled bool, provider media.Provider, license media.LicenseGuard, tokens media.TokenIssuer) *playback.Service {
	t.Helper()
	s, err := playback.New(playback.Config{Store: store, Enabled: enabled, Provider: provider, License: license, Tokens: tokens, ManifestBaseURL: "https://media.example.test/media/v1", Now: func() time.Time { return time.Date(2026, 10, 11, 1, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRequestPlaybackPersistsFiveMinuteSessionAndVersionedIdempotencyKey(t *testing.T) {
	store := &fakeStore{target: target()}
	s := service(t, store, true, &fakeProvider{}, fakeLicense{}, fakeTokens{raw: "token"})
	grant, err := s.RequestPlayback(actor(), 9)
	if err != nil {
		t.Fatal(err)
	}
	if grant.ExpiresAt.Sub(time.Date(2026, 10, 11, 1, 0, 0, 0, time.UTC)) != 300*time.Second {
		t.Fatalf("ttl=%s", grant.ExpiresAt.Sub(time.Date(2026, 10, 11, 1, 0, 0, 0, time.UTC)))
	}
	if store.create.Target.SourceVersion != 7 || store.create.StartJobKey != "start:9:7" {
		t.Fatalf("create input lost source version/idempotency key: %+v", store.create)
	}
	if !strings.HasPrefix(grant.ManifestURL, "https://") || store.create.TokenHash != sha256.Sum256([]byte("token")) {
		t.Fatalf("grant or token hash missing: %+v", grant)
	}
}

func TestRequestPlaybackReturnsSourceVersionConflictWithoutGrant(t *testing.T) {
	store := &fakeStore{target: target(), createErr: domain.ErrConflict}
	s := service(t, store, true, &fakeProvider{}, fakeLicense{}, fakeTokens{raw: "token"})
	if _, err := s.RequestPlayback(actor(), 9); !errors.Is(err, playback.ErrConflict) {
		t.Fatalf("source version conflict err=%v", err)
	}
}

func TestRequestPlaybackFailsClosedWhenCapabilityOrLicenseIsMissing(t *testing.T) {
	store := &fakeStore{target: target()}
	s := service(t, store, false, nil, nil, nil)
	if _, err := s.RequestPlayback(actor(), 9); !errors.Is(err, playback.ErrUnavailable) {
		t.Fatalf("disabled playback err=%v", err)
	}
	s = service(t, store, true, &fakeProvider{}, nil, fakeTokens{raw: "token"})
	if _, err := s.RequestPlayback(actor(), 9); !errors.Is(err, playback.ErrUnavailable) {
		t.Fatalf("missing license err=%v", err)
	}
	if store.created {
		t.Fatal("fail-closed request inserted a session")
	}
	store = &fakeStore{target: target()}
	s = service(t, store, true, &fakeProvider{}, fakeLicense{err: media.ErrLicenseRequired}, fakeTokens{raw: "token"})
	if _, err := s.RequestPlayback(actor(), 9); !errors.Is(err, playback.ErrLicenseRequired) {
		t.Fatalf("license rejection err=%v", err)
	}
}

func TestProcessMediaJobSkipsStopWhenStreamHasLiveReplacement(t *testing.T) {
	store := &fakeStore{
		claim:      media.Lease{Job: media.Job{ID: "7", Kind: media.JobStop, CameraID: 9, SourceVersion: 7, StreamID: "stream-7", SessionID: "session-7", Attempts: 1}, LeaseToken: "lease", LeasedUntil: time.Date(2026, 10, 11, 1, 0, 30, 0, time.UTC)},
		claimOK:    true,
		revalidate: media.JobAuthorization{Active: false, SourceCurrent: true},
	}
	provider := &fakeProvider{}
	s := service(t, store, true, provider, fakeLicense{}, fakeTokens{})
	if ok, err := s.ProcessMediaJob(actor(), "worker"); !ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if provider.stopCalls != 0 {
		t.Fatalf("stop calls=%d", provider.stopCalls)
	}
	if store.finish.Status != media.JobSkipped {
		t.Fatalf("finish=%+v", store.finish)
	}
}

func TestRequestPlaybackFailsClosedWhenTokenIssuerIsMissing(t *testing.T) {
	store := &fakeStore{target: target()}
	s := service(t, store, true, &fakeProvider{}, fakeLicense{}, nil)
	if _, err := s.RequestPlayback(actor(), 9); !errors.Is(err, playback.ErrUnavailable) {
		t.Fatalf("missing token issuer err=%v", err)
	}
	if store.created {
		t.Fatal("missing token issuer inserted a session")
	}
}

func TestRequestPlaybackRequiresAbsoluteHTTPSManifestURLWhenEnabled(t *testing.T) {
	for _, base := range []string{"", "/media/v1", "http://media.example.test/media/v1", "https://", "https://user:pass@media.example.test/media/v1", "https://media.example.test/media/v1?debug=1", "https://media.example.test/media/v1#fragment"} {
		store := &fakeStore{target: target()}
		s, err := playback.New(playback.Config{
			Store: store, Enabled: true, Provider: &fakeProvider{}, License: fakeLicense{}, Tokens: fakeTokens{raw: "token"},
			ManifestBaseURL: base, Now: func() time.Time { return time.Date(2026, 10, 11, 1, 0, 0, 0, time.UTC) },
		})
		if err != nil {
			t.Fatalf("base %q New err=%v", base, err)
		}
		if _, err := s.RequestPlayback(actor(), 9); !errors.Is(err, playback.ErrUnavailable) {
			t.Fatalf("base %q err=%v", base, err)
		}
		if store.created {
			t.Fatalf("base %q persisted a session", base)
		}
	}
	if _, err := playback.New(playback.Config{Store: &fakeStore{}}); err != nil {
		t.Fatalf("disabled service construction err=%v", err)
	}
}

func TestReadAndStopPreserveOwnerOnlyStoreContract(t *testing.T) {
	store := &fakeStore{session: media.Session{ID: "s", TenantID: 21, UserID: 999, State: media.SessionReady}}
	s := service(t, store, false, nil, nil, nil)
	if _, err := s.ReadPlayback(actor(), "s"); !errors.Is(err, playback.ErrNotFound) {
		t.Fatalf("cross-user read err=%v", err)
	}
	store.stopErr = media.ErrNotFound
	if err := s.StopPlayback(actor(), "s"); !errors.Is(err, playback.ErrNotFound) {
		t.Fatalf("cross-user stop err=%v", err)
	}
}

func TestReadPlaybackExpiresAtExactBoundary(t *testing.T) {
	now := time.Date(2026, 10, 11, 1, 5, 0, 0, time.UTC)
	store := &fakeStore{session: media.Session{ID: "s", TenantID: 21, UserID: 34, State: media.SessionReady, ExpiresAt: now}}
	s, err := playback.New(playback.Config{Store: store, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	session, err := s.ReadPlayback(actor(), "s")
	if err != nil || session.State != media.SessionExpired {
		t.Fatalf("boundary state=%+v err=%v", session, err)
	}
}

func TestProcessMediaJobUsesLeaseAndRecordsUnknownWithoutReplay(t *testing.T) {
	store := &fakeStore{
		claim:      media.Lease{Job: media.Job{ID: "j", Kind: media.JobStart, CameraID: 9, SourceVersion: 7, StreamID: "stream-7", Attempts: 1}, LeaseToken: "lease"},
		claimOK:    true,
		revalidate: media.JobAuthorization{Target: target(), Active: true, SourceCurrent: true},
	}
	s := service(t, store, true, &fakeProvider{startErr: media.ErrUnknown}, fakeLicense{}, fakeTokens{raw: "token"})
	ok, err := s.ProcessMediaJob(actor(), "worker-1")
	if !ok || !errors.Is(err, media.ErrUnknown) {
		t.Fatalf("unknown process ok=%v err=%v", ok, err)
	}
	if store.finish.Status != media.JobUnknown || store.finishLease.LeaseToken != "lease" {
		t.Fatalf("unknown result=%+v lease=%+v", store.finish, store.finishLease)
	}
}

func TestProcessMediaJobLeaseCASFailureIsUnknownToCaller(t *testing.T) {
	store := &fakeStore{
		claim:      media.Lease{Job: media.Job{ID: "j", Kind: media.JobStart, CameraID: 9, SourceVersion: 7, StreamID: "stream-7", Attempts: 1}, LeaseToken: "lease"},
		claimOK:    true,
		revalidate: media.JobAuthorization{Target: target(), Active: true, SourceCurrent: true},
		finishErr:  media.ErrLeaseLost,
	}
	s := service(t, store, true, &fakeProvider{}, fakeLicense{}, fakeTokens{raw: "token"})
	ok, err := s.ProcessMediaJob(actor(), "worker-1")
	if !ok || !errors.Is(err, media.ErrLeaseLost) {
		t.Fatalf("lease loss ok=%v err=%v", ok, err)
	}
}

func TestProcessMediaJobSkipsInactiveOrStaleStartWithoutProvider(t *testing.T) {
	for _, tc := range []struct {
		name string
		auth media.JobAuthorization
	}{
		{name: "inactive", auth: media.JobAuthorization{Target: target(), Active: false, SourceCurrent: true}},
		{name: "stale-source", auth: media.JobAuthorization{Target: target(), Active: true, SourceCurrent: false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{
				claim:      media.Lease{Job: media.Job{ID: "j", Kind: media.JobStart, CameraID: 9, SourceVersion: 7, StreamID: "stream-7", Attempts: 1}, LeaseToken: "lease"},
				claimOK:    true,
				revalidate: tc.auth,
			}
			provider := &fakeProvider{}
			s := service(t, store, true, provider, fakeLicense{}, fakeTokens{raw: "token"})
			ok, err := s.ProcessMediaJob(actor(), "worker-1")
			if !ok || err != nil {
				t.Fatalf("skip ok=%v err=%v", ok, err)
			}
			if store.finish.Status != media.JobSkipped {
				t.Fatalf("finish status=%s, want skipped", store.finish.Status)
			}
			if provider.startCalls != 0 {
				t.Fatalf("provider start calls=%d, want 0", provider.startCalls)
			}
		})
	}
}

func TestAuthorizeMediaChecksAudienceClaimsHashAndPendingState(t *testing.T) {
	now := time.Date(2026, 10, 11, 1, 0, 0, 0, time.UTC)
	claims := media.MediaClaims{SessionID: "s", TenantID: 21, CameraID: 9, UserID: 34, SourceVersion: 7, Audience: "iolink-media-v1", IssuedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Minute), JTI: "j"}
	store := &fakeStore{authorize: media.MediaAccess{SessionID: "s", TenantID: 21, CameraID: 9, SourceVersion: 7, State: media.SessionPending, ExpiresAt: claims.ExpiresAt}}
	s, err := playback.New(playback.Config{Store: store, Tokens: fakeTokens{claims: claims}, Enabled: true, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AuthorizeMedia(context.Background(), "token", "s"); !errors.Is(err, playback.ErrPending) {
		t.Fatalf("pending err=%v", err)
	}
	if store.authorizeHash != sha256.Sum256([]byte("token")) {
		t.Fatal("token hash was not passed to authorization store")
	}
	bad := claims
	bad.Audience = "other"
	s, _ = playback.New(playback.Config{Store: store, Tokens: fakeTokens{claims: bad}, Enabled: true, Now: func() time.Time { return now }})
	if _, err := s.AuthorizeMedia(context.Background(), "token", "s"); !errors.Is(err, playback.ErrUnauthorized) {
		t.Fatalf("audience err=%v", err)
	}
}

func TestAuthorizeMediaRejectsTokenErrorsAndRevokedSessionsWithoutActor(t *testing.T) {
	now := time.Date(2026, 10, 11, 1, 0, 0, 0, time.UTC)
	claims := media.MediaClaims{SessionID: "s", TenantID: 21, CameraID: 9, UserID: 34, SourceVersion: 7, Audience: "iolink-media-v1", IssuedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Minute), JTI: "j"}
	store := &fakeStore{authorize: media.MediaAccess{SessionID: "s", TenantID: 21, CameraID: 9, SourceVersion: 7, State: media.SessionRevoked, ExpiresAt: claims.ExpiresAt}}
	s, err := playback.New(playback.Config{Store: store, Tokens: fakeTokens{claims: claims}, Enabled: true, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AuthorizeMedia(context.Background(), "token", "s"); !errors.Is(err, playback.ErrForbidden) {
		t.Fatalf("revoked session err=%v", err)
	}
	store = &fakeStore{}
	s, _ = playback.New(playback.Config{Store: store, Tokens: fakeTokens{claims: claims, verifyErr: errors.New("invalid token")}, Enabled: true, Now: func() time.Time { return now }})
	if _, err := s.AuthorizeMedia(context.Background(), "token", "s"); !errors.Is(err, playback.ErrUnauthorized) {
		t.Fatalf("token verification err=%v", err)
	}
	if store.authorizeHash != ([32]byte{}) {
		t.Fatal("invalid token reached authorization store")
	}
}

func TestAuthorizeMediaFailsClosedWhenVideoIsDisabled(t *testing.T) {
	store := &fakeStore{}
	s := service(t, store, false, nil, nil, fakeTokens{})
	if _, err := s.AuthorizeMedia(context.Background(), "token", "s"); !errors.Is(err, playback.ErrUnavailable) {
		t.Fatalf("disabled gateway err=%v", err)
	}
}
