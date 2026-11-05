package store

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
)

type InternalOrderNote struct {
	ID          int64
	AuthorEmail string
	Body        string
	CreatedAt   time.Time
}

type EmailDelivery struct {
	ID          int64
	Recipient   string
	Subject     string
	SentAt      *time.Time
	AvailableAt time.Time
	Attempts    int
	LastError   string
}

type EmailDeliveryQueueRecord struct {
	ID           int64
	OrderID      string
	CampaignName string
	Recipient    string
	Subject      string
	SentAt       *time.Time
	AvailableAt  time.Time
	ClaimedUntil *time.Time
	Attempts     int
	LastError    string
	Status       string
}

type OrderStatusChange struct {
	FromStatus string
	ToStatus   string
	Reason     string
	CreatedAt  time.Time
}

type PaymentEventSummary struct {
	ProviderEventID string
	EventType       string
	ProcessedAt     time.Time
}
type InvoiceDocument struct {
	Type, Number, Reason, CreatedBy string
	IssuedAt                        time.Time
	TotalCents                      int
}

type OrderItemOperation struct {
	ProductName, Status, OriginalDeliveryDate, DelayedDeliveryDate string
	UpdatedBy                                                      string
	UpdatedAt                                                      time.Time
	OrderedQuantity, ShippedQuantity                               int
}
type OrderItemOperationHistory struct {
	ProductName, Action, Detail, ActorEmail string
	CreatedAt                               time.Time
}

type OrderShipmentItem struct {
	ProductName string
	Quantity    int
}

type OrderShipment struct {
	ID                                           int64
	Carrier, TrackingNumber, TrackingURL, Source string
	SupplierName, CreatedBy                      string
	ShippedAt                                    time.Time
	Items                                        []OrderShipmentItem
}

type OrderItemSupplier struct {
	ProductName, SupplierName string
}

type OrderItemRequest struct {
	OrderID, ProductName, SKU, Status, Reason, ActorEmail string
	RequestType, ReturnReference                          string
	Quantity                                              int
	CustomerName, CustomerEmail                           string
	OrderCreatedAt, RequestedAt                           time.Time
}

type ShipmentDelayRecord struct {
	OrderID, ProductName, SKU, OrderStatus, CustomerName, CustomerEmail, UpdatedBy string
	Quantity                                                                       int
	OriginalDeliveryDate, DelayedDeliveryDate                                      string
	OrderCreatedAt, UpdatedAt                                                      time.Time
}

type OrderManagementDetail struct {
	Order                Order
	CustomerAccount      *OrderCustomerAccount
	InternalNotes        []InternalOrderNote
	Emails               []EmailDelivery
	StatusHistory        []OrderStatusChange
	PaymentEvents        []PaymentEventSummary
	InvoiceDocuments     []InvoiceDocument
	ItemOperations       []OrderItemOperation
	ItemOperationHistory []OrderItemOperationHistory
	ItemSuppliers        []OrderItemSupplier
	Shipments            []OrderShipment
}

type OrderCustomerAccount struct {
	Email, FullName, Phone, CustomerType, CompanyName, VATID, BuyerReference string
	FraudFlagged                                                             bool
	FraudReason, FraudFlaggedBy                                              string
	FraudFlaggedAt                                                           *time.Time
	Address                                                                  Address
}

type CustomerOperationsRecord struct {
	ID             int64
	Email          string
	Profile        CustomerProfile
	CreatedAt      time.Time
	OrderCount     int
	LastOrderAt    *time.Time
	FraudFlagged   bool
	FraudReason    string
	FraudFlaggedAt *time.Time
	FraudFlaggedBy string
}

type CustomerFraudStatus struct {
	Flagged   bool
	Reason    string
	FlaggedAt *time.Time
	FlaggedBy string
}

func (s *Store) CustomersForOperationsPage(ctx context.Context, search string, limit, offset int) ([]CustomerOperationsRecord, int, error) {
	search = "%" + strings.TrimSpace(search) + "%"
	const where = `WHERE ($1 = '%%' OR u.email ILIKE $1 OR u.full_name ILIKE $1 OR u.company_name ILIKE $1 OR u.phone ILIKE $1)`
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u `+where, search).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id,u.email,u.full_name,u.phone,u.customer_type,u.company_name,u.vat_id,u.buyer_reference,
			u.address_line1,u.address_line2,u.postal_code,u.city,u.country_code,u.created_at,
			COUNT(o.id),MAX(o.created_at),u.fraud_flagged,COALESCE(u.fraud_reason,''),u.fraud_flagged_at,
			COALESCE(u.fraud_flagged_by,'')
		FROM users u
		LEFT JOIN orders o ON o.customer_id=u.id
		`+where+`
		GROUP BY u.id
		ORDER BY u.created_at DESC,u.id DESC
		LIMIT $2 OFFSET $3`, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	customers := make([]CustomerOperationsRecord, 0)
	for rows.Next() {
		var customer CustomerOperationsRecord
		if err := rows.Scan(&customer.ID, &customer.Email, &customer.Profile.FullName, &customer.Profile.Phone,
			&customer.Profile.CustomerType, &customer.Profile.CompanyName, &customer.Profile.VATID,
			&customer.Profile.BuyerReference, &customer.Profile.Address.AddressLine1, &customer.Profile.Address.AddressLine2,
			&customer.Profile.Address.PostalCode, &customer.Profile.Address.City, &customer.Profile.Address.CountryCode,
			&customer.CreatedAt, &customer.OrderCount, &customer.LastOrderAt, &customer.FraudFlagged, &customer.FraudReason,
			&customer.FraudFlaggedAt, &customer.FraudFlaggedBy); err != nil {
			return nil, 0, err
		}
		customers = append(customers, customer)
	}
	return customers, total, rows.Err()
}

func (s *Store) SetCustomerFraudFlag(ctx context.Context, customerID int64, flagged bool, reason, actor string) (CustomerFraudStatus, error) {
	reason = strings.TrimSpace(reason)
	actor = strings.TrimSpace(actor)
	if customerID < 1 || len([]rune(reason)) < 5 || len([]rune(reason)) > 500 || actor == "" {
		return CustomerFraudStatus{}, ErrInvalidCheckout
	}

	var status CustomerFraudStatus
	err := s.withTransaction(ctx, func(tx *sql.Tx) error {
		var current bool
		var currentReason, currentActor string
		var currentAt *time.Time
		if err := tx.QueryRowContext(ctx, `SELECT fraud_flagged,COALESCE(fraud_reason,''),fraud_flagged_at,
				COALESCE(fraud_flagged_by,'') FROM users WHERE id=$1 FOR UPDATE`, customerID).
			Scan(&current, &currentReason, &currentAt, &currentActor); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if current != flagged {
			if flagged {
				_, err := tx.ExecContext(ctx, `UPDATE users
					SET fraud_flagged=TRUE,fraud_reason=$2,fraud_flagged_at=NOW(),fraud_flagged_by=$3
					WHERE id=$1`, customerID, reason, actor)
				if err != nil {
					return err
				}
			} else {
				_, err := tx.ExecContext(ctx, `UPDATE users
					SET fraud_flagged=FALSE,fraud_reason=NULL,fraud_flagged_at=NULL,fraud_flagged_by=NULL
					WHERE id=$1`, customerID)
				if err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO customer_fraud_audit(customer_id,flagged,reason,actor_email)
				VALUES($1,$2,$3,$4)`, customerID, flagged, reason, actor); err != nil {
				return err
			}
		}
		status.Flagged = flagged
		if flagged {
			status.Reason, status.FlaggedAt, status.FlaggedBy = currentReason, currentAt, currentActor
			if current != flagged {
				status.Reason, status.FlaggedBy = reason, actor
				if err := tx.QueryRowContext(ctx, `SELECT fraud_flagged_at FROM users WHERE id=$1`, customerID).Scan(&status.FlaggedAt); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return status, err
}

func ensureCustomerNotFraudBlockedTx(ctx context.Context, tx *sql.Tx, customerID int64, email string) error {
	var flagged bool
	var err error
	if customerID > 0 {
		err = tx.QueryRowContext(ctx, `SELECT fraud_flagged FROM users WHERE id=$1 FOR UPDATE`, customerID).Scan(&flagged)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT fraud_flagged FROM users WHERE LOWER(email)=LOWER($1) FOR UPDATE`, strings.TrimSpace(email)).Scan(&flagged)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if flagged {
		return ErrCustomerFraudBlocked
	}
	return nil
}

func ensureOrderNotFraudBlockedTx(ctx context.Context, tx *sql.Tx, orderID string) error {
	var customerID int64
	var email string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(customer_id,0),customer_email
		FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&customerID, &email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return ensureCustomerNotFraudBlockedTx(ctx, tx, customerID, email)
}

func (s *Store) OrderManagementDetail(ctx context.Context, orderID string) (OrderManagementDetail, error) {
	orders, err := s.ordersForOperations(ctx, "WHERE id = $1", orderID)
	if err != nil {
		return OrderManagementDetail{}, err
	}
	if len(orders) == 0 {
		return OrderManagementDetail{}, ErrNotFound
	}
	detail := OrderManagementDetail{Order: orders[0]}
	detail.CustomerAccount, err = customerAccountForOrder(ctx, s.db, orderID)
	if err != nil {
		return OrderManagementDetail{}, err
	}
	if detail.InternalNotes, err = internalNotesForOrder(ctx, s.db, orderID); err != nil {
		return OrderManagementDetail{}, err
	}
	if detail.Emails, err = emailsForOrder(ctx, s.db, orderID); err != nil {
		return OrderManagementDetail{}, err
	}
	if detail.StatusHistory, err = statusHistoryForOrder(ctx, s.db, orderID); err != nil {
		return OrderManagementDetail{}, err
	}
	if detail.PaymentEvents, err = paymentEventsForOrder(ctx, s.db, orderID); err != nil {
		return OrderManagementDetail{}, err
	}
	if detail.InvoiceDocuments, err = invoiceDocumentsForOrder(ctx, s.db, orderID); err != nil {
		return OrderManagementDetail{}, err
	}
	if detail.ItemOperations, err = itemOperationsForOrder(ctx, s.db, orderID); err != nil {
		return OrderManagementDetail{}, err
	}
	if detail.ItemSuppliers, err = itemSuppliersForOrder(ctx, s.db, orderID); err != nil {
		return OrderManagementDetail{}, err
	}
	if detail.ItemOperationHistory, err = itemOperationHistoryForOrder(ctx, s.db, orderID); err != nil {
		return OrderManagementDetail{}, err
	}
	if detail.Shipments, err = shipmentHistoryForOrder(ctx, s.db, orderID); err != nil {
		return OrderManagementDetail{}, err
	}
	return detail, nil
}

func itemSuppliersForOrder(ctx context.Context, q queryer, orderID string) ([]OrderItemSupplier, error) {
	rows, err := q.QueryContext(ctx, `SELECT product_name, supplier_name
		FROM order_items WHERE order_id=$1 AND BTRIM(COALESCE(supplier_name,'')) <> ''
		ORDER BY product_name`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	suppliers := make([]OrderItemSupplier, 0)
	for rows.Next() {
		var supplier OrderItemSupplier
		if err := rows.Scan(&supplier.ProductName, &supplier.SupplierName); err != nil {
			return nil, err
		}
		suppliers = append(suppliers, supplier)
	}
	return suppliers, rows.Err()
}

func customerAccountForOrder(ctx context.Context, q queryer, orderID string) (*OrderCustomerAccount, error) {
	account := new(OrderCustomerAccount)
	err := q.QueryRowContext(ctx, `SELECT u.email,u.full_name,u.phone,u.customer_type,u.company_name,u.vat_id,u.buyer_reference,
		u.fraud_flagged,COALESCE(u.fraud_reason,''),u.fraud_flagged_at,COALESCE(u.fraud_flagged_by,''),
		u.address_line1,u.address_line2,u.postal_code,u.city,u.country_code
		FROM orders o JOIN users u ON u.id=o.customer_id WHERE o.id=$1`, orderID).Scan(
		&account.Email, &account.FullName, &account.Phone, &account.CustomerType, &account.CompanyName, &account.VATID,
		&account.BuyerReference, &account.FraudFlagged, &account.FraudReason, &account.FraudFlaggedAt, &account.FraudFlaggedBy,
		&account.Address.AddressLine1, &account.Address.AddressLine2,
		&account.Address.PostalCode, &account.Address.City, &account.Address.CountryCode)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return account, nil
}

func itemOperationsForOrder(ctx context.Context, q executor, orderID string) ([]OrderItemOperation, error) {
	if _, err := q.ExecContext(ctx, `INSERT INTO order_item_operations (order_id, product_name, original_delivery_date)
		SELECT oi.order_id, oi.product_name, (o.created_at::date + INTERVAL '3 days')::date FROM order_items oi JOIN orders o ON o.id=oi.order_id WHERE oi.order_id=$1 ON CONFLICT DO NOTHING`, orderID); err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, `SELECT op.product_name,op.status,COALESCE(op.original_delivery_date::text,''),
		COALESCE(op.delayed_delivery_date::text,''),op.updated_by,op.updated_at,oi.quantity,
		COALESCE((SELECT SUM(si.quantity) FROM shipment_items si
			WHERE si.order_id=oi.order_id AND si.product_name=oi.product_name),0)
		FROM order_item_operations op
		JOIN order_items oi ON oi.order_id=op.order_id AND oi.product_name=op.product_name
		WHERE op.order_id=$1 ORDER BY op.product_name`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OrderItemOperation{}
	for rows.Next() {
		var x OrderItemOperation
		if err := rows.Scan(&x.ProductName, &x.Status, &x.OriginalDeliveryDate, &x.DelayedDeliveryDate,
			&x.UpdatedBy, &x.UpdatedAt, &x.OrderedQuantity, &x.ShippedQuantity); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func shipmentHistoryForOrder(ctx context.Context, q queryer, orderID string) ([]OrderShipment, error) {
	rows, err := q.QueryContext(ctx, `SELECT s.id,s.carrier,s.tracking_number,COALESCE(s.tracking_url,''),
		s.source,s.supplier_name,s.created_by,s.shipped_at,si.product_name,si.quantity
		FROM shipments s
		LEFT JOIN shipment_items si ON si.shipment_id=s.id
		WHERE s.order_id=$1
		ORDER BY s.shipped_at DESC,s.id DESC,si.product_name`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	shipments := make([]OrderShipment, 0)
	indices := make(map[int64]int)
	for rows.Next() {
		var shipment OrderShipment
		var productName sql.NullString
		var quantity sql.NullInt64
		if err := rows.Scan(&shipment.ID, &shipment.Carrier, &shipment.TrackingNumber, &shipment.TrackingURL,
			&shipment.Source, &shipment.SupplierName, &shipment.CreatedBy, &shipment.ShippedAt,
			&productName, &quantity); err != nil {
			return nil, err
		}
		index, exists := indices[shipment.ID]
		if !exists {
			shipment.Items = make([]OrderShipmentItem, 0)
			index = len(shipments)
			indices[shipment.ID] = index
			shipments = append(shipments, shipment)
		}
		if productName.Valid && quantity.Valid {
			shipments[index].Items = append(shipments[index].Items, OrderShipmentItem{
				ProductName: productName.String,
				Quantity:    int(quantity.Int64),
			})
		}
	}
	return shipments, rows.Err()
}

func itemOperationHistoryForOrder(ctx context.Context, q queryer, orderID string) ([]OrderItemOperationHistory, error) {
	rows, err := q.QueryContext(ctx, `SELECT product_name,action,detail,actor_email,created_at FROM order_item_operation_history WHERE order_id=$1 ORDER BY id DESC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OrderItemOperationHistory{}
	for rows.Next() {
		var x OrderItemOperationHistory
		if err := rows.Scan(&x.ProductName, &x.Action, &x.Detail, &x.ActorEmail, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) OrderItemRequestsForOperationsPage(ctx context.Context, search, status string, limit, offset int) ([]OrderItemRequest, int, error) {
	search = "%" + strings.TrimSpace(search) + "%"
	const from = `FROM order_item_operations op
		JOIN orders o ON o.id=op.order_id
		JOIN order_items oi ON oi.order_id=op.order_id AND oi.product_name=op.product_name
		LEFT JOIN products p ON p.id=oi.product_id
		LEFT JOIN LATERAL (
			SELECT detail,actor_email,created_at
			FROM order_item_operation_history h
			WHERE h.order_id=op.order_id AND h.product_name=op.product_name AND h.action=op.status
			ORDER BY h.id DESC LIMIT 1
		) latest ON TRUE
		LEFT JOIN LATERAL (
			SELECT request_type,'RET-' || LPAD(id::text,8,'0') AS reference
			FROM warehouse_return_cases r
			WHERE r.order_id=op.order_id AND r.product_name=op.product_name
			ORDER BY r.id DESC LIMIT 1
		) ret ON TRUE`
	const where = `WHERE op.status IN ('return_open','replacement_requested')
		AND ($2 = 'all' OR op.status = $2)
		AND ($1 = '%%' OR op.order_id ILIKE $1 OR op.product_name ILIKE $1
			OR COALESCE(NULLIF(oi.sku,''),p.sku,'') ILIKE $1
			OR o.customer_name ILIKE $1 OR o.customer_email ILIKE $1)`
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) `+from+where, search, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT op.order_id,op.product_name,COALESCE(NULLIF(oi.sku,''),p.sku,'—'),
		op.status,COALESCE(latest.detail,''),COALESCE(latest.actor_email,''),oi.quantity,
		COALESCE(ret.request_type,''),COALESCE(ret.reference,''),
		o.customer_name,o.customer_email,o.created_at,COALESCE(latest.created_at,op.updated_at) `+
		from+where+` ORDER BY COALESCE(latest.created_at,op.updated_at) DESC,op.order_id DESC,op.product_name
		LIMIT $3 OFFSET $4`, search, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	requests := make([]OrderItemRequest, 0)
	for rows.Next() {
		var request OrderItemRequest
		if err := rows.Scan(&request.OrderID, &request.ProductName, &request.SKU, &request.Status,
			&request.Reason, &request.ActorEmail, &request.Quantity, &request.RequestType, &request.ReturnReference, &request.CustomerName,
			&request.CustomerEmail, &request.OrderCreatedAt, &request.RequestedAt); err != nil {
			return nil, 0, err
		}
		requests = append(requests, request)
	}
	return requests, total, rows.Err()
}

func (s *Store) ShipmentDelaysForOperationsPage(ctx context.Context, search string, limit, offset int) ([]ShipmentDelayRecord, int, error) {
	search = "%" + strings.TrimSpace(search) + "%"
	const from = `FROM order_item_operations op
		JOIN orders o ON o.id=op.order_id
		JOIN order_items oi ON oi.order_id=op.order_id AND oi.product_name=op.product_name
		LEFT JOIN products p ON p.id=oi.product_id`
	const where = `WHERE op.delayed_delivery_date IS NOT NULL
		AND ($1 = '%%' OR op.order_id ILIKE $1 OR op.product_name ILIKE $1
			OR COALESCE(NULLIF(oi.sku,''),p.sku,'') ILIKE $1
			OR o.customer_name ILIKE $1 OR o.customer_email ILIKE $1)`
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) `+from+where, search).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT op.order_id,op.product_name,COALESCE(NULLIF(oi.sku,''),p.sku,'—'),
		op.status,oi.quantity,COALESCE(op.original_delivery_date::text,''),op.delayed_delivery_date::text,
		o.customer_name,o.customer_email,o.created_at,op.updated_by,op.updated_at `+
		from+where+` ORDER BY op.delayed_delivery_date DESC,op.updated_at DESC,op.order_id DESC,op.product_name
		LIMIT $2 OFFSET $3`, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	delays := make([]ShipmentDelayRecord, 0)
	for rows.Next() {
		var delay ShipmentDelayRecord
		if err := rows.Scan(&delay.OrderID, &delay.ProductName, &delay.SKU, &delay.OrderStatus,
			&delay.Quantity, &delay.OriginalDeliveryDate, &delay.DelayedDeliveryDate, &delay.CustomerName,
			&delay.CustomerEmail, &delay.OrderCreatedAt, &delay.UpdatedBy, &delay.UpdatedAt); err != nil {
			return nil, 0, err
		}
		delays = append(delays, delay)
	}
	return delays, total, rows.Err()
}

func (s *Store) UpdateOrderItemOperation(ctx context.Context, orderID, productName, action, detail, actor string) error {
	valid := map[string]bool{"ready_to_process": true, "stock_confirmed": true, "payment_received": true, "exported": true, "fulfillment": true, "fulfillment_completed": true, "delay": true}
	if !valid[action] || strings.TrimSpace(productName) == "" || strings.TrimSpace(actor) == "" {
		return ErrInvalidCheckout
	}
	if action == "delay" {
		if _, err := time.Parse("2006-01-02", detail); err != nil {
			return ErrInvalidCheckout
		}
	}
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		if err := ensureOrderNotFraudBlockedTx(ctx, tx, orderID); err != nil {
			return err
		}
		if _, err := itemOperationsForOrder(ctx, tx, orderID); err != nil {
			return err
		}
		if action == "delay" {
			result, err := tx.ExecContext(ctx, `UPDATE order_item_operations SET delayed_delivery_date=$3::date,updated_by=$4,updated_at=NOW() WHERE order_id=$1 AND product_name=$2`, orderID, productName, detail, actor)
			if err != nil {
				return err
			}
			changed, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if changed != 1 {
				return ErrNotFound
			}
		} else {
			result, err := tx.ExecContext(ctx, `UPDATE order_item_operations SET status=$3,updated_by=$4,updated_at=NOW() WHERE order_id=$1 AND product_name=$2`, orderID, productName, action, actor)
			if err != nil {
				return err
			}
			changed, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if changed != 1 {
				return ErrNotFound
			}
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO order_item_operation_history(order_id,product_name,action,detail,actor_email) VALUES($1,$2,$3,$4,$5)`, orderID, productName, action, detail, actor)
		return err
	})
}

func (s *Store) ApplyOrderItemAction(ctx context.Context, orderID string, productNames []string, action, reason, actor string) error {
	statusByAction := map[string]string{
		"return":                  "return_open",
		"replacement":             "replacement_requested",
		"replacement_with_return": "replacement_requested",
	}
	status, ok := statusByAction[action]
	if !ok || len(productNames) == 0 || len(productNames) > 100 || strings.TrimSpace(reason) == "" ||
		len([]rune(reason)) > 500 || strings.TrimSpace(actor) == "" {
		return ErrInvalidCheckout
	}
	seen := make(map[string]struct{}, len(productNames))
	for _, name := range productNames {
		name = strings.TrimSpace(name)
		if name == "" {
			return ErrInvalidCheckout
		}
		if _, exists := seen[name]; exists {
			return ErrInvalidCheckout
		}
		seen[name] = struct{}{}
	}
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var orderStatus string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&orderStatus); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if orderStatus != "fulfilled" {
			return ErrInvalidTransition
		}
		if err := ensureOrderNotFraudBlockedTx(ctx, tx, orderID); err != nil {
			return err
		}
		if _, err := itemOperationsForOrder(ctx, tx, orderID); err != nil {
			return err
		}
		for name := range seen {
			if action == "return" || action == "replacement_with_return" {
				var existing bool
				if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
					SELECT 1 FROM warehouse_return_cases
					WHERE order_id=$1 AND product_name=$2
				)`, orderID, name).Scan(&existing); err != nil {
					return err
				}
				if existing {
					return ErrReturnAlreadyOpen
				}
				var productID sql.NullInt64
				var sku string
				if err := tx.QueryRowContext(ctx, `SELECT product_id,COALESCE(sku,'') FROM order_items
					WHERE order_id=$1 AND product_name=$2`, orderID, name).Scan(&productID, &sku); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return ErrNotFound
					}
					return err
				}
				if !productID.Valid {
					return ErrInvalidReturnProduct
				}
				requestType := action
				if _, err := tx.ExecContext(ctx, `INSERT INTO warehouse_return_cases
					(order_id,product_id,product_name,sku,quantity,request_type,reason,created_by)
					SELECT $1,$2,$3,$4,quantity,$5,$6,$7 FROM order_items
					WHERE order_id=$1 AND product_name=$3`, orderID, productID.Int64, name, sku, requestType, strings.TrimSpace(reason), actor); err != nil {
					return err
				}
			}
			result, err := tx.ExecContext(ctx, `UPDATE order_item_operations
				SET status=$3,updated_by=$4,updated_at=NOW() WHERE order_id=$1 AND product_name=$2`,
				orderID, name, status, actor)
			if err != nil {
				return err
			}
			changed, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if changed != 1 {
				return ErrNotFound
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO order_item_operation_history
				(order_id,product_name,action,detail,actor_email) VALUES($1,$2,$3,$4,$5)`,
				orderID, name, status, strings.TrimSpace(reason), actor); err != nil {
				return err
			}
		}
		names := make([]string, 0, len(seen))
		for name := range seen {
			names = append(names, name)
		}
		sort.Strings(names)
		subject, body := "Deine Rückgabe wird bearbeitet", "Wir haben deine Rückgabeanfrage für folgende Artikel erhalten:\n- "+strings.Join(names, "\n- ")+"\n\nGrund: "+strings.TrimSpace(reason)
		if action == "replacement" || action == "replacement_with_return" {
			subject = "Deine Ersatzanfrage wird bearbeitet"
			body = "Wir haben deine Ersatzanfrage für folgende Artikel erhalten:\n- " + strings.Join(names, "\n- ") + "\n\nGrund: " + strings.TrimSpace(reason)
		}
		return enqueueOrderEmail(ctx, tx, orderID, subject, body)
	})
}

func (s *Store) QueueOrderSummaryEmail(ctx context.Context, orderID, body string) (string, error) {
	if strings.TrimSpace(body) == "" || len([]rune(body)) > 12000 {
		return "", ErrInvalidCheckout
	}
	var recipient string
	err := s.db.QueryRowContext(ctx, `INSERT INTO email_outbox_events(order_id,recipient,subject,body)
		SELECT o.id,COALESCE(u.email,o.customer_email),'Deine Bestellübersicht · ' || o.id,$2
		FROM orders o LEFT JOIN users u ON u.id=o.customer_id WHERE o.id=$1
		RETURNING recipient`, orderID, body).Scan(&recipient)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return recipient, err
}

func (s *Store) CreateInvoiceDocument(ctx context.Context, orderID, documentType, reason, createdBy string) (InvoiceDocument, error) {
	if documentType != "credit_note" && documentType != "cancellation" {
		return InvoiceDocument{}, ErrInvalidCheckout
	}
	if strings.TrimSpace(reason) == "" || len([]rune(reason)) > 500 {
		return InvoiceDocument{}, ErrInvalidCheckout
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT total_cents FROM orders WHERE id=$1 AND status IN ('paid','fulfilled','cancelled')`, orderID).Scan(&total); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return InvoiceDocument{}, ErrNotFound
		}
		return InvoiceDocument{}, err
	}
	prefix := "GS"
	if documentType == "cancellation" {
		prefix = "ST"
	}
	number := prefix + "-" + time.Now().UTC().Format("20060102-150405") + "-" + orderID
	var doc InvoiceDocument
	err := s.db.QueryRowContext(ctx, `INSERT INTO invoice_documents (order_id,document_type,document_number,total_cents,reason,created_by) VALUES ($1,$2,$3,$4,$5,$6) RETURNING document_type,document_number,reason,created_by,issued_at,total_cents`, orderID, documentType, number, -total, strings.TrimSpace(reason), createdBy).Scan(&doc.Type, &doc.Number, &doc.Reason, &doc.CreatedBy, &doc.IssuedAt, &doc.TotalCents)
	return doc, err
}
func invoiceDocumentsForOrder(ctx context.Context, q queryer, orderID string) ([]InvoiceDocument, error) {
	rows, err := q.QueryContext(ctx, `SELECT document_type,document_number,reason,created_by,issued_at,total_cents FROM invoice_documents WHERE order_id=$1 ORDER BY id`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	docs := []InvoiceDocument{}
	for rows.Next() {
		var d InvoiceDocument
		if err := rows.Scan(&d.Type, &d.Number, &d.Reason, &d.CreatedBy, &d.IssuedAt, &d.TotalCents); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

func (s *Store) AddInternalOrderNote(ctx context.Context, orderID, authorEmail, body string) (InternalOrderNote, error) {
	body = strings.TrimSpace(body)
	if body == "" || len([]rune(body)) > 2000 {
		return InternalOrderNote{}, ErrInvalidCheckout
	}
	note := InternalOrderNote{}
	err := s.db.QueryRowContext(ctx, `INSERT INTO order_internal_notes (order_id, author_email, body)
		SELECT id, $2, $3 FROM orders WHERE id = $1 RETURNING id, author_email, body, created_at`, orderID, authorEmail, body).
		Scan(&note.ID, &note.AuthorEmail, &note.Body, &note.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return InternalOrderNote{}, ErrNotFound
	}
	return note, err
}

func internalNotesForOrder(ctx context.Context, q queryer, orderID string) ([]InternalOrderNote, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, author_email, body, created_at FROM order_internal_notes WHERE order_id = $1 ORDER BY created_at DESC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	notes := make([]InternalOrderNote, 0)
	for rows.Next() {
		var note InternalOrderNote
		if err := rows.Scan(&note.ID, &note.AuthorEmail, &note.Body, &note.CreatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
}

func emailsForOrder(ctx context.Context, q queryer, orderID string) ([]EmailDelivery, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, recipient, subject, sent_at, available_at, attempts, COALESCE(last_error, '') FROM email_outbox_events WHERE order_id = $1 ORDER BY id DESC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	emails := make([]EmailDelivery, 0)
	for rows.Next() {
		var email EmailDelivery
		if err := rows.Scan(&email.ID, &email.Recipient, &email.Subject, &email.SentAt, &email.AvailableAt, &email.Attempts, &email.LastError); err != nil {
			return nil, err
		}
		emails = append(emails, email)
	}
	return emails, rows.Err()
}

func (s *Store) EmailDeliveriesForOperationsPage(ctx context.Context, search, status string, limit, offset int) ([]EmailDeliveryQueueRecord, int, error) {
	search = "%" + strings.TrimSpace(search) + "%"
	const deliveries = `WITH deliveries AS (
		SELECT delivery.id, delivery.order_id, campaign.name AS campaign_name, delivery.recipient,
			delivery.subject, delivery.sent_at, delivery.available_at, delivery.claimed_until, delivery.attempts,
			COALESCE(delivery.last_error, '') AS last_error,
			CASE WHEN delivery.cancelled_at IS NOT NULL THEN 'cancelled'
				WHEN delivery.sent_at IS NOT NULL THEN 'sent'
				WHEN delivery.claimed_until > NOW() THEN 'sending'
				WHEN delivery.attempts > 0 THEN 'retrying'
				ELSE 'pending' END AS status
		FROM email_outbox_events delivery
		LEFT JOIN marketing_email_campaigns campaign ON campaign.id = delivery.campaign_id
	)`
	const where = `WHERE ($1 = '%%' OR COALESCE(order_id, '') ILIKE $1 OR recipient ILIKE $1
		OR subject ILIKE $1 OR COALESCE(campaign_name, '') ILIKE $1)
		AND ($2 = 'all' OR status = $2)`
	var total int
	if err := s.db.QueryRowContext(ctx, deliveries+` SELECT COUNT(*) FROM deliveries `+where, search, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, deliveries+`
		SELECT id, COALESCE(order_id, ''), COALESCE(campaign_name, ''), recipient, subject,
			sent_at, available_at, claimed_until, attempts, last_error, status
		FROM deliveries `+where+`
		ORDER BY id DESC LIMIT $3 OFFSET $4`, search, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	deliveriesPage := make([]EmailDeliveryQueueRecord, 0)
	for rows.Next() {
		var delivery EmailDeliveryQueueRecord
		if err := rows.Scan(&delivery.ID, &delivery.OrderID, &delivery.CampaignName, &delivery.Recipient, &delivery.Subject,
			&delivery.SentAt, &delivery.AvailableAt, &delivery.ClaimedUntil, &delivery.Attempts,
			&delivery.LastError, &delivery.Status); err != nil {
			return nil, 0, err
		}
		deliveriesPage = append(deliveriesPage, delivery)
	}
	return deliveriesPage, total, rows.Err()
}

func statusHistoryForOrder(ctx context.Context, q queryer, orderID string) ([]OrderStatusChange, error) {
	rows, err := q.QueryContext(ctx, `SELECT COALESCE(from_status, ''), to_status, reason, created_at FROM order_status_history WHERE order_id = $1 ORDER BY id DESC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	changes := make([]OrderStatusChange, 0)
	for rows.Next() {
		var change OrderStatusChange
		if err := rows.Scan(&change.FromStatus, &change.ToStatus, &change.Reason, &change.CreatedAt); err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, rows.Err()
}

func paymentEventsForOrder(ctx context.Context, q queryer, orderID string) ([]PaymentEventSummary, error) {
	rows, err := q.QueryContext(ctx, `SELECT provider_event_id, event_type, processed_at FROM payment_events WHERE order_id = $1 ORDER BY id DESC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]PaymentEventSummary, 0)
	for rows.Next() {
		var event PaymentEventSummary
		if err := rows.Scan(&event.ProviderEventID, &event.EventType, &event.ProcessedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
