ALTER TABLE tenants ADD COLUMN IF NOT EXISTS permission_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE tenant_memberships ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE tenant_memberships ADD COLUMN IF NOT EXISTS permission_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE tenant_memberships ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='tenant_memberships'::regclass AND conname='tenant_memberships_role_valid') THEN
        ALTER TABLE tenant_memberships ADD CONSTRAINT tenant_memberships_role_valid CHECK (role IN ('owner','admin','member','viewer','support'));
    END IF;
END $$;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='tenant_memberships'::regclass AND conname='tenant_memberships_support_expiry') THEN
        ALTER TABLE tenant_memberships ADD CONSTRAINT tenant_memberships_support_expiry CHECK (role <> 'support' OR expires_at IS NOT NULL);
    END IF;
END $$;
INSERT INTO tenant_migration_reconciliation(migration_name,resource_type,resource_id,resolution)
SELECT '010_tenant_rbac','farm',f.id::text,'platform_admin_owner_preserved_without_membership'
FROM farms f JOIN users u ON u.id=f.owner_id WHERE u.authority='ADMIN';
INSERT INTO tenant_migration_reconciliation(migration_name,resource_type,resource_id,resolution)
SELECT '010_tenant_rbac','tenant_membership',tm.tenant_id::text || ':' || tm.user_id::text,'platform_admin_ordinary_membership_deactivated'
FROM tenant_memberships tm JOIN users u ON u.id=tm.user_id WHERE u.authority='ADMIN' AND tm.role<>'support';
UPDATE tenant_memberships tm SET active=FALSE,permission_version=permission_version+1
FROM users u WHERE u.id=tm.user_id AND u.authority='ADMIN' AND tm.role<>'support';
CREATE INDEX IF NOT EXISTS idx_tenant_memberships_active ON tenant_memberships(user_id, tenant_id) WHERE active;
INSERT INTO tenant_memberships(tenant_id,user_id,role)
SELECT f.tenant_id, f.owner_id, 'owner'
FROM farms f JOIN users u ON u.id=f.owner_id
WHERE u.authority='USER'
ON CONFLICT (tenant_id,user_id) DO NOTHING;
CREATE TABLE IF NOT EXISTS farm_memberships (
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    farm_id BIGINT NOT NULL REFERENCES farms(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner','admin','member','viewer','support')),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (farm_id,user_id),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);
UPDATE farm_memberships fm
SET tenant_id=f.tenant_id
FROM farms f
WHERE f.id=fm.farm_id AND fm.tenant_id<>f.tenant_id;
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='farms'::regclass AND conname='farms_id_tenant_unique') THEN
        ALTER TABLE farms ADD CONSTRAINT farms_id_tenant_unique UNIQUE (id, tenant_id);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='farm_memberships'::regclass AND conname='farm_memberships_farm_tenant_fk') THEN
        ALTER TABLE farm_memberships ADD CONSTRAINT farm_memberships_farm_tenant_fk FOREIGN KEY (farm_id, tenant_id) REFERENCES farms(id, tenant_id) ON DELETE CASCADE;
    END IF;
END $$;
ALTER TABLE farm_memberships ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='farm_memberships'::regclass AND conname='farm_memberships_support_expiry') THEN
        ALTER TABLE farm_memberships ADD CONSTRAINT farm_memberships_support_expiry CHECK (role <> 'support' OR expires_at IS NOT NULL);
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_farm_memberships_user ON farm_memberships(user_id,tenant_id,farm_id) WHERE active;
INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role)
SELECT f.tenant_id,f.id,f.owner_id,'owner' FROM farms f JOIN users u ON u.id=f.owner_id WHERE u.authority='USER'
ON CONFLICT (farm_id,user_id) DO NOTHING;
