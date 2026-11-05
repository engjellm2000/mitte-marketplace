-- Orders approved before the receivable-registration change must also become
-- visible in the payment portal. Shipped orders already have the same row, so
-- the unique order key makes this safe to run once during migration.
INSERT INTO payment_transactions (order_id, provider, reference, amount_cents)
SELECT o.id, 'morgenform_invoice', 'MF-' || UPPER(o.id), o.total_cents
FROM orders o
WHERE o.payment_provider = 'morgenform_invoice'
  AND o.status IN ('invoice_approved', 'fulfilled', 'paid')
ON CONFLICT (order_id) DO NOTHING;
