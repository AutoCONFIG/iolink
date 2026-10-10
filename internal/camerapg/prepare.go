package camerapg

import (
	"context"
	"slices"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func (t *transaction) CheckTarget(ctx context.Context, target camera.Target) error {
	_, err := t.prepare(ctx, target, false)
	return err
}

func (t *transaction) Prepare(ctx context.Context, target camera.Target) (camera.Binding, error) {
	return t.prepare(ctx, target, true)
}

func (t *transaction) prepare(ctx context.Context, target camera.Target, allocateID bool) (camera.Binding, error) {
	binding := camera.Binding{ID: target.CameraID, SourceVersion: 1, Enabled: true}
	var oldFarm int64
	if target.CameraID != 0 {
		if err := t.tx.QueryRow(ctx, `SELECT farm_id FROM video_cameras WHERE id=$1 AND tenant_id=$2`, target.CameraID, t.tenantID).Scan(&oldFarm); err != nil {
			return binding, err
		}
	}
	if target.PondID != 0 {
		if err := t.tx.QueryRow(ctx, `SELECT p.farm_id FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE p.id=$1 AND f.tenant_id=$2`, target.PondID, t.tenantID).Scan(&binding.FarmID); err != nil {
			return binding, err
		}
	} else {
		binding.FarmID = oldFarm
	}
	farms := []int64{binding.FarmID}
	if oldFarm != 0 && oldFarm != binding.FarmID {
		farms = append(farms, oldFarm)
	}
	slices.Sort(farms)
	for _, farm := range farms {
		if err := t.lockFarm(ctx, farm); err != nil {
			return binding, err
		}
	}
	if target.PondID != 0 {
		var pondID int64
		if err := t.tx.QueryRow(ctx, `SELECT id FROM ponds WHERE id=$1 AND farm_id=$2 FOR SHARE`, target.PondID, binding.FarmID).Scan(&pondID); err != nil {
			return binding, err
		}
	}
	if target.CameraID != 0 {
		if err := t.tx.QueryRow(ctx, `SELECT source_version,enabled FROM video_cameras WHERE id=$1 AND tenant_id=$2 AND farm_id=$3 FOR UPDATE`, target.CameraID, t.tenantID, oldFarm).Scan(&binding.SourceVersion, &binding.Enabled); err != nil {
			return binding, err
		}
		if target.PondID != 0 {
			if !binding.Enabled {
				return binding, domain.ErrNotFound
			}
			binding.SourceVersion++
		}
	} else if allocateID {
		if err := t.tx.QueryRow(ctx, `SELECT nextval(pg_get_serial_sequence('video_cameras','id'))`).Scan(&binding.ID); err != nil {
			return binding, err
		}
	}
	return binding, nil
}

func (t *transaction) checkGBSource(ctx context.Context, source domain.GBVideoSource) error {
	var enabled, present bool
	err := t.tx.QueryRow(ctx, `SELECT enabled FROM video_gb_devices WHERE id=$1 AND tenant_id=$2 FOR SHARE`, source.DeviceID(), t.tenantID).Scan(&enabled)
	if err != nil {
		return err
	}
	if !enabled {
		return camera.ErrInvalid
	}
	err = t.tx.QueryRow(ctx, `SELECT present FROM video_gb_channels WHERE device_id=$1 AND channel_id=$2 AND tenant_id=$3 FOR SHARE`, source.DeviceID(), source.ChannelID(), t.tenantID).Scan(&present)
	if err != nil {
		return err
	}
	if !present {
		return camera.ErrInvalid
	}
	return nil
}
