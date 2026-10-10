ALTER TABLE video_gb_devices
    ADD COLUMN registered_at TIMESTAMPTZ;

ALTER TABLE video_gb_devices
    ADD CONSTRAINT video_gb_devices_registered_fields_check
    CHECK (registration_state <> 'registered'
        OR (registered_at IS NOT NULL
            AND registered_until IS NOT NULL
            AND peer_address IS NOT NULL
            AND peer_port IS NOT NULL
            AND transport IS NOT NULL));

CREATE UNIQUE INDEX video_gb_devices_registered_peer_key
    ON video_gb_devices (peer_address, peer_port)
    WHERE registration_state = 'registered'
      AND peer_address IS NOT NULL
      AND peer_port IS NOT NULL;

ALTER TABLE video_gb_catalogs
    ADD COLUMN received_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN received_pages INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN last_page_at TIMESTAMPTZ,
    ADD COLUMN completed_at TIMESTAMPTZ,
    ADD COLUMN failed_at TIMESTAMPTZ;

ALTER TABLE video_gb_catalogs
    ADD CONSTRAINT video_gb_catalogs_received_count_check
        CHECK (received_count BETWEEN 0 AND 1000),
    ADD CONSTRAINT video_gb_catalogs_received_pages_check
        CHECK (received_pages BETWEEN 0 AND 1000),
    ADD CONSTRAINT video_gb_catalogs_expected_received_check
        CHECK (expected_count IS NULL OR received_count <= expected_count),
    ADD CONSTRAINT video_gb_catalogs_state_timestamps_check
        CHECK ((state = 'complete') = (completed_at IS NOT NULL)
            AND (state = 'failed') = (failed_at IS NOT NULL)
            AND (state <> 'complete' OR (expected_count IS NOT NULL AND received_count = expected_count))
            AND ((state = 'collecting' AND completed_at IS NULL AND failed_at IS NULL)
                OR state IN ('complete','failed')));

CREATE INDEX video_gb_catalogs_device_state_expiry
    ON video_gb_catalogs (device_id, state, expires_at);

CREATE INDEX video_gb_channels_device_present_id
    ON video_gb_channels (device_id, present, channel_id);
