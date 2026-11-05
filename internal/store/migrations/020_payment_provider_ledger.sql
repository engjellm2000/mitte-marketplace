-- A provider is recorded on the order rather than inferred from a session id.
-- This keeps Stripe and the first-party bank-transfer ledger independently auditable.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS payment_provider TEXT NOT NULL DEFAULT 'stripe'
    CHECK (payment_provider IN ('stripe', 'morgenform_transfer'));

CREATE TABLE IF NOT EXISTS payment_transactions (
    id BIGSERIAL PRIMARY KEY,
    order_id TEXT NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('morgenform_transfer')),
    reference TEXT NOT NULL UNIQUE,
    amount_cents INTEGER NOT NULL CHECK (amount_cents >= 0),
    currency TEXT NOT NULL DEFAULT 'EUR' CHECK (currency = 'EUR'),
    status TEXT NOT NULL DEFAULT 'awaiting_payment' CHECK (status IN ('awaiting_payment', 'received', 'rejected')),
    payer_name TEXT NOT NULL DEFAULT '',
    payer_iban_last4 TEXT NOT NULL DEFAULT '',
    received_at TIMESTAMPTZ,
    received_by TEXT,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS payment_transactions_status_created_idx ON payment_transactions(status, created_at DESC);
