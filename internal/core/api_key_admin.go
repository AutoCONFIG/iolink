package core

import (
	"context"
	"fmt"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/persistence"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ListAPIKeyAuditEvents(ctx context.Context, tenantID, actorID int64, limit int) ([]domain.APIKeyAuditEvent, error) {
	if err := s.RequireLicenseFeature(ctx, "openapi"); err != nil {
		return nil, err
	}
	if err := s.authorizeAPIKeyRead(ctx, tenantID, actorID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `SELECT id,tenant_id,actor_id,action,resource_id,metadata,created_at FROM audit_events WHERE tenant_id=$1 AND action LIKE 'api_key.%' ORDER BY created_at DESC,id DESC LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list api key audit: %w", err)
	}
	defer rows.Close()
	out := make([]domain.APIKeyAuditEvent, 0)
	for rows.Next() {
		var item domain.APIKeyAuditEvent
		if err := rows.Scan(&item.ID, &item.TenantID, &item.ActorID, &item.Action, &item.ResourceID, &item.Metadata, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan api key audit: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) authorizeAPIKeyRead(ctx context.Context, tenantID, actorID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := s.authorizeAPIKeyAdmin(ctx, tx, tenantID, actorID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) authorizeAPIKeyAdmin(ctx context.Context, tx pgx.Tx, tenantID, actorID int64) error {
	if actor, ok := domain.PlatformActorFromContext(ctx); ok {
		if actor.ID != actorID {
			return domain.ErrForbidden
		}
		return persistence.AuthorizePlatformWrite(ctx, tx, actorID)
	}
	scoped, ok := domain.TenantID(ctx)
	actor, actorOK := domain.TenantUserID(ctx)
	if !ok || !actorOK || scoped != tenantID || actor != actorID {
		return domain.ErrForbidden
	}
	role := domain.TenantRole(ctx)
	if role != "owner" && role != "admin" {
		return domain.ErrForbidden
	}
	return persistence.AuthorizeTenantWrite(ctx, tx, s.policy, "api-keys", "write")
}
