-- Catalog management: lifecycle, safe editing, explicit inventory adjustments and auditability.
ALTER TABLE products
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS seo_title TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS seo_description TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

ALTER TABLE products DROP CONSTRAINT IF EXISTS products_status_check;
ALTER TABLE products ADD CONSTRAINT products_status_check
    CHECK (status IN ('draft', 'active', 'scheduled', 'archived'));

CREATE INDEX IF NOT EXISTS products_status_idx ON products(status);
CREATE INDEX IF NOT EXISTS products_updated_at_idx ON products(updated_at DESC);

CREATE TABLE IF NOT EXISTS product_inventory_adjustments (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(id),
    delta INTEGER NOT NULL CHECK (delta <> 0),
    inventory_before INTEGER NOT NULL CHECK (inventory_before >= 0),
    inventory_after INTEGER NOT NULL CHECK (inventory_after >= 0),
    reason TEXT NOT NULL,
    reference TEXT NOT NULL DEFAULT '',
    actor_email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS product_inventory_adjustments_product_idx
    ON product_inventory_adjustments(product_id, created_at DESC);

CREATE TABLE IF NOT EXISTS product_audit_log (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT REFERENCES products(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    changes JSONB NOT NULL DEFAULT '{}'::jsonb,
    actor_email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS product_audit_log_product_idx
    ON product_audit_log(product_id, created_at DESC);

-- Existing products are immediately publishable; subsequent writes update this timestamp.
UPDATE products SET updated_at = created_at WHERE updated_at IS NULL;
