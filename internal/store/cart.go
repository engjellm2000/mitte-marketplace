package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"mitte/internal/commerce"
)

func (s *Store) SetCartItem(ctx context.Context, sessionToken string, productID int64, quantity int) (Cart, error) {
	if quantity < 0 || quantity > commerce.MaxLineQuantity {
		return Cart{}, ErrInvalidQuantity
	}

	var cartID string
	err := s.withTransaction(ctx, func(tx *sql.Tx) error {
		cart, err := s.getOrCreateCart(ctx, tx, sessionToken)
		if err != nil {
			return err
		}
		cartID = cart.ID
		if quantity == 0 {
			_, err := tx.ExecContext(ctx, `DELETE FROM cart_items WHERE cart_id = $1 AND product_id = $2`, cart.ID, productID)
			return err
		}

		var inventory int
		err = tx.QueryRowContext(ctx, `SELECT inventory FROM products WHERE id = $1 FOR UPDATE`, productID).Scan(&inventory)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if quantity > inventory {
			return ErrOutOfStock
		}
		const upsert = `INSERT INTO cart_items (cart_id, product_id, quantity) VALUES ($1, $2, $3)
			ON CONFLICT (cart_id, product_id) DO UPDATE SET quantity = EXCLUDED.quantity`
		_, err = tx.ExecContext(ctx, upsert, cart.ID, productID, quantity)
		return err
	})
	if err != nil {
		return Cart{}, err
	}
	return s.cartWithItems(ctx, s.db, cartID)
}

func (s *Store) ApplyPromotion(ctx context.Context, sessionToken, code string) (Cart, error) {
	code = strings.ToUpper(strings.TrimSpace(code))

	var cartID string
	err := s.withTransaction(ctx, func(tx *sql.Tx) error {
		cart, err := s.getOrCreateCart(ctx, tx, sessionToken)
		if err != nil {
			return err
		}
		cartID = cart.ID
		if code == "" {
			_, err = tx.ExecContext(ctx, `UPDATE carts SET promotion_code = NULL, updated_at = NOW() WHERE id = $1`, cart.ID)
			return err
		}

		promo, err := loadPromotion(ctx, tx, code)
		if err != nil {
			return err
		}
		if !promo.Active {
			return ErrPromotionInactive
		}
		_, err = tx.ExecContext(ctx, `UPDATE carts SET promotion_code = $2, updated_at = NOW() WHERE id = $1`, cart.ID, promo.Code)
		return err
	})
	if err != nil {
		return Cart{}, err
	}
	return s.cartWithItems(ctx, s.db, cartID)
}

func (s *Store) withQuote(ctx context.Context, q queryer, cart Cart) (Cart, error) {
	lines := make([]commerce.Line, 0, len(cart.Items))
	for _, item := range cart.Items {
		lines = append(lines, commerce.Line{
			ProductID:      item.Product.ID,
			Name:           item.Product.Name,
			CategorySlug:   item.Product.Category.Slug,
			CategorySlugs:  item.Product.CategorySlugs,
			ProductSlug:    item.Product.Slug,
			UnitPriceCents: item.Product.PriceCents,
			Quantity:       item.Quantity,
		})
	}
	var promo *commerce.Promotion
	if cart.PromotionCode != "" {
		loaded, err := loadPromotion(ctx, q, cart.PromotionCode)
		if err == nil {
			promo = &loaded.Promotion
		} else if !errors.Is(err, ErrUnknownPromotion) && !errors.Is(err, ErrPromotionInactive) {
			return Cart{}, err
		}
	} else {
		var err error
		promo, err = s.bestAutomaticPromotion(ctx, q, lines)
		if err != nil {
			return Cart{}, err
		}
	}
	quote, err := commerce.ComputeQuote(lines, promo)
	if err != nil {
		return Cart{}, err
	}
	cart.Quote = quote
	cart.TotalCents = quote.TotalCents
	return cart, nil
}

type storedPromotion struct {
	commerce.Promotion
	MaxRedemptions *int
	PerEmailLimit  int
}

func loadPromotion(ctx context.Context, q queryer, code string) (storedPromotion, error) {
	const query = `SELECT code, name, kind, percent_bps, amount_cents, min_subtotal_cents,
		COALESCE(category_slug, ''), COALESCE(product_slug, ''), active,
		max_redemptions, per_email_limit
		FROM promotions
		WHERE upper(code) = upper($1)
		  AND (starts_at IS NULL OR starts_at <= NOW())
		  AND (ends_at IS NULL OR ends_at >= NOW())`
	var promo storedPromotion
	var kind string
	err := q.QueryRowContext(ctx, query, code).Scan(&promo.Code, &promo.Name, &kind, &promo.PercentBPS, &promo.AmountCents,
		&promo.MinSubtotalCents, &promo.CategorySlug, &promo.ProductSlug, &promo.Active, &promo.MaxRedemptions, &promo.PerEmailLimit)
	if errors.Is(err, sql.ErrNoRows) {
		return storedPromotion{}, ErrUnknownPromotion
	}
	if err != nil {
		return storedPromotion{}, err
	}
	promo.Kind = commerce.PromotionKind(kind)
	if !promo.Active {
		return storedPromotion{}, ErrPromotionInactive
	}
	return promo, nil
}

func reservePromotion(ctx context.Context, tx executor, promo storedPromotion, orderID, email string, discountCents int) error {
	if promo.Code == "" {
		return nil
	}
	var lockedCode string
	if err := tx.QueryRowContext(ctx, `SELECT code FROM promotions WHERE code = $1 FOR UPDATE`, promo.Code).Scan(&lockedCode); err != nil {
		return err
	}
	if promo.MaxRedemptions != nil {
		var used int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM promotion_redemptions WHERE code = $1`, promo.Code).Scan(&used); err != nil {
			return err
		}
		if used >= *promo.MaxRedemptions {
			return ErrPromotionExhausted
		}
	}
	if promo.PerEmailLimit > 0 {
		var used int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM promotion_redemptions WHERE code = $1 AND customer_email = $2`, promo.Code, email).Scan(&used); err != nil {
			return err
		}
		if used >= promo.PerEmailLimit {
			return ErrPromotionLimit
		}
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO promotion_redemptions (code, order_id, customer_email, amount_cents)
		VALUES ($1, $2, $3, $4)`, promo.Code, orderID, email, discountCents)
	return err
}

func stripeCheckoutItems(quote commerce.Quote) []CheckoutLineItem {
	items := commerce.StripeItems(quote)
	converted := make([]CheckoutLineItem, len(items))
	for i, item := range items {
		converted[i] = CheckoutLineItem{Name: item.Name, AmountCents: item.AmountCents, Quantity: item.Quantity}
	}
	return converted
}
