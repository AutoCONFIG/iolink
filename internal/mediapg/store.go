package mediapg

import (
	"context"
	"errors"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/media"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxWorkerID = 128
	maxJobKey   = 256
	jobStart    = "video.start"
	jobStop     = "video.stop"
)

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

var _ media.Store = (*Store)(nil)
var _ media.SegmentMappingStore = (*Store)(nil)

func (s *Store) PreparePlayback(ctx context.Context, cameraID int64) (media.PlaybackTarget, error) {
	if s == nil || s.pool == nil || cameraID <= 0 {
		return media.PlaybackTarget{}, media.ErrInvalid
	}
	tenantID, userID, role, ok := actorContext(ctx)
	if !ok {
		return media.PlaybackTarget{}, media.ErrForbidden
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return media.PlaybackTarget{}, media.ErrUnavailable
	}
	defer tx.Rollback(context.Background())
	target, err := loadTarget(ctx, tx, tenantID, userID, role, cameraID)
	if err != nil {
		return media.PlaybackTarget{}, mapStoreError(err)
	}
	if target.StreamID == "" {
		target.StreamID = uuid.NewString()
	}
	if err = loadLicense(ctx, tx, &target.License); err != nil {
		return media.PlaybackTarget{}, media.ErrUnavailable
	}
	if err = tx.Commit(ctx); err != nil {
		return media.PlaybackTarget{}, media.ErrUnavailable
	}
	return target, nil
}

func (s *Store) CreatePlayback(ctx context.Context, in media.CreateSession) (media.Session, error) {
	if s == nil || s.pool == nil || !validCreate(in) {
		return media.Session{}, media.ErrInvalid
	}
	tenantID, userID, role, ok := actorContext(ctx)
	if !ok {
		return media.Session{}, media.ErrForbidden
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return media.Session{}, media.ErrUnavailable
	}
	defer tx.Rollback(context.Background())
	authoritative, err := loadTarget(ctx, tx, tenantID, userID, role, in.Session.CameraID)
	if err != nil {
		return media.Session{}, mapStoreError(err)
	}
	if authoritative.SourceVersion != in.Session.SourceVersion || authoritative.TenantID != in.Session.TenantID || authoritative.UserID != in.Session.UserID || authoritative.CameraID != in.Session.CameraID || authoritative.UserTokenVersion != in.Session.UserTokenVersion || authoritative.TenantPermissionVersion != in.Session.TenantPermissionVersion || authoritative.MemberPermissionVersion != in.Session.MemberPermissionVersion {
		return media.Session{}, media.ErrConflict
	}
	var currentLicense media.LicenseSnapshot
	if err = loadLicense(ctx, tx, &currentLicense); err != nil {
		return media.Session{}, media.ErrUnavailable
	}
	if currentLicense.DeploymentID != in.Target.License.DeploymentID || currentLicense.SHA256 != in.Target.License.SHA256 {
		return media.Session{}, media.ErrLicenseUnavailable
	}
	streamID, err := ensureStream(ctx, tx, in)
	if err != nil {
		return media.Session{}, mapStoreError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO video_sessions
 (id,stream_id,camera_id,tenant_id,source_version,user_id,user_token_version,tenant_permission_version,member_permission_version,token_hash,state,created_at,expires_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'pending',$11,$12)`, in.Session.ID, streamID, in.Session.CameraID, in.Session.TenantID, in.Session.SourceVersion, in.Session.UserID, in.Session.UserTokenVersion, in.Session.TenantPermissionVersion, in.Session.MemberPermissionVersion, in.TokenHash[:], in.Session.CreatedAt, in.Session.ExpiresAt); err != nil {
		return media.Session{}, mapStoreError(err)
	}
	payload := map[string]any{"camera_id": in.Session.CameraID, "source_version": in.Session.SourceVersion, "stream_id": streamID, "action": "start"}
	jobInsert, err := tx.Exec(ctx, `INSERT INTO jobs(tenant_id,kind,idempotency_key,payload)
	VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,kind,idempotency_key) DO NOTHING`, in.Session.TenantID, jobStart, in.StartJobKey, payload)
	if err != nil {
		return media.Session{}, mapStoreError(err)
	}
	if jobInsert.RowsAffected() != 1 {
		return media.Session{}, media.ErrConflict
	}
	in.Session.StreamID = streamID
	if err = tx.Commit(ctx); err != nil {
		return media.Session{}, media.ErrUnavailable
	}
	return in.Session, nil
}

func (s *Store) ReadOwnedPlayback(ctx context.Context, sessionID string) (media.Session, error) {
	if s == nil || s.pool == nil || !validUUID(sessionID) {
		return media.Session{}, media.ErrInvalid
	}
	tenantID, userID, role, ok := actorContext(ctx)
	if !ok {
		return media.Session{}, media.ErrForbidden
	}
	var out media.Session
	err := s.pool.QueryRow(ctx, sessionQuery+` AND vs.tenant_id=$2 AND vs.user_id=$3
 AND EXISTS (SELECT 1 FROM tenants t JOIN tenant_memberships tm ON tm.tenant_id=t.id
 WHERE t.id=vs.tenant_id AND t.active AND tm.user_id=$3 AND tm.active AND (tm.expires_at IS NULL OR tm.expires_at>clock_timestamp()) AND tm.role=$4
 AND ($5::bigint IS NULL OR t.permission_version=$5))`, sessionID, tenantID, userID, role, claimedPermissionVersion(ctx)).Scan(sessionArgs(&out)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.Session{}, media.ErrNotFound
	}
	if err != nil {
		return media.Session{}, media.ErrUnavailable
	}
	return out, nil
}

func (s *Store) StopOwnedPlayback(ctx context.Context, sessionID string, now time.Time) error {
	if s == nil || s.pool == nil || !validUUID(sessionID) || now.IsZero() {
		return media.ErrInvalid
	}
	tenantID, userID, role, ok := actorContext(ctx)
	if !ok {
		return media.ErrForbidden
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return media.ErrUnavailable
	}
	defer tx.Rollback(context.Background())
	var sess media.Session
	err = tx.QueryRow(ctx, sessionQuery+` AND vs.tenant_id=$2 AND vs.user_id=$3
 AND EXISTS (SELECT 1 FROM tenants t JOIN tenant_memberships tm ON tm.tenant_id=t.id
 WHERE t.id=vs.tenant_id AND t.active AND tm.user_id=$3 AND tm.active AND (tm.expires_at IS NULL OR tm.expires_at>clock_timestamp()) AND tm.role=$4
 AND ($5::bigint IS NULL OR t.permission_version=$5)) FOR UPDATE`, sessionID, tenantID, userID, role, claimedPermissionVersion(ctx)).Scan(sessionArgs(&sess)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.ErrNotFound
	}
	if err != nil {
		return media.ErrUnavailable
	}
	if sess.State == media.SessionPending || sess.State == media.SessionReady {
		if _, err = tx.Exec(ctx, `UPDATE video_sessions SET state='revoked',revoked_at=$2 WHERE id=$1 AND state IN ('pending','ready')`, sessionID, now.UTC()); err != nil {
			return media.ErrUnavailable
		}
		payload := map[string]any{"camera_id": sess.CameraID, "source_version": sess.SourceVersion, "stream_id": sess.StreamID, "action": "stop"}
		if _, err = tx.Exec(ctx, `INSERT INTO jobs(tenant_id,kind,idempotency_key,payload)
 VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,kind,idempotency_key) DO NOTHING`, sess.TenantID, jobStop, sess.StreamID, payload); err != nil {
			return media.ErrUnavailable
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return media.ErrUnavailable
	}
	return nil
}
