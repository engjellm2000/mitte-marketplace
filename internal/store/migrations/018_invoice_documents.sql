CREATE TABLE IF NOT EXISTS invoice_documents (
    id BIGSERIAL PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    document_type TEXT NOT NULL CHECK (document_type IN ('invoice', 'credit_note', 'cancellation')),
    document_number TEXT NOT NULL UNIQUE,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    total_cents INTEGER NOT NULL,
    currency TEXT NOT NULL DEFAULT 'EUR',
    reason TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL DEFAULT 'system',
    UNIQUE (order_id, document_type)
);
INSERT INTO invoice_documents (order_id, document_type, document_number, issued_at, total_cents, currency)
SELECT order_id, 'invoice', invoice_number, issued_at, total_cents, currency FROM invoices
ON CONFLICT (document_number) DO NOTHING;
