package core

import (
	"context"
	"fmt"
	"strings"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func (r *deviceRepo) ListPageByUser(ctx context.Context, userID int64, query domain.DeviceListQuery) (domain.DevicePage, error) {
	if query.Limit < 1 || query.Limit > 200 || query.Offset < 0 || query.PondID < 0 {
		return domain.DevicePage{}, domain.ErrInvalidRange
	}
	where := `WHERE ` + appFarmScope(ctx, "f", 1) + ` AND ($2 OR d.disabled_at IS NULL) AND ($3=0 OR d.pond_id=$3)`
	args := []any{userID, query.IncludeDisabled, query.PondID}
	if tenantID, scoped := domain.TenantID(ctx); scoped {
		where += ` AND f.tenant_id=$4`
		args = append(args, tenantID)
	}
	switch query.Status {
	case "", "all":
	case "disabled":
		where += ` AND d.disabled_at IS NOT NULL`
	case "active":
		where += ` AND d.disabled_at IS NULL`
	case "online", "offline":
		where += fmt.Sprintf(` AND d.status=$%d AND d.disabled_at IS NULL`, len(args)+1)
		args = append(args, query.Status)
	default:
		return domain.DevicePage{}, domain.ErrInvalidRange
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		index := len(args) + 1
		where += fmt.Sprintf(` AND (d.device_no ILIKE $%d OR coalesce(d.name,'') ILIKE $%d OR coalesce(d.model,'') ILIKE $%d OR p.name ILIKE $%d)`, index, index, index, index)
		args = append(args, "%"+search+"%")
	}
	base := ` FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id ` + where
	filterArgs := len(args)
	limitIndex := filterArgs + 1
	args = append(args, query.Limit, query.Offset)
	q := fmt.Sprintf(`SELECT d.id,d.pond_id,d.device_no,coalesce(d.name,''),coalesce(d.model,''),d.status,d.last_seen_at,d.created_at,d.disabled_at,coalesce(d.report_interval,60),count(*) OVER()%s ORDER BY d.id LIMIT $%d OFFSET $%d`, base, limitIndex, limitIndex+1)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return domain.DevicePage{}, err
	}
	result := domain.DevicePage{Items: []domain.Device{}}
	for rows.Next() {
		var d domain.Device
		if err := rows.Scan(&d.ID, &d.PondID, &d.DeviceNo, &d.Name, &d.Model, &d.Status, &d.LastSeenAt, &d.CreatedAt, &d.DisabledAt, &d.ReportInterval, &result.Total); err != nil {
			rows.Close()
			return domain.DevicePage{}, err
		}
		result.Items = append(result.Items, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return domain.DevicePage{}, err
	}
	if len(result.Items) == 0 {
		if err := r.pool.QueryRow(ctx, `SELECT count(*)`+base, args[:filterArgs]...).Scan(&result.Total); err != nil {
			return domain.DevicePage{}, err
		}
	}
	return result, nil
}
