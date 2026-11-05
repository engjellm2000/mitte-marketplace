ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ALTER COLUMN status SET DEFAULT 'pending_payment';
ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (status IN ('pending_payment', 'paid', 'fulfilled', 'cancelled'));
