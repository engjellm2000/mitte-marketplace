package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"mitte/internal/commerce"
)

var ErrInvalidTransition = errors.New("invalid order status transition")
var ErrInsufficientInventory = errors.New("insufficient inventory at payment")

var allowedTransitions = map[string][]string{
	"pending_payment":  {"paid", "invoice_approved", "cancelled"},
	"invoice_approved": {"fulfilled", "cancelled"},
	"paid":             {"fulfilled", "cancelled"},
	"fulfilled":        {"paid"},
	"cancelled":        {},
}

var allowedReceivableTransitions = map[string]map[string]bool{
	"created":  {"sent": true, "declined": true, "cancelled": true},
	"sent":     {"paid": true, "overdue": true, "cancelled": true},
	"overdue":  {"paid": true, "cancelled": true},
	"paid":     {"sent": true, "paid_out": true}, // sent is a correction after reallocation.
	"paid_out": {"completed": true},
}

type Order struct {
	ID                      string
	Status                  string
	TotalCents              int
	SubtotalCents           int
	DiscountCents           int
	ShippingCents           int
	TaxCents                int
	CustomerName            string
	CustomerEmail           string
	PromotionCode           string
	StripeCheckoutSessionID string
	StripePaymentIntentID   string
	PaidAt                  *time.Time
	CreatedAt               time.Time
	CustomerType            string
	CompanyName             string
	VATID                   string
	BuyerReference          string
	ProcurementMethod       string
	Note                    string
	BillingAddress          Address
	ShippingAddress         Address
	Items                   []OrderItem
	Shipment                *Shipment
	Invoice                 *Invoice
	PaymentProvider         string
}

type Shipment struct {
	Carrier, TrackingNumber, TrackingURL string
	ShippedAt                            time.Time
}

type Invoice struct {
	Number     string
	IssuedAt   time.Time
	DueAt      time.Time
	TotalCents int
}

type OrderItem struct {
	SKU            string
	Name           string
	UnitPriceCents int
	Quantity       int
	SupplierName   string `json:"-"`
}

type OperationsOrderItem struct {
	OrderID        string
	ProductName    string
	SKU            string
	Quantity       int
	UnitPriceCents int
	OrderStatus    string
	CustomerName   string
	CustomerEmail  string
	OrderCreatedAt time.Time
}

type CheckoutLineItem = commerce.CheckoutLineItem
type CheckoutResult = commerce.CheckoutResult

func ValidOrderStatusTransition(from, to string) bool {
	for _, next := range allowedTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

func (s *Store) AttachCheckoutSession(ctx context.Context, orderID, sessionID, checkoutURL string) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `UPDATE orders SET stripe_checkout_session_id = $2, stripe_checkout_url = $3
			WHERE id = $1 AND status IN ('pending_payment', 'paid')
			AND (stripe_checkout_session_id IS NULL OR stripe_checkout_session_id = $2)`, orderID, sessionID, checkoutURL)
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

func (s *Store) MarkCheckoutSagaRetryable(ctx context.Context, orderID, reason string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE checkout_sagas
		SET status = 'retryable', attempts = attempts + 1, last_error = LEFT($2, 1000), updated_at = NOW()
		WHERE order_id = $1 AND status IN ('started', 'retryable')`, orderID, reason)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return ErrInvalidTransition
	}
	return nil
}

func (s *Store) ConfigureTransferCheckout(ctx context.Context, orderID, sessionID, checkoutURL, reference string, amountCents int) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `UPDATE orders
			SET stripe_checkout_session_id = $2, stripe_checkout_url = $3, payment_provider = 'mitte_transfer'
			WHERE id = $1 AND status = 'pending_payment'
			AND (stripe_checkout_session_id IS NULL OR stripe_checkout_session_id = $2)`, orderID, sessionID, checkoutURL)
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
		if _, err := tx.ExecContext(ctx, `INSERT INTO payment_transactions (order_id, provider, reference, amount_cents)
			VALUES ($1, 'mitte_transfer', $2, $3) ON CONFLICT (order_id) DO NOTHING`, orderID, reference, amountCents); err != nil {
			return err
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

func (s *Store) OrderByID(ctx context.Context, orderID string) (Order, error) {
	const query = `SELECT id, status, total_cents, COALESCE(subtotal_cents, total_cents), discount_cents, shipping_cents, tax_cents,
		customer_name, customer_email, COALESCE(promotion_code, ''),
		COALESCE(stripe_checkout_session_id, ''), COALESCE(stripe_payment_intent_id, ''), paid_at
		FROM orders WHERE id = $1`
	order, err := scanOrder(s.db.QueryRowContext(ctx, query, orderID))
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	return order, err
}

// OrdersForCustomer returns only the authenticated customer's own immutable
// order snapshots, including the products and addresses used at checkout.
func (s *Store) OrdersForCustomer(ctx context.Context, customerID int64) ([]Order, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, status, total_cents, COALESCE(subtotal_cents, total_cents), discount_cents, shipping_cents, tax_cents,
		customer_name, customer_email, COALESCE(promotion_code, ''), COALESCE(stripe_checkout_session_id, ''),
		COALESCE(stripe_payment_intent_id, ''), paid_at, created_at, customer_type, COALESCE(company_name, ''),
		COALESCE(vat_id, ''), COALESCE(buyer_reference, ''), procurement_method, COALESCE(customer_note, ''), payment_provider
		FROM orders WHERE customer_id = $1 ORDER BY created_at DESC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]Order, 0)
	for rows.Next() {
		var order Order
		if err := rows.Scan(&order.ID, &order.Status, &order.TotalCents, &order.SubtotalCents, &order.DiscountCents,
			&order.ShippingCents, &order.TaxCents, &order.CustomerName, &order.CustomerEmail, &order.PromotionCode,
			&order.StripeCheckoutSessionID, &order.StripePaymentIntentID, &order.PaidAt, &order.CreatedAt, &order.CustomerType,
			&order.CompanyName, &order.VATID, &order.BuyerReference, &order.ProcurementMethod, &order.Note, &order.PaymentProvider); err != nil {
			return nil, err
		}
		if order.Items, err = orderItems(ctx, s.db, order.ID); err != nil {
			return nil, err
		}
		if order.BillingAddress, err = orderAddress(ctx, s.db, order.ID, "billing"); err != nil {
			return nil, err
		}
		if order.ShippingAddress, err = orderAddress(ctx, s.db, order.ID, "shipping"); err != nil {
			return nil, err
		}
		if order.Shipment, err = shipmentForOrder(ctx, s.db, order.ID); err != nil {
			return nil, err
		}
		if order.Invoice, err = invoiceForOrder(ctx, s.db, order.ID); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

// OrdersForOperations returns the immutable order snapshots required by the
// internal fulfilment queue. It must only be called after staff authorization.
func (s *Store) OrdersForOperations(ctx context.Context) ([]Order, error) {
	return s.ordersForOperations(ctx, "")
}

type OperationsOrderSummary struct {
	Open              int `json:"open"`
	Paid              int `json:"paid"`
	Awaiting          int `json:"awaiting"`
	ReadyToPick       int `json:"readyToPick"`
	FulfillmentActive int `json:"fulfillmentActive"`
	OpenValueCents    int `json:"openValueCents"`
}

type OperationsBacklogItem struct {
	ID                string
	Status            string
	FulfillmentStatus string
	Stage             string
	CustomerName      string
	CustomerEmail     string
	TotalCents        int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type OperationsBacklogSummary struct {
	Total            int `json:"total"`
	Payment          int `json:"payment"`
	NeedsFulfillment int `json:"needsFulfillment"`
	InProgress       int `json:"inProgress"`
	Held             int `json:"held"`
}

func (s *Store) OperationsOrderSummary(ctx context.Context) (OperationsOrderSummary, error) {
	var summary OperationsOrderSummary
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FILTER (WHERE o.status <> 'cancelled'),
			COUNT(*) FILTER (WHERE o.status IN ('paid', 'fulfilled')),
			COUNT(*) FILTER (WHERE o.status IN ('pending_payment', 'invoice_approved')),
			COUNT(*) FILTER (WHERE o.status = 'paid' AND f.order_id IS NULL),
			COUNT(*) FILTER (WHERE f.status IN ('ready_to_pick', 'picking', 'packed', 'label_ready')),
			COALESCE(SUM(o.total_cents) FILTER (WHERE o.status <> 'cancelled'), 0)
		FROM orders o
		LEFT JOIN warehouse_fulfillments f ON f.order_id = o.id`).Scan(
		&summary.Open, &summary.Paid, &summary.Awaiting, &summary.ReadyToPick,
		&summary.FulfillmentActive, &summary.OpenValueCents)
	return summary, err
}

func (s *Store) OperationsBacklogPage(ctx context.Context, search, stage string, limit, offset int) ([]OperationsBacklogItem, int, OperationsBacklogSummary, error) {
	const backlogWhere = `(o.status IN ('pending_payment', 'invoice_approved')
		OR (o.status = 'paid' AND (f.order_id IS NULL OR f.status IN ('ready_to_pick', 'picking', 'packed', 'label_ready', 'held'))))`
	const stageExpression = `CASE
		WHEN o.status IN ('pending_payment', 'invoice_approved') THEN 'payment'
		WHEN f.status = 'held' THEN 'held'
		WHEN f.order_id IS NULL THEN 'needs_fulfillment'
		ELSE 'in_progress'
	END`

	var summary OperationsBacklogSummary
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),
		COUNT(*) FILTER (WHERE o.status IN ('pending_payment', 'invoice_approved')),
		COUNT(*) FILTER (WHERE o.status = 'paid' AND f.order_id IS NULL),
		COUNT(*) FILTER (WHERE o.status = 'paid' AND f.status IN ('ready_to_pick', 'picking', 'packed', 'label_ready')),
		COUNT(*) FILTER (WHERE o.status = 'paid' AND f.status = 'held')
		FROM orders o LEFT JOIN warehouse_fulfillments f ON f.order_id = o.id
		WHERE `+backlogWhere).Scan(&summary.Total, &summary.Payment, &summary.NeedsFulfillment, &summary.InProgress, &summary.Held)
	if err != nil {
		return nil, 0, OperationsBacklogSummary{}, err
	}

	args := []any{}
	where := " WHERE " + backlogWhere
	if stage != "" && stage != "all" {
		args = append(args, stage)
		where += fmt.Sprintf(" AND ("+stageExpression+") = $%d", len(args))
	}
	search = "%" + strings.TrimSpace(search) + "%"
	args = append(args, search)
	searchParameter := fmt.Sprintf("$%d", len(args))
	where += " AND (" + searchParameter + " = '%%' OR o.id ILIKE " + searchParameter +
		" OR o.customer_name ILIKE " + searchParameter + " OR o.customer_email ILIKE " + searchParameter + ")"

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders o
		LEFT JOIN warehouse_fulfillments f ON f.order_id = o.id`+where, args...).Scan(&total); err != nil {
		return nil, 0, OperationsBacklogSummary{}, err
	}

	pageArgs := append([]any(nil), args...)
	pageArgs = append(pageArgs, limit, offset)
	query := `SELECT o.id, o.status, COALESCE(f.status, ''), ` + stageExpression + `,
		o.customer_name, o.customer_email, o.total_cents, o.created_at, COALESCE(f.updated_at, o.created_at)
		FROM orders o LEFT JOIN warehouse_fulfillments f ON f.order_id = o.id` + where +
		fmt.Sprintf(" ORDER BY o.created_at ASC, o.id ASC LIMIT $%d OFFSET $%d", len(pageArgs)-1, len(pageArgs))
	rows, err := s.db.QueryContext(ctx, query, pageArgs...)
	if err != nil {
		return nil, 0, OperationsBacklogSummary{}, err
	}
	defer rows.Close()

	items := make([]OperationsBacklogItem, 0)
	for rows.Next() {
		var item OperationsBacklogItem
		if err := rows.Scan(&item.ID, &item.Status, &item.FulfillmentStatus, &item.Stage,
			&item.CustomerName, &item.CustomerEmail, &item.TotalCents, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, OperationsBacklogSummary{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, OperationsBacklogSummary{}, err
	}
	return items, total, summary, nil
}

func (s *Store) OrdersForOperationsPage(ctx context.Context, search, status string, limit, offset int) ([]Order, int, error) {
	search = "%" + strings.TrimSpace(search) + "%"
	const where = `WHERE ($1 = '%%' OR id ILIKE $1 OR customer_name ILIKE $1 OR customer_email ILIKE $1 OR COALESCE(company_name, '') ILIKE $1)
		AND ($2 = 'all' OR ($2 = 'open' AND status <> 'cancelled') OR ($2 = 'paid' AND status IN ('paid', 'fulfilled')) OR status = $2)`
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders `+where, search, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := operationsOrderSelect + where + operationsOrderOrder + ` LIMIT $3 OFFSET $4`
	orders, err := s.queryOrdersForOperations(ctx, query, search, status, limit, offset)
	return orders, total, err
}

func (s *Store) OrdersForOperationsMatrixPage(ctx context.Context, search, kind string, limit, offset int) ([]Order, int, error) {
	matrixWhere := map[string]string{
		"fulfillment":         "TRUE",
		"fulfillment-clarify": "status = 'paid' AND NOT EXISTS (SELECT 1 FROM warehouse_fulfillments f WHERE f.order_id = orders.id)",
		"payment":             "TRUE",
		"payment-clarify":     "status IN ('pending_payment', 'invoice_approved')",
		"old-orders":          "status IN ('fulfilled', 'cancelled')",
		"old-ops-clarify":     "status IN ('fulfilled', 'cancelled')",
	}
	filter, ok := matrixWhere[kind]
	if !ok {
		return nil, 0, fmt.Errorf("invalid operations matrix kind %q", kind)
	}
	search = "%" + strings.TrimSpace(search) + "%"
	where := `WHERE (` + filter + `)
		AND ($1 = '%%' OR id ILIKE $1 OR customer_name ILIKE $1 OR customer_email ILIKE $1 OR COALESCE(company_name, '') ILIKE $1)`
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders `+where, search).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := operationsOrderSelect + where + operationsOrderOrder + ` LIMIT $2 OFFSET $3`
	orders, err := s.queryOrdersForOperations(ctx, query, search, limit, offset)
	return orders, total, err
}

func (s *Store) OrderItemsForOperationsPage(ctx context.Context, search, status string, limit, offset int) ([]OperationsOrderItem, int, error) {
	search = "%" + strings.TrimSpace(search) + "%"
	const where = `WHERE ($1 = '%%' OR o.id ILIKE $1 OR oi.product_name ILIKE $1
		OR COALESCE(NULLIF(oi.sku, ''), p.sku, '') ILIKE $1
		OR o.customer_name ILIKE $1 OR o.customer_email ILIKE $1)
		AND ($2 = 'all' OR ($2 = 'open' AND o.status <> 'cancelled')
			OR ($2 = 'paid' AND o.status IN ('paid', 'fulfilled')) OR o.status = $2)`
	var total int
	base := `FROM order_items oi JOIN orders o ON o.id=oi.order_id LEFT JOIN products p ON p.id=oi.product_id `
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) `+base+where, search, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT o.id,oi.product_name,COALESCE(NULLIF(oi.sku, ''),p.sku,'—'),
		oi.quantity,oi.unit_price_cents,o.status,o.customer_name,o.customer_email,o.created_at `+
		base+where+` ORDER BY o.created_at DESC,o.id DESC,oi.product_name LIMIT $3 OFFSET $4`, search, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]OperationsOrderItem, 0)
	for rows.Next() {
		var item OperationsOrderItem
		if err := rows.Scan(&item.OrderID, &item.ProductName, &item.SKU, &item.Quantity, &item.UnitPriceCents,
			&item.OrderStatus, &item.CustomerName, &item.CustomerEmail, &item.OrderCreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Store) OrdersForCustomerOperationsPage(ctx context.Context, customerID int64, limit, offset int) ([]Order, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE customer_id=$1`, customerID).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := operationsOrderSelect + `WHERE customer_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`
	orders, err := s.queryOrdersForOperations(ctx, query, customerID, limit, offset)
	return orders, total, err
}

func (s *Store) ordersForOperations(ctx context.Context, condition string, args ...any) ([]Order, error) {
	query := operationsOrderSelect + condition + operationsOrderOrder
	return s.queryOrdersForOperations(ctx, query, args...)
}

const operationsOrderSelect = `SELECT id, status, total_cents, COALESCE(subtotal_cents, total_cents), discount_cents, shipping_cents, tax_cents,
		customer_name, customer_email, COALESCE(promotion_code, ''), COALESCE(stripe_checkout_session_id, ''),
		COALESCE(stripe_payment_intent_id, ''), paid_at, created_at, customer_type, COALESCE(company_name, ''),
		COALESCE(vat_id, ''), COALESCE(buyer_reference, ''), procurement_method, COALESCE(customer_note, '')
		FROM orders `

const operationsOrderOrder = ` ORDER BY CASE status WHEN 'paid' THEN 0 WHEN 'pending_payment' THEN 1 WHEN 'fulfilled' THEN 2 ELSE 3 END, created_at ASC, id ASC`

func (s *Store) queryOrdersForOperations(ctx context.Context, query string, args ...any) ([]Order, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]Order, 0)
	for rows.Next() {
		var order Order
		if err := rows.Scan(&order.ID, &order.Status, &order.TotalCents, &order.SubtotalCents, &order.DiscountCents,
			&order.ShippingCents, &order.TaxCents, &order.CustomerName, &order.CustomerEmail, &order.PromotionCode,
			&order.StripeCheckoutSessionID, &order.StripePaymentIntentID, &order.PaidAt, &order.CreatedAt, &order.CustomerType,
			&order.CompanyName, &order.VATID, &order.BuyerReference, &order.ProcurementMethod, &order.Note); err != nil {
			return nil, err
		}
		if order.Items, err = orderItems(ctx, s.db, order.ID); err != nil {
			return nil, err
		}
		if order.BillingAddress, err = orderAddress(ctx, s.db, order.ID, "billing"); err != nil {
			return nil, err
		}
		if order.ShippingAddress, err = orderAddress(ctx, s.db, order.ID, "shipping"); err != nil {
			return nil, err
		}
		if order.Shipment, err = shipmentForOrder(ctx, s.db, order.ID); err != nil {
			return nil, err
		}
		if order.Invoice, err = invoiceForOrder(ctx, s.db, order.ID); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func shipmentForOrder(ctx context.Context, q queryer, orderID string) (*Shipment, error) {
	shipment := new(Shipment)
	err := q.QueryRowContext(ctx, `SELECT carrier, tracking_number, COALESCE(tracking_url, ''), shipped_at
		FROM shipments WHERE order_id = $1 ORDER BY shipped_at DESC, id DESC LIMIT 1`, orderID).
		Scan(&shipment.Carrier, &shipment.TrackingNumber, &shipment.TrackingURL, &shipment.ShippedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return shipment, nil
}

func invoiceForOrder(ctx context.Context, q queryer, orderID string) (*Invoice, error) {
	invoice := new(Invoice)
	err := q.QueryRowContext(ctx, `SELECT invoice_number, issued_at, due_at, total_cents FROM invoices WHERE order_id = $1`, orderID).Scan(&invoice.Number, &invoice.IssuedAt, &invoice.DueAt, &invoice.TotalCents)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return invoice, nil
}

func orderItems(ctx context.Context, q queryer, orderID string) ([]OrderItem, error) {
	rows, err := q.QueryContext(ctx, `SELECT COALESCE(NULLIF(oi.sku, ''), p.sku, '—'), oi.product_name, oi.unit_price_cents, oi.quantity,
			COALESCE(oi.supplier_name,'')
		FROM order_items oi LEFT JOIN products p ON p.id = oi.product_id
		WHERE oi.order_id = $1 ORDER BY oi.product_name`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]OrderItem, 0)
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(&item.SKU, &item.Name, &item.UnitPriceCents, &item.Quantity, &item.SupplierName); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func orderAddress(ctx context.Context, q queryer, orderID, addressType string) (Address, error) {
	var address Address
	err := q.QueryRowContext(ctx, `SELECT recipient_name, COALESCE(company_name, ''), address_line1, COALESCE(address_line2, ''),
		postal_code, city, country_code FROM order_addresses WHERE order_id = $1 AND address_type = $2`, orderID, addressType).
		Scan(&address.RecipientName, &address.CompanyName, &address.AddressLine1, &address.AddressLine2, &address.PostalCode, &address.City, &address.CountryCode)
	if errors.Is(err, sql.ErrNoRows) {
		return Address{}, nil
	}
	return address, err
}

func (s *Store) OrderByStripeSessionID(ctx context.Context, sessionID string) (Order, error) {
	const query = `SELECT id, status, total_cents, COALESCE(subtotal_cents, total_cents), discount_cents, shipping_cents, tax_cents,
		customer_name, customer_email, COALESCE(promotion_code, ''),
		COALESCE(stripe_checkout_session_id, ''), COALESCE(stripe_payment_intent_id, ''), paid_at
		FROM orders WHERE stripe_checkout_session_id = $1`
	order, err := scanOrder(s.db.QueryRowContext(ctx, query, sessionID))
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	return order, err
}

func (s *Store) RecordPaymentEvent(ctx context.Context, providerEventID, eventType, orderID string, payload []byte) (bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `INSERT INTO payment_events (provider_event_id, event_type, order_id, payload)
		VALUES ($1, $2, NULLIF($3, ''), $4::jsonb)
		ON CONFLICT (provider_event_id) DO NOTHING
		RETURNING id`, providerEventID, eventType, orderID, payload).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// RecordCheckoutPaymentConfirmation keeps an audit record when the customer
// return page verifies a paid Checkout session before its webhook arrives.
func (s *Store) RecordCheckoutPaymentConfirmation(ctx context.Context, orderID, sessionID string) error {
	_, err := s.RecordPaymentEvent(ctx, "checkout_session:"+sessionID, "checkout.session.completed", orderID,
		[]byte(`{"source":"verified_checkout_return","session_id":"`+sessionID+`"}`))
	return err
}

func transitionOrderStatus(ctx context.Context, q executor, orderID, from, to, reason string) error {
	if !ValidOrderStatusTransition(from, to) {
		return ErrInvalidTransition
	}
	var current string
	if err := q.QueryRowContext(ctx, `SELECT status FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&current); err != nil {
		return err
	}
	if current != from {
		return fmt.Errorf("order %s status is %q, expected %q: %w", orderID, current, from, ErrInvalidTransition)
	}
	const update = `UPDATE orders SET status = $2,
		paid_at = CASE WHEN $2 = 'paid' THEN NOW() ELSE paid_at END,
		cancelled_at = CASE WHEN $2 = 'cancelled' THEN NOW() WHEN $2 = 'paid' THEN NULL ELSE cancelled_at END
		WHERE id = $1`
	if _, err := q.ExecContext(ctx, update, orderID, to); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `INSERT INTO order_status_history (order_id, from_status, to_status, reason)
		VALUES ($1, $2, $3, $4)`, orderID, from, to, reason)
	if err != nil {
		return err
	}
	return enqueueOrderEvent(ctx, q, orderID, "order.status_changed", map[string]string{
		"order_id": orderID, "from_status": from, "to_status": to, "reason": reason,
	})
}

func scanOrder(row *sql.Row) (Order, error) {
	var order Order
	err := row.Scan(&order.ID, &order.Status, &order.TotalCents, &order.SubtotalCents, &order.DiscountCents, &order.ShippingCents, &order.TaxCents,
		&order.CustomerName, &order.CustomerEmail, &order.PromotionCode,
		&order.StripeCheckoutSessionID, &order.StripePaymentIntentID, &order.PaidAt)
	return order, err
}
