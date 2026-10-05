package core

import (
	"context"
	"fmt"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func (s *Service) ListOpenPonds(ctx context.Context, tenantID int64, scope domain.APIKeyResourceScope) ([]domain.Pond, error) {
	q := `SELECT p.id,p.farm_id,p.name,coalesce(p.area_mu,0),p.created_at FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE f.tenant_id=$1`
	args := []any{tenantID}
	if len(scope.FarmIDs) > 0 {
		q += ` AND f.id=ANY($2)`
		args = append(args, scope.FarmIDs)
	}
	idx := len(args) + 1
	if len(scope.PondIDs) > 0 {
		q += fmt.Sprintf(` AND p.id=ANY($%d)`, idx)
		args = append(args, scope.PondIDs)
	}
	q += ` ORDER BY p.id`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Pond
	for rows.Next() {
		var p domain.Pond
		if err := rows.Scan(&p.ID, &p.FarmID, &p.Name, &p.AreaMu, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) ListOpenDevices(ctx context.Context, tenantID int64, scope domain.APIKeyResourceScope) ([]domain.Device, error) {
	q := `SELECT d.id,d.pond_id,d.device_no,coalesce(d.name,''),coalesce(d.model,''),d.status,d.last_seen_at,d.created_at,d.disabled_at,coalesce(d.report_interval,60) FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id WHERE f.tenant_id=$1 AND d.disabled_at IS NULL`
	args := []any{tenantID}
	if len(scope.FarmIDs) > 0 {
		q += ` AND f.id=ANY($2)`
		args = append(args, scope.FarmIDs)
	}
	idx := len(args) + 1
	if len(scope.PondIDs) > 0 {
		q += fmt.Sprintf(` AND p.id=ANY($%d)`, idx)
		args = append(args, scope.PondIDs)
		idx++
	}
	if len(scope.DeviceNos) > 0 {
		q += fmt.Sprintf(` AND d.device_no=ANY($%d)`, idx)
		args = append(args, scope.DeviceNos)
	}
	q += ` ORDER BY d.id`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Device
	for rows.Next() {
		var d domain.Device
		if err := rows.Scan(&d.ID, &d.PondID, &d.DeviceNo, &d.Name, &d.Model, &d.Status, &d.LastSeenAt, &d.CreatedAt, &d.DisabledAt, &d.ReportInterval); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Service) ListOpenAlarms(ctx context.Context, tenantID int64, scope domain.APIKeyResourceScope) ([]domain.Alarm, error) {
	q := `SELECT a.id,a.device_no,a.pond_id,a.metric,a.current_value,a.threshold,a.level,coalesce(a.message,''),a.confirmed_at,a.created_at FROM alarms a JOIN ponds p ON p.id=a.pond_id JOIN farms f ON f.id=p.farm_id WHERE f.tenant_id=$1`
	args := []any{tenantID}
	if len(scope.FarmIDs) > 0 {
		q += ` AND f.id=ANY($2)`
		args = append(args, scope.FarmIDs)
	}
	idx := len(args) + 1
	if len(scope.PondIDs) > 0 {
		q += fmt.Sprintf(` AND p.id=ANY($%d)`, idx)
		args = append(args, scope.PondIDs)
		idx++
	}
	if len(scope.DeviceNos) > 0 {
		q += fmt.Sprintf(` AND a.device_no=ANY($%d)`, idx)
		args = append(args, scope.DeviceNos)
	}
	q += ` ORDER BY a.created_at DESC,a.id DESC LIMIT 200`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Alarm
	for rows.Next() {
		var a domain.Alarm
		if err := rows.Scan(&a.ID, &a.DeviceNo, &a.PondID, &a.Metric, &a.CurrentValue, &a.Threshold, &a.Level, &a.Message, &a.ConfirmedAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
