package camera

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
)

type LicenseSnapshot struct {
	DeploymentID       string
	Payload, Signature []byte
	SHA256             string
	MaxSeenAt          time.Time
	ClockError         bool
	Now                time.Time
}
type License interface {
	RequireVideo(context.Context, LicenseSnapshot) error
}
type LicenseVerifier struct {
	PublicKey *rsa.PublicKey
	KeyID     string
}

func (v LicenseVerifier) RequireVideo(ctx context.Context, state LicenseSnapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if v.PublicKey == nil || v.KeyID == "" {
		return ErrUnavailable
	}
	if len(state.Payload) == 0 || len(state.Signature) == 0 {
		return domain.ErrForbidden
	}
	verified, err := license.Verify(license.Envelope{PayloadB64: base64.StdEncoding.EncodeToString(state.Payload), SignatureB64: base64.StdEncoding.EncodeToString(state.Signature)}, v.PublicKey)
	if err != nil || verified.SHA256 != state.SHA256 || verified.Payload.KeyID != v.KeyID {
		return domain.ErrForbidden
	}
	status := license.Status{State: license.Evaluate(verified.Payload, state.DeploymentID, state.Now, state.MaxSeenAt, state.ClockError), Features: verified.Payload.Features}
	if err := status.AllowsFeature("video"); err != nil {
		return domain.ErrForbidden
	}
	return nil
}
