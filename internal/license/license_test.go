package license

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"
)

func signForTest(t *testing.T, privateKey *rsa.PrivateKey, payload []byte) Envelope {
	t.Helper()
	digest := sha256.Sum256(payload)
	signature, err := rsa.SignPSS(rand.Reader, privateKey, crypto.SHA256, digest[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256})
	if err != nil {
		t.Fatal(err)
	}
	return Envelope{PayloadB64: base64.StdEncoding.EncodeToString(payload), SignatureB64: base64.StdEncoding.EncodeToString(signature)}
}

func TestVerify_acceptsValidAndPermanentPayload(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	payload := Payload{LicenseID: "lic-1", DeploymentID: "dep-1", IssuedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), NotBefore: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), MaxDevices: 3, Features: []string{"reports"}, KeyID: "k1"}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	envelope := signForTest(t, key, raw)
	verified, err := Verify(envelope, &key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Payload.LicenseID != "lic-1" || Evaluate(payload, "dep-1", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), time.Time{}, false) != StatePermanent {
		t.Fatalf("unexpected verified payload or state: %+v", verified)
	}
}

func TestVerify_rejectsTamperedAndUnknownPayload(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	envelope := signForTest(t, key, []byte(`{"license_id":"l","deployment_id":"d","issued_at":"2026-01-01T00:00:00Z","not_before":"2026-01-01T00:00:00Z","expires_at":null,"max_devices":1,"features":[],"key_id":"k"}`))
	envelope.PayloadB64 = envelope.PayloadB64[:len(envelope.PayloadB64)-2] + "AA"
	if _, err := Verify(envelope, &key.PublicKey); !errors.Is(err, ErrInvalidSignature) && !errors.Is(err, ErrInvalidPayload) && !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("want tamper rejection, got %v", err)
	}
	envelope = signForTest(t, key, []byte(`{"license_id":"l","deployment_id":"d","issued_at":"2026-01-01T00:00:00Z","not_before":"2026-01-01T00:00:00Z","expires_at":null,"max_devices":1,"features":["unknown"],"key_id":"k"}`))
	if _, err := Verify(envelope, &key.PublicKey); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("want unknown feature rejection, got %v", err)
	}
}

func TestEvaluate_rejectsClockRollbackAndMismatchedInstance(t *testing.T) {
	payload := Payload{LicenseID: "l", DeploymentID: "dep", IssuedAt: time.Unix(0, 0), NotBefore: time.Unix(0, 0), MaxDevices: 1, KeyID: "k"}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if got := Evaluate(payload, "dep", now, now.Add(6*time.Minute), false); got != StateClockError {
		t.Fatalf("got %s", got)
	}
	if got := Evaluate(payload, "other", now, time.Time{}, false); got != StateInstanceMismatch {
		t.Fatalf("got %s", got)
	}
}

func TestVerify_rejectsTrailingPayloadAndNonUTC(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"license_id":"l","deployment_id":"d","issued_at":"2026-01-01T00:00:00+08:00","not_before":"2026-01-01T00:00:00Z","expires_at":null,"max_devices":1,"features":[],"key_id":"k"}`)
	if _, err := Verify(signForTest(t, key, append(raw, []byte(` {}`)...)), &key.PublicKey); !errors.Is(err, ErrInvalidPayload) && !errors.Is(err, ErrInvalidSignature) && !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("want strict trailing rejection, got %v", err)
	}
	if _, err := Verify(signForTest(t, key, raw), &key.PublicKey); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("want UTC rejection, got %v", err)
	}
}

func TestParsePayload_rejectsDuplicateMissingAndInvalidFeatureFields(t *testing.T) {
	cases := []string{
		`{"license_id":"l","license_id":"l2","deployment_id":"d","issued_at":"2026-01-01T00:00:00Z","not_before":"2026-01-01T00:00:00Z","expires_at":null,"max_devices":1,"features":[],"key_id":"k"}`,
		`{"license_id":"l","deployment_id":"d","issued_at":"2026-01-01T00:00:00Z","not_before":"2026-01-01T00:00:00Z","expires_at":null,"max_devices":1,"key_id":"k"}`,
		`{"license_id":"l","deployment_id":"d","issued_at":"2026-01-01T00:00:00Z","not_before":"2026-01-01T00:00:00Z","expires_at":null,"max_devices":1,"features":null,"key_id":"k"}`,
	}
	for _, raw := range cases {
		if _, err := ParsePayload([]byte(raw)); !errors.Is(err, ErrInvalidPayload) {
			t.Fatalf("ParsePayload(%s) error=%v", raw, err)
		}
	}
}

func TestParsePayload_acceptsMaximumRawSizeAndRejectsOneByteOver(t *testing.T) {
	base := []byte(`{"license_id":"l","deployment_id":"d","issued_at":"2026-01-01T00:00:00Z","not_before":"2026-01-01T00:00:00Z","expires_at":null,"max_devices":1,"features":[],"key_id":"k"}`)
	if _, err := ParsePayload(append(base, []byte(strings.Repeat(" ", MaxPayloadBytes-len(base)))...)); err != nil {
		t.Fatalf("maximum payload rejected: %v", err)
	}
	if _, err := ParsePayload(append(base, []byte(strings.Repeat(" ", MaxPayloadBytes-len(base)+1))...)); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("oversized payload error=%v", err)
	}
}

func TestParseEnvelope_rejectsUnknownDuplicateAndTrailingFields(t *testing.T) {
	for _, raw := range []string{
		`{"payload_b64":"YQ==","signature_b64":"Yg==","extra":true}`,
		`{"payload_b64":"YQ==","payload_b64":"Yg==","signature_b64":"Yg=="}`,
		`{"payload_b64":"YQ==","signature_b64":"Yg=="}{}`,
	} {
		if _, err := ParseEnvelope([]byte(raw)); !errors.Is(err, ErrInvalidEnvelope) {
			t.Fatalf("ParseEnvelope(%s) error=%v", raw, err)
		}
	}
}

func TestParsePrivateKey_acceptsSupportedSizesAndRejectsOversized(t *testing.T) {
	for _, bits := range []int{2048, 4096, 8192} {
		key, err := rsa.GenerateKey(rand.Reader, bits)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := pemEncodePKCS1(key)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ParsePrivateKey(raw); err != nil {
			t.Fatalf("%d-bit key rejected: %v", bits, err)
		}
	}
}

func pemEncodePKCS1(key *rsa.PrivateKey) ([]byte, error) {
	raw := x509.MarshalPKCS1PrivateKey(key)
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: raw}), nil
}
