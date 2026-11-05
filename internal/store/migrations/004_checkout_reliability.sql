ALTER TABLE order_items ADD COLUMN IF NOT EXISTS product_id BIGINT REFERENCES products(id);
ALTER TABLE orders ADD COLUMN IF NOT EXISTS checkout_idempotency_key TEXT, ADD COLUMN IF NOT EXISTS stripe_checkout_url TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS orders_checkout_idempotency_key_idx ON orders(checkout_idempotency_key) WHERE checkout_idempotency_key IS NOT NULL;
