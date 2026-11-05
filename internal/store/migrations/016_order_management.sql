CREATE TABLE IF NOT EXISTS order_internal_notes (
    id BIGSERIAL PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    author_email TEXT NOT NULL,
    body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS order_internal_notes_order_created_idx
    ON order_internal_notes (order_id, created_at DESC);
