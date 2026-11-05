ALTER TABLE orders ADD COLUMN IF NOT EXISTS stripe_checkout_session_id TEXT UNIQUE, ADD COLUMN IF NOT EXISTS stripe_payment_intent_id TEXT, ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ, ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS orders_stripe_checkout_session_id_idx ON orders(stripe_checkout_session_id);
CREATE TABLE IF NOT EXISTS payment_events (id BIGSERIAL PRIMARY KEY, provider TEXT NOT NULL DEFAULT 'stripe', provider_event_id TEXT NOT NULL UNIQUE, event_type TEXT NOT NULL, order_id TEXT REFERENCES orders(id), payload JSONB NOT NULL, processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE IF NOT EXISTS order_status_history (id BIGSERIAL PRIMARY KEY, order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE, from_status TEXT, to_status TEXT NOT NULL, reason TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE INDEX IF NOT EXISTS order_status_history_order_id_idx ON order_status_history(order_id);
