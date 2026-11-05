ALTER TABLE users
    ADD COLUMN IF NOT EXISTS recommendation_consent BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS recommendation_consent_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS customer_product_views (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    viewed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, product_id)
);

CREATE INDEX IF NOT EXISTS customer_product_views_recent_idx
    ON customer_product_views (user_id, viewed_at DESC);

CREATE INDEX IF NOT EXISTS orders_customer_paid_recommendation_idx
    ON orders (customer_id, created_at DESC)
    WHERE customer_id IS NOT NULL AND status IN ('paid', 'fulfilled');
