package core_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

func installTestLicense(t *testing.T, p *pgxpool.Pool, svc *core.Service, maxDevices int64) *rsa.PrivateKey {
	t.Helper()
	ctx := context.Background()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	payload := license.Payload{LicenseID: "test-license", IssuedAt: now.Add(-2 * time.Minute), NotBefore: now.Add(-time.Minute), MaxDevices: maxDevices, Features: []string{}, KeyID: "test-key"}
	if err := p.QueryRow(ctx, `SELECT deployment_id FROM deployment_config WHERE singleton=TRUE`).Scan(&payload.DeploymentID); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	signature, err := rsa.SignPSS(rand.Reader, key, crypto.SHA256, digest[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `UPDATE license_state SET payload=$1,signature=$2,payload_sha256=$3,imported_at=now() WHERE singleton=TRUE`, raw, signature, fmt.Sprintf("%x", digest)); err != nil {
		t.Fatal(err)
	}
	svc.SetLicenseRuntime(&core.LicenseRuntime{PublicKey: &key.PublicKey, KeyID: "test-key"})
	return key
}

func TestRegisterDevice_enforcesLicenseQuotaInOneTransaction(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id) VALUES(7001,'m6c-user'); INSERT INTO tenants(name) VALUES('m6c-tenant'); INSERT INTO farms(id,owner_id,tenant_id,name) SELECT 7001,7001,id,'m6c-farm' FROM tenants WHERE name='m6c-tenant'; INSERT INTO ponds(id,farm_id,name) VALUES(7001,7001,'m6c-pond')`); err != nil {
		t.Fatal(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	payload := license.Payload{LicenseID: "m6c-license", DeploymentID: "", IssuedAt: now.Add(-2 * time.Minute), NotBefore: now.Add(-time.Minute), MaxDevices: 2, Features: []string{}, KeyID: "test-key"}
	if err := p.QueryRow(ctx, `SELECT deployment_id FROM deployment_config WHERE singleton=TRUE`).Scan(&payload.DeploymentID); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	signature, err := rsa.SignPSS(rand.Reader, key, crypto.SHA256, digest[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `UPDATE license_state SET payload=$1,signature=$2,payload_sha256=$3,imported_at=now() WHERE singleton=TRUE`, raw, signature, fmt.Sprintf("%x", digest)); err != nil {
		t.Fatal(err)
	}
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := core.NewWithPolicy(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)), policy)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetLicenseRuntime(&core.LicenseRuntime{PublicKey: &key.PublicKey, KeyID: "test-key"})
	invalidEnvelopeRaw := []byte(`{"payload_b64":"e30=","signature_b64":"eA=="}`)
	invalidEnvelope, err := license.ParseEnvelope(invalidEnvelopeRaw)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ImportLicenseRaw(ctx, invalidEnvelopeRaw, invalidEnvelope, 7001); !errors.Is(err, license.ErrInvalidSignature) {
		t.Fatalf("invalid import error=%v", err)
	}
	current, err := svc.LicenseStatus(ctx)
	if err != nil || current.LicenseID == nil || *current.LicenseID != payload.LicenseID {
		t.Fatalf("invalid import replaced license: status=%+v err=%v", current, err)
	}
	var rejected int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action='license.import_rejected' AND actor_id=7001`).Scan(&rejected); err != nil || rejected != 1 {
		t.Fatalf("rejection audit count=%d err=%v", rejected, err)
	}
	var rejectionSHA string
	if err := p.QueryRow(ctx, `SELECT resource_id FROM audit_events WHERE action='license.import_rejected' AND actor_id=7001`).Scan(&rejectionSHA); err != nil {
		t.Fatal(err)
	}
	rawDigest := sha256.Sum256(invalidEnvelopeRaw)
	if rejectionSHA != fmt.Sprintf("%x", rawDigest) {
		t.Fatalf("rejection sha=%q want raw envelope sha=%x", rejectionSHA, rawDigest)
	}
	var wg sync.WaitGroup
	results := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, registerErr := svc.RegisterDevice(ctx, 7001, "m6c", "water", 60)
			results <- registerErr
		}()
	}
	wg.Wait()
	close(results)
	var success, quota int
	for registerErr := range results {
		if registerErr == nil {
			success++
		} else if errors.Is(registerErr, license.ErrQuotaExceeded) {
			quota++
		}
	}
	if success != 2 || quota != 2 {
		t.Fatalf("success=%d quota=%d", success, quota)
	}
	var used int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM devices WHERE disabled_at IS NULL`).Scan(&used); err != nil || used != 2 {
		t.Fatalf("used=%d err=%v", used, err)
	}
}

func TestLicenseStatus_reportsPersistedClockError(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `UPDATE license_clock SET clock_error=TRUE WHERE singleton=TRUE`); err != nil {
		t.Fatal(err)
	}
	svc, err := core.New(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	status, err := svc.LicenseStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != license.StateClockError || len(status.Features) != 0 {
		t.Fatalf("status=%+v", status)
	}
}

func TestReconcileLicenseClock_clearsLatchAndAudits(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id,authority) VALUES(7301,'m6c-reconcile-admin','ADMIN'); UPDATE license_clock SET clock_error=TRUE,max_seen_at=now()-interval '1 minute' WHERE singleton=TRUE`); err != nil {
		t.Fatal(err)
	}
	svc, err := core.New(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ReconcileLicenseClock(ctx, 7301); err != nil {
		t.Fatal(err)
	}
	var clockError bool
	if err := p.QueryRow(ctx, `SELECT clock_error FROM license_clock WHERE singleton=TRUE`).Scan(&clockError); err != nil || clockError {
		t.Fatalf("clock_error=%v err=%v", clockError, err)
	}
	var audits int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action='license.clock_reconciled' AND actor_id=7301`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("audits=%d err=%v", audits, err)
	}
}

func TestRegisterDevice_rejectsWhenLicenseVerificationIsUnavailable(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id) VALUES(7101,'m6c-missing-license'); INSERT INTO tenants(name) VALUES('m6c-missing-license-tenant'); INSERT INTO farms(id,owner_id,tenant_id,name) SELECT 7101,7101,id,'m6c-farm' FROM tenants WHERE name='m6c-missing-license-tenant'; INSERT INTO ponds(id,farm_id,name) VALUES(7101,7101,'m6c-pond')`); err != nil {
		t.Fatal(err)
	}
	svc, err := core.New(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.RegisterDevice(ctx, 7101, "m6c", "water", 60); !errors.Is(err, license.ErrUnavailable) {
		t.Fatalf("missing license registration error=%v", err)
	}
}

func TestRestoreDevice_rechecksLicenseAndClearsShadow(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id) VALUES(7201,'m6c-restore-user'); INSERT INTO tenants(id,name) VALUES(7201,'m6c-restore-tenant'); INSERT INTO tenant_memberships(tenant_id,user_id,role) VALUES(7201,7201,'owner'); INSERT INTO farms(id,owner_id,tenant_id,name) VALUES(7201,7201,7201,'m6c-restore-farm'); INSERT INTO ponds(id,farm_id,name) VALUES(7201,7201,'m6c-restore-pond'); INSERT INTO devices(id,pond_id,device_no,secret_hash,disabled_at,status) VALUES(7201,7201,'m6c-restore-device','hash',now(),'offline'); INSERT INTO device_shadows(device_no,last,pond_id,tenant_id,timestamps,model_version,product_id) SELECT d.device_no,'{}'::jsonb,d.pond_id,f.tenant_id,'{}'::jsonb,d.model_version,d.product_id FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id WHERE d.device_no='m6c-restore-device'`); err != nil {
		t.Fatal(err)
	}
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := core.NewWithPolicy(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)), policy)
	if err != nil {
		t.Fatal(err)
	}
	installTestLicense(t, p, svc, 1)
	scoped := domain.WithTenantUserID(domain.WithTenantRole(domain.WithTenantID(ctx, 7201), "owner"), 7201)
	if err := svc.RestoreDevice(scoped, "m6c-restore-device"); err != nil {
		t.Fatal(err)
	}
	var disabled *time.Time
	if err := p.QueryRow(ctx, `SELECT disabled_at FROM devices WHERE device_no='m6c-restore-device'`).Scan(&disabled); err != nil || disabled != nil {
		t.Fatalf("disabled_at=%v err=%v", disabled, err)
	}
	var shadows int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM device_shadows WHERE device_no='m6c-restore-device'`).Scan(&shadows); err != nil || shadows != 0 {
		t.Fatalf("shadows=%d err=%v", shadows, err)
	}
}
