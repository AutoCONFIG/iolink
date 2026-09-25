-- R03/R04: repairs the legacy telemetry column and permits salted password hashes.
ALTER TABLE sensor_data ADD COLUMN signal INTEGER;
ALTER TABLE users ALTER COLUMN password_hash TYPE TEXT;
ALTER TABLE users ADD COLUMN token_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE users SET must_change_password=TRUE
WHERE authority='ADMIN' AND password_hash='1cd663ce3300b9f52a357c4ae4e114064b0fa066071728aca1d7a98f5916f2e0';
CREATE TABLE audit_events (
    id BIGSERIAL PRIMARY KEY,
    actor_id BIGINT,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
