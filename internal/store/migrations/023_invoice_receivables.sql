-- The development invoice provider needs a real ledger model: an incoming
-- transfer is a separate object and may be allocated (or reallocated) to one
-- or more issued invoices.  The existing payment_transactions row remains the
-- immutable receivable created for each invoice.
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS due_at TIMESTAMPTZ;
UPDATE invoices SET due_at = issued_at + INTERVAL '30 days' WHERE due_at IS NULL;
ALTER TABLE invoices ALTER COLUMN due_at SET NOT NULL;

CREATE TABLE IF NOT EXISTS payment_receipts (
    id BIGSERIAL PRIMARY KEY,
    receipt_reference TEXT NOT NULL UNIQUE,
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
    currency TEXT NOT NULL DEFAULT 'EUR' CHECK (currency = 'EUR'),
    payer_name TEXT NOT NULL,
    payer_iban_last4 TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    received_by TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS payment_receipt_allocations (
    receipt_id BIGINT NOT NULL REFERENCES payment_receipts(id) ON DELETE CASCADE,
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
    PRIMARY KEY (receipt_id, order_id)
);
CREATE INDEX IF NOT EXISTS payment_receipt_allocations_order_idx ON payment_receipt_allocations(order_id);
