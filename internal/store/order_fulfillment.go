package store

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
)

func (s *Store) MarkOrderShipped(ctx context.Context, orderID, carrier, trackingNumber, trackingURL, publicBaseURL string, seller InvoiceSeller) error {
	return s.markOrderShipped(ctx, orderID, "system:shipment", "oms", carrier, trackingNumber, trackingURL, publicBaseURL, seller, false)
}

func (s *Store) MarkOrderShippedBy(ctx context.Context, orderID, actor, carrier, trackingNumber, trackingURL, publicBaseURL string, seller InvoiceSeller) error {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "system:shipment"
	}
	return s.markOrderShipped(ctx, orderID, actor, "oms", carrier, trackingNumber, trackingURL, publicBaseURL, seller, false)
}

func (s *Store) MarkWarehouseShipmentBy(ctx context.Context, orderID, actor, carrier, trackingNumber, trackingURL, publicBaseURL string, seller InvoiceSeller) error {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "system:shipment"
	}
	return s.markOrderShipped(ctx, orderID, actor, "warehouse", carrier, trackingNumber, trackingURL, publicBaseURL, seller, true)
}

type shipmentLine struct {
	ProductName string
	Quantity    int
}

func (s *Store) markOrderShipped(ctx context.Context, orderID, actor, source, carrier, trackingNumber, trackingURL, publicBaseURL string, seller InvoiceSeller, warehouseOnly bool) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		if err := ensureOrderNotFraudBlockedTx(ctx, tx, orderID); err != nil {
			return err
		}
		if err := ensureOrderFulfillmentNotHeldTx(ctx, tx, orderID); err != nil {
			return err
		}
		var provider, status string
		var total int
		if err := tx.QueryRowContext(ctx, `SELECT payment_provider,status,total_cents FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&provider, &status, &total); err != nil {
			return err
		}
		if status != "paid" && !(provider == "mitte_invoice" && status == "invoice_approved") {
			return ErrInvalidTransition
		}
		if _, err := itemOperationsForOrder(ctx, tx, orderID); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT oi.product_name,
				oi.quantity-COALESCE((SELECT SUM(si.quantity) FROM shipment_items si
					WHERE si.order_id=oi.order_id AND si.product_name=oi.product_name),0)
			FROM order_items oi
			WHERE oi.order_id=$1 AND oi.quantity>COALESCE((SELECT SUM(si.quantity) FROM shipment_items si
				WHERE si.order_id=oi.order_id AND si.product_name=oi.product_name),0)
				AND ($2=FALSE OR BTRIM(COALESCE(oi.supplier_name,''))='')
			ORDER BY oi.product_name
			FOR UPDATE OF oi`, orderID, warehouseOnly)
		if err != nil {
			return err
		}
		lines := make([]shipmentLine, 0)
		for rows.Next() {
			var line shipmentLine
			if err := rows.Scan(&line.ProductName, &line.Quantity); err != nil {
				rows.Close()
				return err
			}
			lines = append(lines, line)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if len(lines) == 0 {
			return ErrInvalidTransition
		}
		if err := recordShipmentTx(ctx, tx, orderID, provider, status, total, source, "",
			actor, carrier, trackingNumber, trackingURL, "", publicBaseURL, seller, lines); err != nil {
			return err
		}
		// OMS-entered shipments are staff-confirmed handovers too. Keep the
		// fulfillment record and its audit trail consistent whether the handover
		// came from the warehouse scanner flow or the OMS shipment form.
		if source == "warehouse" || source == "oms" {
			if _, err := tx.ExecContext(ctx, `INSERT INTO warehouse_fulfillments (order_id, status, carrier, tracking_number, tracking_url, handed_over_at)
				VALUES ($1, 'handed_over', $2, $3, NULLIF($4,''), NOW())
				ON CONFLICT (order_id) DO UPDATE SET status='handed_over', carrier=EXCLUDED.carrier, tracking_number=EXCLUDED.tracking_number, tracking_url=EXCLUDED.tracking_url, handed_over_at=NOW(), updated_at=NOW()`, orderID, carrier, trackingNumber, trackingURL); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO warehouse_events (order_id, actor_email, event_type, detail) VALUES ($1,$2,'fulfillment.handed_over',$3)`,
				orderID, actor, strings.TrimSpace(carrier+" "+trackingNumber)); err != nil {
				return err
			}
		}
		return nil
	})
}

func recordShipmentTx(ctx context.Context, tx *sql.Tx, orderID, provider, orderStatus string, total int, source, supplierName, actor, carrier, trackingNumber, trackingURL, note, publicBaseURL string, seller InvoiceSeller, lines []shipmentLine) error {
	var duplicate bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM shipments
		WHERE order_id=$1 AND LOWER(BTRIM(carrier))=LOWER(BTRIM($2))
			AND LOWER(BTRIM(tracking_number))=LOWER(BTRIM($3))
	)`, orderID, carrier, trackingNumber).Scan(&duplicate); err != nil {
		return err
	}
	if duplicate {
		return ErrInvalidTransition
	}
	var shipmentID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO shipments
		(order_id,carrier,tracking_number,tracking_url,source,supplier_name,created_by)
		VALUES($1,$2,$3,NULLIF($4,''),$5,$6,$7) RETURNING id`,
		orderID, strings.TrimSpace(carrier), strings.TrimSpace(trackingNumber), strings.TrimSpace(trackingURL),
		source, strings.TrimSpace(supplierName), strings.TrimSpace(actor)).Scan(&shipmentID); err != nil {
		return err
	}
	shipmentItems := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if line.Quantity < 1 {
			return ErrInvalidCheckout
		}
		var ordered int
		var shipped int
		if err := tx.QueryRowContext(ctx, `SELECT oi.quantity,
			COALESCE((SELECT SUM(si.quantity) FROM shipment_items si
				WHERE si.order_id=oi.order_id AND si.product_name=oi.product_name),0)
			FROM order_items oi WHERE oi.order_id=$1 AND oi.product_name=$2`,
			orderID, line.ProductName).Scan(&ordered, &shipped); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if line.Quantity > ordered-shipped {
			return ErrInvalidTransition
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO shipment_items(shipment_id,order_id,product_name,quantity)
			VALUES($1,$2,$3,$4)`, shipmentID, orderID, line.ProductName, line.Quantity); err != nil {
			return err
		}
		action := "shipment_dispatched"
		if source == "supplier" {
			action = "supplier_shipment_reported"
		}
		detail := fmt.Sprintf("Menge %d von %d · %s · %s", line.Quantity, ordered,
			strings.TrimSpace(carrier), strings.TrimSpace(trackingNumber))
		if strings.TrimSpace(trackingURL) != "" {
			detail += " · " + strings.TrimSpace(trackingURL)
		}
		if strings.TrimSpace(note) != "" {
			detail += " · " + strings.TrimSpace(note)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_item_operation_history(order_id,product_name,action,detail,actor_email)
			VALUES($1,$2,$3,$4,$5)`, orderID, line.ProductName, action, detail, actor); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE order_item_operations
			SET status=CASE WHEN $3::integer + $4::integer >= $5::integer THEN 'fulfillment_completed' ELSE 'fulfillment' END,
				updated_by=$6,updated_at=NOW()
			WHERE order_id=$1 AND product_name=$2`,
			orderID, line.ProductName, shipped, line.Quantity, ordered, actor); err != nil {
			return err
		}
		if source == "supplier" {
			var productID int64
			if err := tx.QueryRowContext(ctx, `SELECT product_id FROM order_items WHERE order_id=$1 AND product_name=$2`,
				orderID, line.ProductName).Scan(&productID); err != nil {
				return err
			}
			if err := insertSupplierOrderEvent(ctx, tx, orderID, productID, line.ProductName, supplierName,
				"shipment_reported", note, "", carrier, trackingNumber, trackingURL, line.Quantity,
				strings.TrimPrefix(actor, "supplier:")); err != nil {
				return err
			}
		}
		shipmentItems = append(shipmentItems, map[string]any{"product_name": line.ProductName, "quantity": line.Quantity})
	}

	var complete bool
	if err := tx.QueryRowContext(ctx, `SELECT NOT EXISTS (
		SELECT 1 FROM order_items oi
		WHERE oi.order_id=$1 AND oi.quantity>COALESCE((
			SELECT SUM(si.quantity) FROM shipment_items si
			WHERE si.order_id=oi.order_id AND si.product_name=oi.product_name
		),0)
	)`, orderID).Scan(&complete); err != nil {
		return err
	}
	if complete {
		if err := transitionOrderStatus(ctx, tx, orderID, orderStatus, "fulfilled", "shipment_dispatched"); err != nil {
			return err
		}
	}

	body := "Eine Sendung zu deiner Bestellung ist unterwegs."
	subject := "Eine Sendung zu deiner Bestellung ist unterwegs"
	if complete {
		body = "Deine Bestellung wurde vollständig versendet."
		subject = "Deine Bestellung ist vollständig versendet"
	}
	body += "\n\nVersanddienstleister: " + strings.TrimSpace(carrier) +
		"\nSendungsnummer: " + strings.TrimSpace(trackingNumber)
	if strings.TrimSpace(trackingURL) != "" {
		body += "\nSendungsverfolgung: " + strings.TrimSpace(trackingURL)
	}
	body += "\n\nVersendete Artikel:"
	for _, line := range lines {
		body += fmt.Sprintf("\n%d × %s", line.Quantity, line.ProductName)
	}
	if err := enqueueOrderEvent(ctx, tx, orderID, "shipment.dispatched", map[string]any{
		"order_id": orderID, "shipment_id": shipmentID, "source": source, "supplier_name": supplierName,
		"carrier": carrier, "tracking_number": trackingNumber, "tracking_url": trackingURL, "items": shipmentItems,
	}); err != nil {
		return err
	}
	if complete {
		invoice, accessToken, err := issueInvoiceTx(ctx, tx, orderID, provider, total, seller)
		if err != nil {
			return err
		}
		if provider == "mitte_invoice" {
			if _, err := tx.ExecContext(ctx, `INSERT INTO payment_transactions (order_id,provider,reference,amount_cents)
				VALUES ($1,'mitte_invoice',$2,$3) ON CONFLICT (order_id) DO NOTHING`,
				orderID, "MF-"+strings.ToUpper(orderID), total); err != nil {
				return err
			}
			if err := setReceivableStatus(ctx, tx, orderID, "sent", "invoice_issued_after_shipment"); err != nil {
				return err
			}
			body += "\n\nDeine Rechnung " + invoice.Number + " ist jetzt verfügbar: " +
				invoiceAccessURL(publicBaseURL, accessToken) + "?format=pdf."
			body += " XML: " + invoiceAccessURL(publicBaseURL, accessToken) + "?format=xml."
			body += " Bitte überweise bis zum " + invoice.DueAt.Format("02.01.2006") + "."
		} else {
			body += "\n\nDeine Rechnung " + invoice.Number + " ist jetzt verfügbar: " +
				invoiceAccessURL(publicBaseURL, accessToken) + "?format=pdf."
			body += " XML: " + invoiceAccessURL(publicBaseURL, accessToken) + "?format=xml."
		}
		subject += " – Rechnung " + invoice.Number
		if err := enqueueOrderEvent(ctx, tx, orderID, "invoice.issued", map[string]any{
			"order_id": orderID, "invoice_number": invoice.Number, "total_cents": invoice.TotalCents,
			"currency": "EUR", "format": "application/pdf, application/xml",
		}); err != nil {
			return err
		}
	}
	return enqueueOrderEmail(ctx, tx, orderID, subject, body)
}

// ExpirePendingOrders reclaims inventory whose payment reservation expired.
// It intentionally selects candidates without locking them: cancellation then
// locks one order at a time, so several workers cannot deadlock while handling
// different orders that contain the same product.
func (s *Store) ExpirePendingOrders(ctx context.Context, limit int) (int, error) {
	if limit < 1 {
		return 0, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM orders
		WHERE status = 'pending_payment' AND inventory_reserved = TRUE
		  AND checkout_expires_at <= NOW()
		ORDER BY checkout_expires_at
		LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var orderIDs []string
	for rows.Next() {
		var orderID string
		if err := rows.Scan(&orderID); err != nil {
			return 0, err
		}
		orderIDs = append(orderIDs, orderID)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	expired := 0
	for _, orderID := range orderIDs {
		cancelled := false
		if err := s.withTransaction(ctx, func(tx *sql.Tx) error {
			var err error
			cancelled, err = cancelPendingOrderTx(ctx, tx, orderID, "checkout_reservation_expired")
			return err
		}); err != nil {
			return expired, err
		}
		if cancelled {
			expired++
		}
	}
	return expired, nil
}

func (s *Store) FulfillStripeCheckout(ctx context.Context, orderID, checkoutSessionID, paymentIntentID string, amountTotal int64, currency string) error {
	return s.fulfillCheckout(ctx, orderID, checkoutSessionID, paymentIntentID, amountTotal, currency, "stripe_checkout_completed")
}

// FulfillMittePay completes a first-party sandbox session. The sandbox
// never accepts or stores card or bank credentials.
func (s *Store) FulfillMittePay(ctx context.Context, orderID, sessionID string, amountTotal int64) error {
	return s.fulfillCheckout(ctx, orderID, sessionID, "mfpay_"+sessionID, amountTotal, "eur", "mitte_pay_confirmed")
}

// ConfigureInvoiceSandbox records the first-party development invoice provider
// before its simulated approval is completed.
func (s *Store) ConfigureInvoiceSandbox(ctx context.Context, orderID, sessionID, checkoutURL string) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `UPDATE orders SET stripe_checkout_session_id = $2, stripe_checkout_url = $3, payment_provider = 'mitte_invoice' WHERE id = $1 AND status = 'pending_payment'`, orderID, sessionID, checkoutURL)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrNotFound
		}
		result, err = tx.ExecContext(ctx, `UPDATE checkout_sagas SET status = 'awaiting_payment', last_error = '', updated_at = NOW()
			WHERE order_id = $1 AND status IN ('started', 'retryable', 'awaiting_payment')`, orderID)
		if err != nil {
			return err
		}
		rows, err = result.RowsAffected()
		if err != nil {
			return err
		}
		if rows != 1 {
			return ErrInvalidTransition
		}
		return nil
	})
}

// ApproveMitteInvoice admits a development invoice order to fulfilment.
// Settlement happens only after full shipment and a recorded incoming transfer.
func (s *Store) ApproveMitteInvoice(ctx context.Context, orderID, sessionID string) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var expected, status, email string
		var totalCents int
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(stripe_checkout_session_id, ''), status, customer_email, total_cents FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&expected, &status, &email, &totalCents); err != nil {
			return err
		}
		if expected != sessionID {
			return ErrInvalidTransition
		}
		if status == "invoice_approved" {
			return nil // idempotent provider callback: do not send duplicate mail.
		}
		if status != "pending_payment" {
			return ErrInvalidTransition
		}
		if _, err := tx.ExecContext(ctx, `UPDATE checkout_sagas SET status = 'completed', last_error = '', updated_at = NOW()
			WHERE order_id = $1`, orderID); err != nil {
			return err
		}
		if err := transitionOrderStatus(ctx, tx, orderID, "pending_payment", "invoice_approved", "mitte_invoice_approved"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO warehouse_fulfillments (order_id, status)
			SELECT $1, 'ready_to_pick'
			WHERE EXISTS (SELECT 1 FROM order_items oi WHERE oi.order_id=$1 AND BTRIM(COALESCE(oi.supplier_name,''))='')
			ON CONFLICT (order_id) DO NOTHING`, orderID); err != nil {
			return err
		}
		// Register the approved receivable immediately. It deliberately has no
		// invoice number or due date yet: both are created only after full shipment.
		if _, err := tx.ExecContext(ctx, `INSERT INTO payment_transactions (order_id, provider, reference, amount_cents)
			VALUES ($1, 'mitte_invoice', $2, $3) ON CONFLICT (order_id) DO NOTHING`, orderID, "MF-"+strings.ToUpper(orderID), totalCents); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO receivable_status_history (order_id, to_status, reason)
			VALUES ($1, 'created', 'invoice_provider_approved') ON CONFLICT DO NOTHING`, orderID); err != nil {
			return err
		}
		if err := enqueueOrderEmail(ctx, tx, orderID, "Eingangsbestätigung deiner Bestellung", "Wir haben deine Bestellung erhalten und prüfen die Bearbeitung."); err != nil {
			return err
		}
		if err := enqueueOrderEmail(ctx, tx, orderID, "Auftragsbestätigung", "Dein Kauf auf Rechnung wurde freigegeben. Wir bereiten deine Bestellung vor; Rechnung und Zahlungsdaten folgen erst nach vollständigem Versand."); err != nil {
			return err
		}
		return enqueueOrderEvent(ctx, tx, orderID, "invoice.approved", map[string]string{"order_id": orderID, "customer_email": email})
	})
}

func (s *Store) fulfillCheckout(ctx context.Context, orderID, checkoutSessionID, paymentIntentID string, amountTotal int64, currency, reason string) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {

		var status, expectedSessionID string
		var totalCents int64
		var reserved bool
		if err := tx.QueryRowContext(ctx, `SELECT status, COALESCE(stripe_checkout_session_id, ''), total_cents, inventory_reserved FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&status, &expectedSessionID, &totalCents, &reserved); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if expectedSessionID != checkoutSessionID || currency != "eur" || amountTotal != totalCents {
			return fmt.Errorf("checkout session does not match order %s", orderID)
		}
		if status == "paid" {
			return nil
		}
		if status != "pending_payment" {
			return fmt.Errorf("order %s has status %q: %w", orderID, status, ErrInvalidTransition)
		}

		if !reserved {
			items, err := orderItemsForFulfillment(ctx, tx, orderID)
			if err != nil {
				return err
			}
			slices.SortFunc(items, func(left, right orderFulfillmentItem) int { return cmp.Compare(left.ProductID, right.ProductID) })
			for _, item := range items {
				var inventory int
				if err := tx.QueryRowContext(ctx, `SELECT inventory FROM products WHERE id = $1 FOR UPDATE`, item.ProductID).Scan(&inventory); err != nil {
					return err
				}
				if inventory < item.Quantity {
					return ErrInsufficientInventory
				}
			}
			for _, item := range items {
				var before, after int
				if err := tx.QueryRowContext(ctx, `UPDATE products SET inventory=inventory-$1 WHERE id=$2 AND inventory >= $1 RETURNING inventory+$1,inventory`,
					item.Quantity, item.ProductID).Scan(&before, &after); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return ErrInsufficientInventory
					}
					return err
				}
				if err := recordInventoryMovementTx(ctx, tx, item.ProductID, "reservation", -item.Quantity, before, after,
					"Stock reserved when payment was confirmed", orderID, "system:payment"); err != nil {
					return err
				}
			}
		}
		if paymentIntentID != "" {
			if _, err := tx.ExecContext(ctx, `UPDATE orders SET stripe_payment_intent_id = $2 WHERE id = $1`, orderID, paymentIntentID); err != nil {
				return err
			}
		}
		if err := transitionOrderStatus(ctx, tx, orderID, "pending_payment", "paid", reason); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE checkout_sagas SET status = 'completed', last_error = '', updated_at = NOW()
			WHERE order_id = $1`, orderID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO warehouse_fulfillments (order_id, status)
			SELECT $1, 'ready_to_pick'
			WHERE EXISTS (SELECT 1 FROM order_items oi WHERE oi.order_id=$1 AND BTRIM(COALESCE(oi.supplier_name,''))='')
			ON CONFLICT (order_id) DO NOTHING`, orderID); err != nil {
			return err
		}
		if err := enqueueOrderEmail(ctx, tx, orderID, "Bestellung bestätigt", "Wir haben deine Bestellung erhalten und die Zahlung bestätigt. Die Gesamtrechnung folgt nach dem vollständigen Versand aller Positionen."); err != nil {
			return err
		}
		return nil
	})
}

// ReconcileLateStripeCheckout restores an order that the expiry job cancelled
// before a paid Stripe session could be delivered. It is deliberately separate
// from normal fulfillment and must only be called with a session retrieved from
// Stripe using the account secret.
func (s *Store) ReconcileLateStripeCheckout(ctx context.Context, orderID, checkoutSessionID, paymentIntentID string, amountTotal int64, currency string) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var status, expectedSessionID string
		var totalCents int64
		if err := tx.QueryRowContext(ctx, `SELECT status, COALESCE(stripe_checkout_session_id, ''), total_cents FROM orders WHERE id = $1 FOR UPDATE`, orderID).
			Scan(&status, &expectedSessionID, &totalCents); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if expectedSessionID != checkoutSessionID || currency != "eur" || amountTotal != totalCents {
			return fmt.Errorf("checkout session does not match order %s", orderID)
		}
		if status == "paid" {
			return nil
		}
		if status != "cancelled" {
			return fmt.Errorf("order %s has status %q: %w", orderID, status, ErrInvalidTransition)
		}
		if err := reserveInventoryForOrder(ctx, tx, orderID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE orders SET inventory_reserved = TRUE, stripe_payment_intent_id = NULLIF($2, '') WHERE id = $1`, orderID, paymentIntentID); err != nil {
			return err
		}
		return transitionLateStripePayment(ctx, tx, orderID)
	})
}

func transitionLateStripePayment(ctx context.Context, tx *sql.Tx, orderID string) error {
	// This path is intentionally not part of the public order state machine. It
	// is reachable only after ReconcileLateStripeCheckout has retrieved a paid
	// Checkout Session from Stripe and re-reserved inventory under this lock.
	if _, err := tx.ExecContext(ctx, `UPDATE orders SET status = 'paid', paid_at = NOW(), cancelled_at = NULL WHERE id = $1 AND status = 'cancelled'`, orderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_status_history (order_id, from_status, to_status, reason)
		VALUES ($1, 'cancelled', 'paid', 'stripe_payment_reconciled_after_expiry')`, orderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE checkout_sagas SET status = 'completed', last_error = '', updated_at = NOW()
		WHERE order_id = $1`, orderID); err != nil {
		return err
	}
	return enqueueOrderEvent(ctx, tx, orderID, "order.status_changed", map[string]string{
		"order_id": orderID, "from_status": "cancelled", "to_status": "paid", "reason": "stripe_payment_reconciled_after_expiry",
	})
}

func reserveInventoryForOrder(ctx context.Context, tx *sql.Tx, orderID string) error {
	items, err := orderItemsForFulfillment(ctx, tx, orderID)
	if err != nil {
		return err
	}
	slices.SortFunc(items, func(left, right orderFulfillmentItem) int { return cmp.Compare(left.ProductID, right.ProductID) })
	for _, item := range items {
		var inventory int
		if err := tx.QueryRowContext(ctx, `SELECT inventory FROM products WHERE id = $1 FOR UPDATE`, item.ProductID).Scan(&inventory); err != nil {
			return err
		}
		if inventory < item.Quantity {
			return ErrInsufficientInventory
		}
	}
	for _, item := range items {
		var before, after int
		if err := tx.QueryRowContext(ctx, `UPDATE products SET inventory=inventory-$1 WHERE id=$2 AND inventory >= $1 RETURNING inventory+$1,inventory`,
			item.Quantity, item.ProductID).Scan(&before, &after); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrInsufficientInventory
			}
			return err
		}
		if err := recordInventoryMovementTx(ctx, tx, item.ProductID, "reservation", -item.Quantity, before, after,
			"Stock reserved when payment was confirmed", orderID, "system:payment"); err != nil {
			return err
		}
	}
	return nil
}

type orderFulfillmentItem struct {
	ProductID int64
	Quantity  int
}

func restoreReservedInventory(ctx context.Context, tx executor, orderID string) error {
	items, err := orderItemsForFulfillment(ctx, tx, orderID)
	if err != nil {
		return err
	}
	slices.SortFunc(items, func(left, right orderFulfillmentItem) int { return cmp.Compare(left.ProductID, right.ProductID) })
	for _, item := range items {
		var before, after int
		if err := tx.QueryRowContext(ctx, `UPDATE products SET inventory=inventory+$1 WHERE id=$2 RETURNING inventory-$1,inventory`,
			item.Quantity, item.ProductID).Scan(&before, &after); err != nil {
			return err
		}
		if err := recordInventoryMovementTx(ctx, tx, item.ProductID, "reservation_release", item.Quantity, before, after,
			"Reserved stock released", orderID, "system:order"); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE orders SET inventory_reserved = FALSE WHERE id = $1`, orderID)
	return err
}

func orderItemsForFulfillment(ctx context.Context, q queryer, orderID string) ([]orderFulfillmentItem, error) {
	rows, err := q.QueryContext(ctx, `SELECT product_id, quantity FROM order_items WHERE order_id = $1 AND product_id IS NOT NULL`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]orderFulfillmentItem, 0)
	for rows.Next() {
		var item orderFulfillmentItem
		if err := rows.Scan(&item.ProductID, &item.Quantity); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("order %s has no fulfillable items", orderID)
	}
	return items, nil
}
