package camerapg

import (
	"context"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
)

// Persistent stop intent is atomic with revocation. A future media worker must
// reconcile the stream under its lease before touching a provider.
func (t *transaction) invalidate(ctx context.Context, b camera.Binding) error {
	rows, err := t.tx.Query(ctx, `SELECT id FROM video_streams WHERE tenant_id=$1 AND camera_id=$2 ORDER BY id FOR UPDATE`, t.tenantID, b.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if _, err = t.tx.Exec(ctx, `UPDATE video_sessions SET state='revoked',revoked_at=clock_timestamp() WHERE tenant_id=$1 AND camera_id=$2 AND state IN ('pending','ready')`, t.tenantID, b.ID); err != nil {
		return err
	}
	_, err = t.tx.Exec(ctx, `INSERT INTO jobs(tenant_id,kind,idempotency_key,payload)
 SELECT tenant_id,'video.stop',id::text,jsonb_build_object('camera_id',camera_id,'source_version',source_version,'stream_id',id,'action','stop')
 FROM video_streams WHERE tenant_id=$1 AND camera_id=$2 AND state<>'stopped'
 ON CONFLICT(tenant_id,kind,idempotency_key) DO NOTHING`, t.tenantID, b.ID)
	return err
}
