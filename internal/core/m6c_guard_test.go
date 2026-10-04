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
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/license"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

func TestDeviceAdmission_persistsClockLatchAfterBusinessRollback(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	svc, err := core.New(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	installTestLicense(t, p, svc, 1)
	if _, err := p.Exec(ctx, `UPDATE license_clock SET max_seen_at=now()+interval '10 minutes',clock_error=FALSE`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.RegisterDevice(ctx, 1, "test", "water", 60); !errors.Is(err, license.ErrClockError) {
		t.Fatalf("admission error=%v", err)
	}
	var latched bool
	if err := p.QueryRow(ctx, `SELECT clock_error FROM license_clock WHERE singleton=TRUE`).Scan(&latched); err != nil || !latched {
		t.Fatalf("clock latch=%v err=%v", latched, err)
	}
	if _, err := p.Exec(ctx, `UPDATE license_clock SET max_seen_at=now()-interval '1 minute'`); err != nil {
		t.Fatal(err)
	}
	if err := svc.ObserveLicenseClock(ctx); !errors.Is(err, license.ErrClockError) {
		t.Fatalf("caught-up clock silently cleared latch: %v", err)
	}
}

func TestLicenseFeatureGuard_controlsFakeFutureEffects(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id) VALUES(7501,'m6c-feature'); INSERT INTO farms(id,owner_id,tenant_id,name) SELECT 7501,7501,id,'feature-farm' FROM tenants WHERE name='__iolink_system__'; INSERT INTO ponds(id,farm_id,name) VALUES(7501,7501,'feature-pond'); INSERT INTO devices(pond_id,device_no,secret_hash) VALUES(7501,'feature-device','hash')`); err != nil {
		t.Fatal(err)
	}
	svc, err := core.New(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	key := installTestLicense(t, p, svc, 5)
	var deployment string
	if err := p.QueryRow(ctx, `SELECT deployment_id FROM deployment_config`).Scan(&deployment); err != nil {
		t.Fatal(err)
	}
	features := []string{"video", "openapi", "automation", "reports"}
	for _, state := range []string{"valid", "permanent", "overage", "expired", "not_before", "clock_error", "missing", "invalid", "instance_mismatch", "unavailable", "monitoring_only"} {
		t.Run(state, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			payload := license.Payload{LicenseID: "guard", DeploymentID: deployment, IssuedAt: now.Add(-time.Hour), NotBefore: now.Add(-time.Minute), MaxDevices: 5, Features: features, KeyID: "test-key"}
			switch state {
			case "valid":
				expires := now.Add(time.Hour)
				payload.ExpiresAt = &expires
			case "overage":
				payload.MaxDevices = 0
			case "expired":
				expires := now.Add(-time.Second)
				payload.ExpiresAt = &expires
			case "not_before":
				payload.NotBefore = now.Add(time.Hour)
			case "instance_mismatch":
				payload.DeploymentID = "another-instance"
			case "monitoring_only":
				payload.Features = []string{}
			}
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(raw)
			signature, err := key.Sign(rand.Reader, digest[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256})
			if err != nil {
				t.Fatal(err)
			}
			if state == "invalid" {
				signature[0] ^= 1
			}
			if state == "missing" {
				raw, signature = nil, nil
			}
			var digestHex *string
			var importedAt *time.Time
			if state != "missing" {
				value := fmt.Sprintf("%x", digest)
				digestHex, importedAt = &value, &now
			}
			if _, err := p.Exec(ctx, `UPDATE license_state SET payload=$1,signature=$2,payload_sha256=$3,imported_at=$4`, raw, signature, digestHex, importedAt); err != nil {
				t.Fatal(err)
			}
			if _, err := p.Exec(ctx, `UPDATE license_clock SET max_seen_at=now(),clock_error=FALSE`); err != nil {
				t.Fatal(err)
			}
			if state == "clock_error" {
				if _, err := p.Exec(ctx, `UPDATE license_clock SET max_seen_at=now()+interval '10 minutes'`); err != nil {
					t.Fatal(err)
				}
			}
			svc.SetLicenseRuntime(&core.LicenseRuntime{PublicKey: &key.PublicKey, KeyID: "test-key"})
			if state == "unavailable" {
				svc.SetLicenseRuntime(nil)
			}
			allowed := state == "valid" || state == "permanent" || state == "overage"
			for _, feature := range features {
				effects := 0
				err := func() error {
					if err := svc.RequireLicenseFeature(ctx, feature); err != nil {
						return err
					}
					effects++
					return nil
				}()
				if allowed && (err != nil || effects != 1) {
					t.Fatalf("%s error=%v effects=%d", feature, err, effects)
				}
				if !allowed && (err == nil || effects != 0) {
					t.Fatalf("%s executed without license: error=%v effects=%d", feature, err, effects)
				}
			}
			if state == "overage" {
				status, err := svc.LicenseStatus(ctx)
				if err != nil || status.State != license.StateOverage || status.MaxDevices != 0 || status.Overage != 1 {
					t.Fatalf("signed status=%+v err=%v", status, err)
				}
				if _, _, err := svc.RegisterDevice(ctx, 7501, "new", "water", 60); !errors.Is(err, license.ErrQuotaExceeded) {
					t.Fatalf("overage device admission=%v", err)
				}
			}
		})
	}
}
