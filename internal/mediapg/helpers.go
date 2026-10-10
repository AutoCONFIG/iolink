package mediapg

import (
	"context"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/media"
	"github.com/jackc/pgx/v5"
)

const sessionQuery = `SELECT vs.id::text,vs.stream_id::text,vs.tenant_id,vs.camera_id,vs.source_version,vs.user_id,vs.user_token_version,vs.tenant_permission_version,vs.member_permission_version,vs.state,coalesce(vs.error_code,''),vs.created_at,vs.expires_at
 FROM video_sessions vs
 JOIN tenants t ON t.id=vs.tenant_id AND t.active AND t.permission_version=vs.tenant_permission_version
 JOIN users u ON u.id=vs.user_id AND u.authority='USER' AND u.token_version=vs.user_token_version
 JOIN video_cameras c ON c.id=vs.camera_id AND c.tenant_id=vs.tenant_id AND c.source_version=vs.source_version
 JOIN farms f ON f.id=c.farm_id AND f.tenant_id=c.tenant_id
 JOIN tenant_memberships tm ON tm.tenant_id=vs.tenant_id AND tm.user_id=vs.user_id AND tm.active AND (tm.expires_at IS NULL OR tm.expires_at>clock_timestamp()) AND tm.permission_version=vs.member_permission_version
 WHERE vs.id=$1 AND (tm.role IN ('owner','admin') OR f.owner_id=vs.user_id OR EXISTS (SELECT 1 FROM farm_memberships fm WHERE fm.farm_id=f.id AND fm.tenant_id=f.tenant_id AND fm.user_id=vs.user_id AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>clock_timestamp())))`

func sessionArgs(s *media.Session) []any {
	return []any{&s.ID, &s.StreamID, &s.TenantID, &s.CameraID, &s.SourceVersion, &s.UserID, &s.UserTokenVersion, &s.TenantPermissionVersion, &s.MemberPermissionVersion, &s.State, &s.ErrorCode, &s.CreatedAt, &s.ExpiresAt}
}

func actorContext(ctx context.Context) (int64, int64, string, bool) {
	if ctx == nil {
		return 0, 0, "", false
	}
	tenant, tenantOK := domain.TenantID(ctx)
	user, userOK := domain.TenantUserID(ctx)
	role := domain.TenantRole(ctx)
	return tenant, user, role, tenantOK && userOK && tenant > 0 && user > 0 && role != ""
}

func loadTarget(ctx context.Context, tx pgx.Tx, tenantID, userID int64, role string, cameraID int64) (media.PlaybackTarget, error) {
	var target media.PlaybackTarget
	err := tx.QueryRow(ctx, `SELECT c.tenant_id,c.id,c.source_version,u.token_version,t.permission_version,tm.permission_version
 FROM video_cameras c JOIN farms f ON f.id=c.farm_id AND f.tenant_id=c.tenant_id
 JOIN tenants t ON t.id=c.tenant_id
 JOIN tenant_memberships tm ON tm.tenant_id=t.id AND tm.user_id=$2 AND tm.role=$3 AND tm.active AND (tm.expires_at IS NULL OR tm.expires_at>clock_timestamp())
 JOIN users u ON u.id=$2
 WHERE c.id=$1 AND c.tenant_id=$4 AND c.enabled AND t.active AND u.authority='USER'
 AND ($3 IN ('owner','admin') OR f.owner_id=$2 OR EXISTS (SELECT 1 FROM farm_memberships fm WHERE fm.farm_id=f.id AND fm.tenant_id=f.tenant_id AND fm.user_id=$2 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>clock_timestamp())))`, cameraID, userID, role, tenantID).Scan(&target.TenantID, &target.CameraID, &target.SourceVersion, &target.UserTokenVersion, &target.TenantPermissionVersion, &target.MemberPermissionVersion)
	if err != nil {
		return media.PlaybackTarget{}, err
	}
	target.UserID = userID
	rows, err := tx.Query(ctx, `SELECT id::text FROM video_streams WHERE camera_id=$1 AND tenant_id=$2 AND source_version=$3`, cameraID, tenantID, target.SourceVersion)
	if err != nil {
		return media.PlaybackTarget{}, err
	}
	if rows.Next() {
		if err = rows.Scan(&target.StreamID); err != nil {
			rows.Close()
			return media.PlaybackTarget{}, err
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return media.PlaybackTarget{}, err
	}
	rows.Close()
	return target, nil
}

func loadLicense(ctx context.Context, tx pgx.Tx, out *media.LicenseSnapshot) error {
	if err := tx.QueryRow(ctx, `SELECT d.deployment_id,ls.payload,ls.signature,coalesce(ls.payload_sha256,''),coalesce(lc.max_seen_at,'epoch'::timestamptz),lc.clock_error,clock_timestamp()
	 FROM deployment_config d CROSS JOIN license_state ls CROSS JOIN license_clock lc WHERE d.singleton AND ls.singleton AND lc.singleton FOR SHARE`).Scan(&out.DeploymentID, &out.Payload, &out.Signature, &out.SHA256, &out.MaxSeenAt, &out.ClockError, &out.Now); err != nil {
		return err
	}
	if out.DeploymentID == "" || len(out.Payload) == 0 || len(out.Signature) == 0 || len(out.SHA256) != 64 || out.ClockError {
		return media.ErrLicenseUnavailable
	}
	return nil
}

func ensureStream(ctx context.Context, tx pgx.Tx, in media.CreateSession) (string, error) {
	streamID := in.Target.StreamID
	if !validUUID(streamID) {
		streamID = in.Session.StreamID
	}
	if !validUUID(streamID) {
		return "", media.ErrInvalid
	}
	if _, err := tx.Exec(ctx, `INSERT INTO video_streams(id,camera_id,tenant_id,source_version,state) VALUES($1,$2,$3,$4,'pending') ON CONFLICT(camera_id,source_version) DO NOTHING`, streamID, in.Session.CameraID, in.Session.TenantID, in.Session.SourceVersion); err != nil {
		return "", err
	}
	var got, state string
	if err := tx.QueryRow(ctx, `SELECT id::text,state FROM video_streams WHERE camera_id=$1 AND tenant_id=$2 AND source_version=$3 FOR UPDATE`, in.Session.CameraID, in.Session.TenantID, in.Session.SourceVersion).Scan(&got, &state); err != nil {
		return "", err
	}
	if state == "stopping" {
		return "", media.ErrConflict
	}
	return got, nil
}
