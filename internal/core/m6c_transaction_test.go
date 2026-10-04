package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

func TestLicenseRejection_reportsAuditFailureAndRetainsCurrentLicense(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	svc := core.NewLicenseService(p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	installTestLicense(t, p, svc, 1)
	if _, err := p.Exec(ctx, `CREATE FUNCTION reject_license_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='license.import_rejected' THEN RAISE EXCEPTION 'audit unavailable'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_rejection BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION reject_license_audit()`); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"payload_b64":"e30=","signature_b64":"eA=="}`)
	envelope, err := license.ParseEnvelope(raw)
	if err != nil {
		t.Fatal(err)
	}
	err = svc.ImportLicenseRaw(ctx, raw, envelope, 1)
	if err == nil || errors.Is(err, license.ErrInvalidSignature) {
		t.Fatalf("audit failure hidden: %v", err)
	}
	current, err := svc.LicenseStatus(ctx)
	if err != nil || current.LicenseID == nil || *current.LicenseID != "test-license" {
		t.Fatalf("current license=%+v err=%v", current, err)
	}
}

func TestLicenseImport_rechecksClockAfterWaitingForLicenseLock(t *testing.T) {
	p := testdb.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id,authority) VALUES(1,'clock-admin','ADMIN')`); err != nil {
		t.Fatal(err)
	}
	svc := core.NewLicenseService(p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	installTestLicense(t, p, svc, 1)
	var payload, signature string
	if err := p.QueryRow(ctx, `SELECT encode(payload,'base64'),encode(signature,'base64') FROM license_state`).Scan(&payload, &signature); err != nil {
		t.Fatal(err)
	}
	envelope := license.Envelope{PayloadB64: strings.ReplaceAll(payload, "\n", ""), SignatureB64: strings.ReplaceAll(signature, "\n", "")}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT singleton FROM license_state WHERE singleton=TRUE FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `UPDATE license_clock SET max_seen_at=now()-interval '1 minute',clock_error=FALSE`); err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC()
	result := make(chan error, 1)
	go func() { result <- svc.ImportLicenseRaw(ctx, raw, envelope, 1) }()
	for {
		var observed time.Time
		if err := p.QueryRow(ctx, `SELECT max_seen_at FROM license_clock`).Scan(&observed); err != nil {
			t.Fatal(err)
		}
		if !observed.Before(started) {
			break
		}
	}
	if _, err := p.Exec(ctx, `UPDATE license_clock SET max_seen_at=now()+interval '10 minutes'`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, license.ErrClockError) {
		t.Fatalf("import ignored new clock high-water: %v", err)
	}
	var latched bool
	if err := p.QueryRow(ctx, `SELECT clock_error FROM license_clock`).Scan(&latched); err != nil || !latched {
		t.Fatalf("latch=%v err=%v", latched, err)
	}
}

func TestLicenseChecks_completeWhenPoolHasOneConnection(t *testing.T) {
	p := testdb.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	cfg := p.Config().Copy()
	cfg.MaxConns = 1
	limited, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer limited.Close()
	svc := core.NewLicenseService(limited, slog.New(slog.NewTextHandler(io.Discard, nil)))
	installTestLicense(t, p, svc, 1)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.LicenseStatus(ctx); err != nil {
				results <- err
				return
			}
			_, _, err := svc.RegisterDevice(ctx, -1, "test", "water", 60)
			if errors.Is(err, context.DeadlineExceeded) {
				results <- err
				return
			}
			if !errors.Is(err, domain.ErrNotFound) {
				results <- errors.Join(errors.New("expected missing pond"), err)
				return
			}
			results <- nil
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
}
