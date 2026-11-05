CREATE OR REPLACE FUNCTION ensure_cart_is_open() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE cart_status TEXT; target_cart_id TEXT;
BEGIN
  target_cart_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.cart_id ELSE NEW.cart_id END;
  SELECT status INTO cart_status FROM carts WHERE id = target_cart_id FOR UPDATE;
  IF NOT FOUND OR cart_status <> 'open' THEN RAISE EXCEPTION 'cart % is not open', target_cart_id USING ERRCODE = '23514'; END IF;
  IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
  RETURN NEW;
END; $$;
DROP TRIGGER IF EXISTS cart_items_require_open_cart ON cart_items;
CREATE TRIGGER cart_items_require_open_cart BEFORE INSERT OR UPDATE OR DELETE ON cart_items FOR EACH ROW EXECUTE FUNCTION ensure_cart_is_open();
CREATE OR REPLACE FUNCTION enforce_order_status_transition() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status = NEW.status OR (OLD.status = 'pending_payment' AND NEW.status IN ('paid', 'cancelled')) OR (OLD.status = 'paid' AND NEW.status IN ('fulfilled', 'cancelled')) THEN RETURN NEW; END IF;
  RAISE EXCEPTION 'invalid order status transition: % -> %', OLD.status, NEW.status USING ERRCODE = '23514';
END; $$;
DROP TRIGGER IF EXISTS orders_enforce_status_transition ON orders;
CREATE TRIGGER orders_enforce_status_transition BEFORE UPDATE OF status ON orders FOR EACH ROW EXECUTE FUNCTION enforce_order_status_transition();
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_financial_snapshot_check;
ALTER TABLE orders ADD CONSTRAINT orders_financial_snapshot_check CHECK (subtotal_cents IS NULL OR (subtotal_cents >= 0 AND discount_cents >= 0 AND discount_cents <= subtotal_cents AND shipping_cents >= 0 AND tax_cents >= 0 AND total_cents = subtotal_cents - discount_cents + shipping_cents)) NOT VALID;
CREATE INDEX IF NOT EXISTS payment_events_order_id_idx ON payment_events(order_id) WHERE order_id IS NOT NULL;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS checkout_session_token TEXT;
UPDATE orders o SET checkout_session_token = c.session_token FROM carts c WHERE c.id = o.cart_id AND o.checkout_session_token IS NULL;
ALTER TABLE orders ALTER COLUMN checkout_session_token SET NOT NULL;
DROP INDEX IF EXISTS orders_checkout_idempotency_key_idx;
CREATE UNIQUE INDEX IF NOT EXISTS orders_session_idempotency_key_idx ON orders(checkout_session_token, checkout_idempotency_key) WHERE checkout_idempotency_key IS NOT NULL;
