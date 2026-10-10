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

func (s *Store) ClaimJob(ctx context.Context, now time.Time, lease time.Duration, workerID string) (media.Lease, bool, error) {
	if s == nil || s.pool == nil || now.IsZero() || lease <= 0 || lease > 5*time.Minute || !validWorkerID(workerID) {
		return media.Lease{}, false, media.ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return media.Lease{}, false, media.ErrUnavailable
	}
	defer tx.Rollback(context.Background())
	var id, tenantID int64
	var kind, payloadRaw string
	var attempts int
	err = tx.QueryRow(ctx, `SELECT id,tenant_id,kind,payload::text,attempts FROM jobs
	 WHERE kind IN ('video.start','video.stop') AND ((status='scheduled' AND available_at<=clock_timestamp()) OR (status='running' AND leased_until IS NOT NULL AND leased_until<=clock_timestamp()))
	 ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &tenantID, &kind, &payloadRaw, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		if err = tx.Commit(ctx); err != nil {
			return media.Lease{}, false, media.ErrUnavailable
		}
		return media.Lease{}, false, nil
	}
	if err != nil {
		return media.Lease{}, false, media.ErrUnavailable
	}
	job, err := decodeJob(id, kind, payloadRaw, attempts)
	if err != nil {
		return media.Lease{}, false, media.ErrUnavailable
	}
	token, err := newLeaseToken()
	if err != nil {
		return media.Lease{}, false, media.ErrUnavailable
	}
	owner := workerID + ":" + token
	var until time.Time
	if err = tx.QueryRow(ctx, `UPDATE jobs SET status='running',attempts=attempts+1,leased_until=clock_timestamp()+($2::bigint * interval '1 microsecond'),lease_owner=$3 WHERE id=$1 RETURNING leased_until`, id, lease.Microseconds(), owner).Scan(&until); err != nil {
		return media.Lease{}, false, media.ErrUnavailable
	}
	if kind == jobStart {
		if _, err = tx.Exec(ctx, `UPDATE video_streams SET state='starting',updated_at=clock_timestamp() WHERE id=$1 AND state IN ('pending','failed','unknown')`, job.StreamID); err != nil {
			return media.Lease{}, false, media.ErrUnavailable
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return media.Lease{}, false, media.ErrUnavailable
	}
	job.Attempts++
	return media.Lease{Job: job, LeaseToken: owner, LeasedUntil: until}, true, nil
}

func (s *Store) RevalidateJob(ctx context.Context, lease media.Lease) (media.JobAuthorization, error) {
	if s == nil || s.pool == nil || !validLease(lease) {
		return media.JobAuthorization{}, media.ErrInvalid
	}
	id, _ := strconv.ParseInt(lease.ID, 10, 64)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return media.JobAuthorization{}, media.ErrUnavailable
	}
	defer tx.Rollback(context.Background())
	var status string
	var tenantID int64
	err = tx.QueryRow(ctx, `SELECT status,tenant_id FROM jobs WHERE id=$1 AND status='running' AND lease_owner=$2 AND leased_until>clock_timestamp() FOR SHARE`, id, lease.LeaseToken).Scan(&status, &tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.JobAuthorization{}, media.ErrLeaseLost
	}
	if err != nil {
		return media.JobAuthorization{}, media.ErrUnavailable
	}
	target := media.PlaybackTarget{TenantID: tenantID, CameraID: lease.CameraID, SourceVersion: lease.SourceVersion, StreamID: lease.StreamID}
	var enabled bool
	var sourceVersion int64
	err = tx.QueryRow(ctx, `SELECT c.enabled,c.source_version FROM video_cameras c WHERE c.id=$1 AND c.tenant_id=$2`, lease.CameraID, tenantID).Scan(&enabled, &sourceVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		if err = tx.Commit(ctx); err != nil {
			return media.JobAuthorization{}, media.ErrUnavailable
		}
		return media.JobAuthorization{Target: target, Active: false, SourceCurrent: false}, nil
	}
	if err != nil {
		return media.JobAuthorization{}, media.ErrUnavailable
	}
	if err = loadLicense(ctx, tx, &target.License); err != nil {
		return media.JobAuthorization{}, media.ErrUnavailable
	}
	if err = tx.Commit(ctx); err != nil {
		return media.JobAuthorization{}, media.ErrUnavailable
	}
	return media.JobAuthorization{Target: target, Active: enabled, SourceCurrent: sourceVersion == lease.SourceVersion}, nil
}

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
	var payload string
	err = tx.QueryRow(ctx, `UPDATE jobs SET status=$2,available_at=$3,leased_until=NULL,lease_owner=NULL,
 payload=CASE WHEN $4='' THEN payload ELSE jsonb_set(payload,'{error_code}',to_jsonb($4::text),true) END
 WHERE id=$1 AND status='running' AND lease_owner=$5 AND leased_until>clock_timestamp() RETURNING payload::text`, id, status, next, result.ErrorCode, lease.LeaseToken).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.ErrLeaseLost
	}
	if err != nil {
		return media.ErrPersistenceUnknown
	}
	job, err := decodeJob(id, kindFromLease(lease), payload, lease.Attempts)
	if err != nil {
		return media.ErrPersistenceUnknown
	}
	if err = finishStreamAndSessions(ctx, tx, job, result); err != nil {
		return media.ErrPersistenceUnknown
	}
	if err = tx.Commit(ctx); err != nil {
		return media.ErrPersistenceUnknown
	}
	return nil
}

func finishStreamAndSessions(ctx context.Context, tx pgx.Tx, job media.Job, result media.JobResult) error {
	streamState := ""
	sessionState := ""
	switch result.Status {
	case media.JobReady:
		streamState, sessionState = "ready", "ready"
	case media.JobStopped:
		streamState = "stopped"
	case media.JobRetryable:
		streamState = "starting"
	case media.JobSkipped, media.JobDisabled, media.JobFailed, media.JobUnknown:
		streamState, sessionState = "failed", "failed"
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
	if result.Status == media.JobStopped {
		_, err := tx.Exec(ctx, `UPDATE video_sessions SET state='revoked',revoked_at=COALESCE(revoked_at,clock_timestamp()) WHERE stream_id=$1 AND state IN ('pending','ready')`, job.StreamID)
		return err
	}
	return nil
}

func decodeJob(id int64, kind, raw string, attempts int) (media.Job, error) {
	var payload struct {
		CameraID      int64  `json:"camera_id"`
		SourceVersion int64  `json:"source_version"`
		StreamID      string `json:"stream_id"`
	}
	if kind != jobStart && kind != jobStop || json.Unmarshal([]byte(raw), &payload) != nil || payload.CameraID <= 0 || payload.SourceVersion <= 0 || !validUUID(payload.StreamID) {
		return media.Job{}, media.ErrInvalid
	}
	jobKind := media.JobStart
	if kind == jobStop {
		jobKind = media.JobStop
	}
	return media.Job{ID: strconv.FormatInt(id, 10), Kind: jobKind, CameraID: payload.CameraID, SourceVersion: payload.SourceVersion, StreamID: payload.StreamID, Attempts: attempts}, nil
}
