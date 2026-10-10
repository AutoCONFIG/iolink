package camerapg_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
)

func TestCreate_whenCurrentSignedLicenseDeniesVideo(t *testing.T) {
	for _, item := range []struct {
		name   string
		change func(*license.Payload)
	}{
		{"video feature absent", func(p *license.Payload) { p.Features = []string{} }},
		{"expired", func(p *license.Payload) {
			now := time.Now().UTC()
			p.IssuedAt = now.Add(-3 * time.Hour)
			p.NotBefore = now.Add(-2 * time.Hour)
			expiry := now.Add(-time.Hour)
			p.ExpiresAt = &expiry
		}},
		{"not yet valid", func(p *license.Payload) { p.NotBefore = time.Now().UTC().Add(time.Hour) }},
		{"wrong key id", func(p *license.Payload) { p.KeyID = "other-key" }},
	} {
		t.Run(item.name, func(t *testing.T) {
			// Given
			f := newFixture(t)
			var raw []byte
			if err := f.pool.QueryRow(t.Context(), `SELECT payload FROM license_state`).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var payload license.Payload
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatal(err)
			}
			item.change(&payload)
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			key, err := testKey()
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(raw)
			signature, err := rsa.SignPSS(rand.Reader, key, crypto.SHA256, digest[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256})
			if err != nil {
				t.Fatal(err)
			}
			exec(t, f.pool, `UPDATE license_state SET payload=$1,signature=$2,payload_sha256=$3`, raw, signature, fmt.Sprintf("%x", digest))
			// When
			_, err = f.service.Create(actor(1, "owner"), configuration(t, 101, false))
			// Then
			if !errors.Is(err, domain.ErrForbidden) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestRead_whenConfigurationDependenciesMissing(t *testing.T) {
	// Given
	f := newFixture(t)
	f.deps.License = nil
	f.deps.Cipher = nil
	f.deps.Availability = nil
	s := camera.New(f.deps)
	// When
	got, err := s.Get(actor(1, "owner"), 101)
	// Then
	if err != nil || got.ID != 101 {
		t.Fatalf("camera=%+v error=%v", got, err)
	}
}

func TestCreate_whenTenantAdministratorConfiguresCamera(t *testing.T) {
	// Given
	f := newFixture(t)
	// When
	got, err := f.service.Create(actor(2, "admin"), configuration(t, 102, false))
	// Then
	if err != nil || got.PondID != 102 || got.SourceVersion != 1 {
		t.Fatalf("camera=%+v error=%v", got, err)
	}
}
