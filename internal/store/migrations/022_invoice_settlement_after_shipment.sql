-- Invoice orders are fulfilment-approved first and settle after the shipment.
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check
    CHECK (status IN ('pending_payment', 'invoice_approved', 'paid', 'fulfilled', 'cancelled'));

ALTER TABLE payment_transactions DROP CONSTRAINT IF EXISTS payment_transactions_provider_check;
ALTER TABLE payment_transactions ADD CONSTRAINT payment_transactions_provider_check
    CHECK (provider IN ('morgenform_transfer', 'morgenform_invoice'));
