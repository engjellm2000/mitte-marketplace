ALTER TABLE products
    ADD COLUMN IF NOT EXISTS barcode TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS supplier_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS unit_cost_cents INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS reorder_point INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS lead_time_days INTEGER NOT NULL DEFAULT 0;

ALTER TABLE products
    DROP CONSTRAINT IF EXISTS products_unit_cost_cents_check,
    ADD CONSTRAINT products_unit_cost_cents_check CHECK (unit_cost_cents >= 0),
    DROP CONSTRAINT IF EXISTS products_reorder_point_check,
    ADD CONSTRAINT products_reorder_point_check CHECK (reorder_point >= 0),
    DROP CONSTRAINT IF EXISTS products_lead_time_days_check,
    ADD CONSTRAINT products_lead_time_days_check CHECK (lead_time_days >= 0);

ALTER TABLE products ALTER COLUMN sku SET DEFAULT (
    'MF-' || CASE WHEN nextval('product_sku_seq') < 1000000
        THEN LPAD(currval('product_sku_seq')::text, 6, '0')
        ELSE currval('product_sku_seq')::text
    END
);

CREATE UNIQUE INDEX IF NOT EXISTS products_barcode_idx
    ON products (barcode) WHERE barcode <> '';
CREATE INDEX IF NOT EXISTS products_inventory_reorder_idx
    ON products (inventory, reorder_point) WHERE status <> 'archived';

CREATE TABLE IF NOT EXISTS product_inventory_movements (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(id),
    movement_type TEXT NOT NULL CHECK (movement_type IN (
        'opening_balance', 'receipt', 'adjustment', 'reservation', 'reservation_release'
    )),
    quantity_delta INTEGER NOT NULL CHECK (quantity_delta <> 0),
    inventory_before INTEGER NOT NULL CHECK (inventory_before >= 0),
    inventory_after INTEGER NOT NULL CHECK (inventory_after >= 0),
    reason TEXT NOT NULL,
    reference TEXT NOT NULL DEFAULT '',
    actor_email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (inventory_after = inventory_before + quantity_delta)
);
CREATE INDEX IF NOT EXISTS product_inventory_movements_product_idx
    ON product_inventory_movements (product_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS product_inventory_movements_created_idx
    ON product_inventory_movements (created_at DESC, id DESC);

-- Establish a truthful starting point for the new available-stock ledger.
INSERT INTO product_inventory_movements (
    product_id, movement_type, quantity_delta, inventory_before, inventory_after,
    reason, actor_email
)
SELECT id, 'opening_balance', inventory, 0, inventory,
       'Available stock snapshot at inventory-ledger launch', 'system:migration-030'
FROM products
WHERE inventory > 0;
