package license

import (
	"errors"
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
	if s.State == StateOverage {
		return ErrQuotaExceeded
	}
	if s.State != StateValid && s.State != StatePermanent {
		return ErrRequired
	}
	if s.Overage > 0 || s.UsedDevices >= s.MaxDevices {
		return ErrQuotaExceeded
	}
	return nil
}

func (s Status) AllowsFeature(feature string) error {
	if s.State != StateValid && s.State != StatePermanent && s.State != StateOverage {
		return ErrFeatureDenied
	}
	for _, enabled := range s.Features {
		if enabled == feature {
			return nil
		}
	}
	return ErrFeatureDenied
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
