CREATE TEMP TABLE legacy_supplier_shipment_groups ON COMMIT DROP AS
WITH imported AS (
    SELECT s.id AS shipment_id, e.id AS event_id, e.order_id, e.product_id,
        e.supplier_name,
        COALESCE(NULLIF(BTRIM(e.carrier), ''), 'Lieferant') AS carrier,
        COALESCE(NULLIF(BTRIM(e.tracking_number), ''), 'LEGACY-' || e.id::text) AS tracking_number,
        COALESCE(NULLIF(LOWER(BTRIM(e.carrier)), ''), 'lieferant') AS carrier_key,
        COALESCE(NULLIF(LOWER(BTRIM(e.tracking_number)), ''), 'legacy-' || e.id::text) AS tracking_key,
        NULLIF(BTRIM(e.tracking_url), '') AS tracking_url,
        e.created_at,
        ROW_NUMBER() OVER (
            PARTITION BY e.order_id, e.product_id,
                COALESCE(NULLIF(LOWER(BTRIM(e.carrier)), ''), 'lieferant'),
                COALESCE(NULLIF(LOWER(BTRIM(e.tracking_number)), ''), 'legacy-' || e.id::text)
            ORDER BY e.created_at, e.id
        ) AS item_event_rank
    FROM shipments s
    JOIN supplier_order_events e ON e.id = s.source_event_id
    WHERE s.source = 'supplier'
      AND s.source_event_id IS NOT NULL
      AND e.event_type = 'shipment_reported'
),
chosen AS (
    SELECT * FROM imported WHERE item_event_rank = 1
),
shipment_groups AS (
    SELECT order_id, supplier_name, carrier_key, tracking_key,
        MIN(shipment_id) AS canonical_shipment_id,
        MIN(event_id) AS canonical_event_id,
        MIN(created_at) AS shipped_at
    FROM chosen
    GROUP BY order_id, supplier_name, carrier_key, tracking_key
)
SELECT chosen.shipment_id, chosen.event_id, chosen.order_id, chosen.product_id,
    shipment_groups.canonical_shipment_id, shipment_groups.canonical_event_id,
    shipment_groups.shipped_at
FROM chosen
JOIN shipment_groups USING (order_id, supplier_name, carrier_key, tracking_key);

DELETE FROM shipments s
WHERE s.source = 'supplier'
  AND s.source_event_id IS NOT NULL
  AND EXISTS (
      SELECT 1 FROM supplier_order_events e
      WHERE e.id = s.source_event_id AND e.event_type = 'shipment_reported'
  )
  AND NOT EXISTS (
      SELECT 1
      FROM legacy_supplier_shipment_groups g
      WHERE g.canonical_shipment_id = s.id
  );

UPDATE shipments s
SET source_event_id = g.canonical_event_id,
    carrier = COALESCE(NULLIF(BTRIM(e.carrier), ''), 'Lieferant'),
    tracking_number = COALESCE(NULLIF(BTRIM(e.tracking_number), ''), 'LEGACY-' || e.id::text),
    tracking_url = NULLIF(BTRIM(e.tracking_url), ''),
    supplier_name = e.supplier_name,
    created_by = 'supplier:' || e.actor_username,
    shipped_at = g.shipped_at
FROM (
    SELECT DISTINCT ON (canonical_shipment_id)
        canonical_shipment_id, canonical_event_id, shipped_at
    FROM legacy_supplier_shipment_groups
    ORDER BY canonical_shipment_id, event_id
) g
JOIN supplier_order_events e ON e.id = g.canonical_event_id
WHERE s.id = g.canonical_shipment_id;

DELETE FROM shipment_items si
USING legacy_supplier_shipment_groups g
WHERE si.shipment_id = g.canonical_shipment_id;

WITH shipment_item_quantities AS (
    SELECT g.canonical_shipment_id, e.order_id, oi.product_name, oi.quantity AS ordered_quantity,
        LEAST(oi.quantity, CASE WHEN e.quantity > 0 THEN e.quantity ELSE oi.quantity END) AS reported_quantity,
        COALESCE(SUM(LEAST(oi.quantity, CASE WHEN e.quantity > 0 THEN e.quantity ELSE oi.quantity END))
            OVER (PARTITION BY e.order_id, e.product_id
                ORDER BY g.shipped_at, g.canonical_shipment_id
                ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING), 0) AS previously_reported_quantity
    FROM legacy_supplier_shipment_groups g
    JOIN supplier_order_events e ON e.id = g.event_id
    JOIN order_items oi ON oi.order_id = e.order_id AND oi.product_id = e.product_id
),
allocated AS (
    SELECT canonical_shipment_id, order_id, product_name,
        LEAST(reported_quantity, GREATEST(0, ordered_quantity-previously_reported_quantity)) AS quantity
    FROM shipment_item_quantities
)
INSERT INTO shipment_items (shipment_id, order_id, product_name, quantity)
SELECT canonical_shipment_id, order_id, product_name, quantity
FROM allocated
WHERE quantity > 0
ON CONFLICT (shipment_id, order_id, product_name) DO NOTHING;

DELETE FROM shipments s
WHERE s.source = 'supplier'
  AND EXISTS (
      SELECT 1 FROM supplier_order_events e
      WHERE e.id = s.source_event_id AND e.event_type = 'shipment_reported'
  )
  AND NOT EXISTS (
      SELECT 1 FROM shipment_items si WHERE si.shipment_id = s.id
  );
