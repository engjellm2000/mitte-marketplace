ALTER TABLE users
    ADD COLUMN email_verified_at TIMESTAMPTZ;

UPDATE user_sessions
SET revoked_at = NOW()
WHERE revoked_at IS NULL;

ALTER TABLE email_outbox_events
    ALTER COLUMN order_id DROP NOT NULL;

CREATE TABLE user_auth_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose TEXT NOT NULL CHECK (purpose IN ('verify_email', 'password_reset')),
    token_hash BYTEA NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    CHECK (expires_at > created_at)
);

CREATE INDEX user_auth_tokens_active_user_idx
    ON user_auth_tokens (user_id, purpose, expires_at)
    WHERE consumed_at IS NULL;

CREATE INDEX user_auth_tokens_expiry_idx
    ON user_auth_tokens (expires_at);
