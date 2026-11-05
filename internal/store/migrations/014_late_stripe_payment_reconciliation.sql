-- Stripe Checkout sessions remain valid for 45 minutes. Keep the inventory
-- reservation longer than that window, so a valid payment is never cancelled
-- locally before Stripe can report it.
UPDATE orders
SET checkout_expires_at = GREATEST(checkout_expires_at, created_at + INTERVAL '50 minutes')
WHERE status = 'pending_payment' AND checkout_expires_at IS NOT NULL;

-- A payment can be accepted by Stripe while the application is temporarily
-- unreachable. The application only uses this transition after retrieving and
-- validating that exact paid Stripe Checkout session server-side.
CREATE OR REPLACE FUNCTION enforce_order_status_transition() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status = NEW.status
    OR (OLD.status = 'pending_payment' AND NEW.status IN ('paid', 'cancelled'))
    OR (OLD.status = 'paid' AND NEW.status IN ('fulfilled', 'cancelled'))
    OR (OLD.status = 'cancelled' AND NEW.status = 'paid') THEN
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'invalid order status transition: % -> %', OLD.status, NEW.status USING ERRCODE = '23514';
END; $$;
