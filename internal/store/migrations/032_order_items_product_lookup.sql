CREATE INDEX IF NOT EXISTS order_items_product_id_idx
    ON order_items (product_id)
    WHERE product_id IS NOT NULL;
