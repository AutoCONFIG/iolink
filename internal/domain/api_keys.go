package domain

import "time"

type APIKeyResourceScope struct {
	FarmIDs   []int64  `json:"farm_ids,omitempty"`
	PondIDs   []int64  `json:"pond_ids,omitempty"`
	DeviceNos []string `json:"device_nos,omitempty"`
}

type APIKey struct {
	KeyID      string             `json:"key_id"`
	TenantID   int64              `json:"tenant_id"`
	Name       string             `json:"name"`
	Scopes     []string           `json:"scopes"`
	Resources  APIKeyResourceScope `json:"resources"`
	CreatedAt  time.Time          `json:"created_at"`
	RevokedAt  *time.Time         `json:"revoked_at,omitempty"`
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
