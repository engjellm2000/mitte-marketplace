CREATE TABLE IF NOT EXISTS warehouse_fulfillments (
    order_id TEXT PRIMARY KEY REFERENCES orders(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('ready_to_pick', 'picking', 'packed', 'label_ready', 'handed_over', 'cancelled')),
    assigned_to TEXT,
    carrier TEXT,
    tracking_number TEXT,
    tracking_url TEXT,
    label_url TEXT,
    picked_at TIMESTAMPTZ,
    packed_at TIMESTAMPTZ,
    handed_over_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS warehouse_fulfillments_status_idx ON warehouse_fulfillments (status, updated_at);
CREATE TABLE IF NOT EXISTS warehouse_events (
    id BIGSERIAL PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    actor_email TEXT NOT NULL,
    event_type TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS warehouse_events_order_created_idx ON warehouse_events (order_id, created_at DESC);
INSERT INTO warehouse_fulfillments (order_id, status, handed_over_at)
SELECT id, CASE WHEN status = 'fulfilled' THEN 'handed_over' ELSE 'ready_to_pick' END,
       CASE WHEN status = 'fulfilled' THEN NOW() ELSE NULL END
FROM orders WHERE status IN ('paid', 'fulfilled')
ON CONFLICT (order_id) DO NOTHING;
