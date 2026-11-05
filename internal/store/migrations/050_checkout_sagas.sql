CREATE TABLE checkout_sagas (
    order_id TEXT PRIMARY KEY REFERENCES orders(id) ON DELETE CASCADE,
    payment_method TEXT NOT NULL CHECK (payment_method IN ('stripe', 'morgenform_pay', 'morgenform_transfer', 'morgenform_invoice')),
    status TEXT NOT NULL CHECK (status IN ('started', 'retryable', 'awaiting_payment', 'completed', 'compensated')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO checkout_sagas (order_id, payment_method, status, created_at, updated_at)
SELECT id,
       CASE
           WHEN LEFT(stripe_checkout_session_id, 6) = 'mfpay_' THEN 'morgenform_pay'
           WHEN payment_provider IN ('morgenform_transfer', 'morgenform_invoice') THEN payment_provider
           ELSE 'stripe'
       END,
       CASE
           WHEN status IN ('paid', 'fulfilled', 'invoice_approved') THEN 'completed'
           WHEN status = 'cancelled' THEN 'compensated'
           WHEN stripe_checkout_session_id IS NULL THEN 'retryable'
           ELSE 'awaiting_payment'
       END,
       created_at,
       NOW()
FROM orders
WHERE checkout_idempotency_key IS NOT NULL
ON CONFLICT (order_id) DO NOTHING;
