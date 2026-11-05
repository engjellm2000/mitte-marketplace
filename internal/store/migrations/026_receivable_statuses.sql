-- A receivable has a lifecycle independent from order fulfilment and the raw
-- bank-transaction state. Keep its transitions auditable for the development
-- payment portal.
ALTER TABLE payment_transactions ADD COLUMN IF NOT EXISTS receivable_status TEXT NOT NULL DEFAULT 'created'
    CHECK (receivable_status IN ('created', 'sent', 'declined', 'paid', 'paid_out', 'completed', 'overdue', 'cancelled'));

UPDATE payment_transactions p
SET receivable_status = CASE
    WHEN p.status = 'received' THEN 'paid'
    WHEN i.order_id IS NOT NULL THEN 'sent'
    ELSE 'created'
END
FROM invoices i
WHERE i.order_id = p.order_id;

CREATE TABLE IF NOT EXISTS receivable_status_history (
    id BIGSERIAL PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    from_status TEXT,
    to_status TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    changed_by TEXT NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS receivable_status_history_order_created_idx ON receivable_status_history(order_id, created_at DESC);
