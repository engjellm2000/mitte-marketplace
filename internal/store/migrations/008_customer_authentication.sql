CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    password_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_email_normalized_check CHECK (email = lower(email)),
    CONSTRAINT users_email_not_blank_check CHECK (length(email) > 3)
);

CREATE UNIQUE INDEX users_email_unique_idx ON users (email);

CREATE TABLE user_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CONSTRAINT user_sessions_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT user_sessions_token_hash_unique UNIQUE (token_hash)
);

CREATE INDEX user_sessions_active_user_idx
    ON user_sessions (user_id, expires_at)
    WHERE revoked_at IS NULL;

ALTER TABLE orders
    ADD COLUMN customer_id BIGINT REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX orders_customer_created_at_idx
    ON orders (customer_id, created_at DESC)
    WHERE customer_id IS NOT NULL;
