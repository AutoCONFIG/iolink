package license

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

var (
	ErrInvalidEnvelope  = errors.New("license: invalid envelope")
	ErrInvalidSignature = errors.New("license: invalid signature")
	ErrInvalidPayload   = errors.New("license: invalid payload")
	ErrInstanceMismatch = errors.New("license: deployment mismatch")
	ErrUnavailable      = errors.New("license: verification unavailable")
	ErrRequired         = errors.New("license: authorization required")
	ErrQuotaExceeded    = errors.New("license: device quota exceeded")
	ErrFeatureDenied    = errors.New("license: feature unavailable")
	ErrClockError       = errors.New("license: clock error")
)

type State string

const (
	StateMissing          State = "missing"
	StateValid            State = "valid"
	StatePermanent        State = "permanent"
	StateNotBefore        State = "not_before"
	StateExpired          State = "expired"
	StateInvalid          State = "invalid"
	StateInstanceMismatch State = "instance_mismatch"
	StateClockError       State = "clock_error"
	StateOverage          State = "overage"
)

var knownFeatures = map[string]struct{}{
	"video": {}, "openapi": {}, "automation": {}, "reports": {},
}

const (
	MaxPayloadBytes = 45 * 1024
	MinRSABytes     = 2048
	MaxRSABytes     = 8192
)

type Envelope struct {
	PayloadB64   string `json:"payload_b64"`
	SignatureB64 string `json:"signature_b64"`
}

type Payload struct {
	LicenseID    string     `json:"license_id"`
	DeploymentID string     `json:"deployment_id"`
	IssuedAt     time.Time  `json:"issued_at"`
	NotBefore    time.Time  `json:"not_before"`
	ExpiresAt    *time.Time `json:"expires_at"`
	MaxDevices   int64      `json:"max_devices"`
	Features     []string   `json:"features"`
	KeyID        string     `json:"key_id"`
}

type Verified struct {
	Payload    Payload
	PayloadRaw []byte
	Signature  []byte
	SHA256     string
}

type Status struct {
	State         State      `json:"state"`
	LicenseID     *string    `json:"license_id"`
	DeploymentID  *string    `json:"deployment_id"`
	KeyID         *string    `json:"key_id"`
	IssuedAt      *time.Time `json:"issued_at"`
	NotBefore     *time.Time `json:"not_before"`
	ExpiresAt     *time.Time `json:"expires_at"`
	MaxDevices    int64      `json:"max_devices"`
	Features      []string   `json:"features"`
	UsedDevices   int64      `json:"used_devices"`
	Overage       int64      `json:"overage"`
	PayloadSHA256 *string    `json:"payload_sha256"`
}

func (s Status) AllowsExisting() bool {
	return s.State == StateValid || s.State == StatePermanent || s.State == StateOverage || s.State == StateExpired || s.State == StateNotBefore || s.State == StateMissing || s.State == StateInvalid || s.State == StateInstanceMismatch
}

func (s Status) AllowsDeviceAdmission() error {
	if s.State != StateValid && s.State != StatePermanent {
		return ErrRequired
	}
	if s.Overage > 0 || s.UsedDevices >= s.MaxDevices {
		return ErrQuotaExceeded
	}
	return nil
}

func (s Status) AllowsFeature(feature string) error {
	if s.State != StateValid && s.State != StatePermanent {
		return ErrFeatureDenied
	}
	for _, enabled := range s.Features {
		if enabled == feature {
			return nil
		}
	}
	return ErrFeatureDenied
}

func ParsePublicKey(raw string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, fmt.Errorf("%w: public key is not PEM", ErrInvalidPayload)
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PublicKey); ok {
			if rsaKey.N.BitLen() < MinRSABytes || rsaKey.N.BitLen() > MaxRSABytes {
				return nil, fmt.Errorf("%w: RSA modulus size", ErrInvalidPayload)
			}
			return rsaKey, nil
		}
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		if key.N.BitLen() < MinRSABytes || key.N.BitLen() > MaxRSABytes {
			return nil, fmt.Errorf("%w: RSA modulus size", ErrInvalidPayload)
		}
		return key, nil
	}
	return nil, fmt.Errorf("%w: public key is not RSA", ErrInvalidPayload)
}

func Verify(envelope Envelope, publicKey *rsa.PublicKey) (Verified, error) {
	if strings.TrimSpace(envelope.PayloadB64) == "" || strings.TrimSpace(envelope.SignatureB64) == "" || publicKey == nil {
		return Verified{}, ErrInvalidEnvelope
	}
	payloadRaw, err := base64.StdEncoding.DecodeString(envelope.PayloadB64)
	if err != nil || len(payloadRaw) == 0 || len(payloadRaw) > MaxPayloadBytes || !json.Valid(payloadRaw) {
		return Verified{}, fmt.Errorf("%w: payload encoding", ErrInvalidEnvelope)
	}
	signature, err := base64.StdEncoding.DecodeString(envelope.SignatureB64)
	if err != nil || len(signature) == 0 {
		return Verified{}, fmt.Errorf("%w: signature encoding", ErrInvalidEnvelope)
	}
	digest := sha256.Sum256(payloadRaw)
	if err := rsa.VerifyPSS(publicKey, crypto.SHA256, digest[:], signature, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256}); err != nil {
		return Verified{}, ErrInvalidSignature
	}
	var payload Payload
	decoder := json.NewDecoder(strings.NewReader(string(payloadRaw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return Verified{}, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Verified{}, fmt.Errorf("%w: trailing payload", ErrInvalidPayload)
	}
	if !isUTC(payload.IssuedAt) || !isUTC(payload.NotBefore) || (payload.ExpiresAt != nil && !isUTC(*payload.ExpiresAt)) {
		return Verified{}, fmt.Errorf("%w: timestamps must be UTC", ErrInvalidPayload)
	}
	if err := ValidatePayload(payload); err != nil {
		return Verified{}, err
	}
	return Verified{Payload: payload, PayloadRaw: append([]byte(nil), payloadRaw...), Signature: append([]byte(nil), signature...), SHA256: fmt.Sprintf("%x", digest)}, nil
}

func isUTC(value time.Time) bool { return value.Location() == time.UTC }

func ValidatePayload(payload Payload) error {
	if strings.TrimSpace(payload.LicenseID) == "" || strings.TrimSpace(payload.DeploymentID) == "" || strings.TrimSpace(payload.KeyID) == "" {
		return fmt.Errorf("%w: required identifier missing", ErrInvalidPayload)
	}
	if payload.IssuedAt.IsZero() || payload.NotBefore.IsZero() || payload.MaxDevices < 0 {
		return fmt.Errorf("%w: invalid time or device limit", ErrInvalidPayload)
	}
	if payload.ExpiresAt != nil && !payload.ExpiresAt.After(payload.NotBefore) {
		return fmt.Errorf("%w: expiry must be after not_before", ErrInvalidPayload)
	}
	seen := make(map[string]struct{}, len(payload.Features))
	for _, feature := range payload.Features {
		if _, ok := knownFeatures[feature]; !ok {
			return fmt.Errorf("%w: unknown feature", ErrInvalidPayload)
		}
		if _, ok := seen[feature]; ok {
			return fmt.Errorf("%w: duplicate feature", ErrInvalidPayload)
		}
		seen[feature] = struct{}{}
	}
	return nil
}

func Evaluate(payload Payload, deploymentID string, now, maxSeen time.Time, clockError bool) State {
	if clockError || (!maxSeen.IsZero() && now.Before(maxSeen.Add(-5*time.Minute))) {
		return StateClockError
	}
	if payload.DeploymentID != deploymentID {
		return StateInstanceMismatch
	}
	if now.Before(payload.NotBefore) {
		return StateNotBefore
	}
	if payload.ExpiresAt == nil {
		return StatePermanent
	}
	if !now.Before(*payload.ExpiresAt) {
		return StateExpired
	}
	return StateValid
}
