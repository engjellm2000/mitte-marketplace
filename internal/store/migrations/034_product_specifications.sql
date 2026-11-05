ALTER TABLE products
    ADD COLUMN IF NOT EXISTS manufacturer TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS manufacturer_sku TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS gtin TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS material TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS dimensions TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS weight_grams INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS technical_details JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE products
    DROP CONSTRAINT IF EXISTS products_gtin_format_check,
    ADD CONSTRAINT products_gtin_format_check
        CHECK (gtin = '' OR (length(gtin) IN (8, 12, 13, 14) AND gtin ~ '^[0-9]+$')),
    DROP CONSTRAINT IF EXISTS products_weight_grams_check,
    ADD CONSTRAINT products_weight_grams_check CHECK (weight_grams >= 0),
    DROP CONSTRAINT IF EXISTS products_technical_details_object_check,
    ADD CONSTRAINT products_technical_details_object_check CHECK (jsonb_typeof(technical_details) = 'object');

CREATE UNIQUE INDEX IF NOT EXISTS products_gtin_idx ON products (gtin) WHERE gtin <> '';
CREATE INDEX IF NOT EXISTS products_manufacturer_sku_idx ON products (manufacturer, manufacturer_sku)
    WHERE manufacturer <> '' AND manufacturer_sku <> '';
