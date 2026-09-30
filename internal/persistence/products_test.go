package persistence

import (
	"context"
	"errors"
	"fmt"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
	"testing"
	"time"
)

func TestValidateProductModelFields(t *testing.T) {
	min, max := 1.0, 5.0
	valid := []domain.ModelField{{Identifier: "temperature", Type: "number", Unit: "℃", Min: &min, Max: &max, Readable: true}}
	if err := validateFields(valid); err != nil {
		t.Fatalf("valid fields rejected: %v", err)
	}
	tooMany := make([]domain.ModelField, 257)
	for i := range tooMany {
		tooMany[i] = domain.ModelField{Identifier: fmt.Sprintf("field_%d", i), Type: "number"}
	}
	if err := validateFields(tooMany); err == nil {
		t.Fatal("more than 256 fields accepted")
	}
	longUnit := []domain.ModelField{{Identifier: "flow", Type: "number", Unit: "123456789012345678901234567890123"}}
	if err := validateFields(longUnit); err == nil {
		t.Fatal("unit longer than 32 characters accepted")
	}
	for name, fields := range map[string][]domain.ModelField{"bad identifier": {{Identifier: "Temp", Type: "number"}}, "unknown type": {{Identifier: "x", Type: "object"}}, "duplicate": {{Identifier: "x", Type: "number"}, {Identifier: "x", Type: "number"}}, "invalid range": {{Identifier: "x", Type: "number", Min: &max, Max: &min}}, "enum numeric": {{Identifier: "x", Type: "number", Enum: []string{"a"}}}} {
		t.Run(name, func(t *testing.T) {
			if err := validateFields(fields); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if err := validateFields([]domain.ModelField{{Identifier: "mode", Type: "string", Enum: []string{}}}); err == nil {
		t.Fatal("explicit empty enum accepted")
	}
}

func TestProductModelPublishAndTenantIsolation(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var tenantA, tenantB int64
	if err := pool.QueryRow(ctx, `INSERT INTO tenants(name) VALUES('m6a-a') RETURNING id`).Scan(&tenantA); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO tenants(name) VALUES('m6a-b') RETURNING id`).Scan(&tenantB); err != nil {
		t.Fatal(err)
	}
	catalog := NewProductStore(pool)
	product, err := catalog.CreateProduct(ctx, tenantA, "pump")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.CreateProduct(ctx, tenantA, "pump"); err == nil {
		t.Fatal("duplicate tenant product accepted")
	}
	fields := []domain.ModelField{{Identifier: "flow", Type: "number", Unit: "L/min", Readable: true}, {Identifier: "mode", Type: "string", Enum: []string{"auto", "manual"}, Readable: true, Writable: true}}
	model, err := catalog.CreateProductModel(ctx, tenantA, product.ID, 1, fields)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.CreateProductModel(ctx, tenantA, product.ID, 2, []domain.ModelField{{Identifier: "flow", Type: "integer", Unit: "L/min"}}); err == nil {
		t.Fatal("model changed an existing identifier type")
	}
	if err := catalog.AssignDeviceProduct(ctx, tenantA, "missing", product.ID, 1); err == nil {
		t.Fatal("unpublished or missing device assignment accepted")
	}
	if err := catalog.PublishProductModel(ctx, tenantA, product.ID, model.Version); err != nil {
		t.Fatal(err)
	}
	var userID, farmID, pondID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(open_id,nickname) VALUES('m6a-owner','M6a') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tenant_memberships(tenant_id,user_id,role) VALUES($1,$2,'owner')`, tenantA, userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO farms(owner_id,tenant_id,name) VALUES($1,$2,'m6a-farm') RETURNING id`, userID, tenantA).Scan(&farmID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO ponds(farm_id,name) VALUES($1,'m6a-pond') RETURNING id`, farmID).Scan(&pondID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO devices(pond_id,device_no,secret_hash) VALUES($1,'m6a-device','hash')`, pondID); err != nil {
		t.Fatal(err)
	}
	if err := catalog.AssignDeviceProduct(ctx, tenantA, "m6a-device", product.ID, 1); err != nil {
		t.Fatal(err)
	}
	beforeAssignment := time.Now().UTC()
	model2, err := catalog.CreateProductModel(ctx, tenantA, product.ID, 2, fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.PublishProductModel(ctx, tenantA, product.ID, model2.Version); err != nil {
		t.Fatal(err)
	}
	if err := catalog.AssignDeviceProductByActor(ctx, tenantA, "m6a-device", product.ID, 2, userID); err != nil {
		t.Fatal(err)
	}
	var supportID, hiddenFarmID, hiddenPondID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(open_id,nickname) VALUES('m6a-support','Support') RETURNING id`).Scan(&supportID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tenant_memberships(tenant_id,user_id,role,expires_at) VALUES($1,$2,'support',now()+interval '1 hour')`, tenantA, supportID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role,expires_at) SELECT $1,$2,$3,'support',now()+interval '1 hour'`, tenantA, farmID, supportID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO farms(owner_id,tenant_id,name) VALUES($1,$2,'m6a-hidden-farm') RETURNING id`, userID, tenantA).Scan(&hiddenFarmID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO ponds(farm_id,name) VALUES($1,'m6a-hidden-pond') RETURNING id`, hiddenFarmID).Scan(&hiddenPondID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO devices(pond_id,device_no,secret_hash,product_id,model_version) VALUES($1,'m6a-hidden-device','hash',$2,1)`, hiddenPondID, product.ID); err != nil {
		t.Fatal(err)
	}
	supportCtx := domain.WithTenantUserID(domain.WithTenantRole(domain.WithTenantID(ctx, tenantA), "support"), supportID)
	if _, err := catalog.DeviceAssignment(supportCtx, tenantA, "m6a-hidden-device"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("support read of unassigned device err=%v", err)
	}
	if _, err := catalog.DeviceAssignment(supportCtx, tenantA, "m6a-device"); err != nil {
		t.Fatalf("support read of assigned device err=%v", err)
	}
	var assignedProduct int64
	if err := pool.QueryRow(ctx, `SELECT product_id FROM devices WHERE device_no='m6a-device'`).Scan(&assignedProduct); err != nil || assignedProduct != product.ID {
		t.Fatalf("assignment not committed: id=%d err=%v", assignedProduct, err)
	}
	var assignedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT assigned_at FROM devices WHERE device_no='m6a-device'`).Scan(&assignedAt); err != nil || !assignedAt.After(beforeAssignment) {
		t.Fatalf("assignment timestamp=%v err=%v", assignedAt, err)
	}
	var actor int64
	if err := pool.QueryRow(ctx, `SELECT actor_id FROM audit_events WHERE action='device.product_assigned' AND resource_id='m6a-device' ORDER BY created_at DESC LIMIT 1`).Scan(&actor); err != nil || actor != userID {
		t.Fatalf("assignment audit actor=%d err=%v", actor, err)
	}
	if _, err := catalog.GetProductModel(ctx, tenantB, product.ID, 1); err == nil {
		t.Fatal("cross-tenant model read accepted")
	}
	if _, err := pool.Exec(ctx, `UPDATE product_models SET schema='{}' WHERE product_id=$1 AND version=1`, product.ID); err == nil {
		t.Fatal("published model mutation accepted")
	}
}
