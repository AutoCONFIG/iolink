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
	if err := s.pool.QueryRow(ctx, `SELECT d.secret_hash FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id JOIN tenants t ON t.id=f.tenant_id WHERE d.device_no=$1 AND d.disabled_at IS NULL AND t.active`, deviceNo).Scan(&encoded); err != nil {
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
	if err := s.pool.QueryRow(ctx, `SELECT d.report_interval FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id JOIN tenants t ON t.id=f.tenant_id WHERE d.device_no=$1 AND d.disabled_at IS NULL AND t.active`, no).Scan(&seconds); err != nil || seconds == nil {
		return s.defaultInterval
	}
	return time.Duration(*seconds) * time.Second
}

func (s *Service) DeviceRevision(no string) int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var revision int64
	if err := s.pool.QueryRow(ctx, `SELECT d.session_version FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id JOIN tenants t ON t.id=f.tenant_id WHERE d.device_no=$1 AND d.disabled_at IS NULL AND t.active`, no).Scan(&revision); err != nil {
		return -1
	}
	return revision
}

func (s *Service) GenericProduct(no string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var generic bool
	err := s.pool.QueryRow(ctx, `SELECT NOT (p.name='water-quality' AND t.name='__iolink_system__') FROM devices d JOIN products p ON p.id=d.product_id JOIN tenants t ON t.id=p.tenant_id JOIN ponds po ON po.id=d.pond_id JOIN farms f ON f.id=po.farm_id JOIN tenants ft ON ft.id=f.tenant_id WHERE d.device_no=$1 AND d.disabled_at IS NULL AND t.active AND ft.active`, no).Scan(&generic)
	return err == nil && generic
}

func (s *Service) ProductKind(no string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var water bool
	err := s.pool.QueryRow(ctx, `SELECT p.name='water-quality' AND t.name='__iolink_system__'
		FROM devices d JOIN products p ON p.id=d.product_id JOIN tenants t ON t.id=p.tenant_id
		JOIN ponds po ON po.id=d.pond_id JOIN farms f ON f.id=po.farm_id
		JOIN tenants ft ON ft.id=f.tenant_id
		WHERE d.device_no=$1 AND d.disabled_at IS NULL AND t.active AND ft.active`, no).Scan(&water)
	if err != nil {
		return "", err
	}
	if water {
		return "water", nil
	}
	return "generic", nil
}
