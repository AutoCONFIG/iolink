package persistence

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotOwned = errors.New("resource is not owned by tenant")

type Store struct {
	Pool *pgxpool.Pool
}

func (s Store) Within(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

var sqlIdentifier = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func TenantPredicate(alias string) (string, error) {
	if alias == "" {
		return "tenant_id = $1", nil
	}
	if !sqlIdentifier.MatchString(alias) {
		return "", fmt.Errorf("invalid SQL identifier %q", alias)
	}
	return alias + ".tenant_id = $1", nil
}

type TelemetrySnapshot struct {
	DeviceNo string
	TenantID int64
	PondID   int64
}

const telemetrySnapshotQuery = `SELECT d.device_no, s.tenant_id, d.pond_id
	FROM devices d JOIN device_shadows s ON s.device_no=d.device_no
	JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id
	WHERE d.device_no=$2
	AND f.tenant_id=$1
	AND EXISTS (SELECT 1 FROM tenant_memberships tm JOIN tenants t ON t.id=tm.tenant_id
		WHERE tm.tenant_id=f.tenant_id AND tm.tenant_id=$1 AND tm.user_id=f.owner_id AND t.active)
	AND s.tenant_id IS NOT NULL AND s.tenant_id=f.tenant_id
	AND d.disabled_at IS NULL`

func ReadTelemetrySnapshot(ctx context.Context, tx pgx.Tx, tenantID int64, deviceNo string) (TelemetrySnapshot, error) {
	var out TelemetrySnapshot
	err := tx.QueryRow(ctx, telemetrySnapshotQuery, tenantID, deviceNo).Scan(&out.DeviceNo, &out.TenantID, &out.PondID)
	return out, err
}
