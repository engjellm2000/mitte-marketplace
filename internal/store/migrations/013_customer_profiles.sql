-- A profile is deliberately separate from order snapshots: customers can keep
-- their details current without changing an already-issued order or invoice.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS full_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS phone TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS customer_type TEXT NOT NULL DEFAULT 'b2c'
        CHECK (customer_type IN ('b2c', 'b2b')),
    ADD COLUMN IF NOT EXISTS company_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS vat_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS buyer_reference TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS address_line1 TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS address_line2 TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS postal_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS city TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS country_code TEXT NOT NULL DEFAULT 'DE'
        CHECK (country_code ~ '^[A-Z]{2}$');
