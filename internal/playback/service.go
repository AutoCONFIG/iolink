package playback

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/media"
	"github.com/google/uuid"
)

const (
	mediaAudience = "iolink-media-v1"
	maxAttempts   = 6
)

var (
	ErrInvalid            = media.ErrInvalid
	ErrUnavailable        = media.ErrUnavailable
	ErrUnauthorized       = media.ErrUnauthorized
	ErrForbidden          = media.ErrForbidden
	ErrNotFound           = media.ErrNotFound
	ErrConflict           = media.ErrConflict
	ErrPending            = media.ErrPending
	ErrDisabled           = media.ErrDisabled
	ErrUnsupported        = media.ErrUnsupported
	ErrRetryable          = media.ErrRetryable
	ErrUnknown            = media.ErrUnknown
	ErrLeaseLost          = media.ErrLeaseLost
	ErrPersistenceUnknown = media.ErrPersistenceUnknown
	ErrLicenseRequired    = media.ErrLicenseRequired
	ErrLicenseUnavailable = media.ErrLicenseUnavailable
	ErrBadMedia           = media.ErrBadMedia
)

type Config struct {
	Store           media.Store
	License         media.LicenseGuard
	Tokens          media.TokenIssuer
	Provider        media.Provider
	Manifest        media.ManifestSource
	Segments        media.SegmentSource
	Mappings        media.SegmentMappingStore
	Enabled         bool
	ManifestBaseURL string
	LeaseDuration   time.Duration
	ManifestTimeout time.Duration
	SegmentTimeout  time.Duration
	Now             func() time.Time
}

type Service struct {
	cfg Config
}

type Grant struct {
	SessionID   string    `json:"session_id"`
	ExpiresAt   time.Time `json:"expires_at"`
	ManifestURL string    `json:"manifest_url"`
}

type MediaAccess = media.MediaAccess

var segmentPathPattern = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})/(\d{2})/(\d{2})-(\d{2})_(\d+)\.ts$`)

func New(cfg Config) (*Service, error) {
	if cfg.Store == nil {
		return nil, ErrUnavailable
	}
	if cfg.ManifestBaseURL == "" {
		cfg.ManifestBaseURL = "/media/v1"
	}
	if cfg.LeaseDuration <= 0 {
		cfg.LeaseDuration = 30 * time.Second
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Service{cfg: cfg}, nil
}

func (s *Service) RequestPlayback(ctx context.Context, cameraID int64) (Grant, error) {
	if err := requireActor(ctx); err != nil {
		return Grant{}, err
	}
	if !s.cfg.Enabled || s.cfg.License == nil || s.cfg.Provider == nil || s.cfg.Tokens == nil {
		return Grant{}, ErrUnavailable
	}
	if !validManifestBaseURL(s.cfg.ManifestBaseURL) {
		return Grant{}, ErrUnavailable
	}
	if cameraID <= 0 {
		return Grant{}, ErrInvalid
	}
	now := s.cfg.Now().UTC()
	target, err := s.cfg.Store.PreparePlayback(ctx, cameraID)
	if err != nil {
		return Grant{}, normalizeStoreError(err)
	}
	if target.TenantID <= 0 || target.CameraID != cameraID || target.SourceVersion <= 0 || target.StreamID == "" {
		return Grant{}, ErrUnavailable
	}
	if err = s.cfg.License.RequireVideo(ctx, target.License); err != nil {
		return Grant{}, normalizeLicenseError(err)
	}
	sessionID := uuid.NewString()
	expiresAt := now.Add(domain.MaxVideoPlaybackTTL)
	claims := media.MediaClaims{
		SessionID:     sessionID,
		TenantID:      target.TenantID,
		CameraID:      cameraID,
		UserID:        target.UserID,
		SourceVersion: target.SourceVersion,
		Audience:      mediaAudience,
		IssuedAt:      now,
		ExpiresAt:     expiresAt,
		JTI:           uuid.NewString(),
	}
	issued, err := s.cfg.Tokens.Issue(ctx, claims)
	if err != nil || issued.Raw == "" {
		return Grant{}, ErrUnavailable
	}
	if err := validateClaims(issued.Claims, claims, now); err != nil {
		return Grant{}, ErrUnavailable
	}
	input := media.CreateSession{
		Session: media.Session{
			ID:                      sessionID,
			StreamID:                target.StreamID,
			TenantID:                target.TenantID,
			CameraID:                cameraID,
			SourceVersion:           target.SourceVersion,
			UserID:                  target.UserID,
			UserTokenVersion:        target.UserTokenVersion,
			TenantPermissionVersion: target.TenantPermissionVersion,
			MemberPermissionVersion: target.MemberPermissionVersion,
			State:                   media.SessionPending,
			CreatedAt:               now,
			ExpiresAt:               expiresAt,
		},
		Target:      target,
		TokenHash:   sha256.Sum256([]byte(issued.Raw)),
		StartJobKey: fmt.Sprintf("start:%d:%d", cameraID, target.SourceVersion),
	}
	if _, err = s.cfg.Store.CreatePlayback(ctx, input); err != nil {
		return Grant{}, normalizeStoreError(err)
	}
	return Grant{SessionID: sessionID, ExpiresAt: expiresAt, ManifestURL: manifestURL(s.cfg.ManifestBaseURL, issued.Raw, sessionID)}, nil
}

func (s *Service) ReadPlayback(ctx context.Context, sessionID string) (media.Session, error) {
	if err := requireActor(ctx); err != nil {
		return media.Session{}, err
	}
	if sessionID == "" {
		return media.Session{}, ErrInvalid
	}
	session, err := s.cfg.Store.ReadOwnedPlayback(ctx, sessionID)
	if err != nil {
		return media.Session{}, normalizeStoreError(err)
	}
	tenantID, _ := domain.TenantID(ctx)
	userID, _ := domain.TenantUserID(ctx)
	if session.TenantID != tenantID || session.UserID != userID {
		return media.Session{}, ErrNotFound
	}
	if (session.State == media.SessionPending || session.State == media.SessionReady) && !s.cfg.Now().Before(session.ExpiresAt) {
		session.State = media.SessionExpired
	}
	return session, nil
}

func (s *Service) StopPlayback(ctx context.Context, sessionID string) error {
	if err := requireActor(ctx); err != nil {
		return err
	}
	if sessionID == "" {
		return ErrInvalid
	}
	if err := s.cfg.Store.StopOwnedPlayback(ctx, sessionID, s.cfg.Now().UTC()); err != nil {
		return normalizeStoreError(err)
	}
	return nil
}

func (s *Service) AuthorizeMedia(ctx context.Context, rawToken, sessionID string) (MediaAccess, error) {
	if rawToken == "" || sessionID == "" {
		return MediaAccess{}, ErrUnauthorized
	}
	if !s.cfg.Enabled || s.cfg.Tokens == nil {
		return MediaAccess{}, ErrUnavailable
	}
	now := s.cfg.Now().UTC()
	claims, err := s.cfg.Tokens.Verify(ctx, rawToken)
	if err != nil || claims.SessionID != sessionID || claims.Audience != mediaAudience {
		return MediaAccess{}, ErrUnauthorized
	}
	if err = validateClaims(claims, claims, now); err != nil {
		return MediaAccess{}, ErrUnauthorized
	}
	access, err := s.cfg.Store.AuthorizeMedia(ctx, claims, sha256.Sum256([]byte(rawToken)), now)
	if err != nil {
		return MediaAccess{}, normalizeMediaStoreError(err)
	}
	if access.SessionID != sessionID || access.TenantID != claims.TenantID || access.CameraID != claims.CameraID || access.SourceVersion != claims.SourceVersion {
		return MediaAccess{}, ErrUnauthorized
	}
	if access.State == media.SessionPending {
		return access, ErrPending
	}
	if access.State != media.SessionReady || !now.Before(access.ExpiresAt) {
		return MediaAccess{}, ErrForbidden
	}
	return access, nil
}

func (s *Service) ReadManifest(ctx context.Context, rawToken, sessionID string) ([]byte, error) {
	if ctx == nil {
		return nil, ErrForbidden
	}
	access, err := s.AuthorizeMedia(ctx, rawToken, sessionID)
	if err != nil {
		return nil, err
	}
	if s.cfg.Manifest == nil {
		return nil, ErrUnavailable
	}
	timeout := s.cfg.ManifestTimeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	fetchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	manifest, err := s.cfg.Manifest.FetchManifest(fetchCtx, access)
	if err != nil {
		return nil, normalizeMediaSourceError(err)
	}
	if len(manifest) == 0 || len(manifest) > media.MaxManifestBytes {
		return nil, ErrBadMedia
	}
	return s.rewriteManifest(fetchCtx, access, rawToken, manifest)
}

func (s *Service) ReadSegment(ctx context.Context, rawToken, sessionID, segmentID string) ([]byte, error) {
	if ctx == nil {
		return nil, ErrForbidden
	}
	access, err := s.AuthorizeMedia(ctx, rawToken, sessionID)
	if err != nil {
		return nil, err
	}
	if s.cfg.Segments == nil || s.cfg.Mappings == nil || segmentID == "" {
		return nil, ErrUnavailable
	}
	timeout := s.cfg.SegmentTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	fetchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	mapping, err := s.cfg.Mappings.ResolveSegmentMapping(fetchCtx, sessionID, segmentID)
	if err != nil {
		return nil, normalizeMediaStoreError(err)
	}
	if mapping.ID != segmentID || mapping.SessionID != sessionID || mapping.StreamID != access.StreamID || mapping.SourceVersion != access.SourceVersion || !validSegmentPath(mapping.Path) {
		return nil, ErrNotFound
	}
	segment, err := s.cfg.Segments.FetchSegment(fetchCtx, access, mapping.Path)
	if err != nil {
		return nil, normalizeMediaSourceError(err)
	}
	if len(segment) == 0 || len(segment) > media.MaxSegmentBytes {
		return nil, ErrBadMedia
	}
	return segment, nil
}

func (s *Service) rewriteManifest(ctx context.Context, access media.MediaAccess, rawToken string, manifest []byte) ([]byte, error) {
	text := strings.ReplaceAll(string(manifest), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || lines[0] != "#EXTM3U" || s.cfg.Mappings == nil {
		return nil, ErrBadMedia
	}
	segmentLines := make([]int, 0, len(lines))
	for i, line := range lines {
		if strings.ContainsAny(line, "\x00\r") {
			return nil, ErrBadMedia
		}
		if strings.HasPrefix(line, "#") {
			if !allowedManifestTag(line) {
				return nil, ErrBadMedia
			}
			continue
		}
		if i == 0 || line == "" {
			return nil, ErrBadMedia
		}
		if !validSegmentPath(line) {
			return nil, ErrBadMedia
		}
		segmentLines = append(segmentLines, i)
	}
	mappings := make([]media.SegmentMapping, 0, len(segmentLines))
	rewritten := append([]string(nil), lines...)
	for _, i := range segmentLines {
		line := lines[i]
		requestedID := uuid.NewString()
		mappings = append(mappings, media.SegmentMapping{ID: requestedID, SessionID: access.SessionID, StreamID: access.StreamID, SourceVersion: access.SourceVersion, Path: line})
		rewritten[i] = "/media/v1/" + url.PathEscape(rawToken) + "/" + url.PathEscape(access.SessionID) + "/segments/" + url.PathEscape(requestedID)
	}
	output := []byte(strings.Join(rewritten, "\n") + "\n")
	if len(output) > media.MaxManifestBytes {
		return nil, ErrBadMedia
	}
	created, err := s.cfg.Mappings.CreateSegmentMappings(ctx, mappings)
	if err != nil {
		return nil, normalizeMediaStoreError(err)
	}
	if len(created) != len(mappings) {
		return nil, ErrUnavailable
	}
	for i, mapping := range created {
		requested := mappings[i]
		if mapping.ID == "" || mapping.ID != requested.ID || mapping.SessionID != access.SessionID || mapping.StreamID != access.StreamID || mapping.SourceVersion != access.SourceVersion || mapping.Path != requested.Path || !validSegmentPath(mapping.Path) {
			return nil, ErrBadMedia
		}
	}
	return output, nil
}

func allowedManifestTag(line string) bool {
	if strings.Contains(line, `URI=`) || strings.Contains(line, "../") || strings.Contains(line, "\\") {
		return false
	}
	if line == "#EXTM3U" || strings.HasPrefix(line, "#EXTINF:") || strings.HasPrefix(line, "#EXT-X-VERSION:") || strings.HasPrefix(line, "#EXT-X-TARGETDURATION:") || strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:") || strings.HasPrefix(line, "#EXT-X-DISCONTINUITY-SEQUENCE:") || line == "#EXT-X-DISCONTINUITY" || strings.HasPrefix(line, "#EXT-X-PROGRAM-DATE-TIME:") || line == "#EXT-X-GAP" || line == "#EXT-X-INDEPENDENT-SEGMENTS" || line == "#EXT-X-ENDLIST" || line == "#EXT-X-PLAYLIST-TYPE:EVENT" {
		return true
	}
	return false
}

func validSegmentPath(path string) bool {
	if path == "" || strings.ContainsAny(path, `%?#\\`) || strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
		return false
	}
	m := segmentPathPattern.FindStringSubmatch(path)
	if len(m) != 6 {
		return false
	}
	if _, err := time.Parse("2006-01-02", m[1]); err != nil {
		return false
	}
	hour, err := strconv.Atoi(m[2])
	if err != nil || hour > 23 {
		return false
	}
	minute, err := strconv.Atoi(m[3])
	if err != nil || minute > 59 {
		return false
	}
	second, err := strconv.Atoi(m[4])
	if err != nil || second > 59 {
		return false
	}
	_, err = strconv.ParseUint(m[5], 10, 32)
	return err == nil
}

func normalizeMediaSourceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, media.ErrUnavailable) || errors.Is(err, media.ErrNotFound) || errors.Is(err, media.ErrForbidden) {
		return err
	}
	return ErrBadMedia
}

func normalizeMediaStoreError(err error) error {
	normalized := normalizeStoreError(err)
	switch normalized {
	case ErrNotFound, ErrForbidden, ErrConflict:
		return normalized
	default:
		return ErrUnavailable
	}
}

func (s *Service) ProcessMediaJob(ctx context.Context, workerID string) (bool, error) {
	if !s.cfg.Enabled || s.cfg.Provider == nil {
		return false, ErrUnavailable
	}
	if workerID == "" {
		return false, ErrInvalid
	}
	now := s.cfg.Now().UTC()
	lease, ok, err := s.cfg.Store.ClaimJob(ctx, now, s.cfg.LeaseDuration, workerID)
	if err != nil || !ok {
		return ok, normalizeStoreError(err)
	}
	auth, err := s.cfg.Store.RevalidateJob(ctx, lease)
	if err != nil {
		return true, normalizeStoreError(err)
	}
	result := media.JobResult{Status: media.JobSkipped, NextAt: now}
	skipped := false
	var providerErr error
	switch lease.Kind {
	case media.JobStart:
		if !auth.Active || !auth.SourceCurrent {
			skipped = true
			break
		}
		if s.cfg.License == nil {
			providerErr = ErrUnavailable
			break
		}
		if providerErr = s.cfg.License.RequireVideo(ctx, auth.Target.License); providerErr == nil {
			providerErr = s.cfg.Provider.Start(ctx, media.StartRequest{CameraID: lease.CameraID, SourceVersion: lease.SourceVersion, StreamID: lease.StreamID})
		} else {
			providerErr = normalizeLicenseError(providerErr)
		}
	case media.JobStop:
		if auth.StopSuperseded {
			skipped = true
			break
		}
		providerErr = s.cfg.Provider.Stop(ctx, media.StopRequest{CameraID: lease.CameraID, SourceVersion: lease.SourceVersion, StreamID: lease.StreamID})
	default:
		providerErr = ErrInvalid
	}
	var returnedErr error
	if !skipped {
		result, returnedErr = jobResult(providerErr, lease.Attempts, now, lease.Kind)
	}
	if err = s.cfg.Store.FinishJob(context.WithoutCancel(ctx), lease, result, now); err != nil {
		if errors.Is(err, media.ErrLeaseLost) {
			return true, ErrLeaseLost
		}
		return true, ErrPersistenceUnknown
	}
	if returnedErr != nil {
		return true, returnedErr
	}
	return true, nil
}

func requireActor(ctx context.Context) error {
	if ctx == nil {
		return ErrForbidden
	}
	tenantID, tenantOK := domain.TenantID(ctx)
	userID, userOK := domain.TenantUserID(ctx)
	if !tenantOK || !userOK || tenantID <= 0 || userID <= 0 || domain.TenantRole(ctx) == "" {
		return ErrForbidden
	}
	return nil
}

func normalizeStoreError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, domain.ErrNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, domain.ErrForbidden) {
		return ErrForbidden
	}
	if errors.Is(err, domain.ErrConflict) {
		return media.ErrConflict
	}
	return err
}

func normalizeLicenseError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, media.ErrLicenseRequired) || errors.Is(err, domain.ErrForbidden) {
		return ErrLicenseRequired
	}
	return ErrLicenseUnavailable
}

func validateClaims(got, want media.MediaClaims, now time.Time) error {
	if got.SessionID != want.SessionID || got.TenantID != want.TenantID || got.CameraID != want.CameraID || got.UserID != want.UserID || got.SourceVersion != want.SourceVersion || got.Audience != mediaAudience || got.JTI == "" || got.ExpiresAt.IsZero() || got.IssuedAt.IsZero() || got.ExpiresAt.Sub(got.IssuedAt) > domain.MaxVideoPlaybackTTL || got.IssuedAt.After(now) || !now.Before(got.ExpiresAt) {
		return ErrUnauthorized
	}
	return nil
}

func manifestURL(base, token, sessionID string) string {
	base = strings.TrimRight(base, "/")
	return base + "/" + url.PathEscape(token) + "/" + url.PathEscape(sessionID) + "/index.m3u8"
}

func validManifestBaseURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.IsAbs() && strings.EqualFold(u.Scheme, "https") && u.Host != "" && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Opaque == ""
}

func jobResult(providerErr error, attempts int, now time.Time, kind media.JobKind) (media.JobResult, error) {
	if providerErr == nil {
		status := media.JobReady
		if kind == media.JobStop {
			status = media.JobStopped
		}
		return media.JobResult{Status: status, NextAt: now}, nil
	}
	if errors.Is(providerErr, ErrUnavailable) || errors.Is(providerErr, media.ErrDisabled) {
		return media.JobResult{Status: media.JobDisabled, ErrorCode: "provider_unavailable", NextAt: now}, providerErr
	}
	if errors.Is(providerErr, media.ErrLicenseUnavailable) {
		return media.JobResult{Status: media.JobDisabled, ErrorCode: "license_unavailable", NextAt: now}, providerErr
	}
	if errors.Is(providerErr, media.ErrLicenseRequired) {
		return media.JobResult{Status: media.JobFailed, ErrorCode: "license_required", NextAt: now}, providerErr
	}
	if errors.Is(providerErr, media.ErrUnsupported) {
		return media.JobResult{Status: media.JobFailed, ErrorCode: "unsupported_codec", NextAt: now}, providerErr
	}
	if errors.Is(providerErr, media.ErrUnknown) || errors.Is(providerErr, context.Canceled) || errors.Is(providerErr, context.DeadlineExceeded) {
		return media.JobResult{Status: media.JobUnknown, ErrorCode: "provider_unknown", NextAt: now}, providerErr
	}
	if errors.Is(providerErr, media.ErrRetryable) || attempts < maxAttempts {
		if attempts >= maxAttempts {
			return media.JobResult{Status: media.JobFailed, ErrorCode: "source_unreachable", NextAt: now}, providerErr
		}
		shift := attempts - 1
		if shift < 0 {
			shift = 0
		}
		if shift > 5 {
			shift = 5
		}
		delay := time.Second * time.Duration(1<<shift)
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
		return media.JobResult{Status: media.JobRetryable, ErrorCode: "source_unreachable", NextAt: now.Add(delay)}, providerErr
	}
	return media.JobResult{Status: media.JobFailed, ErrorCode: "source_unreachable", NextAt: now}, providerErr
}
