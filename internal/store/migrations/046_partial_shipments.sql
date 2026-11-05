ALTER TABLE shipments DROP CONSTRAINT IF EXISTS shipments_order_id_key;
ALTER TABLE shipments
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'oms'
        CHECK (source IN ('supplier', 'warehouse', 'oms')),
    ADD COLUMN IF NOT EXISTS supplier_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS created_by TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_event_id BIGINT REFERENCES supplier_order_events(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS shipments_source_event_id_idx
    ON shipments (source_event_id) WHERE source_event_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS shipments_order_created_idx
    ON shipments (order_id, shipped_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS shipment_items (
    shipment_id BIGINT NOT NULL REFERENCES shipments(id) ON DELETE CASCADE,
    order_id TEXT NOT NULL,
    product_name TEXT NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (shipment_id, order_id, product_name),
    FOREIGN KEY (order_id, product_name)
        REFERENCES order_items(order_id, product_name) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS shipment_items_order_item_idx
    ON shipment_items (order_id, product_name, shipment_id);

ALTER TABLE supplier_order_events
    ADD COLUMN IF NOT EXISTS quantity INTEGER NOT NULL DEFAULT 0 CHECK (quantity >= 0);

INSERT INTO shipment_items (shipment_id, order_id, product_name, quantity)
SELECT s.id, s.order_id, oi.product_name, oi.quantity
FROM shipments s
JOIN order_items oi ON oi.order_id = s.order_id
ON CONFLICT (shipment_id, order_id, product_name) DO NOTHING;

WITH legacy_supplier_events AS (
    INSERT INTO shipments (
        order_id, carrier, tracking_number, tracking_url, shipped_at,
        source, supplier_name, created_by, source_event_id
    )
    SELECT e.order_id, COALESCE(NULLIF(e.carrier, ''), 'Lieferant'),
        COALESCE(NULLIF(e.tracking_number, ''), 'LEGACY-' || e.id::text),
        NULLIF(e.tracking_url, ''), e.created_at, 'supplier', e.supplier_name,
        'supplier:' || e.actor_username, e.id
    FROM supplier_order_events e
    WHERE e.event_type = 'shipment_reported'
      AND NOT EXISTS (
          SELECT 1 FROM shipments existing
          WHERE existing.order_id = e.order_id AND existing.source_event_id IS NULL
      )
    ON CONFLICT (source_event_id) WHERE source_event_id IS NOT NULL DO NOTHING
    RETURNING id, source_event_id
)
INSERT INTO shipment_items (shipment_id, order_id, product_name, quantity)
SELECT legacy_supplier_events.id, e.order_id, oi.product_name, oi.quantity
FROM legacy_supplier_events
JOIN supplier_order_events e ON e.id = legacy_supplier_events.source_event_id
JOIN order_items oi ON oi.order_id = e.order_id AND oi.product_id = e.product_id
ON CONFLICT (shipment_id, order_id, product_name) DO NOTHING;

UPDATE supplier_order_events e
SET quantity = oi.quantity
FROM order_items oi
WHERE e.event_type = 'shipment_reported'
  AND e.quantity = 0
  AND oi.order_id = e.order_id
  AND oi.product_id = e.product_id;

WITH cancelled AS (
    UPDATE warehouse_fulfillments f
    SET status = 'cancelled', updated_at = NOW()
    WHERE f.status IN ('ready_to_pick', 'picking', 'packed', 'label_ready')
      AND NOT EXISTS (
          SELECT 1 FROM order_items oi
          WHERE oi.order_id = f.order_id AND BTRIM(COALESCE(oi.supplier_name, '')) = ''
      )
    RETURNING f.order_id
)
INSERT INTO warehouse_events (order_id, actor_email, event_type, detail)
SELECT order_id, 'system:migration', 'fulfillment.cancelled',
    'No warehouse-managed items; supplier ships this order directly.'
FROM cancelled;
