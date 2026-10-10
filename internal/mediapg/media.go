package mediapg

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/media"
	"github.com/jackc/pgx/v5"
)

func (s *Store) AuthorizeMedia(ctx context.Context, claims media.MediaClaims, tokenHash [32]byte, now time.Time) (media.MediaAccess, error) {
	if s == nil || s.pool == nil || !validClaims(claims) || now.IsZero() || zeroHash(tokenHash) || claims.IssuedAt.After(now) || claims.ExpiresAt.Sub(claims.IssuedAt) > domain.MaxVideoPlaybackTTL {
		return media.MediaAccess{}, media.ErrUnauthorized
	}
	var out media.MediaAccess
	var state string
	var storedHash []byte
	var userID, cameraVersion int64
	var cameraEnabled bool
	err := s.pool.QueryRow(ctx, `SELECT vs.id::text,vs.stream_id::text,vs.tenant_id,vs.camera_id,vs.source_version,vs.state,vs.expires_at,vs.token_hash,vs.user_id,c.enabled,c.source_version
 FROM video_sessions vs JOIN tenants t ON t.id=vs.tenant_id JOIN video_cameras c ON c.id=vs.camera_id AND c.tenant_id=vs.tenant_id
 WHERE vs.id=$1 AND t.active`, claims.SessionID).Scan(&out.SessionID, &out.StreamID, &out.TenantID, &out.CameraID, &out.SourceVersion, &state, &out.ExpiresAt, &storedHash, &userID, &cameraEnabled, &cameraVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.MediaAccess{}, media.ErrNotFound
	}
	if err != nil {
		return media.MediaAccess{}, media.ErrUnavailable
	}
	if out.TenantID != claims.TenantID || out.CameraID != claims.CameraID || out.SourceVersion != claims.SourceVersion || userID != claims.UserID || !cameraEnabled || cameraVersion != claims.SourceVersion || len(storedHash) != sha256.Size || !equalHash(storedHash, tokenHash[:]) {
		return media.MediaAccess{}, media.ErrUnauthorized
	}
	out.State = media.SessionState(state)
	if !now.Before(out.ExpiresAt) || out.State == media.SessionRevoked || out.State == media.SessionExpired || out.State == media.SessionFailed {
		return media.MediaAccess{}, media.ErrForbidden
	}
	if out.State != media.SessionPending && out.State != media.SessionReady {
		return media.MediaAccess{}, media.ErrForbidden
	}
	return out, nil
}

func (s *Store) CreateSegmentMappings(ctx context.Context, mappings []media.SegmentMapping) ([]media.SegmentMapping, error) {
	if s == nil || s.pool == nil || len(mappings) == 0 || len(mappings) > 4096 {
		return nil, media.ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, media.ErrUnavailable
	}
	defer tx.Rollback(context.Background())
	result := make([]media.SegmentMapping, 0, len(mappings))
	for _, mapping := range mappings {
		if !validMapping(mapping) {
			return nil, media.ErrInvalid
		}
		var expires time.Time
		err = tx.QueryRow(ctx, `SELECT expires_at FROM video_sessions WHERE id=$1 AND stream_id=$2 AND source_version=$3 AND state IN ('pending','ready') AND expires_at>clock_timestamp() FOR SHARE`, mapping.SessionID, mapping.StreamID, mapping.SourceVersion).Scan(&expires)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, media.ErrNotFound
		}
		if err != nil {
			return nil, media.ErrUnavailable
		}
		if _, err = tx.Exec(ctx, `INSERT INTO video_segments(session_id,segment_id,provider_name,expires_at) VALUES($1,$2,$3,$4)`, mapping.SessionID, mapping.ID, mapping.Path, expires); err != nil {
			return nil, mapStoreError(err)
		}
		result = append(result, mapping)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, media.ErrUnavailable
	}
	return result, nil
}

func (s *Store) ResolveSegmentMapping(ctx context.Context, sessionID, segmentID string) (media.SegmentMapping, error) {
	if s == nil || s.pool == nil || !validUUID(sessionID) || !validUUID(segmentID) {
		return media.SegmentMapping{}, media.ErrInvalid
	}
	var out media.SegmentMapping
	err := s.pool.QueryRow(ctx, `SELECT segment_id::text,session_id::text,vs.stream_id::text,vs.source_version,provider_name
 FROM video_segments sg JOIN video_sessions vs ON vs.id=sg.session_id
 WHERE sg.session_id=$1 AND sg.segment_id=$2 AND sg.expires_at>clock_timestamp()`, sessionID, segmentID).Scan(&out.ID, &out.SessionID, &out.StreamID, &out.SourceVersion, &out.Path)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.SegmentMapping{}, media.ErrNotFound
	}
	if err != nil {
		return media.SegmentMapping{}, media.ErrUnavailable
	}
	return out, nil
}
