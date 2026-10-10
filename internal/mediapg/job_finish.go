package mediapg

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/media"
	"github.com/jackc/pgx/v5"
)

func (s *Store) FinishJob(ctx context.Context, lease media.Lease, result media.JobResult, now time.Time) error {
	if s == nil || s.pool == nil || !validLease(lease) || now.IsZero() || !validJobResult(result) {
		return media.ErrInvalid
	}
	id, _ := strconv.ParseInt(lease.ID, 10, 64)
	status := jobStatus(result.Status)
	next := result.NextAt.UTC()
	if next.IsZero() {
		next = now.UTC()
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return media.ErrPersistenceUnknown
	}
	defer tx.Rollback(context.Background())
	var payload, kind string
	var tenantID int64
	err = tx.QueryRow(ctx, `SELECT tenant_id,kind,payload::text FROM jobs
	 WHERE id=$1 AND status='running' AND lease_owner=$2 AND leased_until>clock_timestamp() FOR UPDATE`, id, lease.LeaseToken).Scan(&tenantID, &kind, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.ErrLeaseLost
	}
	if err != nil {
		return media.ErrPersistenceUnknown
	}
	job, err := decodeJob(id, kind, payload, lease.Attempts)
	if err != nil {
		return media.ErrLeaseLost
	}
	if job.Kind != lease.Kind || job.CameraID != lease.CameraID || job.SourceVersion != lease.SourceVersion || job.StreamID != lease.StreamID {
		return media.ErrLeaseLost
	}
	var currentSourceVersion int64
	if err = tx.QueryRow(ctx, `SELECT source_version FROM video_cameras WHERE id=$1 AND tenant_id=$2 FOR UPDATE`, job.CameraID, tenantID).Scan(&currentSourceVersion); errors.Is(err, pgx.ErrNoRows) {
		return media.ErrLeaseLost
	} else if err != nil {
		return media.ErrPersistenceUnknown
	}
	if currentSourceVersion != job.SourceVersion {
		return media.ErrLeaseLost
	}
	var streamCamera, streamTenant, streamSource int64
	if err = tx.QueryRow(ctx, `SELECT camera_id,tenant_id,source_version FROM video_streams WHERE id=$1 FOR UPDATE`, job.StreamID).Scan(&streamCamera, &streamTenant, &streamSource); errors.Is(err, pgx.ErrNoRows) {
		return media.ErrLeaseLost
	} else if err != nil {
		return media.ErrPersistenceUnknown
	}
	if streamCamera != job.CameraID || streamTenant != tenantID || streamSource != job.SourceVersion {
		return media.ErrLeaseLost
	}
	rows, err := tx.Query(ctx, `SELECT id::text,camera_id,tenant_id,source_version,state,expires_at>clock_timestamp() FROM video_sessions WHERE stream_id=$1 ORDER BY id FOR UPDATE`, job.StreamID)
	if err != nil {
		return media.ErrPersistenceUnknown
	}
	var sessions, pending, active int
	var payloadSessionFound bool
	for rows.Next() {
		var sessionID string
		var cameraID, sessionTenant, source int64
		var state media.SessionState
		var live bool
		if err = rows.Scan(&sessionID, &cameraID, &sessionTenant, &source, &state, &live); err != nil {
			rows.Close()
			return media.ErrPersistenceUnknown
		}
		if cameraID != job.CameraID || sessionTenant != tenantID || source != job.SourceVersion {
			rows.Close()
			return media.ErrLeaseLost
		}
		sessions++
		if sessionID == job.SessionID {
			payloadSessionFound = true
		}
		if live && state == media.SessionPending {
			pending++
		}
		if live && (state == media.SessionPending || state == media.SessionReady) {
			active++
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return media.ErrPersistenceUnknown
	}
	rows.Close()
	if sessions == 0 || !payloadSessionFound || job.Kind == media.JobStart && pending == 0 {
		return media.ErrLeaseLost
	}
	if _, err = tx.Exec(ctx, `UPDATE jobs SET status=$2,available_at=$3,leased_until=NULL,lease_owner=NULL,
	 payload=CASE WHEN $4='' THEN payload ELSE jsonb_set(payload,'{error_code}',to_jsonb($4::text),true) END
	 WHERE id=$1`, id, status, next, result.ErrorCode); err != nil {
		return media.ErrPersistenceUnknown
	}
	if err = finishStreamAndSessions(ctx, tx, job, result, active); err != nil {
		return media.ErrPersistenceUnknown
	}
	if err = tx.Commit(ctx); err != nil {
		return media.ErrPersistenceUnknown
	}
	return nil
}

func finishStreamAndSessions(ctx context.Context, tx pgx.Tx, job media.Job, result media.JobResult, active int) error {
	streamState := ""
	sessionState := ""
	switch result.Status {
	case media.JobReady:
		streamState, sessionState = "ready", "ready"
	case media.JobStopped:
		streamState = "stopped"
	case media.JobRetryable:
		streamState = "starting"
		if job.Kind == media.JobStop {
			streamState = "stopping"
		}
	case media.JobSkipped, media.JobDisabled, media.JobFailed, media.JobUnknown:
		if job.Kind == media.JobStop && result.Status == media.JobSkipped {
			streamState = "ready"
		} else {
			streamState, sessionState = "failed", "failed"
		}
	default:
		return media.ErrInvalid
	}
	if _, err := tx.Exec(ctx, `UPDATE video_streams SET state=$2,updated_at=clock_timestamp() WHERE id=$1`, job.StreamID, streamState); err != nil {
		return err
	}
	if sessionState != "" {
		_, err := tx.Exec(ctx, `UPDATE video_sessions SET state=$2,error_code=NULLIF($3,'') WHERE stream_id=$1 AND state='pending'`, job.StreamID, sessionState, sessionErrorCode(result.ErrorCode))
		return err
	}
	if job.Kind == media.JobStop && result.Status == media.JobSkipped {
		if active > 0 {
			return nil
		}
		_, err := tx.Exec(ctx, `UPDATE video_streams SET state='stopped',updated_at=clock_timestamp() WHERE id=$1`, job.StreamID)
		return err
	}
	if result.Status == media.JobStopped {
		if active > 0 {
			_, err := tx.Exec(ctx, `UPDATE video_streams SET state='ready',updated_at=clock_timestamp() WHERE id=$1`, job.StreamID)
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE video_sessions SET state='revoked',revoked_at=COALESCE(revoked_at,clock_timestamp()) WHERE stream_id=$1 AND state IN ('pending','ready')`, job.StreamID)
		return err
	}
	return nil
}

func decodeJob(id int64, kind, raw string, attempts int) (media.Job, error) {
	var payload struct {
		SessionID     string `json:"session_id"`
		CameraID      int64  `json:"camera_id"`
		SourceVersion int64  `json:"source_version"`
		StreamID      string `json:"stream_id"`
	}
	if kind != jobStart && kind != jobStop || json.Unmarshal([]byte(raw), &payload) != nil || !validUUID(payload.SessionID) || payload.CameraID <= 0 || payload.SourceVersion <= 0 || !validUUID(payload.StreamID) {
		return media.Job{}, media.ErrInvalid
	}
	jobKind := media.JobStart
	if kind == jobStop {
		jobKind = media.JobStop
	}
	return media.Job{ID: strconv.FormatInt(id, 10), Kind: jobKind, CameraID: payload.CameraID, SourceVersion: payload.SourceVersion, StreamID: payload.StreamID, SessionID: payload.SessionID, Attempts: attempts}, nil
}
