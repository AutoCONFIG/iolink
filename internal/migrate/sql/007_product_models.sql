CREATE TABLE products (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);
CREATE TABLE product_models (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL CHECK (version > 0),
    schema JSONB NOT NULL,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, version)
);
INSERT INTO products(tenant_id, name)
SELECT id, 'water-quality' FROM tenants WHERE name='__iolink_system__'
ON CONFLICT (tenant_id, name) DO NOTHING;
INSERT INTO product_models(product_id, version, schema, published_at)
SELECT p.id, 1,
       '{"fields":[
          {"identifier":"temperature","type":"number","unit":"℃","readable":true,"writable":false,"nullable":true},
          {"identifier":"dissolved_oxygen","type":"number","unit":"mg/L","readable":true,"writable":false,"nullable":true},
          {"identifier":"ph","type":"number","unit":"","readable":true,"writable":false,"nullable":true},
          {"identifier":"turbidity","type":"number","unit":"NTU","readable":true,"writable":false,"nullable":true},
          {"identifier":"salinity","type":"number","unit":"ppt","readable":true,"writable":false,"nullable":true}
       ]}'::jsonb, now()
FROM products p JOIN tenants t ON t.id=p.tenant_id
WHERE t.name='__iolink_system__' AND p.name='water-quality'
ON CONFLICT (product_id, version) DO NOTHING;
CREATE OR REPLACE FUNCTION iolink_default_product() RETURNS BIGINT LANGUAGE sql STABLE AS $$
    SELECT p.id FROM products p JOIN tenants t ON t.id=p.tenant_id
    WHERE t.name='__iolink_system__' AND p.name='water-quality' LIMIT 1
$$;
ALTER TABLE devices ADD COLUMN product_id BIGINT DEFAULT iolink_default_product();
ALTER TABLE devices ADD COLUMN model_version INTEGER DEFAULT 1;
ALTER TABLE devices ADD COLUMN assigned_at TIMESTAMPTZ NOT NULL DEFAULT now();
UPDATE devices d SET product_id=p.id, model_version=1
FROM products p JOIN tenants t ON t.id=p.tenant_id
WHERE t.name='__iolink_system__' AND p.name='water-quality' AND d.product_id IS NULL;
ALTER TABLE devices ALTER COLUMN product_id SET NOT NULL;
ALTER TABLE devices ALTER COLUMN model_version SET NOT NULL;
ALTER TABLE devices ADD CONSTRAINT devices_product_fk FOREIGN KEY (product_id) REFERENCES products(id);
ALTER TABLE devices ADD CONSTRAINT devices_product_model_fk FOREIGN KEY (product_id, model_version)
    REFERENCES product_models(product_id, version);
CREATE INDEX idx_products_tenant ON products(tenant_id, id);
CREATE INDEX idx_product_models_product ON product_models(product_id, version);
CREATE OR REPLACE FUNCTION reject_product_model_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.published_at IS NOT NULL OR EXISTS (SELECT 1 FROM devices WHERE product_id=OLD.product_id AND model_version=OLD.version) THEN
            RAISE EXCEPTION 'published or referenced product model is immutable';
        END IF;
        RETURN OLD;
    END IF;
    IF OLD.published_at IS NOT NULL OR OLD.product_id <> NEW.product_id OR OLD.version <> NEW.version THEN
        RAISE EXCEPTION 'published or referenced product model is immutable';
    END IF;
    IF EXISTS (SELECT 1 FROM devices WHERE product_id=OLD.product_id AND model_version=OLD.version)
       AND (OLD.schema IS DISTINCT FROM NEW.schema) THEN
        RAISE EXCEPTION 'referenced product model schema is immutable';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER product_models_immutable BEFORE UPDATE OR DELETE ON product_models
    FOR EACH ROW EXECUTE FUNCTION reject_product_model_mutation();
