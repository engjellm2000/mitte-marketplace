-- Supports per-order predecessor checks used by parallel relay workers.
CREATE INDEX IF NOT EXISTS order_outbox_unpublished_order_idx
    ON order_outbox_events (order_id, id)
    WHERE published_at IS NULL;
