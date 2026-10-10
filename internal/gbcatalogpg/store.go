package gbcatalogpg

import (
	"context"
	"errors"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/gbcatalog"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

var _ gbcatalog.CatalogStore = (*Store)(nil)

func (s *Store) StartRefresh(ctx context.Context, refresh gbcatalog.Refresh) (gbcatalog.Refresh, error) {
	if s == nil || s.pool == nil || !validRefresh(refresh) {
		return gbcatalog.Refresh{}, gbcatalog.ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return gbcatalog.Refresh{}, gbcatalog.ErrStore
	}
	defer tx.Rollback(context.Background())
	var devicePK, tenantID int64
	if err = tx.QueryRow(ctx, `SELECT id,tenant_id FROM video_gb_devices WHERE device_id=$1 FOR UPDATE`, refresh.DeviceID).Scan(&devicePK, &tenantID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gbcatalog.Refresh{}, gbcatalog.ErrInvalid
		}
		return gbcatalog.Refresh{}, gbcatalog.ErrStore
	}
	var state string
	var existing gbcatalog.Refresh
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `SELECT expected_count,expires_at,state FROM video_gb_catalogs WHERE device_id=$1 AND sn=$2 FOR UPDATE`, devicePK, refresh.SN).Scan(&existing.ExpectedCount, &expiresAt, &state)
	if err == nil && state == "collecting" && expiresAt.After(time.Now()) {
		existing.DeviceID = refresh.DeviceID
		existing.SN = refresh.SN
		existing.Deadline = expiresAt
		if err = tx.Commit(ctx); err != nil {
			return gbcatalog.Refresh{}, gbcatalog.ErrStore
		}
		return existing, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return gbcatalog.Refresh{}, gbcatalog.ErrStore
	}
	if err == nil {
		if state == "collecting" {
			if _, err = tx.Exec(ctx, `UPDATE video_gb_catalogs SET state='failed',failed_at=clock_timestamp() WHERE device_id=$1 AND sn=$2 AND state='collecting'`, devicePK, refresh.SN); err != nil {
				return gbcatalog.Refresh{}, gbcatalog.ErrStore
			}
			if err = tx.Commit(ctx); err != nil {
				return gbcatalog.Refresh{}, gbcatalog.ErrStore
			}
			return gbcatalog.Refresh{}, gbcatalog.ErrDeadline
		}
		return gbcatalog.Refresh{}, gbcatalog.ErrInvalid
	}
	if _, err = tx.Exec(ctx, `INSERT INTO video_gb_catalogs(id,device_id,tenant_id,sn,expected_count,expires_at,state) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,'collecting')`, devicePK, tenantID, refresh.SN, refresh.ExpectedCount, refresh.Deadline); err != nil {
		return gbcatalog.Refresh{}, gbcatalog.ErrStore
	}
	if _, err = tx.Exec(ctx, `UPDATE video_gb_devices SET catalog_state='refreshing' WHERE id=$1`, devicePK); err != nil {
		return gbcatalog.Refresh{}, gbcatalog.ErrStore
	}
	if err = tx.Commit(ctx); err != nil {
		return gbcatalog.Refresh{}, gbcatalog.ErrStore
	}
	return refresh, nil
}

func (s *Store) ReplaceCatalog(ctx context.Context, snapshot gbcatalog.Snapshot) error {
	if s == nil || s.pool == nil || !validSnapshot(snapshot) {
		return gbcatalog.ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return gbcatalog.ErrStore
	}
	defer tx.Rollback(context.Background())
	var catalogID uuid.UUID
	var devicePK, tenantID int64
	var expected int
	var deadline time.Time
	if err = tx.QueryRow(ctx, `SELECT c.id,c.device_id,c.tenant_id,c.expected_count,c.expires_at FROM video_gb_catalogs c JOIN video_gb_devices d ON d.id=c.device_id WHERE d.device_id=$1 AND c.sn=$2 AND c.state='collecting' FOR UPDATE OF c,d`, snapshot.DeviceID, snapshot.SN).Scan(&catalogID, &devicePK, &tenantID, &expected, &deadline); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gbcatalog.ErrInvalid
		}
		return gbcatalog.ErrStore
	}
	if expected != len(snapshot.Items) {
		return gbcatalog.ErrInvalid
	}
	if !snapshot.ObservedAt.Before(deadline) {
		return gbcatalog.ErrDeadline
	}
	if _, err = tx.Exec(ctx, `DELETE FROM video_gb_catalog_items WHERE catalog_id=$1`, catalogID); err != nil {
		return gbcatalog.ErrStore
	}
	for _, item := range snapshot.Items {
		if _, err = tx.Exec(ctx, `INSERT INTO video_gb_catalog_items(catalog_id,channel_id,name,online) VALUES($1,$2,$3,$4)`, catalogID, item.ChannelID, item.Name, item.Online); err != nil {
			return gbcatalog.ErrStore
		}
		if _, err = tx.Exec(ctx, `INSERT INTO video_gb_channels(device_id,tenant_id,channel_id,name,online,present,catalog_sn) VALUES($1,$2,$3,$4,$5,true,$6) ON CONFLICT (device_id,channel_id) DO UPDATE SET tenant_id=EXCLUDED.tenant_id,name=EXCLUDED.name,online=EXCLUDED.online,present=true,catalog_sn=EXCLUDED.catalog_sn`, devicePK, tenantID, item.ChannelID, item.Name, item.Online, snapshot.SN); err != nil {
			return gbcatalog.ErrStore
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE video_gb_channels SET present=false,online=false WHERE device_id=$1 AND channel_id NOT IN (SELECT channel_id FROM video_gb_catalog_items WHERE catalog_id=$2)`, devicePK, catalogID); err != nil {
		return gbcatalog.ErrStore
	}
	if _, err = tx.Exec(ctx, `UPDATE video_gb_catalogs SET state='complete',received_count=$2,received_pages=GREATEST(received_pages,1),last_page_at=$3,completed_at=$3,failed_at=NULL WHERE id=$1`, catalogID, len(snapshot.Items), snapshot.ObservedAt); err != nil {
		return gbcatalog.ErrStore
	}
	if _, err = tx.Exec(ctx, `UPDATE video_gb_devices SET catalog_state='ready' WHERE id=$1`, devicePK); err != nil {
		return gbcatalog.ErrStore
	}
	if err = tx.Commit(ctx); err != nil {
		return gbcatalog.ErrStore
	}
	return nil
}

func (s *Store) MarkStale(ctx context.Context, deviceID string, at time.Time) error {
	if s == nil || s.pool == nil || !validDeviceID(deviceID) || at.IsZero() {
		return gbcatalog.ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return gbcatalog.ErrStore
	}
	defer tx.Rollback(context.Background())
	var devicePK int64
	if err = tx.QueryRow(ctx, `SELECT id FROM video_gb_devices WHERE device_id=$1 FOR UPDATE`, deviceID).Scan(&devicePK); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gbcatalog.ErrInvalid
		}
		return gbcatalog.ErrStore
	}
	if _, err = tx.Exec(ctx, `UPDATE video_gb_catalogs SET state='failed',failed_at=$2 WHERE device_id=$1 AND state='collecting' AND expires_at<=$2`, devicePK, at); err != nil {
		return gbcatalog.ErrStore
	}
	if _, err = tx.Exec(ctx, `UPDATE video_gb_devices SET catalog_state='stale' WHERE id=$1 AND EXISTS (SELECT 1 FROM video_gb_catalogs WHERE device_id=$1 AND state='failed' AND failed_at=$2)`, devicePK, at); err != nil {
		return gbcatalog.ErrStore
	}
	if err = tx.Commit(ctx); err != nil {
		return gbcatalog.ErrStore
	}
	return nil
}

func validRefresh(value gbcatalog.Refresh) bool {
	now := time.Now()
	return validDeviceID(value.DeviceID) && value.SN > 0 && value.ExpectedCount >= 0 && value.ExpectedCount <= gbcatalog.MaxItems && !value.Deadline.IsZero() && value.Deadline.After(now) && value.Deadline.Sub(now) <= gbcatalog.RefreshTimeout+time.Second
}

func validSnapshot(value gbcatalog.Snapshot) bool {
	if !validDeviceID(value.DeviceID) || value.SN <= 0 || value.ObservedAt.IsZero() || len(value.Items) > gbcatalog.MaxItems {
		return false
	}
	seen := make(map[string]struct{}, len(value.Items))
	for _, item := range value.Items {
		if !validDeviceID(item.ChannelID) || len(item.Name) == 0 || len(item.Name) > gbcatalog.MaxNameBytes {
			return false
		}
		if _, ok := seen[item.ChannelID]; ok {
			return false
		}
		seen[item.ChannelID] = struct{}{}
	}
	return true
}

func validDeviceID(value string) bool {
	if len(value) != 20 {
		return false
	}
	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}
