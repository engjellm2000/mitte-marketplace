CREATE TABLE IF NOT EXISTS morgenform_pay_challenges (
    id BIGSERIAL PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL,
    email TEXT NOT NULL,
    code_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS morgenform_pay_challenges_session_idx
    ON morgenform_pay_challenges (session_id, created_at DESC);
