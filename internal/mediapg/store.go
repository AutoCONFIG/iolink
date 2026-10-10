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
		return media.PlaybackTarget{}, media.ErrLicenseUnavailable
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
		return media.Session{}, media.ErrLicenseUnavailable
	}
	if currentLicense.DeploymentID != in.Target.License.DeploymentID || currentLicense.SHA256 != in.Target.License.SHA256 {
		return media.Session{}, media.ErrLicenseUnavailable
	}
	var existing media.Session
	var existingHash []byte
	err = tx.QueryRow(ctx, `SELECT id::text,stream_id::text,tenant_id,camera_id,source_version,user_id,user_token_version,tenant_permission_version,member_permission_version,state,coalesce(error_code,''),created_at,expires_at FROM video_sessions WHERE id=$1 FOR UPDATE`, in.Session.ID).Scan(sessionArgs(&existing)...)
	if err == nil {
		if err = tx.QueryRow(ctx, `SELECT token_hash FROM video_sessions WHERE id=$1`, in.Session.ID).Scan(&existingHash); err != nil {
			return media.Session{}, media.ErrUnavailable
		}
		if existing.StreamID != in.Session.StreamID || existing.TenantID != in.Session.TenantID || existing.CameraID != in.Session.CameraID || existing.SourceVersion != in.Session.SourceVersion || existing.UserID != in.Session.UserID || existing.UserTokenVersion != in.Session.UserTokenVersion || existing.TenantPermissionVersion != in.Session.TenantPermissionVersion || existing.MemberPermissionVersion != in.Session.MemberPermissionVersion || !existing.CreatedAt.Equal(in.Session.CreatedAt) || !existing.ExpiresAt.Equal(in.Session.ExpiresAt) || !equalHash(existingHash, in.TokenHash[:]) {
			return media.Session{}, media.ErrConflict
		}
		if err = tx.Commit(ctx); err != nil {
			return media.Session{}, media.ErrUnavailable
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return media.Session{}, media.ErrUnavailable
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
	payload := map[string]any{"session_id": in.Session.ID, "camera_id": in.Session.CameraID, "source_version": in.Session.SourceVersion, "stream_id": streamID, "action": "start"}
	jobInsert, err := tx.Exec(ctx, `INSERT INTO jobs(tenant_id,kind,idempotency_key,payload)
	VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,kind,idempotency_key) DO NOTHING`, in.Session.TenantID, jobStart, in.StartJobKey, payload)
	if err != nil {
		return media.Session{}, mapStoreError(err)
	}
	if jobInsert.RowsAffected() != 1 {
		var jobID int64
		var kind, raw, jobStatus string
		var attempts int
		if err = tx.QueryRow(ctx, `SELECT id,kind,status,payload::text,attempts FROM jobs WHERE tenant_id=$1 AND kind=$2 AND idempotency_key=$3 FOR UPDATE`, in.Session.TenantID, jobStart, in.StartJobKey).Scan(&jobID, &kind, &jobStatus, &raw, &attempts); err != nil {
			return media.Session{}, media.ErrUnavailable
		}
		job, decodeErr := decodeJob(jobID, kind, raw, attempts)
		if decodeErr != nil || job.CameraID != in.Session.CameraID || job.SourceVersion != in.Session.SourceVersion || job.StreamID != streamID {
			return media.Session{}, media.ErrConflict
		}
		if jobStatus == "failed" || jobStatus == "cancelled" {
			if _, err = tx.Exec(ctx, `UPDATE video_streams SET state='pending',updated_at=clock_timestamp() WHERE id=$1 AND state IN ('failed','unknown','stopped')`, streamID); err != nil {
				return media.Session{}, media.ErrUnavailable
			}
			_, err = tx.Exec(ctx, `INSERT INTO jobs(tenant_id,kind,idempotency_key,payload) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, in.Session.TenantID, jobStart, in.StartJobKey+":"+in.Session.ID, payload)
			if err != nil {
				return media.Session{}, media.ErrUnavailable
			}
		} else {
			updated, execErr := tx.Exec(ctx, `UPDATE video_sessions SET state='ready' WHERE id=$1 AND state='pending' AND EXISTS (SELECT 1 FROM video_streams WHERE id=$2 AND state='ready')`, in.Session.ID, streamID)
			if err = execErr; err != nil {
				return media.Session{}, media.ErrUnavailable
			}
			if updated.RowsAffected() == 0 {
				in.Session.State = media.SessionPending
			}
		}
		if jobStatus == "succeeded" {
			in.Session.State = media.SessionReady
		}
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
	err := s.pool.QueryRow(ctx, sessionQuery+` AND vs.tenant_id=$2 AND vs.user_id=$3 AND tm.role=$4`, sessionID, tenantID, userID, role).Scan(sessionArgs(&out)...)
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
	err = tx.QueryRow(ctx, sessionQuery+` AND vs.tenant_id=$2 AND vs.user_id=$3 AND tm.role=$4 FOR UPDATE`, sessionID, tenantID, userID, role).Scan(sessionArgs(&sess)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.ErrNotFound
	}
	if err != nil {
		return media.ErrUnavailable
	}
	var lockedStream string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM video_streams WHERE id=$1 FOR UPDATE`, sess.StreamID).Scan(&lockedStream); err != nil {
		return media.ErrUnavailable
	}
	rows, err := tx.Query(ctx, `SELECT id::text FROM video_sessions WHERE stream_id=$1 FOR UPDATE`, sess.StreamID)
	if err != nil {
		return media.ErrUnavailable
	}
	for rows.Next() {
		var lockedSession string
		if err = rows.Scan(&lockedSession); err != nil {
			rows.Close()
			return media.ErrUnavailable
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return media.ErrUnavailable
	}
	rows.Close()
	if sess.State == media.SessionPending || sess.State == media.SessionReady {
		if _, err = tx.Exec(ctx, `UPDATE video_sessions SET state='revoked',revoked_at=clock_timestamp() WHERE id=$1 AND state IN ('pending','ready')`, sessionID); err != nil {
			return media.ErrUnavailable
		}
		var active int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM video_sessions WHERE stream_id=$1 AND state IN ('pending','ready')`, sess.StreamID).Scan(&active); err != nil {
			return media.ErrUnavailable
		}
		if active == 0 {
			payload := map[string]any{"session_id": sess.ID, "camera_id": sess.CameraID, "source_version": sess.SourceVersion, "stream_id": sess.StreamID, "action": "stop"}
			if _, err = tx.Exec(ctx, `INSERT INTO jobs(tenant_id,kind,idempotency_key,payload)
 VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,kind,idempotency_key) DO NOTHING`, sess.TenantID, jobStop, sess.StreamID, payload); err != nil {
				return media.ErrUnavailable
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return media.ErrUnavailable
	}
	return nil
}
