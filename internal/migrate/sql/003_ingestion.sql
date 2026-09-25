-- M1: lifecycle, immutable pond snapshots and transactional ingestion.
ALTER TABLE devices ADD COLUMN disabled_at TIMESTAMPTZ;
ALTER TABLE devices ADD COLUMN report_interval INTEGER CHECK (report_interval IN (60,300));
ALTER TABLE sensor_data ADD COLUMN pond_id BIGINT;
-- The legacy schema never recorded a pond snapshot. Only currently registered
-- device attribution can be recovered; orphan legacy rows remain NULL.
UPDATE sensor_data s SET pond_id=d.pond_id FROM devices d WHERE s.device_no=d.device_no;
CREATE INDEX idx_sensor_data_pond_ts ON sensor_data(pond_id, ts DESC);
ALTER TABLE device_shadows ADD COLUMN pond_id BIGINT REFERENCES ponds(id);
ALTER TABLE device_shadows ADD COLUMN timestamps JSONB NOT NULL DEFAULT '{}';
UPDATE device_shadows s SET pond_id=d.pond_id FROM devices d WHERE s.device_no=d.device_no;
UPDATE device_shadows s SET timestamps=(SELECT coalesce(jsonb_object_agg(key,to_jsonb(s.ts)), '{}') FROM jsonb_object_keys(s.last) AS key);
CREATE TABLE ingest_messages (
    device_no VARCHAR(64) NOT NULL,
    message_id VARCHAR(128) NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY(device_no,message_id)
);
CREATE INDEX idx_ingest_messages_expiry ON ingest_messages(received_at);
-- Historical duplicate open alarms must not be silently confirmed or deleted.
-- If any exist, this unique index deliberately fails the entire migration;
-- operators must reconcile them explicitly before retrying.
CREATE UNIQUE INDEX alarms_one_open ON alarms(device_no,pond_id,metric) WHERE confirmed_at IS NULL;
ALTER TABLE alarm_rules ADD CONSTRAINT alarm_rule_valid CHECK (
    metric IN ('temperature','dissolved_oxygen','ph','turbidity','salinity') AND
    level IN ('warning','critical') AND
    (min_value IS NOT NULL OR max_value IS NOT NULL) AND
    (min_value IS NULL OR (min_value > '-Infinity'::float8 AND min_value < 'Infinity'::float8)) AND
    (max_value IS NULL OR (max_value > '-Infinity'::float8 AND max_value < 'Infinity'::float8)) AND
    (min_value IS NULL OR max_value IS NULL OR min_value < max_value)
);
CREATE TABLE notification_outbox (
    id BIGSERIAL PRIMARY KEY,
    alarm_id BIGINT NOT NULL UNIQUE REFERENCES alarms(id),
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
