ALTER TABLE video_gb_devices
    ADD COLUMN realm TEXT NOT NULL DEFAULT '',
    ADD COLUMN allowed_peer_cidrs CIDR[] NOT NULL DEFAULT '{}'::CIDR[];

ALTER TABLE video_gb_devices
    ADD CONSTRAINT video_gb_devices_realm_check
        CHECK (length(realm) <= 128 AND realm ~ '^[ -~]*$'),
    ADD CONSTRAINT video_gb_devices_allowed_peer_cidrs_check
        CHECK (cardinality(allowed_peer_cidrs) <= 64);

DROP INDEX IF EXISTS video_gb_devices_registered_peer_key;
CREATE UNIQUE INDEX video_gb_devices_registered_peer_key
    ON video_gb_devices (peer_address, peer_port, transport)
    WHERE registration_state = 'registered'
      AND peer_address IS NOT NULL
      AND peer_port IS NOT NULL
      AND transport IS NOT NULL;
