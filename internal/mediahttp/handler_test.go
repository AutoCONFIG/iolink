package mediahttp_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/media"
	"git.hyhy.fun/rsplab/iolink/internal/mediahttp"
	"git.hyhy.fun/rsplab/iolink/internal/playback"
)

const (
	mediaToken = "token-abcdefghijkl"
	mediaUUID  = "8e64b082-fb25-4f17-9f01-ae29b98033e1"
	segmentID  = "8e64b082-fb25-4f17-9f01-ae29b98033e2"
)

var mediaNow = time.Date(2026, 10, 11, 1, 0, 0, 0, time.UTC)

type store struct {
	access             media.MediaAccess
	mapping            media.SegmentMapping
	createErr          error
	created            []media.SegmentMapping
	attempted          int
	partialErr         bool
	resolveErr         error
	resolveSawDeadline bool
	authorizeErr       error
}

func (s *store) PreparePlayback(context.Context, int64) (media.PlaybackTarget, error) {
	return media.PlaybackTarget{}, errors.New("unused")
}

func (s *store) CreatePlayback(context.Context, media.CreateSession) (media.Session, error) {
	return media.Session{}, errors.New("unused")
}

func (s *store) ReadOwnedPlayback(context.Context, string) (media.Session, error) {
	return media.Session{}, errors.New("unused")
}

func (s *store) StopOwnedPlayback(context.Context, string, time.Time) error {
	return errors.New("unused")
}

func (s *store) ClaimJob(context.Context, time.Time, time.Duration, string) (media.Lease, bool, error) {
	return media.Lease{}, false, errors.New("unused")
}

func (s *store) RevalidateJob(context.Context, media.Lease) (media.JobAuthorization, error) {
	return media.JobAuthorization{}, errors.New("unused")
}

func (s *store) FinishJob(context.Context, media.Lease, media.JobResult, time.Time) error {
	return errors.New("unused")
}

func (s *store) AuthorizeMedia(context.Context, media.MediaClaims, [32]byte, time.Time) (media.MediaAccess, error) {
	if s.authorizeErr != nil {
		return media.MediaAccess{}, s.authorizeErr
	}
	return s.access, nil
}

func (s *store) CreateSegmentMappings(_ context.Context, mappings []media.SegmentMapping) ([]media.SegmentMapping, error) {
	if s.createErr != nil {
		if s.partialErr && len(mappings) > 0 {
			s.attempted = 1
		}
		return nil, s.createErr
	}
	s.created = append([]media.SegmentMapping(nil), mappings...)
	if len(mappings) > 0 {
		s.mapping = mappings[0]
	}
	return mappings, nil
}

func (s *store) ResolveSegmentMapping(ctx context.Context, _, id string) (media.SegmentMapping, error) {
	_, s.resolveSawDeadline = ctx.Deadline()
	if s.resolveErr != nil {
		return media.SegmentMapping{}, s.resolveErr
	}
	if s.mapping.ID != id {
		return media.SegmentMapping{}, media.ErrNotFound
	}
	return s.mapping, nil
}

type tokens struct{ claims media.MediaClaims }

func (t tokens) Issue(context.Context, media.MediaClaims) (media.IssuedToken, error) {
	return media.IssuedToken{}, errors.New("unused")
}
func (t tokens) Verify(context.Context, string) (media.MediaClaims, error) { return t.claims, nil }

type manifestSource struct{ body []byte }

func (m manifestSource) FetchManifest(context.Context, media.MediaAccess) ([]byte, error) {
	return m.body, nil
}

type segmentSource struct{ body []byte }

func (s segmentSource) FetchSegment(context.Context, media.MediaAccess, string) ([]byte, error) {
	return s.body, nil
}

func service(t *testing.T, state media.SessionState, manifest []byte, segment []byte) (*playback.Service, *store) {
	t.Helper()
	expires := mediaNow.Add(time.Minute)
	s := &store{access: media.MediaAccess{SessionID: mediaUUID, StreamID: "stream-7", TenantID: 21, CameraID: 9, SourceVersion: 7, State: state, ExpiresAt: expires}}
	claims := media.MediaClaims{SessionID: mediaUUID, TenantID: 21, CameraID: 9, UserID: 34, SourceVersion: 7, Audience: "iolink-media-v1", IssuedAt: mediaNow.Add(-time.Second), ExpiresAt: expires, JTI: "jti"}
	playbackService, err := playback.New(playback.Config{Store: s, Tokens: tokens{claims: claims}, Manifest: manifestSource{body: manifest}, Segments: segmentSource{body: segment}, Mappings: s, Enabled: true, Now: func() time.Time { return mediaNow }})
	if err != nil {
		t.Fatal(err)
	}
	return playbackService, s
}

func request(handler http.Handler, path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func TestManifestIsAuthorizedAndRewrittenWithNoStoreHeaders(t *testing.T) {
	s, store := service(t, media.SessionReady, []byte("#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\n2026-10-11/01/02-03_0.ts\n#EXT-X-ENDLIST\n"), []byte("ts"))
	recorder := request(mediahttp.New(s), "/media/v1/"+mediaToken+"/"+mediaUUID+"/index.m3u8")
	if recorder.Code != http.StatusOK || recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("status=%d headers=%v", recorder.Code, recorder.Header())
	}
	if !strings.Contains(recorder.Body.String(), "/segments/"+store.mapping.ID) || strings.Contains(recorder.Body.String(), "2026-10-11") {
		t.Fatalf("manifest was not rewritten: %q", recorder.Body.String())
	}
}

func TestManifestPendingReturns425AndRetryAfter(t *testing.T) {
	s, _ := service(t, media.SessionPending, []byte("#EXTM3U\n"), nil)
	recorder := request(mediahttp.New(s), "/media/v1/"+mediaToken+"/"+mediaUUID+"/index.m3u8")
	if recorder.Code != http.StatusTooEarly || recorder.Header().Get("Retry-After") != "1" {
		t.Fatalf("status=%d headers=%v", recorder.Code, recorder.Header())
	}
}

func TestGenericAuthorizationStoreFailureReturns503(t *testing.T) {
	s, store := service(t, media.SessionReady, []byte("#EXTM3U\n"), nil)
	store.authorizeErr = errors.New("database unavailable")
	recorder := request(mediahttp.New(s), "/media/v1/"+mediaToken+"/"+mediaUUID+"/index.m3u8")
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("generic authorization error status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerRejectsEncodedAndFragmentPathsAndMalformedSegmentID(t *testing.T) {
	s, _ := service(t, media.SessionReady, []byte("#EXTM3U\n"), nil)
	handler := mediahttp.New(s)
	encoded := httptest.NewRequest(http.MethodGet, "/media/v1/"+mediaToken+"/"+mediaUUID+"/index.m3u8", nil)
	encoded.URL.RawPath = "/media/v1/" + mediaToken + "/" + mediaUUID + "/index%2Em3u8"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, encoded)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("encoded path status=%d", recorder.Code)
	}
	fragment := httptest.NewRequest(http.MethodGet, "/media/v1/"+mediaToken+"/"+mediaUUID+"/index.m3u8", nil)
	fragment.URL.Fragment = "unexpected"
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, fragment)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("fragment path status=%d", recorder.Code)
	}
	recorder = request(handler, "/media/v1/"+mediaToken+"/"+mediaUUID+"/segments/not-a-uuid")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("malformed segment status=%d", recorder.Code)
	}
}

func TestManifestRejectsUnsafeTagsAndSegmentRejectsUnknownMapping(t *testing.T) {
	s, _ := service(t, media.SessionReady, []byte("#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=secret\n"), nil)
	handler := mediahttp.New(s)
	recorder := request(handler, "/media/v1/"+mediaToken+"/"+mediaUUID+"/index.m3u8")
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("unsafe manifest status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	recorder = request(handler, "/media/v1/"+mediaToken+"/"+mediaUUID+"/segments/"+segmentID)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown mapping status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestManifestRejectsUnknownTagsAndBatchFailureLeavesNoMappings(t *testing.T) {
	s, store := service(t, media.SessionReady, []byte("#EXTM3U\n#EXT-X-UNKNOWN:blocked\n2026-10-11/01/02-03_0.ts\n"), nil)
	recorder := request(mediahttp.New(s), "/media/v1/"+mediaToken+"/"+mediaUUID+"/index.m3u8")
	if recorder.Code != http.StatusBadGateway || len(store.created) != 0 {
		t.Fatalf("unknown tag status=%d created=%d", recorder.Code, len(store.created))
	}

	s, store = service(t, media.SessionReady, []byte("#EXTM3U\n#EXTINF:2,\n2026-10-11/01/02-03_0.ts\n#EXTINF:2,\n2026-10-11/01/02-05_1.ts\n"), nil)
	store.createErr = errors.New("atomic mapping failure")
	store.partialErr = true
	recorder = request(mediahttp.New(s), "/media/v1/"+mediaToken+"/"+mediaUUID+"/index.m3u8")
	if recorder.Code != http.StatusServiceUnavailable || store.attempted != 1 || len(store.created) != 0 {
		t.Fatalf("batch failure status=%d attempted=%d created=%d", recorder.Code, store.attempted, len(store.created))
	}
}

func TestSegmentResolveUsesBoundedContext(t *testing.T) {
	s, store := service(t, media.SessionReady, nil, nil)
	_, err := s.ReadSegment(context.Background(), mediaToken, mediaUUID, segmentID)
	if !errors.Is(err, playback.ErrNotFound) || !store.resolveSawDeadline {
		t.Fatalf("resolve err=%v deadline=%v", err, store.resolveSawDeadline)
	}
}

func TestManifestExpansionLimitRejectsBeforeBatchWrite(t *testing.T) {
	var source strings.Builder
	source.WriteString("#EXTM3U\n")
	for i := 0; i < 300; i++ {
		source.WriteString("2026-10-11/01/02-03_")
		source.WriteString(strconv.Itoa(i))
		source.WriteString(".ts\n")
	}
	s, store := service(t, media.SessionReady, []byte(source.String()), nil)
	longToken := strings.Repeat("a", 1024)
	_, err := s.ReadManifest(context.Background(), longToken, mediaUUID)
	if !errors.Is(err, playback.ErrBadMedia) || len(store.created) != 0 {
		t.Fatalf("expanded manifest err=%v created=%d", err, len(store.created))
	}
}

func TestSegmentReturnsTSAndOversizeIs502(t *testing.T) {
	s, store := service(t, media.SessionReady, []byte("#EXTM3U\n2026-10-11/01/02-03_0.ts\n"), []byte("ts"))
	handler := mediahttp.New(s)
	manifest := request(handler, "/media/v1/"+mediaToken+"/"+mediaUUID+"/index.m3u8")
	if manifest.Code != http.StatusOK {
		t.Fatal(manifest.Code)
	}
	recorder := request(handler, "/media/v1/"+mediaToken+"/"+mediaUUID+"/segments/"+store.mapping.ID)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "video/mp2t" || recorder.Body.String() != "ts" {
		t.Fatalf("segment response status=%d type=%q body=%q", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
	}
	large := strings.Repeat("x", 8*1024*1024+1)
	s, store = service(t, media.SessionReady, []byte("#EXTM3U\n2026-10-11/01/02-03_0.ts\n"), []byte(large))
	handler = mediahttp.New(s)
	_ = request(handler, "/media/v1/"+mediaToken+"/"+mediaUUID+"/index.m3u8")
	recorder = request(handler, "/media/v1/"+mediaToken+"/"+mediaUUID+"/segments/"+store.mapping.ID)
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("oversize status=%d", recorder.Code)
	}
}

func TestSegmentMappingStoreFailureIs503(t *testing.T) {
	store := &store{access: media.MediaAccess{SessionID: mediaUUID, StreamID: "stream-7", TenantID: 21, CameraID: 9, SourceVersion: 7, State: media.SessionReady, ExpiresAt: mediaNow.Add(time.Minute)}, resolveErr: errors.New("database unavailable")}
	claims := media.MediaClaims{SessionID: mediaUUID, TenantID: 21, CameraID: 9, UserID: 34, SourceVersion: 7, Audience: "iolink-media-v1", IssuedAt: mediaNow.Add(-time.Second), ExpiresAt: mediaNow.Add(time.Minute), JTI: "jti"}
	s, err := playback.New(playback.Config{Store: store, Tokens: tokens{claims: claims}, Enabled: true, Now: func() time.Time { return mediaNow }})
	if err != nil {
		t.Fatal(err)
	}
	recorder := request(mediahttp.New(s), "/media/v1/"+mediaToken+"/"+mediaUUID+"/segments/"+segmentID)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("store failure status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
