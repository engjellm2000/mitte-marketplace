ALTER TABLE users
    ADD COLUMN marketing_email_consent BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN marketing_email_consent_at TIMESTAMPTZ,
    ADD CONSTRAINT users_marketing_email_consent_timestamp_check
        CHECK (marketing_email_consent = (marketing_email_consent_at IS NOT NULL));

CREATE TABLE email_marketing_consent_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event TEXT NOT NULL CHECK (event IN ('subscribed', 'unsubscribed')),
    source TEXT NOT NULL CHECK (source IN ('account', 'unsubscribe')),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX email_marketing_consent_events_user_idx
    ON email_marketing_consent_events (user_id, occurred_at DESC);

CREATE TABLE marketing_email_campaigns (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    subject TEXT NOT NULL,
    preheader TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'queued', 'sent')),
    recipient_count INTEGER NOT NULL DEFAULT 0 CHECK (recipient_count >= 0),
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    queued_at TIMESTAMPTZ,
    sent_at TIMESTAMPTZ
);

CREATE TABLE email_marketing_unsubscribe_tokens (
    token_hash BYTEA PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    used_at TIMESTAMPTZ
);

CREATE INDEX email_marketing_unsubscribe_user_idx
    ON email_marketing_unsubscribe_tokens (user_id);

ALTER TABLE email_outbox_events
    ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    ADD COLUMN campaign_id BIGINT REFERENCES marketing_email_campaigns(id) ON DELETE SET NULL,
    ADD COLUMN preheader TEXT NOT NULL DEFAULT '',
    ADD COLUMN unsubscribe_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN cancelled_at TIMESTAMPTZ,
    ADD CONSTRAINT email_marketing_delivery_context_check
        CHECK (campaign_id IS NULL OR (user_id IS NOT NULL AND order_id IS NULL));

CREATE INDEX email_outbox_campaign_pending_idx
    ON email_outbox_events (campaign_id, available_at, id)
    WHERE campaign_id IS NOT NULL AND sent_at IS NULL AND cancelled_at IS NULL;
