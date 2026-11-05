ALTER TABLE invoices
    ADD COLUMN IF NOT EXISTS access_token_hash BYTEA,
    ADD COLUMN IF NOT EXISTS seller_snapshot JSONB;

CREATE UNIQUE INDEX IF NOT EXISTS invoices_access_token_hash_idx
    ON invoices (access_token_hash)
    WHERE access_token_hash IS NOT NULL;

UPDATE invoices
SET seller_snapshot = '{}'::jsonb
WHERE seller_snapshot IS NULL;
