package camerapg_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func TestCreate_whenCredentialsProvided(t *testing.T) {
	// Given
	f := newFixture(t)
	// When
	got, err := f.service.Create(actor(1, "owner"), configuration(t, 102, true))
	// Then
	if err != nil || got.PondID != 102 || got.SourceVersion != 1 || got.Status != "offline" {
		t.Fatalf("camera=%+v error=%v", got, err)
	}
	var tenant, farm, audits int64
	var cipher []byte
	if err = f.pool.QueryRow(t.Context(), `SELECT tenant_id,farm_id,credential_cipher,(SELECT count(*) FROM audit_events WHERE action='camera.created') FROM video_cameras WHERE id=$1`, got.ID).Scan(&tenant, &farm, &cipher, &audits); err != nil {
		t.Fatal(err)
	}
	if tenant != 101 || farm != 102 || audits != 1 || bytes.Contains(cipher, []byte("camera-password")) {
		t.Fatal("scope, encryption or atomic audit incorrect")
	}
	aad, err := domain.NewVideoCredentialBinding(domain.CameraCredential, 101, got.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := f.deps.Cipher.Open(t.Context(), aad, cipher)
	if err != nil || !bytes.Contains(plain, []byte("camera-password")) {
		t.Fatal("credential cannot be recovered with correct binding")
	}
	for _, binding := range []struct {
		purpose             domain.VideoCredentialPurpose
		tenant, id, version int64
	}{{domain.CameraCredential, 102, got.ID, 1}, {domain.CameraCredential, 101, got.ID + 1, 1}, {domain.CameraCredential, 101, got.ID, 2}, {domain.GBDeviceCredential, 101, got.ID, 1}} {
		other, err := domain.NewVideoCredentialBinding(binding.purpose, binding.tenant, binding.id, binding.version)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.deps.Cipher.Open(t.Context(), other, cipher); err == nil {
			t.Fatal("cipher accepted changed AAD")
		}
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"rtsp://", "192.168", "camera-login", "camera-password", "credential"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("response disclosed source secret")
		}
	}
}

func TestCreate_whenGBBindingChanges(t *testing.T) {
	for _, item := range []struct {
		name    string
		device  int64
		channel string
		want    error
	}{
		{"current", 101, "34020000001310000101", nil},
		{"absent", 101, "34020000001310000102", camera.ErrInvalid},
		{"disabled", 102, "34020000001310000101", camera.ErrInvalid},
		{"foreign", 103, "34020000001310000101", domain.ErrNotFound},
		{"missing device", 999, "34020000001310000101", domain.ErrNotFound},
		{"missing channel", 101, "34020000001310000999", domain.ErrNotFound},
	} {
		t.Run(item.name, func(t *testing.T) {
			// Given
			f := newFixture(t)
			cfg, err := camera.ParseConfiguration(101, "GB", camera.SourceInput{Kind: domain.VideoGB28181, DeviceID: item.device, ChannelID: item.channel})
			if err != nil {
				t.Fatal(err)
			}
			// When
			got, err := f.service.Create(actor(1, "owner"), cfg)
			// Then
			if item.want != nil {
				if !errors.Is(err, item.want) {
					t.Fatalf("error=%v want=%v", err, item.want)
				}
				return
			}
			if err != nil || got.GBDeviceID == nil || *got.GBDeviceID != 101 || got.GBChannelID == nil || *got.GBChannelID != item.channel {
				t.Fatalf("camera=%+v error=%v", got, err)
			}
		})
	}
}

func TestConfigure_whenLicenseOrCapabilityUnavailable(t *testing.T) {
	for _, item := range []struct {
		name, sql string
		edit      func(*camera.Dependencies)
		want      error
	}{
		{"missing license", `UPDATE license_state SET payload=NULL,signature=NULL,payload_sha256=NULL,imported_at=NULL`, nil, domain.ErrForbidden},
		{"wrong deployment", `UPDATE deployment_config SET deployment_id='changed'`, nil, domain.ErrForbidden},
		{"clock latched", `UPDATE license_clock SET clock_error=true`, nil, domain.ErrForbidden},
		{"clock rollback", `UPDATE license_clock SET max_seen_at=now()+interval '10 minutes'`, nil, domain.ErrForbidden},
		{"invalid signature", `UPDATE license_state SET signature=decode(repeat('aa',256),'hex')`, nil, domain.ErrForbidden},
		{"key unavailable", "", func(d *camera.Dependencies) { d.License = camera.LicenseVerifier{} }, camera.ErrUnavailable},
		{"cipher unavailable", "", func(d *camera.Dependencies) { d.Cipher = nil }, camera.ErrUnavailable},
		{"runtime disabled", "", func(d *camera.Dependencies) { d.Availability = nil }, camera.ErrUnavailable},
		{"denied before disabled", `UPDATE license_clock SET clock_error=true`, func(d *camera.Dependencies) { d.Availability = nil }, domain.ErrForbidden},
	} {
		t.Run(item.name, func(t *testing.T) {
			// Given
			f := newFixture(t)
			if item.sql != "" {
				exec(t, f.pool, item.sql)
			}
			if item.edit != nil {
				item.edit(&f.deps)
			}
			s := camera.New(f.deps)
			// When
			_, err := s.Replace(actor(1, "owner"), 101, configuration(t, 101, true))
			// Then
			if !errors.Is(err, item.want) {
				t.Fatalf("error=%v want=%v", err, item.want)
			}
			var version int64
			if err = f.pool.QueryRow(t.Context(), `SELECT source_version FROM video_cameras WHERE id=101`).Scan(&version); err != nil || version != 1 {
				t.Fatalf("version=%d error=%v", version, err)
			}
		})
	}
}
