-- A development-only invoice/factoring sandbox has its own provider value so
-- it can never be mistaken for a settled Stripe or bank-transfer payment.
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_payment_provider_check;
ALTER TABLE orders ADD CONSTRAINT orders_payment_provider_check
    CHECK (payment_provider IN ('stripe', 'morgenform_transfer', 'morgenform_invoice'));
