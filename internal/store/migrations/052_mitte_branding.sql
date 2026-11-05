DO $$
BEGIN
    IF to_regclass('morgenform_pay_challenges') IS NOT NULL
        AND to_regclass('mitte_pay_challenges') IS NULL THEN
        ALTER TABLE morgenform_pay_challenges RENAME TO mitte_pay_challenges;
    END IF;
END $$;

ALTER INDEX IF EXISTS morgenform_pay_challenges_session_idx
    RENAME TO mitte_pay_challenges_session_idx;

ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_payment_provider_check;
ALTER TABLE payment_transactions DROP CONSTRAINT IF EXISTS payment_transactions_provider_check;
ALTER TABLE checkout_sagas DROP CONSTRAINT IF EXISTS checkout_sagas_payment_method_check;

UPDATE orders SET payment_provider = 'mitte_transfer' WHERE payment_provider = 'morgenform_transfer';
UPDATE orders SET payment_provider = 'mitte_invoice' WHERE payment_provider = 'morgenform_invoice';
UPDATE payment_transactions SET provider = 'mitte_transfer' WHERE provider = 'morgenform_transfer';
UPDATE payment_transactions SET provider = 'mitte_invoice' WHERE provider = 'morgenform_invoice';
UPDATE checkout_sagas SET payment_method = 'mitte_pay' WHERE payment_method = 'morgenform_pay';
UPDATE checkout_sagas SET payment_method = 'mitte_transfer' WHERE payment_method = 'morgenform_transfer';
UPDATE checkout_sagas SET payment_method = 'mitte_invoice' WHERE payment_method = 'morgenform_invoice';
UPDATE order_status_history
SET reason = REPLACE(reason, 'morgenform', 'mitte')
WHERE reason LIKE '%morgenform%';

ALTER TABLE orders ADD CONSTRAINT orders_payment_provider_check
    CHECK (payment_provider IN ('stripe', 'mitte_transfer', 'mitte_invoice'));
ALTER TABLE payment_transactions ADD CONSTRAINT payment_transactions_provider_check
    CHECK (provider IN ('mitte_transfer', 'mitte_invoice'));
ALTER TABLE checkout_sagas ADD CONSTRAINT checkout_sagas_payment_method_check
    CHECK (payment_method IN ('stripe', 'mitte_pay', 'mitte_transfer', 'mitte_invoice'));
