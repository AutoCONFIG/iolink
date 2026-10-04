CREATE TABLE IF NOT EXISTS deployment_config (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    deployment_id TEXT NOT NULL UNIQUE CHECK (length(trim(deployment_id)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO deployment_config(singleton, deployment_id) VALUES (TRUE, gen_random_uuid()::text)
ON CONFLICT (singleton) DO NOTHING;
CREATE TABLE IF NOT EXISTS license_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    license_id TEXT,
    deployment_id TEXT,
    key_id TEXT,
    issued_at TIMESTAMPTZ,
    not_before TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    max_devices BIGINT NOT NULL DEFAULT 0 CHECK (max_devices >= 0),
    features JSONB NOT NULL DEFAULT '[]'::jsonb,
    payload BYTEA,
    signature BYTEA,
    payload_sha256 TEXT,
    state TEXT NOT NULL DEFAULT 'missing' CHECK (state IN ('missing','valid','permanent','not_before','expired','invalid','instance_mismatch','clock_error','overage')),
    imported_at TIMESTAMPTZ,
    imported_by BIGINT REFERENCES users(id)
);
INSERT INTO license_state(singleton) VALUES (TRUE) ON CONFLICT (singleton) DO NOTHING;
CREATE TABLE IF NOT EXISTS license_clock (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    max_seen_at TIMESTAMPTZ,
    clock_error BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO license_clock(singleton) VALUES (TRUE) ON CONFLICT (singleton) DO NOTHING;
