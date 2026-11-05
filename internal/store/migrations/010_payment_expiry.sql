-- Inventory is reserved before redirecting to Stripe. A lost or delayed
-- webhook must not retain that stock forever, so the reservation has a
-- database-owned expiry that workers can safely reclaim.
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS checkout_expires_at TIMESTAMPTZ;

UPDATE orders
SET checkout_expires_at = created_at + INTERVAL '30 minutes'
WHERE status = 'pending_payment'
  AND inventory_reserved = TRUE
  AND checkout_expires_at IS NULL;

CREATE INDEX IF NOT EXISTS orders_expired_reservations_idx
    ON orders (checkout_expires_at)
    WHERE status = 'pending_payment' AND inventory_reserved = TRUE;
