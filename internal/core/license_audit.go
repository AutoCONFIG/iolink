package core

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

func (s *Service) RecordLicenseRejection(ctx context.Context, raw []byte, actorID int64, reason string) error {
	digest := sha256.Sum256(raw)
	return s.RecordLicenseRejectionDigest(ctx, fmt.Sprintf("%x", digest), actorID, reason)
}

func (s *Service) RecordLicenseRejectionDigest(ctx context.Context, digestHex string, actorID int64, reason string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	metadata, err := json.Marshal(map[string]string{"sha256": digestHex, "reason": reason})
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id,metadata) VALUES((SELECT id FROM tenants WHERE name='__iolink_system__'),$1,'license.import_rejected','license',$2,$3::jsonb)`, actorID, digestHex, metadata); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) recordLicenseRejection(ctx context.Context, raw []byte, actorID int64, reason string) error {
	return s.RecordLicenseRejection(ctx, raw, actorID, reason)
}
