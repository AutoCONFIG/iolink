package core

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"time"
)

// Device-triple authentication for the access module.
//
// Satisfies access.Authenticator via Go structural typing: core never
// imports the access module — cmd/iolinkd passes the Service in.

// Authenticate verifies a device triple against devices.secret_hash
// (sha256 hex of the device secret). Satisfies access.Authenticator.
func (s *Service) Authenticate(deviceNo, secret string) bool {
	sum := sha256.Sum256([]byte(secret))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var encoded string
	if err := s.pool.QueryRow(ctx, "SELECT secret_hash FROM devices WHERE device_no=$1 AND disabled_at IS NULL", deviceNo).Scan(&encoded); err != nil {
		return false
	}
	stored, err := hex.DecodeString(encoded)
	return err == nil && len(stored) == len(sum) && subtle.ConstantTimeCompare(stored, sum[:]) == 1
}

// HashDeviceSecret is the canonical way to compute secret_hash when
// registering a device (seed SQL / future admin API).
func HashDeviceSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// ReportInterval lets access use per-device overrides with a configured default.
func (s *Service) ReportInterval(no string) time.Duration {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var seconds *int
	if err := s.pool.QueryRow(ctx, "SELECT report_interval FROM devices WHERE device_no=$1 AND disabled_at IS NULL", no).Scan(&seconds); err != nil || seconds == nil {
		return s.defaultInterval
	}
	return time.Duration(*seconds) * time.Second
}

func (s *Service) DeviceRevision(no string) int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var revision int64
	if err := s.pool.QueryRow(ctx, `SELECT session_version FROM devices WHERE device_no=$1 AND disabled_at IS NULL`, no).Scan(&revision); err != nil {
		return -1
	}
	return revision
}
