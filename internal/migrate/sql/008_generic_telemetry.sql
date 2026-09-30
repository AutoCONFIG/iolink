CREATE TABLE telemetry (
    ts TIMESTAMPTZ NOT NULL,
    device_no VARCHAR(64) NOT NULL,
    pond_id BIGINT NOT NULL REFERENCES ponds(id),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    model_version INTEGER NOT NULL,
    product_id BIGINT NOT NULL REFERENCES products(id),
    properties JSONB NOT NULL,
    UNIQUE (ts, device_no)
);
SELECT create_hypertable('telemetry', 'ts', if_not_exists => TRUE);
SELECT add_retention_policy('telemetry', INTERVAL '13 months', if_not_exists => TRUE);
CREATE INDEX idx_telemetry_tenant_device_ts ON telemetry(tenant_id, device_no, ts DESC);
CREATE INDEX idx_telemetry_device_model_ts ON telemetry(device_no, model_version, ts DESC);
UPDATE telemetry t SET product_id=d.product_id FROM devices d WHERE d.device_no=t.device_no AND t.product_id IS NULL;
ALTER TABLE telemetry ALTER COLUMN product_id SET NOT NULL;
ALTER TABLE device_shadows ADD COLUMN model_version INTEGER;
UPDATE device_shadows s SET model_version=d.model_version FROM devices d WHERE d.device_no=s.device_no AND s.model_version IS NULL;
ALTER TABLE device_shadows ALTER COLUMN model_version SET NOT NULL;
ALTER TABLE device_shadows ADD COLUMN product_id BIGINT REFERENCES products(id);
UPDATE device_shadows s SET product_id=d.product_id FROM devices d WHERE d.device_no=s.device_no AND s.product_id IS NULL;
ALTER TABLE device_shadows ALTER COLUMN product_id SET NOT NULL;
ALTER TABLE alarm_rules DROP CONSTRAINT IF EXISTS alarm_rule_valid;
ALTER TABLE alarm_rules ADD CONSTRAINT alarm_rule_valid_generic CHECK (
    metric ~ '^[a-z][a-z0-9_]{0,63}$' AND level IN ('warning','critical') AND
    (min_value IS NOT NULL OR max_value IS NOT NULL) AND
    (min_value IS NULL OR min_value > '-Infinity'::float8) AND
    (max_value IS NULL OR max_value < 'Infinity'::float8) AND
    (min_value IS NULL OR max_value IS NULL OR min_value < max_value)
);
ALTER TABLE alarms ADD COLUMN IF NOT EXISTS product_id BIGINT REFERENCES products(id);
UPDATE alarms a SET product_id=d.product_id FROM devices d WHERE d.device_no=a.device_no AND a.product_id IS NULL;
ALTER TABLE alarms ALTER COLUMN product_id SET DEFAULT iolink_default_product();
ALTER TABLE alarms ALTER COLUMN product_id SET NOT NULL;
DROP INDEX IF EXISTS alarms_one_open;
CREATE UNIQUE INDEX alarms_one_open ON alarms(device_no,pond_id,product_id,metric) WHERE confirmed_at IS NULL;
