package mediapg

import (
	"context"
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
	 WHERE kind IN ('video.start','video.stop') AND attempts < 6 AND ((status='scheduled' AND available_at<=clock_timestamp()) OR (status='running' AND leased_until IS NOT NULL AND leased_until<=clock_timestamp()))
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
	var kind string
	var tenantID int64
	err = tx.QueryRow(ctx, `SELECT kind,tenant_id FROM jobs WHERE id=$1 AND status='running' AND lease_owner=$2 AND leased_until>clock_timestamp() FOR SHARE`, id, lease.LeaseToken).Scan(&kind, &tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.JobAuthorization{}, media.ErrLeaseLost
	}
	if err != nil {
		return media.JobAuthorization{}, media.ErrUnavailable
	}
	wantKind := jobStart
	if lease.Kind == media.JobStop {
		wantKind = jobStop
	}
	if kind != wantKind {
		return media.JobAuthorization{}, media.ErrLeaseLost
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
	var userID, userTokenVersion, tenantPermissionVersion, memberPermissionVersion int64
	var currentUserTokenVersion, currentTenantPermissionVersion, currentMemberPermissionVersion int64
	var sessionState string
	err = tx.QueryRow(ctx, `SELECT s.user_id,s.user_token_version,s.tenant_permission_version,s.member_permission_version,
	 u.token_version,t.permission_version,tm.permission_version,s.state
 FROM video_sessions s
 JOIN video_cameras c ON c.id=s.camera_id AND c.tenant_id=s.tenant_id
 JOIN farms f ON f.id=c.farm_id AND f.tenant_id=c.tenant_id
 JOIN tenants t ON t.id=s.tenant_id AND t.active
 JOIN users u ON u.id=s.user_id AND u.authority='USER'
 JOIN tenant_memberships tm ON tm.tenant_id=s.tenant_id AND tm.user_id=s.user_id AND tm.active AND (tm.expires_at IS NULL OR tm.expires_at>clock_timestamp())
	 WHERE s.id=$5 AND s.stream_id=$1 AND s.camera_id=$2 AND s.tenant_id=$3 AND s.source_version=$4
 AND s.user_token_version=u.token_version AND s.tenant_permission_version=t.permission_version AND s.member_permission_version=tm.permission_version
	 AND ($6 <> 'video.start' OR (s.state IN ('pending','ready') AND s.expires_at>clock_timestamp()))
 AND (tm.role IN ('owner','admin') OR f.owner_id=s.user_id OR EXISTS (SELECT 1 FROM farm_memberships fm WHERE fm.farm_id=f.id AND fm.tenant_id=f.tenant_id AND fm.user_id=s.user_id AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>clock_timestamp())))
		 ORDER BY s.created_at DESC LIMIT 1`, lease.StreamID, lease.CameraID, tenantID, lease.SourceVersion, lease.SessionID, kind).Scan(&userID, &userTokenVersion, &tenantPermissionVersion, &memberPermissionVersion, &currentUserTokenVersion, &currentTenantPermissionVersion, &currentMemberPermissionVersion, &sessionState)
	if errors.Is(err, pgx.ErrNoRows) {
		if err = tx.Commit(ctx); err != nil {
			return media.JobAuthorization{}, media.ErrUnavailable
		}
		return media.JobAuthorization{Target: target, Active: false, SourceCurrent: sourceVersion == lease.SourceVersion}, nil
	}
	if err != nil {
		return media.JobAuthorization{}, media.ErrUnavailable
	}
	if currentUserTokenVersion != userTokenVersion || currentTenantPermissionVersion != tenantPermissionVersion || currentMemberPermissionVersion != memberPermissionVersion {
		return media.JobAuthorization{}, media.ErrLeaseLost
	}
	target.UserID = userID
	target.UserTokenVersion = userTokenVersion
	target.TenantPermissionVersion = tenantPermissionVersion
	target.MemberPermissionVersion = memberPermissionVersion
	if lease.Kind == media.JobStart && enabled && sourceVersion == lease.SourceVersion {
		if err = loadLicense(ctx, tx, &target.License); err != nil {
			return media.JobAuthorization{}, media.ErrLicenseUnavailable
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return media.JobAuthorization{}, media.ErrUnavailable
	}
	return media.JobAuthorization{Target: target, Active: enabled, SourceCurrent: sourceVersion == lease.SourceVersion}, nil
}
