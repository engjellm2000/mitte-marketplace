-- Remove deterministic development fixtures from databases that loaded them.
-- Keep order history and archive/zero any demo product referenced by an order.
DELETE FROM payment_receipt_allocations
WHERE order_id LIKE 'seed-order-%';

DELETE FROM payment_events
WHERE order_id LIKE 'seed-order-%';

DELETE FROM orders
WHERE id LIKE 'seed-order-%';

DELETE FROM carts
WHERE id LIKE 'seed-cart-%'
  AND NOT EXISTS (SELECT 1 FROM orders WHERE orders.cart_id = carts.id);

DELETE FROM users
WHERE email ~ '^kunde-[0-9]{3}@beispiel\.test$'
  AND NOT EXISTS (SELECT 1 FROM orders WHERE orders.customer_id = users.id);

CREATE TEMP TABLE legacy_demo_products ON COMMIT DROP AS
SELECT id, inventory
FROM products
WHERE slug IN (
    'luma-espresso-set', 'mori-tischleuchte', 'fjord-wolldecke', 'aster-vase',
    'forma-schneidebrett', 'sora-leinentasche', 'nilo-wandspiegel', 'alto-karaffe',
    'nord-kissenbezug', 'sola-kerzenhalter', 'kina-schale'
)
   OR slug LIKE 'studio-%'
   OR slug LIKE 'entdecken-%';

DELETE FROM cart_items
WHERE product_id IN (SELECT id FROM legacy_demo_products)
  AND EXISTS (
      SELECT 1
      FROM carts
      WHERE carts.id = cart_items.cart_id
        AND carts.status = 'open'
  );

DELETE FROM product_inventory_adjustments
WHERE product_id IN (SELECT id FROM legacy_demo_products)
  AND NOT EXISTS (
      SELECT 1 FROM order_items WHERE order_items.product_id = product_inventory_adjustments.product_id
  )
  AND NOT EXISTS (
      SELECT 1
      FROM cart_items
      JOIN carts ON carts.id = cart_items.cart_id
      WHERE cart_items.product_id = product_inventory_adjustments.product_id
        AND carts.status = 'checked_out'
  );

DELETE FROM product_inventory_movements
WHERE product_id IN (SELECT id FROM legacy_demo_products)
  AND NOT EXISTS (
      SELECT 1 FROM order_items WHERE order_items.product_id = product_inventory_movements.product_id
  )
  AND NOT EXISTS (
      SELECT 1
      FROM cart_items
      JOIN carts ON carts.id = cart_items.cart_id
      WHERE cart_items.product_id = product_inventory_movements.product_id
        AND carts.status = 'checked_out'
  );

INSERT INTO product_inventory_movements (
    product_id, movement_type, quantity_delta, inventory_before, inventory_after,
    reason, actor_email
)
SELECT legacy_demo_products.id, 'adjustment', -legacy_demo_products.inventory,
       legacy_demo_products.inventory, 0,
       'Removed legacy demo seed inventory', 'system:migration-036'
FROM legacy_demo_products
WHERE legacy_demo_products.inventory > 0
  AND EXISTS (
      SELECT 1 FROM order_items WHERE order_items.product_id = legacy_demo_products.id
      UNION ALL
      SELECT 1
      FROM cart_items
      JOIN carts ON carts.id = cart_items.cart_id
      WHERE cart_items.product_id = legacy_demo_products.id
        AND carts.status = 'checked_out'
  );

UPDATE products
SET status = 'archived', featured = FALSE, archived_at = COALESCE(archived_at, NOW()),
    inventory = 0, updated_at = NOW()
WHERE id IN (SELECT id FROM legacy_demo_products)
  AND (
      EXISTS (SELECT 1 FROM order_items WHERE order_items.product_id = products.id)
      OR EXISTS (
          SELECT 1
          FROM cart_items
          JOIN carts ON carts.id = cart_items.cart_id
          WHERE cart_items.product_id = products.id
            AND carts.status = 'checked_out'
      )
  );


DELETE FROM product_audit_log
WHERE product_id IN (SELECT id FROM legacy_demo_products)
  AND NOT EXISTS (
      SELECT 1 FROM order_items WHERE order_items.product_id = product_audit_log.product_id
  )
  AND NOT EXISTS (
      SELECT 1
      FROM cart_items
      JOIN carts ON carts.id = cart_items.cart_id
      WHERE cart_items.product_id = product_audit_log.product_id
        AND carts.status = 'checked_out'
  );

DELETE FROM products
WHERE id IN (SELECT id FROM legacy_demo_products)
  AND NOT EXISTS (SELECT 1 FROM order_items WHERE order_items.product_id = products.id)
  AND NOT EXISTS (
      SELECT 1
      FROM cart_items
      JOIN carts ON carts.id = cart_items.cart_id
      WHERE cart_items.product_id = products.id
        AND carts.status = 'checked_out'
  );

DELETE FROM promotions
WHERE EXISTS (
    SELECT 1
    FROM (VALUES
        ('MORGEN10', '10% auf den Warenkorb', 'percent', 1000, 0, 0, NULL, NULL, NULL, 0),
        ('WILLKOMMEN', '8 € Willkommensrabatt', 'fixed', 0, 800, 4000, NULL, NULL, 500, 1),
        ('FREIVERSAND', 'Versandkostenfrei', 'free_shipping', 0, 0, 0, NULL, NULL, NULL, 0),
        ('TISCH15', '15% auf alles vom Tisch', 'percent', 1500, 0, 0, 'tisch', NULL, NULL, 0)
    ) AS seeded(code, name, kind, percent_bps, amount_cents, min_subtotal_cents, category_slug, product_slug, max_redemptions, per_email_limit)
    WHERE promotions.code = seeded.code
      AND promotions.name = seeded.name
      AND promotions.kind = seeded.kind
      AND promotions.percent_bps = seeded.percent_bps
      AND promotions.amount_cents = seeded.amount_cents
      AND promotions.min_subtotal_cents = seeded.min_subtotal_cents
      AND promotions.category_slug IS NOT DISTINCT FROM seeded.category_slug
      AND promotions.product_slug IS NOT DISTINCT FROM seeded.product_slug
      AND promotions.max_redemptions IS NOT DISTINCT FROM seeded.max_redemptions
      AND promotions.per_email_limit = seeded.per_email_limit
)
AND NOT EXISTS (SELECT 1 FROM promotion_redemptions WHERE promotion_redemptions.code = promotions.code);

UPDATE carts
SET promotion_code = NULL, updated_at = NOW()
WHERE promotion_code IN ('MORGEN10', 'WILLKOMMEN', 'FREIVERSAND', 'TISCH15')
  AND NOT EXISTS (
      SELECT 1 FROM promotions WHERE promotions.code = carts.promotion_code
  );

-- Keep the storefront's established top-level taxonomy, but discard seeded
-- empty categories. Categories containing non-demo products are retained.
DELETE FROM categories
WHERE slug IN (
    'technologie', 'it', 'geraete', 'mode', 'sport', 'beauty', 'kinder',
    'buero', 'garten', 'tierwelt', 'lebensmittel',
    'beleuchtung', 'deko', 'aufbewahrung', 'moebel', 'geschirr', 'kochen',
    'bar-kaffee', 'bettwaesche', 'badtextilien', 'heimtextilien', 'smart-home',
    'audio', 'wearables', 'fotografie', 'laptops-tablets', 'computer-zubehoer',
    'netzwerk', 'speicher', 'kuechengeraete', 'haushaltsgeraete', 'klima',
    'damenmode', 'herrenmode', 'taschen', 'fitness', 'radfahren', 'camping',
    'hautpflege', 'haarpflege', 'wellness', 'spielzeug', 'baby', 'schule',
    'schreibwaren', 'arbeitsplatz', 'gartengeraete', 'balkon', 'grillen',
    'hunde', 'katzen', 'kaffee-tee', 'feinkost', 'snacks'
)
AND NOT EXISTS (SELECT 1 FROM products WHERE products.category_id = categories.id)
AND NOT EXISTS (SELECT 1 FROM categories child WHERE child.parent_id = categories.id);

DELETE FROM categories
WHERE slug IN (
    'technologie', 'it', 'geraete', 'mode', 'sport', 'beauty', 'kinder',
    'buero', 'garten', 'tierwelt', 'lebensmittel'
)
AND NOT EXISTS (SELECT 1 FROM products WHERE products.category_id = categories.id)
AND NOT EXISTS (SELECT 1 FROM categories child WHERE child.parent_id = categories.id);

INSERT INTO categories (slug, name, description)
VALUES
    ('wohnen', 'Wohnen', 'Objekte für Wohnräume.'),
    ('tisch', 'Tisch', 'Geschirr und Objekte für den gedeckten Tisch.'),
    ('textil', 'Textil', 'Heimtextilien und textile Alltagsobjekte.')
ON CONFLICT (slug) DO NOTHING;
