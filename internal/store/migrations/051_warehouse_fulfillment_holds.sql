ALTER TABLE warehouse_fulfillments DROP CONSTRAINT IF EXISTS warehouse_fulfillments_status_check;
ALTER TABLE warehouse_fulfillments
    ADD CONSTRAINT warehouse_fulfillments_status_check
    CHECK (status IN ('ready_to_pick', 'picking', 'packed', 'label_ready', 'held', 'handed_over', 'cancelled'));

ALTER TABLE warehouse_fulfillments
    ADD COLUMN IF NOT EXISTS hold_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS held_by TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS held_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS resume_status TEXT NOT NULL DEFAULT ''
        CHECK (resume_status IN ('', 'ready_to_pick', 'picking', 'packed', 'label_ready'));

CREATE INDEX IF NOT EXISTS warehouse_fulfillments_held_idx
    ON warehouse_fulfillments (held_at ASC, order_id ASC) WHERE status = 'held';
