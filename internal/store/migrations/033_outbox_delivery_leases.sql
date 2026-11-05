ALTER TABLE order_outbox_events
    ADD COLUMN IF NOT EXISTS claimed_by TEXT,
    ADD COLUMN IF NOT EXISTS claimed_until TIMESTAMPTZ;

ALTER TABLE email_outbox_events
    ADD COLUMN IF NOT EXISTS claimed_by TEXT,
    ADD COLUMN IF NOT EXISTS claimed_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS order_outbox_lease_lookup_idx
    ON order_outbox_events (claimed_until, available_at, id)
    WHERE published_at IS NULL;

CREATE INDEX IF NOT EXISTS email_outbox_lease_lookup_idx
    ON email_outbox_events (claimed_until, available_at, id)
    WHERE sent_at IS NULL;
