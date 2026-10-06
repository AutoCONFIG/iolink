package domain

import (
	"encoding/json"
	"time"
)

type APIKeyResourceScope struct {
	FarmIDs   []int64  `json:"farm_ids,omitempty"`
	PondIDs   []int64  `json:"pond_ids,omitempty"`
	DeviceNos []string `json:"device_nos,omitempty"`
}

type APIKey struct {
	KeyID     string              `json:"key_id"`
	TenantID  int64               `json:"tenant_id"`
	Name      string              `json:"name"`
	Scopes    []string            `json:"scopes"`
	Resources APIKeyResourceScope `json:"resources"`
	CreatedAt time.Time           `json:"created_at"`
	RevokedAt *time.Time          `json:"revoked_at,omitempty"`
}

type APIKeyAuditEvent struct {
	ID         int64           `json:"id"`
	TenantID   int64           `json:"tenant_id"`
	ActorID    *int64          `json:"actor_id,omitempty"`
	Action     string          `json:"action"`
	ResourceID string          `json:"resource_id"`
	Metadata   json.RawMessage `json:"metadata"`
	CreatedAt  time.Time       `json:"created_at"`
}

type OpenRequest struct {
	KeyID     string
	Timestamp int64
	Nonce     string
	Signature string
	Method    string
	Path      string
	Query     string
	Body      []byte
}

type OpenPrincipal struct {
	KeyID     string
	TenantID  int64
	Scopes    []string
	Resources APIKeyResourceScope
}
