ALTER TABLE devices
    ADD COLUMN mqtt_username VARCHAR(128) UNIQUE;

-- Backfill issued usernames for existing devices so they can re-adopt the
-- ThingsPanel-style credential flow (username is viewable in the console).
-- Same shape as registration: iolink-<device_no>-<6hex>.
UPDATE devices SET mqtt_username = 'iolink-' || device_no || '-' || substr(md5(random()::text || clock_timestamp()::text), 1, 6)
WHERE mqtt_username IS NULL;
