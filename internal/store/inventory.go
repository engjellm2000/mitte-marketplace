package store

import (
	"context"
	"strings"
	"time"
)

type InventorySummary struct {
	ProductCount       int64 `json:"productCount"`
	OnHandUnits        int64 `json:"onHandUnits"`
	ReservedUnits      int64 `json:"reservedUnits"`
	AvailableUnits     int64 `json:"availableUnits"`
	LowStockProducts   int64 `json:"lowStockProducts"`
	OutOfStockProducts int64 `json:"outOfStockProducts"`
	StockValueCents    int64 `json:"stockValueCents"`
}

type InventoryMovement struct {
	ID              int64     `json:"id"`
	ProductID       int64     `json:"productId"`
	SKU             string    `json:"sku"`
	ProductName     string    `json:"productName"`
	Type            string    `json:"type"`
	QuantityDelta   int       `json:"quantityDelta"`
	InventoryBefore int       `json:"inventoryBefore"`
	InventoryAfter  int       `json:"inventoryAfter"`
	Reason          string    `json:"reason"`
	Reference       string    `json:"reference"`
	ActorEmail      string    `json:"actorEmail"`
	CreatedAt       time.Time `json:"createdAt"`
}

func (s *Store) InventoryOverview(ctx context.Context, search, stockFilter string, limit, offset int) (InventorySummary, []CatalogProduct, int, error) {
	var summary InventorySummary
	err := s.db.QueryRowContext(ctx, `WITH positions AS (
		SELECT p.inventory, p.reorder_point, p.unit_cost_cents, COALESCE(r.quantity,0) AS reserved
		FROM products p
		LEFT JOIN LATERAL (
			SELECT SUM(oi.quantity)::INTEGER AS quantity
			FROM order_items oi JOIN orders o ON o.id=oi.order_id
			WHERE oi.product_id=p.id AND o.inventory_reserved=TRUE AND o.status IN ('pending_payment','paid','invoice_approved')
		) r ON TRUE
		WHERE p.status <> 'archived'
	)
	SELECT COUNT(*), COALESCE(SUM(inventory::BIGINT+reserved),0), COALESCE(SUM(reserved),0),
		COALESCE(SUM(inventory::BIGINT),0),
		COUNT(*) FILTER (WHERE inventory=0 OR (reorder_point>0 AND inventory<=reorder_point)),
		COUNT(*) FILTER (WHERE inventory=0),
		COALESCE(SUM((inventory+reserved)::BIGINT*unit_cost_cents),0)
	FROM positions`).Scan(&summary.ProductCount, &summary.OnHandUnits, &summary.ReservedUnits, &summary.AvailableUnits,
		&summary.LowStockProducts, &summary.OutOfStockProducts, &summary.StockValueCents)
	if err != nil {
		return InventorySummary{}, nil, 0, err
	}

	where := []string{"p.status <> 'archived'"}
	args := []any{}
	if search = strings.TrimSpace(search); search != "" {
		args = append(args, "%"+search+"%")
		where = append(where, "(p.name ILIKE $1 OR p.slug ILIKE $1 OR p.sku ILIKE $1 OR p.barcode ILIKE $1)")
	}
	switch stockFilter {
	case "low":
		where = append(where, "(p.inventory=0 OR (p.reorder_point>0 AND p.inventory<=p.reorder_point))")
	case "out":
		where = append(where, "p.inventory=0")
	case "available":
		where = append(where, "p.inventory>0 AND (p.reorder_point=0 OR p.inventory>p.reorder_point)")
	}
	queryArgs := append([]any{}, args...)
	countQuery := `SELECT COUNT(*) FROM products p WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, queryArgs...).Scan(&total); err != nil {
		return InventorySummary{}, nil, 0, err
	}
	queryArgs = append(queryArgs, limit, offset)
	query := catalogProductSelect + catalogProductReservedJoin + " WHERE " + strings.Join(where, " AND ") +
		" ORDER BY CASE WHEN p.inventory=0 THEN 0 WHEN p.reorder_point>0 AND p.inventory<=p.reorder_point THEN 1 ELSE 2 END,p.name,p.id LIMIT $" +
		sqlPlaceholder(len(queryArgs)-1) + " OFFSET $" + sqlPlaceholder(len(queryArgs))
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return InventorySummary{}, nil, 0, err
	}
	defer rows.Close()
	products := []CatalogProduct{}
	for rows.Next() {
		product, err := catalogProductScan(rows)
		if err != nil {
			return InventorySummary{}, nil, 0, err
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return InventorySummary{}, nil, 0, err
	}
	return summary, products, total, nil
}

func (s *Store) InventoryMovements(ctx context.Context, productID int64, limit int) ([]InventoryMovement, error) {
	query := `SELECT m.id,m.product_id,p.sku,p.name,m.movement_type,m.quantity_delta,m.inventory_before,m.inventory_after,
		m.reason,m.reference,m.actor_email,m.created_at
		FROM product_inventory_movements m JOIN products p ON p.id=m.product_id`
	args := []any{}
	if productID > 0 {
		args = append(args, productID)
		query += ` WHERE m.product_id=$1`
	}
	args = append(args, limit)
	query += ` ORDER BY m.created_at DESC,m.id DESC LIMIT $` + sqlPlaceholder(len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	movements := []InventoryMovement{}
	for rows.Next() {
		var movement InventoryMovement
		if err := rows.Scan(&movement.ID, &movement.ProductID, &movement.SKU, &movement.ProductName, &movement.Type,
			&movement.QuantityDelta, &movement.InventoryBefore, &movement.InventoryAfter, &movement.Reason,
			&movement.Reference, &movement.ActorEmail, &movement.CreatedAt); err != nil {
			return nil, err
		}
		movements = append(movements, movement)
	}
	return movements, rows.Err()
}
