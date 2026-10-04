CREATE TABLE IF NOT EXISTS deployment_config (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    deployment_id TEXT NOT NULL UNIQUE CHECK (length(trim(deployment_id)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO deployment_config(singleton, deployment_id) VALUES (TRUE, gen_random_uuid()::text)
ON CONFLICT (singleton) DO NOTHING;
CREATE TABLE IF NOT EXISTS license_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    payload BYTEA,
    signature BYTEA,
    payload_sha256 TEXT CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    imported_at TIMESTAMPTZ,
    imported_by BIGINT REFERENCES users(id),
    CHECK ((payload IS NULL AND signature IS NULL AND payload_sha256 IS NULL AND imported_at IS NULL)
        OR (payload IS NOT NULL AND signature IS NOT NULL AND payload_sha256 IS NOT NULL AND imported_at IS NOT NULL))
);
INSERT INTO license_state(singleton) VALUES (TRUE) ON CONFLICT (singleton) DO NOTHING;
CREATE TABLE IF NOT EXISTS license_clock (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    max_seen_at TIMESTAMPTZ,
    clock_error BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO license_clock(singleton) VALUES (TRUE) ON CONFLICT (singleton) DO NOTHING;
