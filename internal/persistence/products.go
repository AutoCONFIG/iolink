package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductStore struct {
	pool   *pgxpool.Pool
	policy domain.PermissionPolicy
}

func NewProductStore(pool *pgxpool.Pool) *ProductStore { return &ProductStore{pool: pool} }

func NewProductStoreWithPolicy(pool *pgxpool.Pool, policy domain.PermissionPolicy) *ProductStore {
	return &ProductStore{pool: pool, policy: policy}
}

func (s *ProductStore) authorize(ctx context.Context, tx pgx.Tx, resource string) error {
	return AuthorizeTenantWrite(ctx, tx, s.policy, resource, "write")
}

func (s *ProductStore) DefaultTenantID(ctx context.Context) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `SELECT id FROM tenants WHERE name='__iolink_system__' AND active`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrNotFound
	}
	return id, err
}

func (s *ProductStore) DeviceAssignment(ctx context.Context, tenantID int64, deviceNo string) (domain.ProductModel, error) {
	var m domain.ProductModel
	var raw []byte
	q := `SELECT m.id,m.product_id,m.version,m.schema,m.published_at,d.assigned_at
        FROM devices d JOIN ponds po ON po.id=d.pond_id JOIN farms f ON f.id=po.farm_id
        JOIN product_models m ON m.product_id=d.product_id AND m.version=d.model_version
		WHERE d.device_no=$1 AND f.tenant_id=$2`
	args := []any{deviceNo, tenantID}
	role := domain.TenantRole(ctx)
	if actorID, ok := domain.TenantUserID(ctx); ok && role != "owner" && role != "admin" {
		q += ` AND EXISTS (SELECT 1 FROM farm_memberships fm JOIN users membership_user ON membership_user.id=fm.user_id AND (membership_user.authority='USER' OR (membership_user.authority='ADMIN' AND fm.role='support' AND fm.expires_at IS NOT NULL)) WHERE fm.farm_id=f.id AND fm.tenant_id=f.tenant_id AND fm.user_id=$3 AND fm.active AND (fm.expires_at IS NULL OR fm.expires_at>now()))`
		args = append(args, actorID)
	}
	err := s.pool.QueryRow(ctx, q, args...).
		Scan(&m.ID, &m.ProductID, &m.Version, &raw, &m.PublishedAt, &m.AssignedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, domain.ErrNotFound
	}
	if err != nil {
		return m, err
	}
	m.Schema = raw
	var decoded struct {
		Fields []domain.ModelField `json:"fields"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return m, err
	}
	m.Fields = decoded.Fields
	return m, nil
}

var identifierRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func validateFields(fields []domain.ModelField) error {
	if len(fields) == 0 || len(fields) > 256 {
		return domain.ErrInvalidProductModel
	}
	seen := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		if !identifierRE.MatchString(f.Identifier) || len(f.Unit) > 32 || (f.Type != "number" && f.Type != "integer" && f.Type != "boolean" && f.Type != "string") {
			return domain.ErrInvalidProductModel
		}
		if _, ok := seen[f.Identifier]; ok {
			return domain.ErrInvalidProductModel
		}
		seen[f.Identifier] = struct{}{}
		if f.Min != nil && f.Max != nil && *f.Min > *f.Max {
			return domain.ErrInvalidProductModel
		}
		if f.Type != "string" && len(f.Enum) > 0 {
			return domain.ErrInvalidProductModel
		}
		if f.Type == "string" {
			if f.Enum != nil && len(f.Enum) == 0 {
				return domain.ErrInvalidProductModel
			}
			vals := map[string]struct{}{}
			for _, v := range f.Enum {
				if v == "" {
					return domain.ErrInvalidProductModel
				}
				if _, ok := vals[v]; ok {
					return domain.ErrInvalidProductModel
				}
				vals[v] = struct{}{}
			}
		}
	}
	return nil
}
func activeTenant(ctx context.Context, tx pgx.Tx, tenantID int64) error {
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT active FROM tenants WHERE id=$1 FOR SHARE`, tenantID).Scan(&ok); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	if !ok {
		return domain.ErrInactiveTenant
	}
	return nil
}

func validateCompatibility(ctx context.Context, tx pgx.Tx, productID int64, fields []domain.ModelField) error {
	rows, err := tx.Query(ctx, `SELECT schema FROM product_models WHERE product_id=$1`, productID)
	if err != nil {
		return err
	}
	defer rows.Close()
	want := make(map[string]domain.ModelField, len(fields))
	for _, f := range fields {
		want[f.Identifier] = f
	}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return err
		}
		var prior struct {
			Fields []domain.ModelField `json:"fields"`
		}
		if err := json.Unmarshal(raw, &prior); err != nil {
			return err
		}
		for _, old := range prior.Fields {
			if current, ok := want[old.Identifier]; ok && (current.Type != old.Type || current.Unit != old.Unit) {
				return domain.ErrInvalidProductModel
			}
		}
	}
	return rows.Err()
}
func (s *ProductStore) CreateProduct(ctx context.Context, tenantID int64, name string) (domain.Product, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 128 {
		return domain.Product{}, domain.ErrInvalidProductModel
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Product{}, err
	}
	defer tx.Rollback(ctx)
	if err = s.authorize(ctx, tx, "products"); err != nil {
		return domain.Product{}, err
	}
	if err = activeTenant(ctx, tx, tenantID); err != nil {
		return domain.Product{}, err
	}
	var p domain.Product
	err = tx.QueryRow(ctx, `INSERT INTO products(tenant_id,name) VALUES($1,$2) RETURNING id,tenant_id,name,created_at`, tenantID, name).Scan(&p.ID, &p.TenantID, &p.Name, &p.CreatedAt)
	p.Builtin = false
	if err != nil {
		return p, normalizeProductError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Product{}, err
	}
	return p, nil
}
func (s *ProductStore) ListProducts(ctx context.Context, tenantID int64) ([]domain.Product, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := activeTenant(ctx, tx, tenantID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT p.id,p.tenant_id,p.name,(t.name='__iolink_system__' AND p.name='water-quality'),(SELECT max(version) FROM product_models pm WHERE pm.product_id=p.id AND pm.published_at IS NOT NULL),p.created_at FROM products p JOIN tenants t ON t.id=p.tenant_id WHERE p.tenant_id=$1 ORDER BY p.id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Product{}
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Builtin, &p.CurrentVersion, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
func (s *ProductStore) CreateProductModel(ctx context.Context, tenantID, productID int64, version int, fields []domain.ModelField) (domain.ProductModel, error) {
	if version < 1 || validateFields(fields) != nil {
		return domain.ProductModel{}, domain.ErrInvalidProductModel
	}
	schema, err := json.Marshal(struct {
		Fields []domain.ModelField `json:"fields"`
	}{fields})
	if err != nil {
		return domain.ProductModel{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.ProductModel{}, err
	}
	defer tx.Rollback(ctx)
	if err := s.authorize(ctx, tx, "products"); err != nil {
		return domain.ProductModel{}, err
	}
	if err = activeTenant(ctx, tx, tenantID); err != nil {
		return domain.ProductModel{}, err
	}
	var owned bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND tenant_id=$2)`, productID, tenantID).Scan(&owned); err != nil {
		return domain.ProductModel{}, err
	}
	if !owned {
		return domain.ProductModel{}, domain.ErrNotFound
	}
	if err := validateCompatibility(ctx, tx, productID, fields); err != nil {
		return domain.ProductModel{}, err
	}
	var m domain.ProductModel
	m.ProductID, m.Version = productID, version
	m.Schema = schema
	m.Fields = fields
	err = tx.QueryRow(ctx, `INSERT INTO product_models(product_id,version,schema) VALUES($1,$2,$3) RETURNING id,created_at`, productID, version, schema).Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return m, normalizeProductError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.ProductModel{}, err
	}
	return m, nil
}
func (s *ProductStore) PublishProductModel(ctx context.Context, tenantID, productID int64, version int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = s.authorize(ctx, tx, "products"); err != nil {
		return err
	}
	if err = activeTenant(ctx, tx, tenantID); err != nil {
		return err
	}
	var published *time.Time
	err = tx.QueryRow(ctx, `SELECT m.published_at FROM product_models m JOIN products p ON p.id=m.product_id WHERE m.product_id=$1 AND m.version=$2 AND p.tenant_id=$3 FOR UPDATE`, productID, version, tenantID).Scan(&published)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if published == nil {
		if _, err = tx.Exec(ctx, `UPDATE product_models SET published_at=now() WHERE product_id=$1 AND version=$2`, productID, version); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *ProductStore) GetProductModel(ctx context.Context, tenantID, productID int64, version int) (domain.ProductModel, error) {
	var m domain.ProductModel
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT m.id,m.product_id,m.version,m.schema,m.published_at,m.created_at FROM product_models m JOIN products p ON p.id=m.product_id JOIN tenants t ON t.id=p.tenant_id WHERE p.tenant_id=$1 AND p.id=$2 AND m.version=$3 AND t.active`, tenantID, productID, version).Scan(&m.ID, &m.ProductID, &m.Version, &raw, &m.PublishedAt, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, domain.ErrNotFound
	}
	if err != nil {
		return m, err
	}
	m.Schema = raw
	var decoded struct {
		Fields []domain.ModelField `json:"fields"`
	}
	if err = json.Unmarshal(raw, &decoded); err != nil {
		return m, fmt.Errorf("decode product model: %w", err)
	}
	m.Fields = decoded.Fields
	return m, nil
}

func (s *ProductStore) ListProductModels(ctx context.Context, tenantID, productID int64) ([]domain.ProductModel, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := activeTenant(ctx, tx, tenantID); err != nil {
		return nil, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND tenant_id=$2)`, productID, tenantID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.ErrNotFound
	}
	rows, err := tx.Query(ctx, `SELECT m.id,m.product_id,m.version,m.schema,m.published_at,m.created_at FROM product_models m JOIN products p ON p.id=m.product_id WHERE p.tenant_id=$1 AND p.id=$2 ORDER BY m.version`, tenantID, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ProductModel{}
	for rows.Next() {
		var m domain.ProductModel
		var raw []byte
		if err := rows.Scan(&m.ID, &m.ProductID, &m.Version, &raw, &m.PublishedAt, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Schema = raw
		var decoded struct {
			Fields []domain.ModelField `json:"fields"`
		}
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil, fmt.Errorf("decode product model: %w", err)
		}
		m.Fields = decoded.Fields
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
func (s *ProductStore) AssignDeviceProduct(ctx context.Context, tenantID int64, deviceNo string, productID int64, version int) error {
	return s.AssignDeviceProductByActor(ctx, tenantID, deviceNo, productID, version, 0)
}

func (s *ProductStore) AssignDeviceProductByActor(ctx context.Context, tenantID int64, deviceNo string, productID int64, version int, actorID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = AuthorizeTenantTarget(ctx, tenantID, actorID); err != nil {
		return err
	}
	if err = s.authorize(ctx, tx, "devices"); err != nil {
		return err
	}
	if err = activeTenant(ctx, tx, tenantID); err != nil {
		return err
	}
	var ok bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM product_models m JOIN products p ON p.id=m.product_id WHERE m.product_id=$1 AND m.version=$2 AND p.tenant_id=$3 AND m.published_at IS NOT NULL)`, productID, version, tenantID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrNotFound
	}
	var oldProductID int64
	var oldVersion int
	err = tx.QueryRow(ctx, `SELECT d.product_id,d.model_version FROM devices d JOIN ponds po ON po.id=d.pond_id JOIN farms f ON f.id=po.farm_id WHERE d.device_no=$1 AND f.tenant_id=$2 FOR UPDATE OF d`, deviceNo, tenantID).Scan(&oldProductID, &oldVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if oldProductID == productID && oldVersion == version {
		return tx.Commit(ctx)
	}
	ct, err := tx.Exec(ctx, `UPDATE devices d SET product_id=$2,model_version=$3,assigned_at=clock_timestamp() FROM ponds po JOIN farms f ON f.id=po.farm_id WHERE d.pond_id=po.id AND d.device_no=$1 AND f.tenant_id=$4`, deviceNo, productID, version, tenantID)
	if err != nil {
		return normalizeProductError(err)
	}
	if ct.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	if _, err = tx.Exec(ctx, `DELETE FROM device_shadows WHERE device_no=$1`, deviceNo); err != nil {
		return err
	}
	var actor any
	if actorID > 0 {
		actor = actorID
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id,metadata) VALUES($1,$2,'device.product_assigned','device',$3,$4::jsonb)`, tenantID, actor, deviceNo, fmt.Sprintf(`{"old_product_id":%d,"old_model_version":%d,"product_id":%d,"model_version":%d}`, oldProductID, oldVersion, productID, version)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func normalizeProductError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}
