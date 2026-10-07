package core

import (
	"context"
	"fmt"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/jackc/pgx/v5"
)

func validateAPIKeyResourceOwnership(ctx context.Context, tx pgx.Tx, tenantID int64, resources domain.APIKeyResourceScope) error {
	var allowed bool
	err := tx.QueryRow(ctx, `SELECT
NOT EXISTS(SELECT 1 FROM unnest($2::bigint[]) AS ids(id) WHERE NOT EXISTS(SELECT 1 FROM farms f WHERE f.id=ids.id AND f.tenant_id=$1)) AND
NOT EXISTS(SELECT 1 FROM unnest($3::bigint[]) AS ids(id) WHERE NOT EXISTS(SELECT 1 FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE p.id=ids.id AND f.tenant_id=$1)) AND
NOT EXISTS(SELECT 1 FROM unnest($4::text[]) AS ids(no) WHERE NOT EXISTS(SELECT 1 FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id WHERE d.device_no=ids.no AND f.tenant_id=$1 AND d.disabled_at IS NULL))`, tenantID, resources.FarmIDs, resources.PondIDs, resources.DeviceNos).Scan(&allowed)
	if err != nil {
		return fmt.Errorf("validate api key resource ownership: %w", err)
	}
	if !allowed {
		return domain.ErrNotFound
	}
	return nil
}
