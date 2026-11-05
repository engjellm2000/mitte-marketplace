package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"mitte/internal/commerce"
)

type RecommendationFeed struct {
	Recommendations []ProductSummary
	RecentlyViewed  []ProductSummary
}

func (s *Store) SetRecommendationConsent(ctx context.Context, userID int64, enabled bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin recommendation consent update: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `UPDATE users
		SET recommendation_consent = $2,
		    recommendation_consent_at = CASE WHEN $2 THEN NOW() ELSE NULL END
		WHERE id = $1`, userID, enabled)
	if err != nil {
		return fmt.Errorf("update recommendation consent: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read recommendation consent update: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	if !enabled {
		if _, err := tx.ExecContext(ctx, `DELETE FROM customer_product_views WHERE user_id = $1`, userID); err != nil {
			return fmt.Errorf("delete recommendation history: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit recommendation consent update: %w", err)
	}
	return nil
}

func (s *Store) RecordProductView(ctx context.Context, userID int64, slug string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin product view record: %w", err)
	}
	defer tx.Rollback()

	var consent bool
	if err := tx.QueryRowContext(ctx, `SELECT recommendation_consent FROM users WHERE id = $1 FOR SHARE`, userID).Scan(&consent); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("check recommendation consent: %w", err)
	}
	if !consent {
		return ErrRecommendationConsentRequired
	}

	result, err := tx.ExecContext(ctx, `INSERT INTO customer_product_views (user_id, product_id, viewed_at)
		SELECT $1, p.id, NOW()
		FROM products p
		WHERE p.slug = $2 AND p.status = 'active'
		ON CONFLICT (user_id, product_id) DO UPDATE SET viewed_at = EXCLUDED.viewed_at`,
		userID, slug)
	if err != nil {
		return fmt.Errorf("record opted-in product view: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read product view result: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM customer_product_views
		WHERE user_id = $1 AND (
			viewed_at < NOW() - INTERVAL '180 days'
			OR product_id NOT IN (
				SELECT product_id FROM customer_product_views
				WHERE user_id = $1 ORDER BY viewed_at DESC LIMIT 50
			)
		)`, userID); err != nil {
		return fmt.Errorf("prune recommendation history: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit product view record: %w", err)
	}
	return nil
}

func (s *Store) Recommendations(ctx context.Context, userID int64) (RecommendationFeed, error) {
	var feed RecommendationFeed
	var consent bool
	if err := s.db.QueryRowContext(ctx, `SELECT recommendation_consent FROM users WHERE id = $1`, userID).Scan(&consent); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return feed, ErrNotFound
		}
		return feed, fmt.Errorf("read recommendation consent: %w", err)
	}
	if !consent {
		return feed, nil
	}

	var err error
	feed.RecentlyViewed, err = s.recentlyViewed(ctx, userID)
	if err != nil {
		return RecommendationFeed{}, err
	}
	feed.Recommendations, err = s.recommendedProducts(ctx, userID)
	if err != nil {
		return RecommendationFeed{}, err
	}
	return feed, nil
}

func (s *Store) recentlyViewed(ctx context.Context, userID int64) ([]ProductSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.id, p.slug, p.name, p.short_description, p.price_cents,
		p.image_url, p.image_alt, p.inventory, c.id, c.slug, c.name, c.description, p.featured, 0::bigint,
		p.sku LIKE 'MITTE-DEMO-%'
		FROM customer_product_views v
		JOIN products p ON p.id = v.product_id AND p.status = 'active'
		JOIN categories c ON c.id = p.category_id
		WHERE v.user_id = $1 AND v.viewed_at >= NOW() - INTERVAL '180 days'
		ORDER BY v.viewed_at DESC LIMIT 8`, userID)
	if err != nil {
		return nil, fmt.Errorf("query recently viewed products: %w", err)
	}
	defer rows.Close()
	return scanProductSummaries(rows)
}

func (s *Store) recommendedProducts(ctx context.Context, userID int64) ([]ProductSummary, error) {
	rows, err := s.db.QueryContext(ctx, `WITH interest AS (
			SELECT p.category_id, SUM(oi.quantity * 3)::bigint AS score
			FROM orders o
			JOIN order_items oi ON oi.order_id = o.id AND oi.product_id IS NOT NULL
			JOIN products p ON p.id = oi.product_id
			WHERE o.customer_id = $1 AND o.status IN ('paid', 'fulfilled')
			GROUP BY p.category_id
			UNION ALL
			SELECT p.category_id, COUNT(*)::bigint AS score
			FROM customer_product_views v
			JOIN products p ON p.id = v.product_id
			WHERE v.user_id = $1 AND v.viewed_at >= NOW() - INTERVAL '180 days'
			GROUP BY p.category_id
		), category_scores AS (
			SELECT category_id, SUM(score)::bigint AS score
			FROM interest GROUP BY category_id
		), sales AS (
			SELECT oi.product_id, SUM(oi.quantity)::bigint AS units_sold
			FROM order_items oi JOIN orders o ON o.id = oi.order_id
			WHERE oi.product_id IS NOT NULL AND o.status IN ('paid', 'fulfilled')
			GROUP BY oi.product_id
		)
		SELECT p.id, p.slug, p.name, p.short_description, p.price_cents,
			p.image_url, p.image_alt, p.inventory, c.id, c.slug, c.name, c.description, p.featured,
			COALESCE(sales.units_sold, 0)::bigint, p.sku LIKE 'MITTE-DEMO-%'
		FROM products p
		JOIN categories c ON c.id = p.category_id
		JOIN category_scores preference ON preference.category_id = p.category_id
		LEFT JOIN sales ON sales.product_id = p.id
		WHERE p.status = 'active' AND p.inventory > 0
			AND NOT EXISTS (
				SELECT 1 FROM customer_product_views v
				WHERE v.user_id = $1 AND v.product_id = p.id
			)
		ORDER BY preference.score DESC, COALESCE(sales.units_sold, 0) DESC,
			p.featured DESC, p.created_at DESC, p.id
		LIMIT 8`, userID)
	if err != nil {
		return nil, fmt.Errorf("query personalized products: %w", err)
	}
	defer rows.Close()
	return scanProductSummaries(rows)
}

func scanProductSummaries(rows *sql.Rows) ([]ProductSummary, error) {
	products := make([]ProductSummary, 0)
	for rows.Next() {
		var product ProductSummary
		if err := rows.Scan(
			&product.ID, &product.Slug, &product.Name, &product.ShortDescription, &product.PriceCents,
			&product.ImageURL, &product.ImageAlt, &product.Inventory, &product.Category.ID,
			&product.Category.Slug, &product.Category.Name, &product.Category.Description, &product.Featured,
			&product.SalesCount, &product.Demo,
		); err != nil {
			return nil, fmt.Errorf("scan recommendation product: %w", err)
		}
		product.PriceGrossCents = product.PriceCents
		product.PriceNetCents = product.PriceCents - commerce.VATIncludedCents(product.PriceCents)
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return products, nil
}
