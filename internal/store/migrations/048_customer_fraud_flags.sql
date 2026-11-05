ALTER TABLE users
    ADD COLUMN IF NOT EXISTS fraud_flagged BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS fraud_reason TEXT,
    ADD COLUMN IF NOT EXISTS fraud_flagged_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS fraud_flagged_by TEXT;

CREATE TABLE IF NOT EXISTS customer_fraud_audit (
    id BIGSERIAL PRIMARY KEY,
    customer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    flagged BOOLEAN NOT NULL,
    reason TEXT NOT NULL CHECK (char_length(reason) BETWEEN 5 AND 500),
    actor_email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS customer_fraud_audit_customer_idx
    ON customer_fraud_audit (customer_id, id DESC);
