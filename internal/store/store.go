package store

import (
	"cmp"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"mitte/internal/commerce"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var ErrNotFound = errors.New("not found")
var ErrInvalidQuantity = errors.New("invalid quantity")
var ErrOutOfStock = errors.New("out of stock")
var ErrEmptyCart = errors.New("cart is empty")
var ErrInvalidCheckout = errors.New("invalid checkout")
var ErrReturnAlreadyOpen = errors.New("warehouse return already open")
var ErrInvalidReturnProduct = errors.New("return item is not linked to inventory")
var ErrUnknownPromotion = errors.New("unknown promotion")
var ErrPromotionInactive = errors.New("promotion inactive")
var ErrPromotionExhausted = errors.New("promotion exhausted")
var ErrPromotionLimit = errors.New("promotion already used")
var ErrInvalidPromotion = errors.New("invalid promotion")
var ErrCustomerFraudBlocked = errors.New("customer account is blocked for fraud review")

type Store struct {
	db *sql.DB
}

// PoolConfig limits this process's share of the database. Size the aggregate
// across all app and worker replicas below the managed database connection
// budget, leaving headroom for migrations and operations.
type PoolConfig struct {
	MaxOpenConns int
	MaxIdleConns int
}

type Category struct {
	ID          int64
	ParentID    int64
	Slug        string
	Name        string
	Description string
}

type Product struct {
	ID               int64
	Slug             string
	SKU              string
	Manufacturer     string            `json:"manufacturer"`
	ManufacturerSKU  string            `json:"manufacturerSku"`
	GTIN             string            `json:"gtin"`
	Material         string            `json:"material"`
	Dimensions       string            `json:"dimensions"`
	WeightGrams      int               `json:"weightGrams"`
	TechnicalDetails map[string]string `json:"technicalDetails"`
	Name             string
	ShortDescription string
	Description      string
	PriceCents       int
	PriceGrossCents  int    `json:"priceGrossCents"`
	PriceNetCents    int    `json:"priceNetCents"`
	SalePriceCents   int    `json:"salePriceCents,omitempty"`
	OfferName        string `json:"offerName,omitempty"`
	OfferCode        string `json:"offerCode,omitempty"`
	OfferDemo        bool   `json:"offerDemo,omitempty"`
	Demo             bool
	ImageURL         string
	ImageAlt         string
	Inventory        int
	Category         Category
	CategorySlugs    []string `json:"-"`
}

type ProductSummary struct {
	ID               int64
	Slug             string
	Name             string
	ShortDescription string
	PriceCents       int
	PriceGrossCents  int    `json:"priceGrossCents"`
	PriceNetCents    int    `json:"priceNetCents"`
	SalePriceCents   int    `json:"salePriceCents,omitempty"`
	OfferName        string `json:"offerName,omitempty"`
	OfferCode        string `json:"offerCode,omitempty"`
	OfferDemo        bool   `json:"offerDemo,omitempty"`
	ImageURL         string
	ImageAlt         string
	Inventory        int
	Featured         bool
	SalesCount       int64
	Demo             bool
	Category         Category
}

type CartItem struct {
	Product  Product
	Quantity int
}

type Cart struct {
	ID            string
	Items         []CartItem
	TotalCents    int
	PromotionCode string
	Quote         commerce.Quote
}

type CheckoutInput = commerce.CheckoutInput
type Address = commerce.Address

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	return OpenWithPool(ctx, databaseURL, PoolConfig{MaxOpenConns: 20, MaxIdleConns: 10})
}

func OpenWithPool(ctx context.Context, databaseURL string, pool PoolConfig) (*Store, error) {
	if pool.MaxOpenConns < 1 {
		pool.MaxOpenConns = 20
	}
	if pool.MaxIdleConns < 0 {
		pool.MaxIdleConns = 0
	}
	if pool.MaxIdleConns > pool.MaxOpenConns {
		pool.MaxIdleConns = pool.MaxOpenConns
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	db.SetMaxOpenConns(pool.MaxOpenConns)
	db.SetMaxIdleConns(pool.MaxIdleConns)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) DatabaseStats() sql.DBStats { return s.db.Stats() }

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return fmt.Errorf("migration ledger: %w", err)
	}

	// Keep schema changes serial even when two release processes start at once.
	// The lock belongs to this transaction, so it is always released on commit or
	// rollback and does not require separate cleanup.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migrations: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(418291837)`); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}

	for _, migration := range appMigrations {
		var applied bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, migration.Version).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %d (%s): %w", migration.Version, migration.Name, err)
		}
		if applied {
			continue
		}
		if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", migration.Version, migration.Name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, migration.Version); err != nil {
			return fmt.Errorf("record migration %d (%s): %w", migration.Version, migration.Name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

func (s *Store) Categories(ctx context.Context) ([]Category, error) {
	const query = `SELECT id, COALESCE(parent_id, 0), slug, name, description FROM categories ORDER BY parent_id NULLS FIRST, name`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var category Category
		if err := rows.Scan(&category.ID, &category.ParentID, &category.Slug, &category.Name, &category.Description); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

func (s *Store) ProductsPage(ctx context.Context, categorySlug, search string, featuredOnly bool, sort string, limit, offset int) ([]ProductSummary, bool, error) {
	if limit < 1 || limit > 100 {
		return nil, false, fmt.Errorf("product page size must be between 1 and 100")
	}
	if offset < 0 {
		return nil, false, fmt.Errorf("product page offset cannot be negative")
	}

	clauses := []string{"TRUE"}
	args := make([]any, 0, 3)

	if categorySlug != "" {
		args = append(args, categorySlug)
		clauses = append(clauses, "p.category_id IN (SELECT id FROM category_tree)")
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		clauses = append(clauses, fmt.Sprintf("(p.name ILIKE $%d OR p.short_description ILIKE $%d)", len(args), len(args)))
	}
	if featuredOnly {
		clauses = append(clauses, "p.featured = TRUE")
	}

	orderBy := "p.featured DESC, p.created_at DESC, p.name, p.id"
	salesJoin := ""
	salesSelect := "0::bigint"
	switch sort {
	case "", "featured":
	case "price-low":
		orderBy = "COALESCE(offer.sale_price_cents, p.price_cents) ASC, p.featured DESC, p.created_at DESC, p.name, p.id"
	case "price-high":
		orderBy = "COALESCE(offer.sale_price_cents, p.price_cents) DESC, p.featured DESC, p.created_at DESC, p.name, p.id"
	case "bestselling":
		salesJoin = ` LEFT JOIN (
			SELECT oi.product_id, SUM(oi.quantity)::bigint AS units_sold
			FROM order_items oi
			JOIN orders o ON o.id = oi.order_id
			WHERE oi.product_id IS NOT NULL AND o.status IN ('paid', 'fulfilled')
			GROUP BY oi.product_id
		) sales ON sales.product_id = p.id`
		salesSelect = "COALESCE(sales.units_sold, 0)::bigint"
		clauses = append(clauses, "COALESCE(sales.units_sold, 0) > 0")
		orderBy = "sales.units_sold DESC, p.featured DESC, p.name, p.id"
	default:
		return nil, false, fmt.Errorf("unsupported product sort order %q", sort)
	}
	args = append(args, limit+1)
	limitParam := len(args)
	args = append(args, offset)
	offsetParam := len(args)
	prefix := ""
	if categorySlug != "" {
		prefix = `WITH RECURSIVE category_tree(id) AS (
			SELECT id FROM categories WHERE slug = $1
			UNION ALL
			SELECT child.id FROM categories child JOIN category_tree parent ON child.parent_id = parent.id
		)`
	}
	query := prefix + ` SELECT p.id, p.slug, p.name, p.short_description, p.price_cents,
		COALESCE(offer.sale_price_cents, 0), COALESCE(offer.name, ''), COALESCE(offer.code, ''),
		COALESCE(offer.demo_only, FALSE),
		p.image_url, p.image_alt, p.inventory, c.id, c.slug, c.name, c.description, p.featured, ` + salesSelect + `,
		(p.sku LIKE 'MITTE-DEMO-%' OR p.sku LIKE 'OBF-%')
		FROM products p
		JOIN categories c ON c.id = p.category_id
		LEFT JOIN LATERAL (
			SELECT pr.code, pr.name, pr.demo_only,
				CASE pr.kind
					WHEN 'percent' THEN (p.price_cents::bigint -
						LEAST(p.price_cents::bigint, (p.price_cents::bigint * pr.percent_bps + 5000) / 10000))::integer
				END AS sale_price_cents
			FROM promotions pr
			WHERE pr.active = TRUE AND pr.public_campaign = TRUE AND pr.auto_apply = TRUE
				AND pr.kind = 'percent'
				AND (pr.starts_at IS NULL OR pr.starts_at <= NOW())
				AND (pr.ends_at IS NULL OR pr.ends_at >= NOW())
				AND pr.min_subtotal_cents <= p.price_cents
				AND (pr.category_slug IS NULL OR pr.category_slug IN (
					WITH RECURSIVE ancestors(id, parent_id, slug) AS (
						SELECT id, parent_id, slug FROM categories WHERE id = c.id
						UNION ALL
						SELECT parent.id, parent.parent_id, parent.slug
						FROM categories parent JOIN ancestors child ON parent.id = child.parent_id
					)
					SELECT slug FROM ancestors
				))
				AND (pr.product_slug IS NULL OR pr.product_slug = p.slug)
				AND (pr.max_redemptions IS NULL OR
					(SELECT COUNT(*) FROM promotion_redemptions redemption WHERE redemption.code = pr.code) < pr.max_redemptions)
				AND pr.percent_bps > 0
				AND (p.price_cents::bigint * pr.percent_bps + 5000) / 10000 > 0
			ORDER BY sale_price_cents ASC, pr.code
			LIMIT 1
		) offer ON TRUE
		` + salesJoin + `
		WHERE p.status = 'active' AND ` + strings.Join(clauses, " AND ") + `
		ORDER BY ` + orderBy + fmt.Sprintf(" LIMIT $%d OFFSET $%d", limitParam, offsetParam)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	products := make([]ProductSummary, 0, limit+1)
	for rows.Next() {
		var product ProductSummary
		err := rows.Scan(
			&product.ID, &product.Slug, &product.Name, &product.ShortDescription, &product.PriceCents,
			&product.SalePriceCents, &product.OfferName, &product.OfferCode, &product.OfferDemo,
			&product.ImageURL, &product.ImageAlt, &product.Inventory, &product.Category.ID,
			&product.Category.Slug, &product.Category.Name, &product.Category.Description, &product.Featured,
			&product.SalesCount, &product.Demo,
		)
		if err != nil {
			return nil, false, fmt.Errorf("scan product summary: %w", err)
		}
		product.PriceGrossCents = product.PriceCents
		product.PriceNetCents = product.PriceCents - commerce.VATIncludedCents(product.PriceCents)
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(products) > limit
	if hasMore {
		products = products[:limit]
	}
	return products, hasMore, nil
}

func (s *Store) ProductBySlug(ctx context.Context, slug string) (Product, error) {
	const query = `SELECT p.id, p.slug, p.name, p.short_description, p.description, p.price_cents,
		COALESCE(offer.sale_price_cents, 0), COALESCE(offer.name, ''), COALESCE(offer.code, ''),
		COALESCE(offer.demo_only, FALSE), (p.sku LIKE 'MITTE-DEMO-%' OR p.sku LIKE 'OBF-%'),
		p.image_url, p.image_alt, p.inventory, c.id, c.slug, c.name, c.description,
		p.sku,p.manufacturer,p.manufacturer_sku,p.gtin,p.material,p.dimensions,p.weight_grams,p.technical_details
		FROM products p JOIN categories c ON c.id = p.category_id
		LEFT JOIN LATERAL (
			SELECT pr.code, pr.name, pr.demo_only,
				CASE pr.kind
					WHEN 'percent' THEN (p.price_cents::bigint -
						LEAST(p.price_cents::bigint, (p.price_cents::bigint * pr.percent_bps + 5000) / 10000))::integer
				END AS sale_price_cents
			FROM promotions pr
			WHERE pr.active = TRUE AND pr.public_campaign = TRUE AND pr.auto_apply = TRUE
				AND pr.kind = 'percent'
				AND (pr.starts_at IS NULL OR pr.starts_at <= NOW())
				AND (pr.ends_at IS NULL OR pr.ends_at >= NOW())
				AND pr.min_subtotal_cents <= p.price_cents
				AND (pr.category_slug IS NULL OR pr.category_slug IN (
					WITH RECURSIVE ancestors(id, parent_id, slug) AS (
						SELECT id, parent_id, slug FROM categories WHERE id = c.id
						UNION ALL
						SELECT parent.id, parent.parent_id, parent.slug
						FROM categories parent JOIN ancestors child ON parent.id = child.parent_id
					)
					SELECT slug FROM ancestors
				))
				AND (pr.product_slug IS NULL OR pr.product_slug = p.slug)
				AND (pr.max_redemptions IS NULL OR
					(SELECT COUNT(*) FROM promotion_redemptions redemption WHERE redemption.code = pr.code) < pr.max_redemptions)
				AND pr.percent_bps > 0
				AND (p.price_cents::bigint * pr.percent_bps + 5000) / 10000 > 0
			ORDER BY sale_price_cents ASC, pr.code
			LIMIT 1
		) offer ON TRUE
		WHERE p.slug = $1 AND p.status = 'active'`

	product, err := scanPublicProduct(s.db.QueryRowContext(ctx, query, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	return product, err
}

func (s *Store) Cart(ctx context.Context, sessionToken string) (Cart, error) {
	cart, err := s.getOrCreateCart(ctx, nil, sessionToken)
	if err != nil {
		return Cart{}, err
	}
	return s.cartWithItems(ctx, s.db, cart.ID)
}

func (s *Store) AddCartItem(ctx context.Context, sessionToken string, productID int64, quantity int) (Cart, error) {
	if quantity < 1 || quantity > 10 {
		return Cart{}, ErrInvalidQuantity
	}

	var cartID string
	err := s.withTransaction(ctx, func(tx *sql.Tx) error {
		cart, err := s.getOrCreateCart(ctx, tx, sessionToken)
		if err != nil {
			return err
		}
		cartID = cart.ID
		var inventory, currentQuantity int
		err = tx.QueryRowContext(ctx, `
    SELECT
        p.inventory,
        COALESCE(ci.quantity, 0)
    FROM products p
    LEFT JOIN cart_items ci
        ON ci.product_id = p.id
        AND ci.cart_id = $1
    WHERE p.id = $2
    FOR UPDATE OF p`, cart.ID, productID).Scan(&inventory, &currentQuantity)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if currentQuantity+quantity > inventory || currentQuantity+quantity > 10 {
			return ErrOutOfStock
		}
		const upsert = `INSERT INTO cart_items (cart_id, product_id, quantity) VALUES ($1, $2, $3)
		ON CONFLICT (cart_id, product_id) DO UPDATE SET quantity = EXCLUDED.quantity`
		_, err = tx.ExecContext(ctx, upsert, cart.ID, productID, currentQuantity+quantity)
		return err
	})
	if err != nil {
		return Cart{}, err
	}
	return s.cartWithItems(ctx, s.db, cartID)
}

func (s *Store) Checkout(ctx context.Context, sessionToken, idempotencyKey string, input CheckoutInput) (CheckoutResult, error) {
	return s.CheckoutForCustomer(ctx, sessionToken, idempotencyKey, 0, input)
}

func (s *Store) CheckoutForCustomer(ctx context.Context, sessionToken, idempotencyKey string, customerID int64, input CheckoutInput) (CheckoutResult, error) {
	commerce.NormalizeCheckoutInput(&input)
	if !commerce.ValidCheckoutInput(input) || len(idempotencyKey) < 16 || len(idempotencyKey) > 255 {
		return CheckoutResult{}, ErrInvalidCheckout
	}

	var checkout CheckoutResult
	err := s.withTransaction(ctx, func(tx *sql.Tx) error {
		if err := ensureCustomerNotFraudBlockedTx(ctx, tx, customerID, input.Email); err != nil {
			return err
		}

		// Duplicate requests must serialize before the initial "does this key
		// exist?" read. This is deliberately transaction-scoped: PostgreSQL
		// releases it on commit/rollback and no lock bookkeeping can leak.
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, sessionToken+":"+idempotencyKey); err != nil {
			return err
		}

		// A browser can retry after losing the response, and two tabs can submit at
		// nearly the same time. Reuse the original order rather than charging twice.
		var existing CheckoutResult
		err := tx.QueryRowContext(ctx, `SELECT o.id, o.total_cents, COALESCE(o.stripe_checkout_session_id, ''), COALESCE(o.stripe_checkout_url, ''),
			COALESCE(s.payment_method, 'stripe'), COALESCE(s.status, 'retryable')
		FROM orders o LEFT JOIN checkout_sagas s ON s.order_id = o.id
		WHERE o.checkout_session_token = $1 AND o.checkout_idempotency_key = $2`, sessionToken, idempotencyKey).
			Scan(&existing.OrderID, &existing.TotalCents, &existing.CheckoutSessionID, &existing.CheckoutURL, &existing.PaymentMethod, &existing.SagaStatus)
		if err == nil {
			existing.LineItems, err = checkoutLineItems(ctx, tx, existing.OrderID)
			if err != nil {
				return err
			}
			checkout = existing
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		cart, err := s.getOrCreateCart(ctx, tx, sessionToken)
		if err != nil {
			return err
		}
		cart, err = s.cartWithItems(ctx, tx, cart.ID)
		if err != nil {
			return err
		}
		if len(cart.Items) == 0 {
			return ErrEmptyCart
		}
		if !commerce.Chargeable(cart.Quote) {
			return ErrInvalidCheckout
		}

		slices.SortFunc(cart.Items, func(left, right CartItem) int { return cmp.Compare(left.Product.ID, right.Product.ID) })
		orderID, err := newOrderNumber(ctx, tx)
		if err != nil {
			return err
		}
		for _, item := range cart.Items {
			var before, after int
			err := tx.QueryRowContext(ctx, `UPDATE products SET inventory=inventory-$1 WHERE id=$2 AND inventory >= $1 RETURNING inventory+$1,inventory`,
				item.Quantity, item.Product.ID).Scan(&before, &after)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrOutOfStock
			}
			if err != nil {
				return err
			}
			if err := recordInventoryMovementTx(ctx, tx, item.Product.ID, "reservation", -item.Quantity, before, after,
				"Stock reserved for checkout", orderID, "system:checkout"); err != nil {
				return err
			}
		}
		quote := cart.Quote
		promoCode := sql.NullString{String: quote.PromotionCode, Valid: quote.PromotionCode != ""}
		const createOrder = `INSERT INTO orders (id, cart_id, customer_id, customer_name, customer_email, total_cents, status, checkout_idempotency_key, checkout_session_token,
		subtotal_cents, discount_cents, shipping_cents, tax_cents, promotion_code, inventory_reserved, checkout_expires_at,
		customer_type, company_name, vat_id, buyer_reference, procurement_method, customer_note)
		VALUES ($1, $2, NULLIF($3, 0), $4, $5, $6, 'pending_payment', $7, $8, $9, $10, $11, $12, $13, TRUE, NOW() + INTERVAL '50 minutes',
		$14, NULLIF($15, ''), NULLIF($16, ''), NULLIF($17, ''), $18, NULLIF($19, ''))`
		if _, err := tx.ExecContext(ctx, createOrder, orderID, cart.ID, customerID, input.Name, input.Email, quote.TotalCents, idempotencyKey,
			sessionToken, quote.SubtotalCents, quote.DiscountCents, quote.ShippingCents, quote.TaxCents, promoCode,
			input.CustomerType, input.CompanyName, input.VATID, input.BuyerReference, input.ProcurementMethod, input.Note); err != nil {
			return err
		}
		paymentMethod := input.PaymentMethod
		if paymentMethod == "" {
			paymentMethod = "stripe"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO checkout_sagas (order_id, payment_method, status)
			VALUES ($1, $2, 'started')`, orderID, paymentMethod); err != nil {
			return err
		}
		if err := createOrderAddress(ctx, tx, orderID, "billing", input.BillingAddress); err != nil {
			return err
		}
		if err := createOrderAddress(ctx, tx, orderID, "shipping", input.ShippingAddress); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_status_history (order_id, from_status, to_status, reason)
		VALUES ($1, NULL, 'pending_payment', 'checkout_created')`, orderID); err != nil {
			return err
		}
		if err := enqueueOrderEvent(ctx, tx, orderID, "order.created", map[string]any{
			"order_id": orderID, "status": "pending_payment", "total_cents": quote.TotalCents,
		}); err != nil {
			return err
		}
		const createItem = `INSERT INTO order_items (order_id, product_id, product_name, sku, unit_price_cents, quantity, supplier_name)
		VALUES ($1, $2, $3, (SELECT sku FROM products WHERE id = $2), $4, $5,
			(SELECT supplier_name FROM products WHERE id = $2))`
		for _, item := range cart.Items {
			if _, err := tx.ExecContext(ctx, createItem, orderID, item.Product.ID, item.Product.Name, item.Product.PriceCents, item.Quantity); err != nil {
				return err
			}
		}

		if quote.PromotionStatus == "applied" || quote.PromotionStatus == "applied_shipping" {
			promo, err := loadPromotion(ctx, tx, quote.PromotionCode)
			if err != nil {
				return err
			}
			if err := reservePromotion(ctx, tx, promo, orderID, input.Email, quote.DiscountCents); err != nil {
				return err
			}
		}

		if _, err := tx.ExecContext(ctx, `UPDATE carts SET status = 'checked_out', updated_at = NOW() WHERE id = $1`, cart.ID); err != nil {
			return err
		}
		checkout = CheckoutResult{OrderID: orderID, TotalCents: quote.TotalCents, LineItems: stripeCheckoutItems(quote),
			PaymentMethod: paymentMethod, SagaStatus: "started"}
		return nil
	})
	if err != nil {
		return CheckoutResult{}, err
	}
	return checkout, nil
}

func NormalizeCheckoutInput(input *CheckoutInput) {
	commerce.NormalizeCheckoutInput(input)
}

func ValidCheckoutInput(input CheckoutInput) bool {
	return commerce.ValidCheckoutInput(input)
}

func createOrderAddress(ctx context.Context, tx *sql.Tx, orderID, addressType string, address Address) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO order_addresses (order_id, address_type, recipient_name, company_name, address_line1, address_line2, postal_code, city, country_code)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, NULLIF($6, ''), $7, $8, $9)`,
		orderID, addressType, address.RecipientName, address.CompanyName, address.AddressLine1, address.AddressLine2, address.PostalCode, address.City, address.CountryCode)
	return err
}

func checkoutLineItems(ctx context.Context, q queryer, orderID string) ([]CheckoutLineItem, error) {
	var discount, shipping int
	var promotionCode string
	if err := q.QueryRowContext(ctx, `SELECT discount_cents, shipping_cents, COALESCE(promotion_code, '') FROM orders WHERE id = $1`, orderID).
		Scan(&discount, &shipping, &promotionCode); err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, `SELECT COALESCE(product_id, 0), product_name, unit_price_cents, quantity FROM order_items WHERE order_id = $1 ORDER BY product_name`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lines := make([]commerce.QuotedLine, 0)
	for rows.Next() {
		var line commerce.QuotedLine
		if err := rows.Scan(&line.ProductID, &line.Name, &line.UnitPriceCents, &line.Quantity); err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stripeCheckoutItems(commerce.ReconstructQuote(lines, discount, shipping, promotionCode, "")), nil
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type executor interface {
	queryer
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *Store) getOrCreateCart(ctx context.Context, q executor, sessionToken string) (Cart, error) {
	if q == nil {
		q = s.db
	}
	var cart Cart
	// A cart is the synchronization boundary for every cart mutation and for
	// checkout. Holding this parent-row lock prevents an item write from racing
	// with the transition from open to checked_out.
	err := q.QueryRowContext(ctx, `SELECT id, COALESCE(promotion_code, '')
		FROM carts WHERE session_token = $1 AND status = 'open' FOR UPDATE`, sessionToken).Scan(&cart.ID, &cart.PromotionCode)
	if err == nil {
		return cart, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Cart{}, err
	}

	cart.ID, err = newID("cart")
	if err != nil {
		return Cart{}, err
	}
	// The partial unique index permits a new cart after checkout, while making
	// simultaneous first requests for the same session converge on one row.
	err = q.QueryRowContext(ctx, `INSERT INTO carts (id, session_token) VALUES ($1, $2)
		ON CONFLICT (session_token) WHERE status = 'open'
		DO UPDATE SET updated_at = carts.updated_at
		RETURNING id, COALESCE(promotion_code, '')`, cart.ID, sessionToken).Scan(&cart.ID, &cart.PromotionCode)
	if err != nil {
		return Cart{}, err
	}
	return cart, nil
}

func (s *Store) cartWithItems(ctx context.Context, q queryer, cartID string) (Cart, error) {
	const query = `WITH RECURSIVE category_ancestors(category_id, current_id, ancestor_slug) AS (
		SELECT id, id, slug FROM categories
		UNION ALL
		SELECT lineage.category_id, parent.id, parent.slug
		FROM category_ancestors lineage
		JOIN categories current ON current.id = lineage.current_id
		JOIN categories parent ON parent.id = current.parent_id
	)
	SELECT p.id, p.slug, p.name, p.short_description, p.description, p.price_cents,
		p.image_url, p.image_alt, p.inventory, c.id, c.slug, c.name, c.description,
		to_json(ARRAY(SELECT ancestor_slug FROM category_ancestors WHERE category_id = c.id)),
		p.sku,p.manufacturer,p.manufacturer_sku,p.gtin,p.material,p.dimensions,p.weight_grams,p.technical_details,ci.quantity
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		JOIN categories c ON c.id = p.category_id
		WHERE ci.cart_id = $1 ORDER BY p.name`

	var promotionCode string
	if err := q.QueryRowContext(ctx, `SELECT COALESCE(promotion_code, '') FROM carts WHERE id = $1`, cartID).Scan(&promotionCode); err != nil {
		return Cart{}, err
	}
	// A checkout passes a *sql.Tx, which has only one PostgreSQL connection.
	// Read this scalar value before opening rows; pgx correctly rejects another
	// statement while a previous result set remains active on that connection.
	rows, err := q.QueryContext(ctx, query, cartID)
	if err != nil {
		return Cart{}, err
	}
	defer rows.Close()

	cart := Cart{ID: cartID, PromotionCode: promotionCode, Items: make([]CartItem, 0)}
	for rows.Next() {
		var item CartItem
		var technicalDetails, categorySlugsJSON []byte
		if err := rows.Scan(&item.Product.ID, &item.Product.Slug, &item.Product.Name, &item.Product.ShortDescription,
			&item.Product.Description, &item.Product.PriceCents, &item.Product.ImageURL, &item.Product.ImageAlt,
			&item.Product.Inventory, &item.Product.Category.ID, &item.Product.Category.Slug, &item.Product.Category.Name,
			&item.Product.Category.Description, &categorySlugsJSON, &item.Product.SKU, &item.Product.Manufacturer, &item.Product.ManufacturerSKU,
			&item.Product.GTIN, &item.Product.Material, &item.Product.Dimensions, &item.Product.WeightGrams,
			&technicalDetails, &item.Quantity); err != nil {
			return Cart{}, err
		}
		if err := json.Unmarshal(categorySlugsJSON, &item.Product.CategorySlugs); err != nil {
			return Cart{}, fmt.Errorf("decode product category ancestry: %w", err)
		}
		if err := json.Unmarshal(technicalDetails, &item.Product.TechnicalDetails); err != nil {
			return Cart{}, fmt.Errorf("decode cart product technical details: %w", err)
		}
		item.Product.setPriceBreakdown()
		cart.Items = append(cart.Items, item)
	}
	if err := rows.Err(); err != nil {
		return Cart{}, err
	}
	return s.withQuote(ctx, q, cart)
}

type scanner interface {
	Scan(...any) error
}

func scanProduct(s scanner) (Product, error) {
	var product Product
	var technicalDetails []byte
	err := s.Scan(&product.ID, &product.Slug, &product.Name, &product.ShortDescription, &product.Description,
		&product.PriceCents, &product.ImageURL, &product.ImageAlt, &product.Inventory, &product.Category.ID,
		&product.Category.Slug, &product.Category.Name, &product.Category.Description, &product.SKU,
		&product.Manufacturer, &product.ManufacturerSKU, &product.GTIN, &product.Material, &product.Dimensions,
		&product.WeightGrams, &technicalDetails)
	if err != nil {
		return Product{}, err
	}
	if err := json.Unmarshal(technicalDetails, &product.TechnicalDetails); err != nil {
		return Product{}, fmt.Errorf("decode product technical details: %w", err)
	}
	product.setPriceBreakdown()
	return product, err
}

func scanPublicProduct(s scanner) (Product, error) {
	var product Product
	var technicalDetails []byte
	err := s.Scan(
		&product.ID, &product.Slug, &product.Name, &product.ShortDescription, &product.Description,
		&product.PriceCents, &product.SalePriceCents, &product.OfferName, &product.OfferCode,
		&product.OfferDemo, &product.Demo, &product.ImageURL, &product.ImageAlt, &product.Inventory,
		&product.Category.ID, &product.Category.Slug, &product.Category.Name, &product.Category.Description,
		&product.SKU, &product.Manufacturer, &product.ManufacturerSKU, &product.GTIN, &product.Material,
		&product.Dimensions, &product.WeightGrams, &technicalDetails,
	)
	if err != nil {
		return Product{}, err
	}
	if err := json.Unmarshal(technicalDetails, &product.TechnicalDetails); err != nil {
		return Product{}, fmt.Errorf("decode product technical details: %w", err)
	}
	product.setPriceBreakdown()
	return product, nil
}

func (product *Product) setPriceBreakdown() {
	product.PriceGrossCents = product.PriceCents
	product.PriceNetCents = product.PriceCents - commerce.VATIncludedCents(product.PriceCents)
}

func newID(prefix string) (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%x", prefix, bytes), nil
}

func newOrderNumber(ctx context.Context, tx *sql.Tx) (string, error) {
	var number int64
	if err := tx.QueryRowContext(ctx, "SELECT nextval('order_number_seq')").Scan(&number); err != nil {
		return "", err
	}
	return formatOrderNumber(number)
}

func formatOrderNumber(number int64) (string, error) {
	if number < 10000000 || number > 99999999 {
		return "", fmt.Errorf("generated order number is outside the eight-digit range: %d", number)
	}
	return fmt.Sprintf("%08d", number), nil
}
