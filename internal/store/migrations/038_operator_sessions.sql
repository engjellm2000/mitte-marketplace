CREATE TABLE operator_sessions (
    token_hash BYTEA PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    principal TEXT NOT NULL CHECK (length(principal) BETWEEN 3 AND 128),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX operator_sessions_expiry_idx ON operator_sessions (expires_at);
