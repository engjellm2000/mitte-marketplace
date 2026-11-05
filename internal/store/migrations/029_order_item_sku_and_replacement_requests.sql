CREATE SEQUENCE IF NOT EXISTS product_sku_seq;

ALTER TABLE products ADD COLUMN IF NOT EXISTS sku TEXT;
UPDATE products
SET sku = 'MF-' || LPAD(id::text, 6, '0')
WHERE sku IS NULL OR BTRIM(sku) = '';

SELECT setval(
    'product_sku_seq',
    GREATEST(COALESCE((SELECT MAX(id) FROM products), 0), 1),
    EXISTS (SELECT 1 FROM products)
);

ALTER TABLE products
    ALTER COLUMN sku SET DEFAULT ('MF-' || LPAD(nextval('product_sku_seq')::text, 6, '0')),
    ALTER COLUMN sku SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS products_sku_idx ON products(sku);

ALTER TABLE order_items ADD COLUMN IF NOT EXISTS sku TEXT;
UPDATE order_items oi
SET sku = p.sku
FROM products p
WHERE oi.product_id = p.id AND (oi.sku IS NULL OR BTRIM(oi.sku) = '');

ALTER TABLE order_item_operations
    DROP CONSTRAINT IF EXISTS order_item_operations_status_check;
ALTER TABLE order_item_operations
    ADD CONSTRAINT order_item_operations_status_check
    CHECK (status IN (
        'ready_to_process', 'stock_confirmed', 'payment_received', 'exported',
        'fulfillment', 'return_open', 'fulfillment_completed', 'cancelled',
        'refunded', 'replacement_created', 'replacement_requested'
    ));
