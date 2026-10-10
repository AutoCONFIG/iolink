-- M7a design proposal only. Not embedded or applied on service startup.
-- Apply after migrations 001..012 in a disposable database.
ALTER TABLE ponds ADD CONSTRAINT ponds_id_farm_video_unique UNIQUE (id, farm_id);

CREATE TABLE video_gb_devices (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    device_id TEXT NOT NULL UNIQUE CHECK (device_id ~ '^[0-9]{20}$'),
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    credential_cipher BYTEA NOT NULL CHECK (octet_length(credential_cipher) >= 29),
    credential_version BIGINT NOT NULL DEFAULT 1 CHECK (credential_version > 0),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    registration_state TEXT NOT NULL DEFAULT 'offline' CHECK (registration_state IN ('offline','registered')),
    registered_until TIMESTAMPTZ,
    last_keepalive_at TIMESTAMPTZ,
    peer_address INET,
    peer_port INTEGER CHECK (peer_port BETWEEN 1 AND 65535),
    transport TEXT CHECK (transport IN ('udp','tcp')),
    catalog_state TEXT NOT NULL DEFAULT 'empty' CHECK (catalog_state IN ('empty','refreshing','ready','stale','failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, tenant_id),
    CHECK (registration_state <> 'registered' OR
           (registered_until IS NOT NULL AND peer_address IS NOT NULL AND peer_port IS NOT NULL AND transport IS NOT NULL))
);

CREATE TABLE video_gb_channels (
    device_id BIGINT NOT NULL,
    tenant_id BIGINT NOT NULL,
    channel_id TEXT NOT NULL CHECK (channel_id ~ '^[0-9]{20}$'),
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    online BOOLEAN NOT NULL DEFAULT FALSE,
    present BOOLEAN NOT NULL DEFAULT TRUE,
    catalog_sn INTEGER NOT NULL CHECK (catalog_sn > 0),
    PRIMARY KEY (device_id, channel_id),
    UNIQUE (device_id, channel_id, tenant_id),
    FOREIGN KEY (device_id, tenant_id) REFERENCES video_gb_devices(id, tenant_id)
);

CREATE TABLE video_gb_catalogs (
    id UUID PRIMARY KEY,
    device_id BIGINT NOT NULL,
    tenant_id BIGINT NOT NULL,
    sn INTEGER NOT NULL CHECK (sn > 0),
    expected_count INTEGER CHECK (expected_count BETWEEN 0 AND 1000),
    expires_at TIMESTAMPTZ NOT NULL,
    state TEXT NOT NULL DEFAULT 'collecting' CHECK (state IN ('collecting','complete','failed')),
    UNIQUE (device_id, sn),
    FOREIGN KEY (device_id, tenant_id) REFERENCES video_gb_devices(id, tenant_id)
);
CREATE TABLE video_gb_catalog_items (
    catalog_id UUID NOT NULL REFERENCES video_gb_catalogs(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL CHECK (channel_id ~ '^[0-9]{20}$'),
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    online BOOLEAN NOT NULL,
    PRIMARY KEY (catalog_id, channel_id)
);

CREATE TABLE video_cameras (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    farm_id BIGINT NOT NULL,
    pond_id BIGINT NOT NULL,
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    source_kind TEXT NOT NULL CHECK (source_kind IN ('rtsp','gb28181')),
    source_version BIGINT NOT NULL DEFAULT 1 CHECK (source_version > 0),
    rtsp_uri TEXT CHECK (length(rtsp_uri) BETWEEN 1 AND 2048),
    credential_cipher BYTEA CHECK (octet_length(credential_cipher) >= 29),
    gb_device_id BIGINT,
    gb_channel_id TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    status TEXT NOT NULL DEFAULT 'offline' CHECK (status IN ('offline','connecting','ready','failed')),
    error_code TEXT CHECK (error_code IN ('source_unreachable','unsupported_codec','provider_unavailable','registration_expired','catalog_stale')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, tenant_id),
    FOREIGN KEY (farm_id, tenant_id) REFERENCES farms(id, tenant_id),
    FOREIGN KEY (pond_id, farm_id) REFERENCES ponds(id, farm_id),
    FOREIGN KEY (gb_device_id, gb_channel_id, tenant_id) REFERENCES video_gb_channels(device_id, channel_id, tenant_id),
    CHECK ((source_kind='rtsp' AND rtsp_uri IS NOT NULL AND gb_device_id IS NULL AND gb_channel_id IS NULL)
        OR (source_kind='gb28181' AND rtsp_uri IS NULL AND credential_cipher IS NULL
            AND gb_device_id IS NOT NULL AND gb_channel_id IS NOT NULL))
);
CREATE INDEX idx_video_cameras_tenant_pond ON video_cameras(tenant_id, pond_id, id) WHERE enabled;

CREATE TABLE video_streams (
    id UUID PRIMARY KEY,
    camera_id BIGINT NOT NULL,
    tenant_id BIGINT NOT NULL,
    source_version BIGINT NOT NULL CHECK (source_version > 0),
    state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','starting','ready','stopping','stopped','failed','unknown')),
    rtp_port INTEGER CHECK (rtp_port BETWEEN 30000 AND 30038 AND rtp_port % 2 = 0),
    ssrc TEXT CHECK (ssrc ~ '^[0-9]{10}$'),
    call_id UUID,
    cseq BIGINT CHECK (cseq > 0),
    dialog_json JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (camera_id, source_version),
    UNIQUE (id, camera_id, tenant_id, source_version),
    FOREIGN KEY (camera_id, tenant_id) REFERENCES video_cameras(id, tenant_id),
    CHECK ((rtp_port IS NULL AND ssrc IS NULL AND call_id IS NULL AND cseq IS NULL)
        OR (rtp_port IS NOT NULL AND ssrc IS NOT NULL AND call_id IS NOT NULL AND cseq IS NOT NULL))
);
CREATE UNIQUE INDEX idx_video_rtp_port_active ON video_streams(rtp_port)
    WHERE rtp_port IS NOT NULL AND state <> 'stopped';

CREATE TABLE video_sessions (
    id UUID PRIMARY KEY,
    stream_id UUID NOT NULL,
    camera_id BIGINT NOT NULL,
    tenant_id BIGINT NOT NULL,
    source_version BIGINT NOT NULL CHECK (source_version > 0),
    user_id BIGINT NOT NULL REFERENCES users(id),
    user_token_version BIGINT NOT NULL CHECK (user_token_version >= 0),
    tenant_permission_version BIGINT NOT NULL CHECK (tenant_permission_version >= 0),
    member_permission_version BIGINT NOT NULL CHECK (member_permission_version >= 0),
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
    state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','ready','failed','revoked','expired')),
    error_code TEXT CHECK (error_code IN ('source_unreachable','unsupported_codec','provider_unavailable','registration_expired','catalog_stale')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    FOREIGN KEY (stream_id, camera_id, tenant_id, source_version)
        REFERENCES video_streams(id, camera_id, tenant_id, source_version),
    CHECK (expires_at > created_at AND expires_at <= created_at + interval '5 minutes'),
    CHECK ((state='revoked') = (revoked_at IS NOT NULL))
);
CREATE INDEX idx_video_sessions_live ON video_sessions(camera_id, expires_at) WHERE state IN ('pending','ready');
CREATE INDEX idx_video_sessions_user ON video_sessions(tenant_id, user_id, id);

CREATE TABLE video_segments (
    session_id UUID NOT NULL REFERENCES video_sessions(id) ON DELETE CASCADE,
    segment_id UUID NOT NULL,
    provider_name TEXT NOT NULL CHECK (provider_name ~ '^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])/([01][0-9]|2[0-3])/[0-5][0-9]-[0-5][0-9]_[0-9]+[.]ts$' AND length(provider_name)<=128),
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (session_id, segment_id),
    UNIQUE (session_id, provider_name)
);
