CREATE INDEX IF NOT EXISTS products_active_featured_created_idx
    ON products (featured DESC, created_at DESC, name, id)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS products_active_price_low_idx
    ON products (price_cents, featured DESC, created_at DESC, name, id)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS products_active_price_high_idx
    ON products (price_cents DESC, featured DESC, created_at DESC, name, id)
    WHERE status = 'active';
