-- M2: nullable farm ownership, device lifecycle and token/session revisions.
ALTER TABLE farms ALTER COLUMN owner_id DROP NOT NULL;
ALTER TABLE devices ADD COLUMN session_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE devices ADD COLUMN name VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE users ADD CONSTRAINT users_authority_valid CHECK (authority IN ('USER','ADMIN'));
CREATE INDEX idx_users_user_nickname ON users(authority, nickname, id);
