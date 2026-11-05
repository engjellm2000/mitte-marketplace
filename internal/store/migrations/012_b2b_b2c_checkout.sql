-- Checkout data is stored on the order, not just at the customer profile. This
-- keeps invoices and fulfilment historically correct when a customer updates an
-- address later.
ALTER TABLE orders
    ADD COLUMN customer_type TEXT NOT NULL DEFAULT 'b2c'
        CHECK (customer_type IN ('b2c', 'b2b')),
    ADD COLUMN company_name TEXT,
    ADD COLUMN vat_id TEXT,
    ADD COLUMN buyer_reference TEXT,
    ADD COLUMN procurement_method TEXT NOT NULL DEFAULT 'online_payment'
        CHECK (procurement_method IN ('online_payment', 'purchase_order', 'eprocurement')),
    ADD COLUMN customer_note TEXT;

CREATE TABLE order_addresses (
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    address_type TEXT NOT NULL CHECK (address_type IN ('billing', 'shipping')),
    recipient_name TEXT NOT NULL,
    company_name TEXT,
    address_line1 TEXT NOT NULL,
    address_line2 TEXT,
    postal_code TEXT NOT NULL,
    city TEXT NOT NULL,
    country_code TEXT NOT NULL DEFAULT 'DE'
        CHECK (country_code ~ '^[A-Z]{2}$'),
    PRIMARY KEY (order_id, address_type)
);

CREATE INDEX order_addresses_postal_code_idx ON order_addresses (postal_code);
