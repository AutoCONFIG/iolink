package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/license"
)

func TestSignerAcceptsMaximumPayloadAndProducesVerifiableEnvelope(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "issuer.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o600); err != nil {
		t.Fatal(err)
	}
	base := `{"license_id":"l","deployment_id":"d","issued_at":"2026-01-01T00:00:00Z","not_before":"2026-01-01T00:00:00Z","expires_at":null,"max_devices":1,"features":[],"key_id":"k"}`
	payload := base + strings.Repeat(" ", 45*1024-len(base))
	cmd := exec.Command("go", "run", ".", "--private-key", keyPath)
	cmd.Stdin = strings.NewReader(payload)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("signer failed: %v", err)
	}
	var envelope license.Envelope
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatal(err)
	}
	if _, err := license.Verify(envelope, &key.PublicKey); err != nil {
		t.Fatalf("signed envelope rejected: %v", err)
	}
}

func TestSignerRejectsPayloadOverMaximumAndInsecureKeyPermissions(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "issuer.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o644); err != nil {
		t.Fatal(err)
	}
	base := `{"license_id":"l","deployment_id":"d","issued_at":"2026-01-01T00:00:00Z","not_before":"2026-01-01T00:00:00Z","expires_at":null,"max_devices":1,"features":[],"key_id":"k"}`
	if err := os.Chmod(keyPath, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".", "--private-key", keyPath)
	cmd.Stdin = strings.NewReader(base + strings.Repeat(" ", 45*1024-len(base)+1))
	if err := cmd.Run(); err == nil {
		t.Fatalf("oversized input unexpectedly accepted")
	}
	if err := os.Chmod(keyPath, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("go", "run", ".", "--private-key", keyPath)
	cmd.Stdin = strings.NewReader(base)
	if err := cmd.Run(); err == nil {
		t.Fatalf("insecure key unexpectedly accepted")
	}
}
