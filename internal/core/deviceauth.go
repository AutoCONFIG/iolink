package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
)

// Device-triple authentication for the access module.
//
// Satisfies access.Authenticator via Go structural typing: core never
// imports the access module — cmd/iolinkd passes the Service in.

// Authenticate verifies a device triple against devices.secret_hash
// (sha256 hex of the device secret). Satisfies access.Authenticator.
func (s *Service) Authenticate(deviceNo, secret string) bool {
	sum := sha256.Sum256([]byte(secret))
	want := hex.EncodeToString(sum[:])
	var n int
	err := s.pool.QueryRow(context.Background(),
		`SELECT count(1) FROM devices WHERE device_no=$1 AND secret_hash=$2`,
		deviceNo, want).Scan(&n)
	return err == nil && n > 0
}

// HashDeviceSecret is the canonical way to compute secret_hash when
// registering a device (seed SQL / future admin API).
func HashDeviceSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
