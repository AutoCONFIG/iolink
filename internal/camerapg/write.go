package camerapg

import (
	"context"
	"strconv"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func (t *transaction) Write(ctx context.Context, mutation camera.Mutation) (camera.Camera, error) {
	cfg, b := mutation.Configuration, mutation.Binding
	var uri *string
	var deviceID *int64
	var channelID *string
	switch source := cfg.Source().(type) {
	case domain.RTSPVideoSource:
		value := source.URI()
		uri = &value
	case domain.GBVideoSource:
		if err := t.checkGBSource(ctx, source); err != nil {
			return camera.Camera{}, err
		}
		device, channel := source.DeviceID(), source.ChannelID()
		deviceID, channelID = &device, &channel
	default:
		return camera.Camera{}, camera.ErrInvalid
	}
	action := "camera.created"
	var result camera.Camera
	var err error
	args := []any{b.ID, t.tenantID, b.FarmID, cfg.PondID(), cfg.Name(), cfg.Source().Kind(), b.SourceVersion, uri, mutation.CredentialCipher, deviceID, channelID}
	if b.SourceVersion == 1 {
		result, err = scanCamera(t.tx.QueryRow(ctx, `INSERT INTO video_cameras AS c(id,tenant_id,farm_id,pond_id,name,source_kind,source_version,rtsp_uri,credential_cipher,gb_device_id,gb_channel_id)
  VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING `+cameraColumns, args...))
	} else {
		action = "camera.replaced"
		result, err = scanCamera(t.tx.QueryRow(ctx, `UPDATE video_cameras c SET farm_id=$3,pond_id=$4,name=$5,source_kind=$6,source_version=$7,
  rtsp_uri=$8,credential_cipher=$9,gb_device_id=$10,gb_channel_id=$11,status='offline',error_code=NULL,updated_at=clock_timestamp()
  WHERE id=$1 AND tenant_id=$2 RETURNING `+cameraColumns, args...))
		if err == nil {
			err = t.invalidate(ctx, b)
		}
	}
	if err != nil {
		return camera.Camera{}, err
	}
	if err = t.audit(ctx, b.ID, action); err != nil {
		return camera.Camera{}, err
	}
	return result, nil
}

func (t *transaction) Disable(ctx context.Context, b camera.Binding) error {
	if !b.Enabled {
		return t.invalidate(ctx, b)
	}
	if _, err := t.tx.Exec(ctx, `UPDATE video_cameras SET enabled=FALSE,status='offline',error_code=NULL,updated_at=clock_timestamp() WHERE id=$1 AND tenant_id=$2`, b.ID, t.tenantID); err != nil {
		return err
	}
	if err := t.invalidate(ctx, b); err != nil {
		return err
	}
	return t.audit(ctx, b.ID, "camera.disabled")
}

func (t *transaction) audit(ctx context.Context, id int64, action string) error {
	_, err := t.tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id) VALUES($1,$2,$3,'camera',$4)`, t.tenantID, t.actorID, action, strconv.FormatInt(id, 10))
	return err
}
