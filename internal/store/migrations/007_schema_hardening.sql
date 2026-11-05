-- Validate the financial snapshot after migration 005 backfilled legacy rows.
ALTER TABLE orders
    VALIDATE CONSTRAINT orders_financial_snapshot_check;

-- A promotion has exactly one pricing mechanism. Keeping that invariant in the
-- database prevents invalid administrator or integration writes from reaching
-- quote calculation.
ALTER TABLE promotions
    ADD CONSTRAINT promotions_definition_check
    CHECK (
        (kind = 'percent' AND percent_bps BETWEEN 1 AND 10000 AND amount_cents = 0)
        OR (kind = 'fixed' AND percent_bps = 0 AND amount_cents > 0)
        OR (kind = 'free_shipping' AND percent_bps = 0 AND amount_cents = 0)
    ) NOT VALID,
    ADD CONSTRAINT promotions_max_redemptions_check
    CHECK (max_redemptions IS NULL OR max_redemptions > 0) NOT VALID;

ALTER TABLE promotions
    VALIDATE CONSTRAINT promotions_definition_check,
    VALIDATE CONSTRAINT promotions_max_redemptions_check;

-- These indexes match the predicates used for per-customer promotion limits
-- and chronological order-status lookups.
CREATE INDEX IF NOT EXISTS promotion_redemptions_code_email_idx
    ON promotion_redemptions (code, customer_email);

CREATE INDEX IF NOT EXISTS order_status_history_order_created_at_idx
    ON order_status_history (order_id, created_at DESC);
