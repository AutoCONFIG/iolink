package core

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/jackc/pgx/v5"
)

var validOpenScopes = map[string]bool{
	"ponds:read": true, "devices:read": true, "alarms:read": true,
}

func validateAPIKeyInput(name string, scopes []string, resources domain.APIKeyResourceScope) error {
	if strings.TrimSpace(name) == "" || len([]byte(name)) > 128 {
		return domain.ErrInvalidAPIKey
	}
	if len(scopes) == 0 || len(scopes) > 16 {
		return domain.ErrInvalidAPIKey
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if !validOpenScopes[scope] {
			return domain.ErrInvalidAPIKey
		}
		if _, ok := seen[scope]; ok {
			return domain.ErrInvalidAPIKey
		}
		seen[scope] = struct{}{}
	}
	if len(resources.FarmIDs) > 100 || len(resources.PondIDs) > 100 || len(resources.DeviceNos) > 100 {
		return domain.ErrInvalidAPIKey
	}
	for _, ids := range [2][]int64{resources.FarmIDs, resources.PondIDs} {
		for _, id := range ids {
			if id <= 0 {
				return domain.ErrInvalidAPIKey
			}
		}
	}
	for _, deviceNo := range resources.DeviceNos {
		if strings.TrimSpace(deviceNo) == "" {
			return domain.ErrInvalidAPIKey
		}
	}
	return nil
}

func (s *Service) IssueAPIKey(ctx context.Context, tenantID int64, name string, scopes []string, resources domain.APIKeyResourceScope, actorID int64) (domain.APIKey, string, error) {
	if err := s.RequireLicenseFeature(ctx, "openapi"); err != nil {
		return domain.APIKey{}, "", err
	}
	if err := validateAPIKeyInput(name, scopes, resources); err != nil {
		return domain.APIKey{}, "", err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.APIKey{}, "", fmt.Errorf("begin api key issue: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := s.authorizeAPIKeyAdmin(ctx, tx, tenantID, actorID); err != nil {
		return domain.APIKey{}, "", err
	}
	if err := validateAPIKeyResourceOwnership(ctx, tx, tenantID, resources); err != nil {
		return domain.APIKey{}, "", err
	}
	keyID, err := randomKeyID()
	if err != nil {
		return domain.APIKey{}, "", fmt.Errorf("generate api key id: %w", err)
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return domain.APIKey{}, "", fmt.Errorf("generate api key secret: %w", err)
	}
	ciphertext, nonce, err := encryptAPISecret(s.rootSecret(), secret)
	if err != nil {
		return domain.APIKey{}, "", err
	}
	resourceJSON, err := json.Marshal(resources)
	if err != nil {
		return domain.APIKey{}, "", fmt.Errorf("encode api key resources: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO api_keys(key_id,tenant_id,name,scopes,resources,encrypted_secret,secret_nonce,created_by,rate_tokens) VALUES($1,$2,$3,$4,$5::jsonb,$6,$7,$8,$9)`, keyID, tenantID, strings.TrimSpace(name), scopes, resourceJSON, ciphertext, nonce, actorID, float64(s.openRateBurst)); err != nil {
		return domain.APIKey{}, "", fmt.Errorf("insert api key: %w", err)
	}
	metadata, err := json.Marshal(struct {
		Scopes []string `json:"scopes"`
	}{Scopes: scopes})
	if err != nil {
		return domain.APIKey{}, "", fmt.Errorf("encode api key audit: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id,metadata) VALUES($1,$2,'api_key.issued','api_key',$3,$4::jsonb)`, tenantID, actorID, keyID, metadata); err != nil {
		return domain.APIKey{}, "", fmt.Errorf("audit api key issue: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.APIKey{}, "", fmt.Errorf("commit api key issue: %w", err)
	}
	return domain.APIKey{KeyID: keyID, TenantID: tenantID, Name: strings.TrimSpace(name), Scopes: scopes, Resources: resources, CreatedAt: time.Now().UTC()}, base64.RawURLEncoding.EncodeToString(secret), nil
}

func (s *Service) ListAPIKeys(ctx context.Context, tenantID, actorID int64) ([]domain.APIKey, error) {
	if err := s.RequireLicenseFeature(ctx, "openapi"); err != nil {
		return nil, err
	}
	if tenantID <= 0 {
		return nil, errors.New("tenant is required")
	}
	if err := s.authorizeAPIKeyRead(ctx, tenantID, actorID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT key_id,name,scopes,resources,created_at,revoked_at FROM api_keys WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()
	out := make([]domain.APIKey, 0)
	for rows.Next() {
		var key domain.APIKey
		var raw []byte
		if err := rows.Scan(&key.KeyID, &key.Name, &key.Scopes, &raw, &key.CreatedAt, &key.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		if err := json.Unmarshal(raw, &key.Resources); err != nil {
			return nil, fmt.Errorf("decode api key resources: %w", err)
		}
		key.TenantID = tenantID
		out = append(out, key)
	}
	return out, rows.Err()
}

func (s *Service) RevokeAPIKey(ctx context.Context, tenantID, actorID int64, keyID string) error {
	if err := s.RequireLicenseFeature(ctx, "openapi"); err != nil {
		return err
	}
	return s.revokeAPIKey(ctx, tenantID, actorID, keyID)
}

func (s *Service) RotateAPIKey(ctx context.Context, tenantID, actorID int64, keyID string) (domain.APIKey, string, error) {
	if err := s.RequireLicenseFeature(ctx, "openapi"); err != nil {
		return domain.APIKey{}, "", err
	}
	if err := s.authorizeAPIKeyRead(ctx, tenantID, actorID); err != nil {
		return domain.APIKey{}, "", err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.APIKey{}, "", fmt.Errorf("begin api key rotation: %w", err)
	}
	defer tx.Rollback(ctx)
	var old domain.APIKey
	if err := s.authorizeAPIKeyAdmin(ctx, tx, tenantID, actorID); err != nil {
		return domain.APIKey{}, "", err
	}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT name,scopes,resources FROM api_keys WHERE key_id=$1 AND tenant_id=$2 AND revoked_at IS NULL FOR UPDATE`, keyID, tenantID).Scan(&old.Name, &old.Scopes, &raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.APIKey{}, "", domain.ErrNotFound
		}
		return domain.APIKey{}, "", fmt.Errorf("lock api key rotation: %w", err)
	}
	if err := json.Unmarshal(raw, &old.Resources); err != nil {
		return domain.APIKey{}, "", fmt.Errorf("decode api key rotation: %w", err)
	}
	if err := validateAPIKeyResourceOwnership(ctx, tx, tenantID, old.Resources); err != nil {
		return domain.APIKey{}, "", err
	}
	newID, err := randomKeyID()
	if err != nil {
		return domain.APIKey{}, "", err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return domain.APIKey{}, "", err
	}
	ciphertext, nonce, err := encryptAPISecret(s.rootSecret(), secret)
	if err != nil {
		return domain.APIKey{}, "", err
	}
	resourceJSON, err := json.Marshal(old.Resources)
	if err != nil {
		return domain.APIKey{}, "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE api_keys SET revoked_at=now() WHERE key_id=$1`, keyID); err != nil {
		return domain.APIKey{}, "", fmt.Errorf("revoke old api key: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO api_keys(key_id,tenant_id,name,scopes,resources,encrypted_secret,secret_nonce,created_by,rotated_from,rate_tokens) VALUES($1,$2,$3,$4,$5::jsonb,$6,$7,$8,$9,$10)`, newID, tenantID, old.Name, old.Scopes, resourceJSON, ciphertext, nonce, actorID, keyID, float64(s.openRateBurst)); err != nil {
		return domain.APIKey{}, "", fmt.Errorf("insert rotated api key: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id,metadata) VALUES($1,$2,'api_key.rotated','api_key',$3,$4::jsonb)`, tenantID, actorID, newID, fmt.Sprintf(`{"rotated_from":%q}`, keyID)); err != nil {
		return domain.APIKey{}, "", fmt.Errorf("audit api key rotation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.APIKey{}, "", fmt.Errorf("commit api key rotation: %w", err)
	}
	return domain.APIKey{KeyID: newID, TenantID: tenantID, Name: old.Name, Scopes: old.Scopes, Resources: old.Resources, CreatedAt: time.Now().UTC()}, base64.RawURLEncoding.EncodeToString(secret), nil
}

func (s *Service) revokeAPIKey(ctx context.Context, tenantID, actorID int64, keyID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin api key state: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := s.authorizeAPIKeyAdmin(ctx, tx, tenantID, actorID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `UPDATE api_keys SET revoked_at=coalesce(revoked_at,now()) WHERE key_id=$1 AND tenant_id=$2`, keyID, tenantID)
	if err != nil {
		return fmt.Errorf("update api key state: %w", err)
	}
	if result.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(tenant_id,actor_id,action,resource_type,resource_id) VALUES($1,$2,'api_key.revoked','api_key',$3)`, tenantID, actorID, keyID); err != nil {
		return fmt.Errorf("audit api key state: %w", err)
	}
	return tx.Commit(ctx)
}
