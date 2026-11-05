CREATE TABLE warehouse_return_cases (
    id BIGSERIAL PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id),
    product_id BIGINT NOT NULL REFERENCES products(id),
    product_name TEXT NOT NULL,
    sku TEXT NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    received_quantity INTEGER NOT NULL DEFAULT 0 CHECK (received_quantity >= 0 AND received_quantity <= quantity),
    request_type TEXT NOT NULL CHECK (request_type IN ('return', 'replacement_with_return')),
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'expected' CHECK (status IN ('expected', 'received', 'restocked', 'damaged', 'not_restockable')),
    parcel_reference TEXT NOT NULL DEFAULT '',
    received_by TEXT NOT NULL DEFAULT '',
    received_at TIMESTAMPTZ,
    inspected_by TEXT NOT NULL DEFAULT '',
    inspected_at TIMESTAMPTZ,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX warehouse_return_cases_status_created_idx
    ON warehouse_return_cases (status, created_at DESC, id DESC);
CREATE UNIQUE INDEX warehouse_return_cases_order_product_idx
    ON warehouse_return_cases (order_id, product_name);

ALTER TABLE product_inventory_movements
    DROP CONSTRAINT IF EXISTS product_inventory_movements_movement_type_check;
ALTER TABLE product_inventory_movements
    ADD CONSTRAINT product_inventory_movements_movement_type_check
    CHECK (movement_type IN ('opening_balance', 'receipt', 'adjustment', 'reservation', 'reservation_release', 'return_restock'));
