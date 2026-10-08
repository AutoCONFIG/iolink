package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

func TestLicenseCLI_rejectionAuditsRawFileWithoutResettingOnlineDevices(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id,authority) VALUES(7601,'cli-admin','ADMIN'); INSERT INTO farms(id,owner_id,tenant_id,name) SELECT 7601,7601,id,'cli-farm' FROM tenants WHERE name='__iolink_system__'; INSERT INTO ponds(id,farm_id,name) VALUES(7601,7601,'cli-pond'); INSERT INTO devices(pond_id,device_no,secret_hash,status) VALUES(7601,'cli-device','hash','online')`); err != nil {
		t.Fatal(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(t.TempDir(), "public.pem")
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey)}), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IOLINK_PG_DSN", p.Config().ConnString())
	t.Setenv("IOLINK_LICENSE_PUBLIC_KEY_FILE", keyFile)
	t.Setenv("IOLINK_LICENSE_KEY_ID", "test-key")
	for _, raw := range []string{`{"payload_b64":"e30=","signature_b64":"eA=="}`, `{"payload_b64":"first","payload_b64":"second"}`, strings.Repeat("x", 64*1024+17)} {
		if err := run([]string{"license", "import"}, strings.NewReader(raw), io.Discard); err == nil {
			t.Fatal("invalid license accepted")
		}
		digest := sha256.Sum256([]byte(raw))
		var count int
		if err := p.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action='license.import_rejected' AND resource_id=$1 AND metadata->>'sha256'=$1`, fmt.Sprintf("%x", digest)).Scan(&count); err != nil || count != 1 {
			t.Fatalf("raw rejection audit=%d err=%v", count, err)
		}
		var status string
		if err := p.QueryRow(ctx, `SELECT status FROM devices WHERE device_no='cli-device'`).Scan(&status); err != nil || status != "online" {
			t.Fatalf("CLI mutated online device: status=%s err=%v", status, err)
		}
	}
	if err := run([]string{"license", "reconcile-clock"}, strings.NewReader(""), io.Discard); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := p.QueryRow(ctx, `SELECT status FROM devices WHERE device_no='cli-device'`).Scan(&status); err != nil || status != "online" {
		t.Fatalf("reconcile mutated online device: status=%s err=%v", status, err)
	}
}

func TestSetupCLI_auditsMalformedInputWithoutPartialAccounts(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IOLINK_PG_DSN", p.Config().ConnString())
	if err := run([]string{"setup", "init"}, strings.NewReader(`{"platform_password":`), io.Discard); err == nil {
		t.Fatal("malformed setup accepted")
	}
	var rejected, users int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action='setup.init_rejected' AND metadata->>'reason'='invalid_input'`).Scan(&rejected); err != nil {
		t.Fatal(err)
	}
	if err := p.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if rejected != 1 || users != 0 {
		t.Fatalf("rejections=%d users=%d", rejected, users)
	}
}

func TestSetupCLI_rollsBackAccountsAndReportsAuditFailure(t *testing.T) {
	for _, action := range []string{"setup.initialized", "setup.init_rejected"} {
		t.Run(action, func(t *testing.T) {
			p := testdb.New(t)
			ctx := context.Background()
			if err := migrate.Up(ctx, p); err != nil {
				t.Fatal(err)
			}
			t.Setenv("IOLINK_PG_DSN", p.Config().ConnString())
			if _, err := p.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION fail_setup_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='%s' THEN RAISE EXCEPTION 'audit unavailable'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_setup BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION fail_setup_audit()`, action)); err != nil {
				t.Fatal(err)
			}
			input := `{"platform_username":"platform","platform_password":"Platform-test-123"}`
			if action == "setup.init_rejected" {
				input = `{}`
			}
			err := run([]string{"setup", "init"}, strings.NewReader(input), io.Discard)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
				t.Fatalf("audit error hidden: %v", err)
			}
			var accounts int
			if err := p.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&accounts); err != nil {
				t.Fatal(err)
			}
			if accounts != 0 {
				t.Fatalf("partial accounts=%d", accounts)
			}
		})
	}
}
