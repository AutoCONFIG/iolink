package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/event"
)

// ---- domain.Repository implementations over pgx ----
// appapi sees only the interfaces; swap to gRPC here when modules split.

type pondRepo struct{ pool *pgxpool.Pool }

func (r *pondRepo) ListByUser(ctx context.Context, userID int64) ([]domain.Pond, error) {
	q := `SELECT p.id, p.farm_id, p.name, coalesce(p.area_mu,0), p.created_at
		FROM ponds p
		JOIN farms f ON f.id = p.farm_id
		WHERE (f.owner_id=$1 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=f.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=f.id AND fm.user_id=$1 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=f.tenant_id))`
	args := []any{userID}
	if tenantID, ok := domain.TenantID(ctx); ok {
		q += ` AND f.tenant_id = $2`
		args = append(args, tenantID)
	}
	q += ` ORDER BY p.id`
	rows, err := r.pool.Query(ctx, q, args...)
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

func (r *pondRepo) Get(ctx context.Context, id int64) (domain.Pond, error) {
	var p domain.Pond
	err := r.pool.QueryRow(ctx,
		`SELECT id, farm_id, name, coalesce(area_mu,0), created_at FROM ponds WHERE id=$1`, id).
		Scan(&p.ID, &p.FarmID, &p.Name, &p.AreaMu, &p.CreatedAt)
	return p, err
}

func (r *pondRepo) GetByUser(ctx context.Context, id, userID int64) (domain.Pond, error) {
	var p domain.Pond
	q := `SELECT p.id,p.farm_id,p.name,coalesce(p.area_mu,0),p.created_at FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE p.id=$1 AND (f.owner_id=$2 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=f.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=f.id AND fm.user_id=$2 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=f.tenant_id))`
	args := []any{id, userID}
	if tenantID, ok := domain.TenantID(ctx); ok {
		q += ` AND f.tenant_id=$3`
		args = append(args, tenantID)
	}
	err := r.pool.QueryRow(ctx, q, args...).Scan(&p.ID, &p.FarmID, &p.Name, &p.AreaMu, &p.CreatedAt)
	return p, err
}

type deviceRepo struct{ pool *pgxpool.Pool }

func (r *deviceRepo) ListByPond(ctx context.Context, pondID int64) ([]domain.Device, error) {
	const q = `SELECT id, pond_id, device_no, coalesce(name,''), coalesce(model,''), status, last_seen_at, created_at, disabled_at, coalesce(report_interval,60)
		FROM devices WHERE pond_id=$1 AND disabled_at IS NULL ORDER BY id`
	rows, err := r.pool.Query(ctx, q, pondID)
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

func (r *deviceRepo) GetByDeviceNo(ctx context.Context, no string) (domain.Device, error) {
	var d domain.Device
	err := r.pool.QueryRow(ctx,
		`SELECT id, pond_id, device_no, coalesce(name,''), coalesce(model,''), status, last_seen_at, created_at,disabled_at,coalesce(report_interval,$2)
		 FROM devices WHERE device_no=$1`, no, int(60)).
		Scan(&d.ID, &d.PondID, &d.DeviceNo, &d.Name, &d.Model, &d.Status, &d.LastSeenAt, &d.CreatedAt, &d.DisabledAt, &d.ReportInterval)
	return d, err
}

func (r *deviceRepo) GetByDeviceNoForUser(ctx context.Context, no string, userID int64) (domain.Device, error) {
	var d domain.Device
	q := `SELECT d.id,d.pond_id,d.device_no,coalesce(d.name,''),coalesce(d.model,''),d.status,d.last_seen_at,d.created_at,d.disabled_at,coalesce(d.report_interval,60) FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id WHERE d.device_no=$1 AND (f.owner_id=$2 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=f.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=f.id AND fm.user_id=$2 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=f.tenant_id)) AND d.disabled_at IS NULL`
	args := []any{no, userID}
	if tenantID, ok := domain.TenantID(ctx); ok {
		q += ` AND f.tenant_id=$3`
		args = append(args, tenantID)
	}
	err := r.pool.QueryRow(ctx, q, args...).Scan(&d.ID, &d.PondID, &d.DeviceNo, &d.Name, &d.Model, &d.Status, &d.LastSeenAt, &d.CreatedAt, &d.DisabledAt, &d.ReportInterval)
	return d, err
}

func (r *deviceRepo) UpdateStatus(ctx context.Context, no string, s domain.DeviceStatus) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE devices SET status=$2, last_seen_at=now() WHERE device_no=$1`, no, string(s))
	return err
}

type telemetryRepo struct {
	pool            *pgxpool.Pool
	defaultInterval time.Duration
	policy          domain.PermissionPolicy
}

var telemetryMetricRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func (r *telemetryRepo) ModelLatestForUser(ctx context.Context, deviceNo string, userID int64) (domain.DeviceModelLatest, error) {
	result := domain.DeviceModelLatest{DeviceNo: deviceNo, Properties: map[string]json.RawMessage{}}
	var schema []byte
	var shadow []byte
	q := `SELECT d.product_id,d.model_version,m.schema,s.ts,s.last FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id JOIN product_models m ON m.product_id=d.product_id AND m.version=d.model_version AND m.published_at IS NOT NULL LEFT JOIN device_shadows s ON s.device_no=d.device_no AND s.pond_id=d.pond_id AND s.product_id=d.product_id AND s.model_version=d.model_version WHERE d.device_no=$1 AND (f.owner_id=$2 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=f.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=f.id AND fm.user_id=$2 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=f.tenant_id)) AND d.disabled_at IS NULL`
	args := []any{deviceNo, userID}
	if tenantID, ok := domain.TenantID(ctx); ok {
		q += ` AND f.tenant_id=$3`
		args = append(args, tenantID)
	}
	err := r.pool.QueryRow(ctx, q, args...).Scan(&result.ProductID, &result.ModelVersion, &schema, &result.Timestamp, &shadow)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, domain.ErrNotFound
	}
	if err != nil {
		return result, err
	}
	var model struct {
		Fields []domain.ModelField `json:"fields"`
	}
	if err := json.Unmarshal(schema, &model); err != nil {
		return result, err
	}
	for _, field := range model.Fields {
		if field.Readable {
			result.Fields = append(result.Fields, field)
		}
	}
	if len(shadow) > 0 {
		if err := json.Unmarshal(shadow, &result.Properties); err != nil {
			return result, err
		}
		for key := range result.Properties {
			readable := false
			for _, field := range result.Fields {
				if field.Identifier == key {
					readable = true
					break
				}
			}
			if !readable {
				delete(result.Properties, key)
			}
		}
	}
	return result, nil
}

func (r *telemetryRepo) SubmitTelemetry(ctx context.Context, deviceNo string, userID int64, ts time.Time, properties map[string]json.RawMessage) (domain.TelemetryV2Result, error) {
	var result domain.TelemetryV2Result
	result.DeviceNo = deviceNo
	result.Accepted = []string{}
	result.Rejected = []domain.TelemetryRejected{}
	if ts.IsZero() || len(properties) == 0 {
		return result, domain.ErrInvalidProductModel
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	var tenantID, pondID, productID int64
	var modelVersion int
	var builtinWater bool
	var schema []byte
	q := `SELECT f.tenant_id,d.pond_id,d.product_id,d.model_version,m.schema,
		pr.id=(SELECT wp.id FROM products wp JOIN tenants wt ON wt.id=wp.tenant_id WHERE wt.name='__iolink_system__' AND wp.name='water-quality' LIMIT 1)
		FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id JOIN tenants t ON t.id=f.tenant_id AND t.active
		JOIN product_models m ON m.product_id=d.product_id AND m.version=d.model_version JOIN products pr ON pr.id=d.product_id
		WHERE d.device_no=$1 AND (f.owner_id=$2 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=f.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=f.id AND fm.user_id=$2 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=f.tenant_id)) AND d.disabled_at IS NULL AND m.published_at IS NOT NULL`
	args := []any{deviceNo, userID}
	if scopedTenant, ok := domain.TenantID(ctx); ok {
		q += ` AND f.tenant_id=$3`
		args = append(args, scopedTenant)
	}
	q += ` FOR UPDATE OF d`
	err = tx.QueryRow(ctx, q, args...).Scan(&tenantID, &pondID, &productID, &modelVersion, &schema, &builtinWater)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, domain.ErrNotFound
	}
	if err != nil {
		return result, err
	}
	if err := r.authorizeTelemetryWrite(ctx, tx, tenantID, userID); err != nil {
		return result, err
	}
	var decoded struct {
		Fields []domain.ModelField `json:"fields"`
	}
	if err := json.Unmarshal(schema, &decoded); err != nil {
		return result, err
	}
	accepted, rejected := domain.ValidateTelemetryProperties(decoded.Fields, properties)
	result.ModelVersion, result.Rejected = modelVersion, rejected
	for identifier := range accepted {
		result.Accepted = append(result.Accepted, identifier)
	}
	sort.Strings(result.Accepted)
	if len(rejected) > 0 || len(accepted) == 0 {
		return result, nil
	}
	raw, err := json.Marshal(accepted)
	if err != nil {
		return result, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO telemetry(ts,device_no,pond_id,tenant_id,product_id,model_version,properties) VALUES($1,$2,$3,$4,$5,$6,$7)`, ts, deviceNo, pondID, tenantID, productID, modelVersion, raw); err != nil {
		return result, err
	}
	if builtinWater {
		water := func(name string) *float64 {
			value, ok := accepted[name]
			if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return nil
			}
			var number float64
			if json.Unmarshal(value, &number) != nil {
				return nil
			}
			return &number
		}
		var signal *int
		if value := water("signal"); value != nil {
			n := int(*value)
			signal = &n
		}
		if _, err = tx.Exec(ctx, `INSERT INTO sensor_data(ts,device_no,pond_id,tenant_id,temperature,dissolved_oxygen,ph,turbidity,salinity,battery,signal) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, ts, deviceNo, pondID, tenantID, water("temperature"), water("dissolved_oxygen"), water("ph"), water("turbidity"), water("salinity"), water("battery"), signal); err != nil {
			return result, err
		}
	}
	if err = genericShadowUpsert(ctx, tx, event.Event{DeviceNo: deviceNo, Ts: ts}, pondID, tenantID, productID, modelVersion, accepted); err != nil {
		return result, err
	}
	numeric := make(map[string]float64)
	for name, value := range accepted {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			continue
		}
		var number float64
		if json.Unmarshal(value, &number) == nil {
			numeric[name] = number
		}
	}
	if _, err = r.alarmForTelemetry(ctx, tx, deviceNo, pondID, productID, ts, numeric); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func (r *telemetryRepo) alarmForTelemetry(ctx context.Context, tx pgx.Tx, deviceNo string, pondID, productID int64, ts time.Time, properties map[string]float64) ([]domain.Alarm, error) {
	return (&alarmEngine{}).evaluate(ctx, tx, event.Event{DeviceNo: deviceNo, ProductID: productID, Ts: ts, Properties: properties}, pondID)
}

func (r *telemetryRepo) HistoryV2ForUser(ctx context.Context, deviceNo string, userID int64, metric string, from, to time.Time, limit int) ([]domain.TelemetryHistoryPoint, string, error) {
	if !telemetryMetricRE.MatchString(metric) || limit < 1 || limit > 10000 {
		return nil, "", domain.ErrInvalidProductModel
	}
	var visible int
	visibleQ := `SELECT 1 WHERE EXISTS (SELECT 1 FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id WHERE d.device_no=$1 AND (f.owner_id=$2 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=f.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=f.id AND fm.user_id=$2 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=f.tenant_id)) AND d.disabled_at IS NULL) OR EXISTS (SELECT 1 FROM telemetry t JOIN ponds hp ON hp.id=t.pond_id JOIN farms hf ON hf.id=hp.farm_id WHERE t.device_no=$1 AND (hf.owner_id=$2 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=hf.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=hf.id AND fm.user_id=$2 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=hf.tenant_id)) AND t.product_id IS NOT NULL)`
	visibleArgs := []any{deviceNo, userID}
	if tenantID, ok := domain.TenantID(ctx); ok {
		visibleQ = `SELECT 1 WHERE EXISTS (SELECT 1 FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id WHERE d.device_no=$1 AND (f.owner_id=$2 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=f.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=f.id AND fm.user_id=$2 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=f.tenant_id)) AND f.tenant_id=$3 AND d.disabled_at IS NULL) OR EXISTS (SELECT 1 FROM telemetry t JOIN ponds hp ON hp.id=t.pond_id JOIN farms hf ON hf.id=hp.farm_id WHERE t.device_no=$1 AND (hf.owner_id=$2 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=hf.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=hf.id AND fm.user_id=$2 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=hf.tenant_id)) AND hf.tenant_id=$3 AND t.product_id IS NOT NULL)`
		visibleArgs = append(visibleArgs, tenantID)
	}
	err := r.pool.QueryRow(ctx, visibleQ, visibleArgs...).Scan(&visible)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", domain.ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	modelQ := `SELECT m.product_id,m.version,m.schema FROM product_models m WHERE m.published_at IS NOT NULL AND EXISTS (SELECT 1 FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id WHERE d.device_no=$1 AND d.product_id=m.product_id`
	modelArgs := []any{deviceNo}
	scopedTenant, scoped := domain.TenantID(ctx)
	if scoped {
		modelQ += ` AND f.tenant_id=$2`
		modelArgs = append(modelArgs, scopedTenant)
	}
	modelQ += `) UNION SELECT DISTINCT t.product_id,m.version,m.schema FROM telemetry t JOIN ponds hp ON hp.id=t.pond_id JOIN farms hf ON hf.id=hp.farm_id JOIN product_models m ON m.product_id=t.product_id AND m.version=t.model_version WHERE t.device_no=$1 AND m.published_at IS NOT NULL`
	if scoped {
		modelQ += ` AND hf.tenant_id=$2`
	}
	modelQ += ` ORDER BY 1,2`
	rows, err := r.pool.Query(ctx, modelQ, modelArgs...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	fieldsByModel := make(map[string]domain.ModelField)
	for rows.Next() {
		var historicalProductID int64
		var version int
		var schema []byte
		if err := rows.Scan(&historicalProductID, &version, &schema); err != nil {
			return nil, "", err
		}
		var model struct {
			Fields []domain.ModelField `json:"fields"`
		}
		if err := json.Unmarshal(schema, &model); err != nil {
			return nil, "", err
		}
		for _, field := range model.Fields {
			if field.Identifier == metric && field.Readable {
				key := fmt.Sprintf("%d:%d", historicalProductID, version)
				fieldsByModel[key] = field
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(fieldsByModel) == 0 {
		return nil, "", domain.ErrInvalidProductModel
	}
	unit := ""
	for _, field := range fieldsByModel {
		if unit == "" {
			unit = field.Unit
		} else if unit != field.Unit {
			unit = ""
			break
		}
	}
	historyQ := `SELECT t.ts,t.product_id,t.model_version,t.properties->$4 FROM telemetry t JOIN ponds hp ON hp.id=t.pond_id JOIN farms hf ON hf.id=hp.farm_id WHERE t.device_no=$1 AND (hf.owner_id=$5 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=hf.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=hf.id AND fm.user_id=$5 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=hf.tenant_id)) AND t.ts >= $2 AND t.ts < $3 AND t.properties ? $4 AND EXISTS (SELECT 1 FROM product_models pm, jsonb_array_elements(pm.schema->'fields') field WHERE pm.product_id=t.product_id AND pm.version=t.model_version AND field->>'identifier'=$4 AND coalesce((field->>'readable')::boolean,false)`
	historyArgs := []any{deviceNo, from, to, metric, userID}
	if scoped {
		historyQ += ` AND hf.tenant_id=$6`
		historyArgs = append(historyArgs, scopedTenant)
	}
	limitArg := len(historyArgs) + 1
	historyQ += fmt.Sprintf(`) ORDER BY t.ts LIMIT $%d`, limitArg)
	historyArgs = append(historyArgs, limit)
	rows, err = r.pool.Query(ctx, historyQ, historyArgs...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	points := make([]domain.TelemetryHistoryPoint, 0)
	for rows.Next() {
		var point domain.TelemetryHistoryPoint
		var productID int64
		var raw []byte
		if err := rows.Scan(&point.Timestamp, &productID, &point.ModelVersion, &raw); err != nil {
			return nil, "", err
		}
		if err := json.Unmarshal(raw, &point.Value); err != nil {
			return nil, "", fmt.Errorf("decode telemetry history value: %w", err)
		}
		if field, ok := fieldsByModel[fmt.Sprintf("%d:%d", productID, point.ModelVersion)]; ok {
			point.Unit = field.Unit
		}
		points = append(points, point)
	}
	return points, unit, rows.Err()
}

// Latest reads the device shadow (kept fresh by every report) instead of
// scanning the wide time-series table.
func (r *telemetryRepo) Latest(ctx context.Context, no string) (domain.Reading, error) {
	var water bool
	q := `SELECT p.name='water-quality' AND t.name='__iolink_system__' FROM devices d JOIN products p ON p.id=d.product_id JOIN tenants t ON t.id=p.tenant_id JOIN ponds po ON po.id=d.pond_id JOIN farms f ON f.id=po.farm_id WHERE d.device_no=$1 AND t.active AND EXISTS (SELECT 1 FROM tenants ft WHERE ft.id=f.tenant_id AND ft.active)`
	args := []any{no}
	if tenantID, ok := domain.TenantID(ctx); ok {
		q += ` AND f.tenant_id=$2`
		args = append(args, tenantID)
	}
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&water); err != nil {
		return domain.Reading{}, err
	}
	if !water {
		return domain.Reading{}, domain.ErrInvalidProductModel
	}
	var raw []byte
	var ts time.Time
	var stamps []byte
	var pond int64
	var interval int
	shadowQ := `SELECT s.last,s.ts,s.timestamps,s.pond_id,coalesce(d.report_interval,$2) FROM device_shadows s JOIN devices d ON d.device_no=s.device_no AND d.pond_id=s.pond_id JOIN ponds po ON po.id=d.pond_id JOIN farms f ON f.id=po.farm_id JOIN tenants t ON t.id=f.tenant_id WHERE s.device_no=$1 AND d.disabled_at IS NULL AND t.active`
	shadowArgs := []any{no, int(r.defaultInterval.Seconds())}
	if tenantID, ok := domain.TenantID(ctx); ok {
		shadowQ += ` AND f.tenant_id=$3`
		shadowArgs = append(shadowArgs, tenantID)
	}
	err := r.pool.QueryRow(ctx, shadowQ, shadowArgs...).
		Scan(&raw, &ts, &stamps, &pond, &interval)
	if err != nil {
		return domain.Reading{}, err
	}
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		return domain.Reading{}, err
	}
	m := make(map[string]float64, len(generic))
	for key, value := range generic {
		var number float64
		if err := json.Unmarshal(value, &number); err == nil && !math.IsNaN(number) && !math.IsInf(number, 0) {
			m[key] = number
		}
	}
	var timestamps map[string]time.Time
	if err := json.Unmarshal(stamps, &timestamps); err != nil {
		return domain.Reading{}, err
	}
	var signal *int
	if v, ok := m["signal"]; ok {
		x := int(v)
		signal = &x
	}
	ptr := func(k string) *float64 {
		if v, ok := m[k]; ok {
			return &v
		}
		return nil
	}
	return domain.Reading{
		DeviceNo: no,
		PondID:   pond, Timestamps: timestamps, ReportInterval: interval, Battery: ptr("battery"), Signal: signal,
		Timestamp:   ts,
		Temperature: ptr("temperature"),
		DO:          ptr("dissolved_oxygen"),
		PH:          ptr("ph"),
		Turbidity:   ptr("turbidity"),
		Salinity:    ptr("salinity"),
	}, nil
}

func (r *telemetryRepo) History(ctx context.Context, no, metric string, from, to time.Time, maxPoints int) ([]domain.MetricPoint, error) {
	col, ok := domain.MetricColumns[metric]
	if !ok {
		return nil, domain.ErrUnknownMetric
	}
	if maxPoints < 1 || maxPoints > 200 || !to.After(from) {
		return nil, domain.ErrInvalidRange
	}
	// Origin at from avoids an extra bucket at either endpoint; [from,to).
	width := int64(math.Ceil(float64(to.Sub(from).Microseconds()) / float64(maxPoints)))
	if width < 1 {
		width = 1
	}
	bucket := fmt.Sprintf("%d microseconds", width)
	q := `SELECT date_bin($4::interval,ts,$2::timestamptz),avg(` + col + `) FROM sensor_data sd JOIN ponds p ON p.id=sd.pond_id JOIN farms f ON f.id=p.farm_id JOIN tenants t ON t.id=f.tenant_id WHERE sd.device_no=$1 AND sd.pond_id=(SELECT pond_id FROM devices WHERE device_no=$1) AND ts >= $2 AND ts < $3 AND ` + col + ` IS NOT NULL AND t.active`
	args := []any{no, from, to, bucket, maxPoints}
	if tenantID, ok := domain.TenantID(ctx); ok {
		q += ` AND f.tenant_id=$6`
		args = append(args, tenantID)
	}
	q += ` GROUP BY 1 ORDER BY 1 LIMIT $5`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.MetricPoint{}
	for rows.Next() {
		var p domain.MetricPoint
		if err := rows.Scan(&p.Ts, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *telemetryRepo) HistoryForUser(ctx context.Context, no string, userID int64, metric string, from, to time.Time, maxPoints int) ([]domain.MetricPoint, error) {
	var water bool
	if err := r.pool.QueryRow(ctx, `SELECT p.name='water-quality' AND t.name='__iolink_system__' FROM devices d JOIN products p ON p.id=d.product_id JOIN tenants t ON t.id=p.tenant_id WHERE d.device_no=$1`, no).Scan(&water); err != nil {
		return nil, err
	}
	if !water {
		return nil, domain.ErrInvalidProductModel
	}
	col, ok := domain.MetricColumns[metric]
	if !ok {
		return nil, domain.ErrUnknownMetric
	}
	if maxPoints < 1 || maxPoints > 200 || !to.After(from) {
		return nil, domain.ErrInvalidRange
	}
	width := int64(math.Ceil(float64(to.Sub(from).Microseconds()) / float64(maxPoints)))
	if width < 1 {
		width = 1
	}
	bucket := fmt.Sprintf("%d microseconds", width)
	q := `SELECT date_bin($5::interval,sd.ts,$3::timestamptz),avg(` + col + `)
		FROM sensor_data sd
		JOIN ponds p ON p.id=sd.pond_id
		JOIN farms f ON f.id=p.farm_id
		WHERE sd.device_no=$1 AND (f.owner_id=$2 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=f.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=f.id AND fm.user_id=$2 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=f.tenant_id)) AND sd.ts >= $3 AND sd.ts < $4 AND sd.` + col + ` IS NOT NULL
		`
	args := []any{no, userID, from, to, bucket}
	if tenantID, ok := domain.TenantID(ctx); ok {
		q += ` AND f.tenant_id=$6`
		args = append(args, tenantID)
		q += ` GROUP BY 1 ORDER BY 1 LIMIT $7`
		args = append(args, maxPoints)
	} else {
		q += ` GROUP BY 1 ORDER BY 1 LIMIT $6`
		args = append(args, maxPoints)
	}
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.MetricPoint{}
	for rows.Next() {
		var p domain.MetricPoint
		if err := rows.Scan(&p.Ts, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type alarmRepo struct{ pool *pgxpool.Pool }

func (r *alarmRepo) ListByUser(ctx context.Context, userID int64, limit int) ([]domain.Alarm, error) {
	q := `SELECT a.id, a.device_no, a.pond_id, a.metric, a.current_value, a.threshold,
		a.level, coalesce(a.message,''), a.confirmed_at, a.created_at
		FROM alarms a
		JOIN ponds p ON p.id = a.pond_id
		JOIN farms f ON f.id = p.farm_id
		WHERE (f.owner_id=$1 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=f.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=f.id AND fm.user_id=$1 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=f.tenant_id))`
	args := []any{userID}
	idx := 2
	if tenantID, ok := domain.TenantID(ctx); ok {
		q += fmt.Sprintf(` AND f.tenant_id = $%d`, idx)
		args = append(args, tenantID)
		idx++
	}
	q += `
		ORDER BY a.created_at DESC, a.id DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT $%d", idx)
		args = append(args, limit)
	}
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Alarm
	for rows.Next() {
		var al domain.Alarm
		if err := rows.Scan(&al.ID, &al.DeviceNo, &al.PondID, &al.Metric, &al.CurrentValue,
			&al.Threshold, &al.Level, &al.Message, &al.ConfirmedAt, &al.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, al)
	}
	return out, rows.Err()
}

func (r *alarmRepo) ListByUserFiltered(ctx context.Context, userID int64, level domain.AlarmLevel, onlyUnconfirmed bool, limit, offset int) ([]domain.Alarm, error) {
	q := `SELECT a.id, a.device_no, a.pond_id, a.metric, a.current_value, a.threshold,
		a.level, coalesce(a.message,''), a.confirmed_at, a.created_at
		FROM alarms a JOIN ponds p ON p.id=a.pond_id JOIN farms f ON f.id=p.farm_id
		WHERE (f.owner_id=$1 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=f.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=f.id AND fm.user_id=$1 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=f.tenant_id))`
	args := []any{userID}
	idx := 2
	if tenantID, ok := domain.TenantID(ctx); ok {
		q += fmt.Sprintf(" AND f.tenant_id=$%d", idx)
		args = append(args, tenantID)
		idx++
	}
	if level != "" {
		q += fmt.Sprintf(" AND a.level=$%d", idx)
		args = append(args, string(level))
		idx++
	}
	if onlyUnconfirmed {
		q += " AND a.confirmed_at IS NULL"
	}
	q += fmt.Sprintf(" ORDER BY a.created_at DESC, a.id DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Alarm{}
	for rows.Next() {
		var a domain.Alarm
		if err := rows.Scan(&a.ID, &a.DeviceNo, &a.PondID, &a.Metric, &a.CurrentValue, &a.Threshold, &a.Level, &a.Message, &a.ConfirmedAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *alarmRepo) Confirm(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE alarms SET confirmed_at=now() WHERE id=$1 AND confirmed_at IS NULL`, id)
	return err
}

func (r *alarmRepo) ConfirmByUser(ctx context.Context, id, userID int64) error {
	q := `UPDATE alarms a SET confirmed_at=now() FROM ponds p JOIN farms f ON f.id=p.farm_id WHERE a.id=$1 AND a.pond_id=p.id AND (f.owner_id=$2 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=f.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=f.id AND fm.user_id=$2 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=f.tenant_id)) AND a.confirmed_at IS NULL`
	args := []any{id, userID}
	if tenantID, ok := domain.TenantID(ctx); ok {
		q += ` AND f.tenant_id=$3`
		args = append(args, tenantID)
	}
	ct, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		var confirmed bool
		existsQ := `SELECT EXISTS(
			SELECT 1 FROM alarms a
			JOIN ponds p ON p.id=a.pond_id
			JOIN farms f ON f.id=p.farm_id
			WHERE a.id=$1 AND (f.owner_id=$2 AND EXISTS (SELECT 1 FROM users owner WHERE owner.id=f.owner_id AND owner.authority='USER') OR EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=f.id AND fm.user_id=$2 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()) AND fm.tenant_id=f.tenant_id)) AND a.confirmed_at IS NOT NULL
		`
		existsArgs := []any{id, userID}
		if tenantID, ok := domain.TenantID(ctx); ok {
			existsQ += ` AND f.tenant_id=$3`
			existsArgs = append(existsArgs, tenantID)
		}
		existsQ += `)`
		if err := r.pool.QueryRow(ctx, existsQ, existsArgs...).Scan(&confirmed); err != nil {
			return err
		}
		if confirmed {
			return nil
		}
		return domain.ErrNotFound
	}
	return nil
}

type alarmRuleRepo struct{ pool *pgxpool.Pool }

func (r *alarmRuleRepo) RulesForDevice(ctx context.Context, no string) ([]domain.AlarmRule, error) {
	const q = `SELECT r.id, r.pond_id, r.metric, r.min_value, r.max_value, r.level, r.enabled
		FROM alarm_rules r JOIN devices d ON d.pond_id = r.pond_id
		WHERE d.device_no=$1 AND r.enabled`
	rows, err := r.pool.Query(ctx, q, no)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AlarmRule
	for rows.Next() {
		var ar domain.AlarmRule
		if err := rows.Scan(&ar.ID, &ar.PondID, &ar.Metric, &ar.Min, &ar.Max, &ar.Level, &ar.Enabled); err != nil {
			return nil, err
		}
		out = append(out, ar)
	}
	return out, rows.Err()
}
