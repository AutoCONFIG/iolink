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
	"unicode/utf8"
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
	ErrNotBefore        = errors.New("license: not yet valid")
	ErrExpired          = errors.New("license: expired")
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
	MaxPayloadBytes      = 45 * 1024
	MaxEnvelopeBytes     = 64 * 1024
	MaxPayloadB64Chars   = 62000
	MaxSignatureB64Chars = 1500
	MinRSABytes          = 2048
	MaxRSABytes          = 8192
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
	return s.State == StateValid || s.State == StatePermanent || s.State == StateOverage || s.State == StateExpired || s.State == StateNotBefore || s.State == StateMissing || s.State == StateInvalid || s.State == StateInstanceMismatch || s.State == StateClockError
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

func ParsePrivateKey(raw []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("%w: private key is not PEM", ErrInvalidPayload)
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		if key.N.BitLen() >= MinRSABytes && key.N.BitLen() <= MaxRSABytes {
			return key, nil
		}
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: private key is not RSA", ErrInvalidPayload)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok || key.N.BitLen() < MinRSABytes || key.N.BitLen() > MaxRSABytes {
		return nil, fmt.Errorf("%w: RSA modulus size", ErrInvalidPayload)
	}
	return key, nil
}

func Verify(envelope Envelope, publicKey *rsa.PublicKey) (Verified, error) {
	if publicKey == nil || strings.TrimSpace(envelope.PayloadB64) == "" || strings.TrimSpace(envelope.SignatureB64) == "" || len(envelope.PayloadB64) > MaxPayloadB64Chars || len(envelope.SignatureB64) > MaxSignatureB64Chars {
		return Verified{}, ErrInvalidEnvelope
	}
	payloadRaw, err := base64.StdEncoding.DecodeString(envelope.PayloadB64)
	if err != nil || len(payloadRaw) == 0 || len(payloadRaw) > MaxPayloadBytes || !utf8.Valid(payloadRaw) || !json.Valid(payloadRaw) {
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
	payload, err := ParsePayload(payloadRaw)
	if err != nil {
		return Verified{}, err
	}
	return Verified{Payload: payload, PayloadRaw: append([]byte(nil), payloadRaw...), Signature: append([]byte(nil), signature...), SHA256: fmt.Sprintf("%x", digest)}, nil
}

func ParseEnvelope(raw []byte) (Envelope, error) {
	if len(raw) == 0 || len(raw) > MaxEnvelopeBytes || !utf8.Valid(raw) || !json.Valid(raw) || hasDuplicateJSONKeys(raw) {
		return Envelope{}, ErrInvalidEnvelope
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var envelope Envelope
	if err := decoder.Decode(&envelope); err != nil {
		return Envelope{}, fmt.Errorf("%w: envelope encoding", ErrInvalidEnvelope)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Envelope{}, fmt.Errorf("%w: trailing envelope", ErrInvalidEnvelope)
	}
	if strings.TrimSpace(envelope.PayloadB64) == "" || strings.TrimSpace(envelope.SignatureB64) == "" || len(envelope.PayloadB64) > MaxPayloadB64Chars || len(envelope.SignatureB64) > MaxSignatureB64Chars {
		return Envelope{}, ErrInvalidEnvelope
	}
	return envelope, nil
}

func ParsePayload(raw []byte) (Payload, error) {
	if len(raw) == 0 || len(raw) > MaxPayloadBytes || !utf8.Valid(raw) || !json.Valid(raw) || hasDuplicateJSONKeys(raw) {
		return Payload{}, fmt.Errorf("%w: payload encoding", ErrInvalidPayload)
	}
	var payload Payload
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return Payload{}, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Payload{}, fmt.Errorf("%w: trailing payload", ErrInvalidPayload)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Payload{}, fmt.Errorf("%w: payload object", ErrInvalidPayload)
	}
	for _, name := range []string{"license_id", "deployment_id", "issued_at", "not_before", "expires_at", "max_devices", "features", "key_id"} {
		if _, ok := fields[name]; !ok {
			return Payload{}, fmt.Errorf("%w: required field missing", ErrInvalidPayload)
		}
	}
	if string(fields["features"]) == "null" {
		return Payload{}, fmt.Errorf("%w: features must be an array", ErrInvalidPayload)
	}
	for _, name := range []string{"issued_at", "not_before"} {
		if !strictUTCTimestamp(fields[name]) {
			return Payload{}, fmt.Errorf("%w: timestamps must use UTC Z", ErrInvalidPayload)
		}
	}
	if string(fields["expires_at"]) != "null" && !strictUTCTimestamp(fields["expires_at"]) {
		return Payload{}, fmt.Errorf("%w: timestamps must use UTC Z", ErrInvalidPayload)
	}
	if !isUTC(payload.IssuedAt) || !isUTC(payload.NotBefore) || (payload.ExpiresAt != nil && !isUTC(*payload.ExpiresAt)) {
		return Payload{}, fmt.Errorf("%w: timestamps must be UTC", ErrInvalidPayload)
	}
	if err := ValidatePayload(payload); err != nil {
		return Payload{}, err
	}
	return payload, nil
}

func strictUTCTimestamp(raw json.RawMessage) bool {
	var value string
	if json.Unmarshal(raw, &value) != nil || !strings.HasSuffix(value, "Z") {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func isUTC(value time.Time) bool { return value.Location() == time.UTC }

func ValidatePayload(payload Payload) error {
	if !validIdentifier(payload.LicenseID) || !validIdentifier(payload.DeploymentID) || !validIdentifier(payload.KeyID) {
		return fmt.Errorf("%w: required identifier missing", ErrInvalidPayload)
	}
	if payload.IssuedAt.IsZero() || payload.NotBefore.IsZero() || payload.MaxDevices < 0 || payload.IssuedAt.After(payload.NotBefore) {
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

func validIdentifier(value string) bool {
	return len(value) > 0 && len(value) <= 128 && strings.TrimSpace(value) == value
}

func hasDuplicateJSONKeys(raw []byte) bool {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	var walk func() bool
	walk = func() bool {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		switch delimiter := token.(type) {
		case json.Delim:
			switch delimiter {
			case '{':
				seen := make(map[string]struct{})
				for decoder.More() {
					keyToken, err := decoder.Token()
					if err != nil {
						return false
					}
					key := keyToken.(string)
					if _, exists := seen[key]; exists {
						return true
					}
					seen[key] = struct{}{}
					if walk() {
						return true
					}
				}
				_, _ = decoder.Token()
			case '[':
				for decoder.More() {
					if walk() {
						return true
					}
				}
				_, _ = decoder.Token()
			}
		}
		return false
	}
	return walk()
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
