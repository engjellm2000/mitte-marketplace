ALTER TABLE order_items
    ADD COLUMN IF NOT EXISTS supplier_name TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS order_items_supplier_name_idx
    ON order_items (supplier_name, order_id) WHERE supplier_name <> '';

CREATE TABLE IF NOT EXISTS supplier_order_events (
    id BIGSERIAL PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL,
    product_name TEXT NOT NULL,
    supplier_name TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN (
        'order_accepted', 'shipment_reported', 'delay_reported', 'cancellation_requested'
    )),
    note TEXT NOT NULL DEFAULT '',
    delayed_delivery_date DATE,
    carrier TEXT NOT NULL DEFAULT '',
    tracking_number TEXT NOT NULL DEFAULT '',
    tracking_url TEXT NOT NULL DEFAULT '',
    actor_username TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    resolved_by TEXT NOT NULL DEFAULT '',
    resolution_note TEXT NOT NULL DEFAULT '',
    resolution_status TEXT NOT NULL DEFAULT '' CHECK (resolution_status IN ('', 'handled', 'rejected'))
);

CREATE INDEX IF NOT EXISTS supplier_order_events_order_idx
    ON supplier_order_events (order_id, product_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS supplier_order_events_pending_idx
    ON supplier_order_events (created_at DESC, id DESC)
    WHERE event_type = 'cancellation_requested' AND resolved_at IS NULL;
