package camerapg

import (
	"context"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"github.com/jackc/pgx/v5"
)

const cameraColumns = `c.id,c.pond_id,c.name,c.source_kind,c.source_version,c.gb_device_id,c.gb_channel_id,c.status,c.error_code`

func scanCamera(row pgx.Row) (camera.Camera, error) {
	var c camera.Camera
	err := row.Scan(&c.ID, &c.PondID, &c.Name, &c.SourceKind, &c.SourceVersion, &c.GBDeviceID, &c.GBChannelID, &c.Status, &c.ErrorCode)
	return c, err
}

func (t *transaction) List(ctx context.Context, page camera.Page) (camera.List, error) {
	result := camera.List{Items: []camera.Camera{}}
	rows, err := t.tx.Query(ctx, `SELECT `+cameraColumns+` FROM video_cameras c JOIN farms f ON f.id=c.farm_id AND f.tenant_id=c.tenant_id
 WHERE c.tenant_id=$1 AND c.enabled AND `+farmScope+` AND c.id>$5 ORDER BY c.id LIMIT $6`, t.tenantID, t.actorID, t.role, t.authority, page.AfterID, page.Limit+1)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanCamera(rows)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, c)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	if len(result.Items) > page.Limit {
		result.Items = result.Items[:page.Limit]
		id := result.Items[len(result.Items)-1].ID
		result.NextAfterID = &id
	}
	return result, nil
}

func (t *transaction) Read(ctx context.Context, id int64) (camera.Camera, error) {
	var farmID int64
	if err := t.tx.QueryRow(ctx, `SELECT farm_id FROM video_cameras WHERE tenant_id=$1 AND id=$2 AND enabled`, t.tenantID, id).Scan(&farmID); err != nil {
		return camera.Camera{}, err
	}
	if err := t.lockFarm(ctx, farmID); err != nil {
		return camera.Camera{}, err
	}
	return scanCamera(t.tx.QueryRow(ctx, `SELECT `+cameraColumns+` FROM video_cameras c WHERE c.tenant_id=$1 AND c.id=$2 AND c.farm_id=$3 AND c.enabled FOR SHARE`, t.tenantID, id, farmID))
}
