CREATE TABLE IF NOT EXISTS order_item_operations (
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'ready_to_process' CHECK (status IN ('ready_to_process','stock_confirmed','payment_received','exported','fulfillment','return_open','fulfillment_completed','cancelled','refunded','replacement_created')),
    original_delivery_date DATE,
    delayed_delivery_date DATE,
    updated_by TEXT NOT NULL DEFAULT 'system',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (order_id, product_name)
);
CREATE TABLE IF NOT EXISTS order_item_operation_history (
    id BIGSERIAL PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_name TEXT NOT NULL,
    action TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    actor_email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS order_item_operation_history_order_idx ON order_item_operation_history(order_id, created_at DESC);
