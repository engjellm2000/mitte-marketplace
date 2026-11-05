package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type SupplierOrderItem struct {
	OrderID              string
	OrderStatus          string
	OrderCreatedAt       time.Time
	ProductID            int64
	ProductName          string
	SKU                  string
	Quantity             int
	ShippedQuantity      int
	RemainingQuantity    int
	SupplierName         string
	RecipientName        string
	CompanyName          string
	AddressLine1         string
	AddressLine2         string
	PostalCode           string
	City                 string
	CountryCode          string
	OriginalDelivery     string
	DelayedDelivery      string
	LatestEvent          string
	LatestEventNote      string
	LatestEventDate      string
	LatestEventResolved  bool
	LatestResolution     string
	LatestResolutionNote string
	LatestCarrier        string
	LatestTracking       string
	LatestTrackingURL    string
	Accepted             bool
}

type SupplierCancellationRequest struct {
	ID            int64
	OrderID       string
	ProductID     int64
	ProductName   string
	SKU           string
	SupplierName  string
	Quantity      int
	Note          string
	ActorUsername string
	CreatedAt     time.Time
	OrderStatus   string
}

func (s *Store) SupplierOrderItems(ctx context.Context, supplierName string) ([]SupplierOrderItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT o.id,o.status,o.created_at,oi.product_id,oi.product_name,COALESCE(oi.sku,''),oi.quantity,
			COALESCE(shipped.quantity,0),GREATEST(0,oi.quantity-COALESCE(shipped.quantity,0)),
			oi.supplier_name,a.recipient_name,COALESCE(a.company_name,''),a.address_line1,
			COALESCE(a.address_line2,''),a.postal_code,a.city,a.country_code,
			COALESCE(op.original_delivery_date::text,''),COALESCE(op.delayed_delivery_date::text,''),
			COALESCE(latest.event_type,''),COALESCE(latest.note,''),COALESCE(latest.delayed_delivery_date::text,''),
			(latest.resolved_at IS NOT NULL),COALESCE(latest.resolution_status,''),COALESCE(latest.resolution_note,''),
			COALESCE(latest.carrier,''),COALESCE(latest.tracking_number,''),COALESCE(latest.tracking_url,''),
			EXISTS(SELECT 1 FROM supplier_order_events accepted WHERE accepted.order_id=oi.order_id
				AND accepted.product_id=oi.product_id AND accepted.event_type='order_accepted')
		FROM order_items oi
		JOIN orders o ON o.id=oi.order_id
		LEFT JOIN users u ON u.id=o.customer_id OR (o.customer_id IS NULL AND LOWER(u.email)=LOWER(o.customer_email))
		JOIN order_addresses a ON a.order_id=o.id AND a.address_type='shipping'
		LEFT JOIN order_item_operations op ON op.order_id=oi.order_id AND op.product_name=oi.product_name
		LEFT JOIN LATERAL (
			SELECT SUM(si.quantity) AS quantity
			FROM shipment_items si
			WHERE si.order_id=oi.order_id AND si.product_name=oi.product_name
		) shipped ON TRUE
		LEFT JOIN LATERAL (
			SELECT event_type,note,delayed_delivery_date,carrier,tracking_number,tracking_url,resolved_at,resolution_status,resolution_note
			FROM supplier_order_events e
			WHERE e.order_id=oi.order_id AND e.product_id=oi.product_id
			ORDER BY e.id DESC LIMIT 1
		) latest ON TRUE
		WHERE LOWER(BTRIM(oi.supplier_name))=LOWER(BTRIM($1))
			AND oi.product_id IS NOT NULL
			AND o.status IN ('paid','invoice_approved','fulfilled')
			AND COALESCE(u.fraud_flagged,FALSE)=FALSE
		ORDER BY o.created_at, o.id, oi.product_name`, strings.TrimSpace(supplierName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SupplierOrderItem, 0)
	for rows.Next() {
		var item SupplierOrderItem
		if err := rows.Scan(&item.OrderID, &item.OrderStatus, &item.OrderCreatedAt, &item.ProductID,
			&item.ProductName, &item.SKU, &item.Quantity, &item.ShippedQuantity, &item.RemainingQuantity,
			&item.SupplierName, &item.RecipientName,
			&item.CompanyName, &item.AddressLine1, &item.AddressLine2, &item.PostalCode, &item.City,
			&item.CountryCode, &item.OriginalDelivery, &item.DelayedDelivery, &item.LatestEvent,
			&item.LatestEventNote, &item.LatestEventDate, &item.LatestEventResolved, &item.LatestResolution,
			&item.LatestResolutionNote, &item.LatestCarrier, &item.LatestTracking, &item.LatestTrackingURL,
			&item.Accepted); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SupplierCancellationRequests(ctx context.Context) ([]SupplierCancellationRequest, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id,e.order_id,e.product_id,e.product_name,COALESCE(oi.sku,''),e.supplier_name,
			oi.quantity,e.note,e.actor_username,e.created_at,o.status
		FROM supplier_order_events e
		JOIN orders o ON o.id=e.order_id
		JOIN order_items oi ON oi.order_id=e.order_id AND oi.product_id=e.product_id
		WHERE e.event_type='cancellation_requested' AND e.resolved_at IS NULL
		ORDER BY e.created_at,e.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	requests := make([]SupplierCancellationRequest, 0)
	for rows.Next() {
		var request SupplierCancellationRequest
		if err := rows.Scan(&request.ID, &request.OrderID, &request.ProductID, &request.ProductName,
			&request.SKU, &request.SupplierName, &request.Quantity, &request.Note, &request.ActorUsername,
			&request.CreatedAt, &request.OrderStatus); err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (s *Store) RecordSupplierOrderAction(ctx context.Context, orderID string, productID int64, supplierName, username, action, note, delayDate, carrier, trackingNumber, trackingURL string) error {
	if strings.TrimSpace(orderID) == "" || productID < 1 || strings.TrimSpace(supplierName) == "" || strings.TrimSpace(username) == "" {
		return ErrInvalidCheckout
	}
	if action != "accept" && action != "delay" && action != "cancellation_request" {
		return ErrInvalidCheckout
	}
	if action == "delay" {
		if _, err := time.Parse("2006-01-02", delayDate); err != nil {
			return ErrInvalidCheckout
		}
	}
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var productName, sku, orderStatus string
		err := tx.QueryRowContext(ctx, `SELECT oi.product_name,COALESCE(oi.sku,''),o.status
			FROM order_items oi JOIN orders o ON o.id=oi.order_id
			WHERE oi.order_id=$1 AND oi.product_id=$2
				AND LOWER(BTRIM(oi.supplier_name))=LOWER(BTRIM($3))
			FOR UPDATE OF o,oi`, orderID, productID, strings.TrimSpace(supplierName)).Scan(&productName, &sku, &orderStatus)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if action != "cancellation_request" {
			if err := ensureOrderNotFraudBlockedTx(ctx, tx, orderID); err != nil {
				return err
			}
		}
		if orderStatus != "paid" && orderStatus != "invoice_approved" {
			return ErrInvalidTransition
		}
		if _, err := itemOperationsForOrder(ctx, tx, orderID); err != nil {
			return err
		}
		var remaining int
		var cancellationPending bool
		if err := tx.QueryRowContext(ctx, `SELECT
			oi.quantity-COALESCE((SELECT SUM(si.quantity) FROM shipment_items si
				WHERE si.order_id=oi.order_id AND si.product_name=oi.product_name),0),
			EXISTS(SELECT 1 FROM supplier_order_events WHERE order_id=$1 AND product_id=$2
				AND event_type='cancellation_requested' AND resolved_at IS NULL)
			FROM order_items oi WHERE oi.order_id=$1 AND oi.product_id=$2`,
			orderID, productID).Scan(&remaining, &cancellationPending); err != nil {
			return err
		}
		if remaining <= 0 || cancellationPending && action != "cancellation_request" {
			return ErrInvalidTransition
		}
		if action == "accept" {
			var accepted bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
				SELECT 1 FROM supplier_order_events
				WHERE order_id=$1 AND product_id=$2 AND event_type='order_accepted'
			)`, orderID, productID).Scan(&accepted); err != nil {
				return err
			}
			if accepted {
				return ErrInvalidTransition
			}
			if _, err := tx.ExecContext(ctx, `UPDATE order_item_operations SET status='stock_confirmed',updated_by=$3,updated_at=NOW()
				WHERE order_id=$1 AND product_name=$2`, orderID, productName, "supplier:"+username); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO order_item_operation_history(order_id,product_name,action,detail,actor_email)
				VALUES($1,$2,'supplier_order_accepted',$3,$4)`, orderID, productName, strings.TrimSpace(note), "supplier:"+username); err != nil {
				return err
			}
			return insertSupplierOrderEvent(ctx, tx, orderID, productID, productName, supplierName,
				"order_accepted", note, "", "", "", "", 0, username)
		}
		if action == "cancellation_request" {
			var pending bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
				SELECT 1 FROM supplier_order_events
				WHERE order_id=$1 AND product_id=$2 AND event_type='cancellation_requested' AND resolved_at IS NULL
			)`, orderID, productID).Scan(&pending); err != nil {
				return err
			}
			if pending {
				return ErrInvalidTransition
			}
			if err := insertSupplierOrderEvent(ctx, tx, orderID, productID, productName, supplierName,
				"cancellation_requested", note, "", "", "", "", 0, username); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO order_item_operation_history(order_id,product_name,action,detail,actor_email)
				VALUES($1,$2,'supplier_cancellation_requested',$3,$4)`, orderID, productName, strings.TrimSpace(note), "supplier:"+username)
			return err
		}
		if action == "delay" {
			result, err := tx.ExecContext(ctx, `UPDATE order_item_operations
				SET delayed_delivery_date=$3::date,updated_by=$4,updated_at=NOW()
				WHERE order_id=$1 AND product_name=$2`, orderID, productName, delayDate, "supplier:"+username)
			if err != nil {
				return err
			}
			if changed, err := result.RowsAffected(); err != nil {
				return err
			} else if changed != 1 {
				return ErrNotFound
			}
			detail := strings.TrimSpace(note)
			if detail != "" {
				detail = delayDate + " — " + detail
			} else {
				detail = delayDate
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO order_item_operation_history(order_id,product_name,action,detail,actor_email)
				VALUES($1,$2,'supplier_delay_reported',$3,$4)`, orderID, productName, detail, "supplier:"+username); err != nil {
				return err
			}
			return insertSupplierOrderEvent(ctx, tx, orderID, productID, productName, supplierName,
				"delay_reported", note, delayDate, "", "", "", 0, username)
		}
		return ErrInvalidCheckout
	})
}

type SupplierShipmentLine struct {
	ProductID int64 `json:"productId"`
	Quantity  int   `json:"quantity"`
}

func (s *Store) RecordSupplierShipment(ctx context.Context, orderID, supplierName, username, note, carrier, trackingNumber, trackingURL, publicBaseURL string, seller InvoiceSeller, items []SupplierShipmentLine) error {
	if strings.TrimSpace(orderID) == "" || strings.TrimSpace(supplierName) == "" ||
		strings.TrimSpace(username) == "" || strings.TrimSpace(carrier) == "" ||
		strings.TrimSpace(trackingNumber) == "" || len(items) == 0 || len(items) > 50 ||
		len([]rune(note)) > 1000 {
		return ErrInvalidCheckout
	}
	seen := make(map[int64]bool, len(items))
	for _, item := range items {
		if item.ProductID < 1 || item.Quantity < 1 || seen[item.ProductID] {
			return ErrInvalidCheckout
		}
		seen[item.ProductID] = true
	}
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var provider, status string
		var total int
		if err := tx.QueryRowContext(ctx, `SELECT payment_provider,status,total_cents FROM orders WHERE id=$1 FOR UPDATE`,
			orderID).Scan(&provider, &status, &total); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status != "paid" && status != "invoice_approved" {
			return ErrInvalidTransition
		}
		if _, err := itemOperationsForOrder(ctx, tx, orderID); err != nil {
			return err
		}
		lines := make([]shipmentLine, 0, len(items))
		for _, item := range items {
			var productName string
			var ordered, shipped int
			err := tx.QueryRowContext(ctx, `SELECT oi.product_name,oi.quantity,
				COALESCE((SELECT SUM(si.quantity) FROM shipment_items si
					WHERE si.order_id=oi.order_id AND si.product_name=oi.product_name),0)
				FROM order_items oi
				WHERE oi.order_id=$1 AND oi.product_id=$2
					AND LOWER(BTRIM(oi.supplier_name))=LOWER(BTRIM($3))
				FOR UPDATE OF oi`, orderID, item.ProductID, strings.TrimSpace(supplierName)).
				Scan(&productName, &ordered, &shipped)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			if err != nil {
				return err
			}
			if item.Quantity > ordered-shipped {
				return ErrInvalidTransition
			}
			var cancellationPending bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
				SELECT 1 FROM supplier_order_events
				WHERE order_id=$1 AND product_id=$2 AND event_type='cancellation_requested' AND resolved_at IS NULL
			)`, orderID, item.ProductID).Scan(&cancellationPending); err != nil {
				return err
			}
			if cancellationPending {
				return ErrInvalidTransition
			}
			lines = append(lines, shipmentLine{ProductName: productName, Quantity: item.Quantity})
		}
		return recordShipmentTx(ctx, tx, orderID, provider, status, total, "supplier",
			strings.TrimSpace(supplierName), "supplier:"+username, carrier, trackingNumber, trackingURL,
			strings.TrimSpace(note), publicBaseURL, seller, lines)
	})
}

func (s *Store) ReviewSupplierCancellationRequest(ctx context.Context, eventID int64, status, note, actor string) error {
	if eventID < 1 || (status != "handled" && status != "rejected") ||
		strings.TrimSpace(note) == "" || len([]rune(note)) > 1000 || strings.TrimSpace(actor) == "" {
		return ErrInvalidCheckout
	}
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var orderID, productName string
		err := tx.QueryRowContext(ctx, `UPDATE supplier_order_events
			SET resolved_at=NOW(),resolved_by=$2,resolution_note=$3,resolution_status=$4
			WHERE id=$1 AND event_type='cancellation_requested' AND resolved_at IS NULL
			RETURNING order_id,product_name`, eventID, actor, strings.TrimSpace(note), status).Scan(&orderID, &productName)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO order_item_operation_history(order_id,product_name,action,detail,actor_email)
			VALUES($1,$2,$3,$4,$5)`, orderID, productName, "supplier_cancellation_"+status, strings.TrimSpace(note), actor)
		return err
	})
}

func insertSupplierOrderEvent(ctx context.Context, tx *sql.Tx, orderID string, productID int64, productName, supplierName, eventType, note, delayDate, carrier, trackingNumber, trackingURL string, quantity int, username string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO supplier_order_events
		(order_id,product_id,product_name,supplier_name,event_type,note,delayed_delivery_date,carrier,tracking_number,tracking_url,quantity,actor_username)
		VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,'')::date,$8,$9,$10,$11,$12)`,
		orderID, productID, productName, strings.TrimSpace(supplierName), eventType, strings.TrimSpace(note),
		delayDate, strings.TrimSpace(carrier), strings.TrimSpace(trackingNumber), strings.TrimSpace(trackingURL), quantity, username)
	return err
}
