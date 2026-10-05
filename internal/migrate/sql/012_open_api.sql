CREATE TABLE IF NOT EXISTS api_keys (
    key_id TEXT PRIMARY KEY CHECK (key_id ~ '^ik_[A-Za-z0-9_-]{22}$'),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 128),
    scopes TEXT[] NOT NULL CHECK (cardinality(scopes) > 0),
    resources JSONB NOT NULL DEFAULT '{}'::jsonb,
    encrypted_secret BYTEA NOT NULL,
    secret_nonce BYTEA NOT NULL,
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    rotated_from TEXT REFERENCES api_keys(key_id),
    rate_tokens DOUBLE PRECISION NOT NULL DEFAULT 10,
    rate_last_refill TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_api_keys_tenant ON api_keys(tenant_id) WHERE revoked_at IS NULL;
CREATE TABLE IF NOT EXISTS api_key_nonces (
    key_id TEXT NOT NULL REFERENCES api_keys(key_id) ON DELETE CASCADE,
    nonce_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (key_id, nonce_hash)
);
CREATE INDEX IF NOT EXISTS idx_api_key_nonces_expiry ON api_key_nonces(expires_at);
