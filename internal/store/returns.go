package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type WarehouseReturn struct {
	ID               int64      `json:"id"`
	Reference        string     `json:"reference"`
	OrderID          string     `json:"orderId"`
	ProductID        int64      `json:"productId"`
	ProductName      string     `json:"productName"`
	SKU              string     `json:"sku"`
	Quantity         int        `json:"quantity"`
	ReceivedQuantity int        `json:"receivedQuantity"`
	RequestType      string     `json:"requestType"`
	Reason           string     `json:"reason"`
	Status           string     `json:"status"`
	ParcelReference  string     `json:"parcelReference"`
	CustomerName     string     `json:"customerName"`
	CustomerEmail    string     `json:"customerEmail"`
	CreatedBy        string     `json:"createdBy"`
	CreatedAt        time.Time  `json:"createdAt"`
	ReceivedBy       string     `json:"receivedBy"`
	ReceivedAt       *time.Time `json:"receivedAt"`
	InspectedBy      string     `json:"inspectedBy"`
	InspectedAt      *time.Time `json:"inspectedAt"`
}

func (s *Store) WarehouseReturns(ctx context.Context, status, search string, limit, offset int) ([]WarehouseReturn, int, error) {
	search = "%" + strings.TrimSpace(search) + "%"
	where := `WHERE ($1 = 'all' OR r.status=$1)
		AND ($2 = '%%' OR r.order_id ILIKE $2 OR r.product_name ILIKE $2 OR r.sku ILIKE $2
			OR o.customer_name ILIKE $2 OR o.customer_email ILIKE $2 OR ('RET-' || LPAD(r.id::text,8,'0')) ILIKE $2)`
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM warehouse_return_cases r JOIN orders o ON o.id=r.order_id `+where, status, search).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT r.id,'RET-' || LPAD(r.id::text,8,'0'),r.order_id,r.product_id,
		r.product_name,r.sku,r.quantity,r.received_quantity,r.request_type,r.reason,r.status,r.parcel_reference,
		COALESCE(o.customer_name,''),COALESCE(o.customer_email,''),r.created_by,r.created_at,
		r.received_by,r.received_at,r.inspected_by,r.inspected_at
		FROM warehouse_return_cases r JOIN orders o ON o.id=r.order_id `+where+`
		ORDER BY CASE WHEN r.status='expected' THEN 0 WHEN r.status='received' THEN 1 ELSE 2 END,
		r.created_at DESC,r.id DESC LIMIT $3 OFFSET $4`, status, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	cases := make([]WarehouseReturn, 0)
	for rows.Next() {
		var item WarehouseReturn
		if err := rows.Scan(&item.ID, &item.Reference, &item.OrderID, &item.ProductID, &item.ProductName,
			&item.SKU, &item.Quantity, &item.ReceivedQuantity, &item.RequestType, &item.Reason, &item.Status, &item.ParcelReference,
			&item.CustomerName, &item.CustomerEmail, &item.CreatedBy, &item.CreatedAt,
			&item.ReceivedBy, &item.ReceivedAt, &item.InspectedBy, &item.InspectedAt); err != nil {
			return nil, 0, err
		}
		cases = append(cases, item)
	}
	return cases, total, rows.Err()
}

func (s *Store) ReceiveWarehouseReturn(ctx context.Context, id int64, quantity int, parcelReference, actor string) error {
	if id <= 0 || quantity <= 0 || strings.TrimSpace(actor) == "" || len([]rune(parcelReference)) > 120 {
		return ErrInvalidCheckout
	}
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var requested int
		var status string
		err := tx.QueryRowContext(ctx, `SELECT quantity,status FROM warehouse_return_cases WHERE id=$1 FOR UPDATE`, id).Scan(&requested, &status)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != "expected" {
			return ErrInvalidTransition
		}
		if quantity > requested {
			return ErrInvalidQuantity
		}
		_, err = tx.ExecContext(ctx, `UPDATE warehouse_return_cases
			SET status='received',received_quantity=$2,parcel_reference=$3,received_by=$4,received_at=NOW(),updated_at=NOW()
			WHERE id=$1`, id, quantity, strings.TrimSpace(parcelReference), actor)
		return err
	})
}

func (s *Store) InspectWarehouseReturn(ctx context.Context, id int64, disposition, actor string) error {
	if id <= 0 || strings.TrimSpace(actor) == "" ||
		(disposition != "restock" && disposition != "damaged" && disposition != "not_restockable") {
		return ErrInvalidCheckout
	}
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var productID int64
		var quantity int
		var orderID, productName string
		err := tx.QueryRowContext(ctx, `SELECT product_id,received_quantity,order_id,product_name
			FROM warehouse_return_cases WHERE id=$1 AND status='received' FOR UPDATE`,
			id).Scan(&productID, &quantity, &orderID, &productName)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidTransition
		}
		if err != nil {
			return err
		}
		if disposition == "restock" {
			var before int
			if err := tx.QueryRowContext(ctx, `SELECT inventory FROM products WHERE id=$1 FOR UPDATE`, productID).Scan(&before); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrNotFound
				}
				return err
			}
			after := before + quantity
			if after < before {
				return ErrInvalidQuantity
			}
			if _, err := tx.ExecContext(ctx, `UPDATE products SET inventory=$2,updated_at=NOW() WHERE id=$1`, productID, after); err != nil {
				return err
			}
			if err := recordInventoryMovementTx(ctx, tx, productID, "return_restock", quantity, before, after,
				"Warehouse return inspected and approved for resale", fmt.Sprintf("RET-%08d / %s", id, orderID), actor); err != nil {
				return err
			}
		}
		result, err := tx.ExecContext(ctx, `UPDATE warehouse_return_cases
			SET status=$2,inspected_by=$3,inspected_at=NOW(),updated_at=NOW()
			WHERE id=$1 AND status='received'`, id, dispositionStatus(disposition), actor)
		if err != nil {
			return err
		}
		return requireReturnTransition(result)
	})
}

func dispositionStatus(disposition string) string {
	if disposition == "restock" {
		return "restocked"
	}
	return disposition
}

func requireReturnTransition(result sql.Result) error {
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return ErrInvalidTransition
	}
	return nil
}
