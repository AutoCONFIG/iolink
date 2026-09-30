CREATE TABLE telemetry_backfill_checkpoints (
    job_name TEXT PRIMARY KEY,
    last_device_no VARCHAR(64) NOT NULL DEFAULT '',
    last_ts TIMESTAMPTZ NOT NULL DEFAULT 'epoch',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
