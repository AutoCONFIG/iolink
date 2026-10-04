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

	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/license"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

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
	payload := license.Payload{LicenseID: "m6c-license", DeploymentID: "", IssuedAt: now, NotBefore: now.Add(-time.Minute), MaxDevices: 2, Features: []string{}, KeyID: "test-key"}
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
	features, _ := json.Marshal(payload.Features)
	if _, err := p.Exec(ctx, `UPDATE license_state SET license_id=$1,deployment_id=$2,key_id=$3,issued_at=$4,not_before=$5,max_devices=$6,features=$7,payload=$8,signature=$9,payload_sha256=$10,state='permanent' WHERE singleton=TRUE`, payload.LicenseID, payload.DeploymentID, payload.KeyID, payload.IssuedAt, payload.NotBefore, payload.MaxDevices, features, raw, signature, fmt.Sprintf("%x", digest)); err != nil {
		t.Fatal(err)
	}
	svc, err := core.New(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	svc.SetLicenseRuntime(&core.LicenseRuntime{PublicKey: &key.PublicKey, KeyID: "test-key"})
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
