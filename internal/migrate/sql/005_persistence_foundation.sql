CREATE TABLE IF NOT EXISTS tenants (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS tenants_name_key ON tenants(name);
INSERT INTO tenants(name, active) VALUES ('__iolink_system__', TRUE)
ON CONFLICT (name) DO UPDATE SET active=TRUE;
CREATE TABLE IF NOT EXISTS tenant_migration_reconciliation (
    id BIGSERIAL PRIMARY KEY,
    migration_name TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    resolution TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS tenant_memberships (
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_tenant_memberships_user ON tenant_memberships(user_id, tenant_id);

ALTER TABLE sensor_data ADD COLUMN IF NOT EXISTS tenant_id BIGINT REFERENCES tenants(id);
CREATE INDEX IF NOT EXISTS idx_sensor_data_tenant_device_ts ON sensor_data(tenant_id, device_no, ts DESC);
ALTER TABLE device_shadows ADD COLUMN IF NOT EXISTS tenant_id BIGINT REFERENCES tenants(id);
CREATE INDEX IF NOT EXISTS idx_device_shadows_tenant ON device_shadows(tenant_id, device_no);
ALTER TABLE farms ADD COLUMN IF NOT EXISTS tenant_id BIGINT REFERENCES tenants(id);
INSERT INTO tenant_migration_reconciliation(migration_name,resource_type,resource_id,resolution)
SELECT '005_persistence_foundation','farm',f.id::text,'assigned_default_tenant'
FROM farms f
WHERE f.tenant_id IS NULL;
UPDATE farms
SET tenant_id = (SELECT id FROM tenants WHERE name='__iolink_system__' AND active LIMIT 1)
WHERE tenant_id IS NULL;
DO $$
DECLARE
    null_count BIGINT;
BEGIN
    SELECT count(*) INTO null_count FROM farms WHERE tenant_id IS NULL;
    IF null_count > 0 THEN
        RAISE EXCEPTION 'farms contains % NULL tenant_id rows; operator must backfill fixture ownership and retry', null_count;
    END IF;
END $$;
ALTER TABLE farms ALTER COLUMN tenant_id SET NOT NULL;
UPDATE sensor_data sd
SET tenant_id = f.tenant_id
FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id
WHERE sd.device_no=d.device_no AND sd.tenant_id IS NULL;
INSERT INTO tenant_migration_reconciliation(migration_name,resource_type,resource_id,resolution)
SELECT '005_persistence_foundation','sensor_data',sd.device_no || ':' || sd.ts::text,'assigned_default_tenant_orphan'
FROM sensor_data sd
WHERE sd.tenant_id IS NULL;
UPDATE device_shadows ds
SET tenant_id = f.tenant_id
FROM devices d JOIN ponds p ON p.id=d.pond_id JOIN farms f ON f.id=p.farm_id
WHERE ds.device_no=d.device_no AND ds.tenant_id IS NULL;
INSERT INTO tenant_migration_reconciliation(migration_name,resource_type,resource_id,resolution)
SELECT '005_persistence_foundation','device_shadow',ds.device_no,'assigned_default_tenant_orphan'
FROM device_shadows ds
WHERE ds.tenant_id IS NULL;
UPDATE sensor_data
SET tenant_id = (SELECT id FROM tenants WHERE name='__iolink_system__' AND active LIMIT 1)
WHERE tenant_id IS NULL;
UPDATE device_shadows
SET tenant_id = (SELECT id FROM tenants WHERE name='__iolink_system__' AND active LIMIT 1)
WHERE tenant_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_farms_tenant_owner ON farms(tenant_id, owner_id);
INSERT INTO tenant_migration_reconciliation(migration_name,resource_type,resource_id,resolution)
SELECT '005_persistence_foundation','pond',p.id::text,'associated_with_farm:' || p.farm_id::text
FROM ponds p;
INSERT INTO tenant_migration_reconciliation(migration_name,resource_type,resource_id,resolution)
SELECT '005_persistence_foundation','device',d.device_no,'associated_with_pond:' || d.pond_id::text
FROM devices d;

CREATE TABLE IF NOT EXISTS jobs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    kind TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled','running','succeeded','retryable','failed','cancelled')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    leased_until TIMESTAMPTZ,
    lease_owner TEXT,
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, kind, idempotency_key),
    CHECK (length(trim(kind)) > 0 AND length(trim(idempotency_key)) > 0)
);
CREATE INDEX IF NOT EXISTS idx_jobs_claim ON jobs(status, available_at, leased_until);

CREATE TABLE IF NOT EXISTS audit_events (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT REFERENCES tenants(id),
    actor_id BIGINT REFERENCES users(id),
    action TEXT NOT NULL CHECK (length(trim(action)) > 0),
    resource_type TEXT NOT NULL CHECK (length(trim(resource_type)) > 0),
    resource_id TEXT NOT NULL CHECK (length(trim(resource_id)) > 0),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS tenant_id BIGINT REFERENCES tenants(id);
ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='audit_events'::regclass AND conname='audit_events_action_nonempty') THEN
        ALTER TABLE audit_events ADD CONSTRAINT audit_events_action_nonempty CHECK (length(trim(action)) > 0);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='audit_events'::regclass AND conname='audit_events_resource_type_nonempty') THEN
        ALTER TABLE audit_events ADD CONSTRAINT audit_events_resource_type_nonempty CHECK (length(trim(resource_type)) > 0);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='audit_events'::regclass AND conname='audit_events_resource_id_nonempty') THEN
        ALTER TABLE audit_events ADD CONSTRAINT audit_events_resource_id_nonempty CHECK (length(trim(resource_id)) > 0);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='audit_events'::regclass AND conname='audit_events_actor_fk') THEN
        ALTER TABLE audit_events ADD CONSTRAINT audit_events_actor_fk FOREIGN KEY (actor_id) REFERENCES users(id);
    END IF;
END $$;
INSERT INTO tenant_migration_reconciliation(migration_name,resource_type,resource_id,resolution)
SELECT '005_persistence_foundation','audit_event',ae.id::text,'assigned_default_tenant_orphan'
FROM audit_events ae
WHERE ae.tenant_id IS NULL;
UPDATE audit_events
SET tenant_id = (SELECT id FROM tenants WHERE name='__iolink_system__' AND active LIMIT 1)
WHERE tenant_id IS NULL;
DO $$
DECLARE
    null_count BIGINT;
BEGIN
    SELECT count(*) INTO null_count FROM audit_events WHERE tenant_id IS NULL;
    IF null_count > 0 THEN
        RAISE EXCEPTION 'audit_events contains % NULL tenant_id rows; operator must backfill fixture ownership and retry', null_count;
    END IF;
END $$;
ALTER TABLE audit_events ALTER COLUMN tenant_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_audit_tenant_time ON audit_events(tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS outbox_events (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    topic TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','sent','retryable','failed')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, topic, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_outbox_claim ON outbox_events(status, available_at);
