package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CatalogProduct struct {
	Product
	Status           string            `json:"status"`
	Featured         bool              `json:"featured"`
	SEOTitle         string            `json:"seoTitle"`
	SEODescription   string            `json:"seoDescription"`
	Barcode          string            `json:"barcode"`
	SupplierName     string            `json:"supplierName"`
	Manufacturer     string            `json:"manufacturer"`
	ManufacturerSKU  string            `json:"manufacturerSku"`
	GTIN             string            `json:"gtin"`
	Material         string            `json:"material"`
	Dimensions       string            `json:"dimensions"`
	WeightGrams      int               `json:"weightGrams"`
	TechnicalDetails map[string]string `json:"technicalDetails"`
	UnitCostCents    int               `json:"unitCostCents"`
	ReorderPoint     int               `json:"reorderPoint"`
	LeadTimeDays     int               `json:"leadTimeDays"`
	ReservedStock    int               `json:"reservedStock"`
	OnHand           int               `json:"onHand"`
	UpdatedAt        time.Time         `json:"updatedAt"`
	ArchivedAt       *time.Time        `json:"archivedAt,omitempty"`
}

type CatalogProductInput struct {
	SKU              string            `json:"sku"`
	Slug             string            `json:"slug"`
	Name             string            `json:"name"`
	ShortDescription string            `json:"shortDescription"`
	Description      string            `json:"description"`
	PriceCents       int               `json:"priceCents"`
	ImageURL         string            `json:"imageUrl"`
	ImageAlt         string            `json:"imageAlt"`
	CategoryID       int64             `json:"categoryId"`
	Featured         bool              `json:"featured"`
	SEOTitle         string            `json:"seoTitle"`
	SEODescription   string            `json:"seoDescription"`
	Barcode          string            `json:"barcode"`
	SupplierName     string            `json:"supplierName"`
	Manufacturer     string            `json:"manufacturer"`
	ManufacturerSKU  string            `json:"manufacturerSku"`
	GTIN             string            `json:"gtin"`
	Material         string            `json:"material"`
	Dimensions       string            `json:"dimensions"`
	WeightGrams      int               `json:"weightGrams"`
	TechnicalDetails map[string]string `json:"technicalDetails"`
	UnitCostCents    int               `json:"unitCostCents"`
	ReorderPoint     int               `json:"reorderPoint"`
	LeadTimeDays     int               `json:"leadTimeDays"`
	InitialStock     int               `json:"initialStock"`
}

type CatalogPriceRecord struct {
	ID           int64
	SKU          string
	Name         string
	PriceCents   int
	Available    int
	Status       string
	CategoryName string
	UpdatedAt    time.Time
}

func (s *Store) CatalogPricesPage(ctx context.Context, search, status string, sellableOnly bool, limit, offset int) ([]CatalogPriceRecord, int, error) {
	search = strings.TrimSpace(search)
	args := []any{}
	where := []string{"TRUE"}
	if search != "" {
		args = append(args, "%"+search+"%")
		where = append(where, "(p.name ILIKE $1 OR p.sku ILIKE $1 OR p.slug ILIKE $1)")
	}
	if status != "" {
		args = append(args, status)
		where = append(where, "p.status = $"+sqlPlaceholder(len(args)))
	}
	if sellableOnly {
		where = append(where, "p.status = 'active' AND p.inventory > 0")
	}
	whereClause := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM products p WHERE `+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, `SELECT p.id,p.sku,p.name,p.price_cents,p.inventory,p.status,c.name,p.updated_at
		FROM products p JOIN categories c ON c.id=p.category_id
		WHERE `+whereClause+`
		ORDER BY p.updated_at DESC,p.id DESC
		LIMIT $`+sqlPlaceholder(len(args)-1)+` OFFSET $`+sqlPlaceholder(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := make([]CatalogPriceRecord, 0)
	for rows.Next() {
		var record CatalogPriceRecord
		if err := rows.Scan(&record.ID, &record.SKU, &record.Name, &record.PriceCents, &record.Available, &record.Status, &record.CategoryName, &record.UpdatedAt); err != nil {
			return nil, 0, err
		}
		result = append(result, record)
	}
	return result, total, rows.Err()
}

func catalogProductScan(row interface{ Scan(...any) error }) (CatalogProduct, error) {
	var p CatalogProduct
	var technicalDetails []byte
	err := row.Scan(&p.ID, &p.Slug, &p.SKU, &p.Name, &p.ShortDescription, &p.Description, &p.PriceCents,
		&p.ImageURL, &p.ImageAlt, &p.Inventory, &p.Category.ID, &p.Category.Slug, &p.Category.Name,
		&p.Category.Description, &p.Status, &p.Featured, &p.SEOTitle, &p.SEODescription,
		&p.Barcode, &p.SupplierName, &p.UnitCostCents, &p.ReorderPoint, &p.LeadTimeDays,
		&p.ReservedStock, &p.OnHand, &p.UpdatedAt, &p.ArchivedAt, &p.Manufacturer, &p.ManufacturerSKU,
		&p.GTIN, &p.Material, &p.Dimensions, &p.WeightGrams, &technicalDetails)
	if err != nil {
		return CatalogProduct{}, err
	}
	if err := json.Unmarshal(technicalDetails, &p.TechnicalDetails); err != nil {
		return CatalogProduct{}, fmt.Errorf("decode catalog technical details: %w", err)
	}
	p.setPriceBreakdown()
	return p, err
}

const catalogProductSelect = `SELECT p.id,p.slug,p.sku,p.name,p.short_description,p.description,p.price_cents,p.image_url,p.image_alt,p.inventory,
 c.id,c.slug,c.name,c.description,p.status,p.featured,p.seo_title,p.seo_description,p.barcode,p.supplier_name,p.unit_cost_cents,p.reorder_point,p.lead_time_days,
 COALESCE(r.quantity,0),p.inventory+COALESCE(r.quantity,0),p.updated_at,p.archived_at,p.manufacturer,p.manufacturer_sku,p.gtin,p.material,p.dimensions,p.weight_grams,p.technical_details
 FROM products p JOIN categories c ON c.id=p.category_id`

const catalogProductReservedJoin = ` LEFT JOIN LATERAL (
    SELECT SUM(oi.quantity)::INTEGER AS quantity
    FROM order_items oi
    JOIN orders o ON o.id=oi.order_id
    WHERE oi.product_id=p.id AND o.inventory_reserved=TRUE AND o.status IN ('pending_payment','paid','invoice_approved')
 ) r ON TRUE`

func (s *Store) CatalogProducts(ctx context.Context, search, status string, limit, offset int) ([]CatalogProduct, int, error) {
	if limit < 1 || limit > 100 {
		return nil, 0, fmt.Errorf("catalog page size must be between 1 and 100")
	}
	if offset < 0 {
		return nil, 0, fmt.Errorf("catalog page offset cannot be negative")
	}
	args := []any{}
	where := []string{"TRUE"}
	if strings.TrimSpace(search) != "" {
		args = append(args, "%"+strings.TrimSpace(search)+"%")
		where = append(where, "(p.name ILIKE $1 OR p.slug ILIKE $1 OR p.image_alt ILIKE $1 OR p.sku ILIKE $1 OR p.barcode ILIKE $1 OR p.manufacturer ILIKE $1 OR p.manufacturer_sku ILIKE $1 OR p.gtin ILIKE $1)")
	}
	if status != "" {
		args = append(args, status)
		where = append(where, "p.status = $"+sqlPlaceholder(len(args)))
	}
	whereClause := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM products p WHERE `+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	pageArgs := append(append([]any{}, args...), limit, offset)
	query := catalogProductSelect + catalogProductReservedJoin + " WHERE " + whereClause +
		" ORDER BY p.updated_at DESC,p.id DESC LIMIT $" + sqlPlaceholder(len(pageArgs)-1) +
		" OFFSET $" + sqlPlaceholder(len(pageArgs))
	rows, err := s.db.QueryContext(ctx, query, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := make([]CatalogProduct, 0, limit)
	for rows.Next() {
		p, err := catalogProductScan(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, p)
	}
	return result, total, rows.Err()
}

func sqlPlaceholder(n int) string { return strconv.Itoa(n) }

func (s *Store) CatalogProduct(ctx context.Context, id int64) (CatalogProduct, error) {
	p, err := catalogProductScan(s.db.QueryRowContext(ctx, catalogProductSelect+catalogProductReservedJoin+" WHERE p.id=$1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return CatalogProduct{}, ErrNotFound
	}
	return p, err
}

func (s *Store) CreateCatalogProduct(ctx context.Context, input CatalogProductInput, actor string) (CatalogProduct, error) {
	var id int64
	if input.TechnicalDetails == nil {
		input.TechnicalDetails = map[string]string{}
	}
	err := s.withTransaction(ctx, func(tx *sql.Tx) error {
		details, err := json.Marshal(input.TechnicalDetails)
		if err != nil {
			return fmt.Errorf("encode catalog technical details: %w", err)
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO products(category_id,slug,name,short_description,description,price_cents,image_url,image_alt,featured,status,seo_title,seo_description,sku,barcode,supplier_name,unit_cost_cents,reorder_point,lead_time_days,inventory,manufacturer,manufacturer_sku,gtin,material,dimensions,weight_grams,technical_details)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'draft',$10,$11,COALESCE(NULLIF($12,''),'MF-' || CASE WHEN nextval('product_sku_seq') < 1000000 THEN LPAD(currval('product_sku_seq')::text,6,'0') ELSE currval('product_sku_seq')::text END),$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25::jsonb) RETURNING id`,
			input.CategoryID, input.Slug, input.Name, input.ShortDescription, input.Description, input.PriceCents, input.ImageURL, input.ImageAlt, input.Featured,
			input.SEOTitle, input.SEODescription, input.SKU, input.Barcode, input.SupplierName, input.UnitCostCents, input.ReorderPoint, input.LeadTimeDays, input.InitialStock,
			input.Manufacturer, input.ManufacturerSKU, input.GTIN, input.Material, input.Dimensions, input.WeightGrams, details).Scan(&id)
		if err != nil {
			return err
		}
		if input.InitialStock > 0 {
			if err := recordInventoryMovementTx(ctx, tx, id, "opening_balance", input.InitialStock, 0, input.InitialStock, "Initial stock at product creation", "", actor); err != nil {
				return err
			}
		}
		return auditCatalogTx(ctx, tx, id, "created", input, actor)
	})
	if err != nil {
		return CatalogProduct{}, err
	}
	return s.CatalogProduct(ctx, id)
}

func (s *Store) UpdateCatalogProduct(ctx context.Context, id int64, input CatalogProductInput, actor string) (CatalogProduct, error) {
	if input.TechnicalDetails == nil {
		input.TechnicalDetails = map[string]string{}
	}
	err := s.withTransaction(ctx, func(tx *sql.Tx) error {
		details, err := json.Marshal(input.TechnicalDetails)
		if err != nil {
			return fmt.Errorf("encode catalog technical details: %w", err)
		}
		result, err := tx.ExecContext(ctx, `UPDATE products SET category_id=$2,slug=$3,name=$4,short_description=$5,description=$6,price_cents=$7,image_url=$8,image_alt=$9,featured=$10,seo_title=$11,seo_description=$12,barcode=$13,supplier_name=$14,unit_cost_cents=$15,reorder_point=$16,lead_time_days=$17,manufacturer=$18,manufacturer_sku=$19,gtin=$20,material=$21,dimensions=$22,weight_grams=$23,technical_details=$24::jsonb,updated_at=NOW() WHERE id=$1 AND status <> 'archived'`,
			id, input.CategoryID, input.Slug, input.Name, input.ShortDescription, input.Description, input.PriceCents, input.ImageURL, input.ImageAlt,
			input.Featured, input.SEOTitle, input.SEODescription, input.Barcode, input.SupplierName, input.UnitCostCents, input.ReorderPoint, input.LeadTimeDays,
			input.Manufacturer, input.ManufacturerSKU, input.GTIN, input.Material, input.Dimensions, input.WeightGrams, details)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return auditCatalogTx(ctx, tx, id, "updated", input, actor)
	})
	if err != nil {
		return CatalogProduct{}, err
	}
	return s.CatalogProduct(ctx, id)
}

func (s *Store) SetCatalogStatus(ctx context.Context, id int64, status, actor string) error {
	if status != "draft" && status != "active" && status != "scheduled" && status != "archived" {
		return errors.New("invalid catalog status")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE products SET status=$2,archived_at=CASE WHEN $2='archived' THEN NOW() ELSE NULL END,updated_at=NOW() WHERE id=$1`, id, status)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return s.auditCatalog(ctx, id, "status_changed", map[string]string{"status": status}, actor)
}

func (s *Store) AdjustCatalogInventory(ctx context.Context, id int64, delta int, reason, reference, actor string) (CatalogProduct, error) {
	if delta == 0 || strings.TrimSpace(reason) == "" {
		return CatalogProduct{}, ErrInvalidQuantity
	}
	err := s.withTransaction(ctx, func(tx *sql.Tx) error {
		var before int
		if err := tx.QueryRowContext(ctx, `SELECT inventory FROM products WHERE id=$1 FOR UPDATE`, id).Scan(&before); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		after := before + delta
		if after < 0 {
			return ErrOutOfStock
		}
		if _, err := tx.ExecContext(ctx, `UPDATE products SET inventory=$2,updated_at=NOW() WHERE id=$1`, id, after); err != nil {
			return err
		}
		if err := recordInventoryMovementTx(ctx, tx, id, "adjustment", delta, before, after, strings.TrimSpace(reason), strings.TrimSpace(reference), actor); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO product_inventory_adjustments(product_id,delta,inventory_before,inventory_after,reason,reference,actor_email) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, delta, before, after, reason, reference, actor)
		if err != nil {
			return err
		}
		return auditCatalogTx(ctx, tx, id, "inventory_adjusted", map[string]any{"delta": delta, "before": before, "after": after, "reason": reason, "reference": reference}, actor)
	})
	if err != nil {
		return CatalogProduct{}, err
	}
	return s.CatalogProduct(ctx, id)
}

func (s *Store) ReceiveCatalogInventory(ctx context.Context, id int64, quantity int, reason, reference, actor string) (CatalogProduct, error) {
	if quantity <= 0 || strings.TrimSpace(reason) == "" {
		return CatalogProduct{}, ErrInvalidQuantity
	}
	err := s.withTransaction(ctx, func(tx *sql.Tx) error {
		var before int
		if err := tx.QueryRowContext(ctx, `SELECT inventory FROM products WHERE id=$1 FOR UPDATE`, id).Scan(&before); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		after := before + quantity
		if after < before {
			return ErrInvalidQuantity
		}
		if _, err := tx.ExecContext(ctx, `UPDATE products SET inventory=$2,updated_at=NOW() WHERE id=$1`, id, after); err != nil {
			return err
		}
		if err := recordInventoryMovementTx(ctx, tx, id, "receipt", quantity, before, after, strings.TrimSpace(reason), strings.TrimSpace(reference), actor); err != nil {
			return err
		}
		return auditCatalogTx(ctx, tx, id, "inventory_received", map[string]any{"quantity": quantity, "before": before, "after": after, "reason": reason, "reference": reference}, actor)
	})
	if err != nil {
		return CatalogProduct{}, err
	}
	return s.CatalogProduct(ctx, id)
}

func (s *Store) CatalogAudit(ctx context.Context, id int64) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT action,changes,actor_email,created_at FROM product_audit_log WHERE product_id=$1 ORDER BY created_at DESC LIMIT 100`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var action, actor string
		var raw []byte
		var at time.Time
		if err := rows.Scan(&action, &raw, &actor, &at); err != nil {
			return nil, err
		}
		var changes any
		_ = json.Unmarshal(raw, &changes)
		out = append(out, map[string]any{"action": action, "changes": changes, "actorEmail": actor, "createdAt": at})
	}
	return out, rows.Err()
}

func auditCatalogTx(ctx context.Context, tx *sql.Tx, id int64, action string, changes any, actor string) error {
	raw, _ := json.Marshal(changes)
	_, err := tx.ExecContext(ctx, `INSERT INTO product_audit_log(product_id,action,changes,actor_email) VALUES($1,$2,$3::jsonb,$4)`, id, action, string(raw), actor)
	return err
}

func recordInventoryMovementTx(ctx context.Context, tx executor, productID int64, movementType string, delta, before, after int, reason, reference, actor string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO product_inventory_movements(product_id,movement_type,quantity_delta,inventory_before,inventory_after,reason,reference,actor_email)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, productID, movementType, delta, before, after, reason, reference, actor)
	return err
}
func (s *Store) auditCatalog(ctx context.Context, id int64, action string, changes any, actor string) error {
	raw, _ := json.Marshal(changes)
	_, err := s.db.ExecContext(ctx, `INSERT INTO product_audit_log(product_id,action,changes,actor_email) VALUES($1,$2,$3::jsonb,$4)`, id, action, string(raw), actor)
	return err
}
