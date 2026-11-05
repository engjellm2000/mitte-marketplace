package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"mitte/internal/commerce"
)

var (
	promotionCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{2,39}$`)
	campaignSlugPattern  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

type MarketingPromotion struct {
	Code             string                 `json:"code"`
	Name             string                 `json:"name"`
	Kind             commerce.PromotionKind `json:"kind"`
	PercentBPS       int                    `json:"percentBps"`
	AmountCents      int                    `json:"amountCents"`
	MinSubtotalCents int                    `json:"minSubtotalCents"`
	CategorySlug     string                 `json:"categorySlug"`
	ProductSlug      string                 `json:"productSlug"`
	Active           bool                   `json:"active"`
	StartsAt         *time.Time             `json:"startsAt"`
	EndsAt           *time.Time             `json:"endsAt"`
	MaxRedemptions   *int                   `json:"maxRedemptions"`
	PerEmailLimit    int                    `json:"perEmailLimit"`
	PublicCampaign   bool                   `json:"publicCampaign"`
	AutoApply        bool                   `json:"autoApply"`
	CampaignSlug     string                 `json:"campaignSlug"`
	Headline         string                 `json:"headline"`
	Description      string                 `json:"description"`
	ImageURL         string                 `json:"imageUrl"`
	ImageAlt         string                 `json:"imageAlt"`
	CTALabel         string                 `json:"ctaLabel"`
	Featured         bool                   `json:"featured"`
	DemoOnly         bool                   `json:"demoOnly"`
}

func (s *Store) PublicMarketingPromotions(ctx context.Context) ([]MarketingPromotion, error) {
	return s.marketingPromotions(ctx, `WHERE public_campaign = TRUE AND active = TRUE
		AND (starts_at IS NULL OR starts_at <= NOW())
		AND (ends_at IS NULL OR ends_at >= NOW())
		AND (max_redemptions IS NULL OR
			(SELECT COUNT(*) FROM promotion_redemptions redemption WHERE redemption.code = promotions.code) < max_redemptions)`, true)
}

func (s *Store) AdminMarketingPromotions(ctx context.Context) ([]MarketingPromotion, error) {
	return s.marketingPromotions(ctx, "", false)
}

func (s *Store) marketingPromotions(ctx context.Context, filter string, publicOnly bool) ([]MarketingPromotion, error) {
	query := `SELECT code, name, kind, percent_bps, amount_cents, min_subtotal_cents,
		COALESCE(category_slug, ''), COALESCE(product_slug, ''), active, starts_at, ends_at,
		max_redemptions, per_email_limit,
		public_campaign, auto_apply, COALESCE(campaign_slug, ''), headline, description,
		image_url, image_alt, cta_label, featured, demo_only
		FROM promotions ` + filter
	if publicOnly {
		query += ` ORDER BY featured DESC, starts_at NULLS FIRST, code`
	} else {
		query += ` ORDER BY active DESC, starts_at NULLS FIRST, code`
	}
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	promotions := make([]MarketingPromotion, 0)
	for rows.Next() {
		promotion, err := scanMarketingPromotion(rows)
		if err != nil {
			return nil, fmt.Errorf("scan marketing promotion: %w", err)
		}
		promotions = append(promotions, promotion)
	}
	return promotions, rows.Err()
}

func (s *Store) SaveMarketingPromotion(ctx context.Context, promotion MarketingPromotion) error {
	promotion.Code = strings.ToUpper(strings.TrimSpace(promotion.Code))
	promotion.Name = strings.TrimSpace(promotion.Name)
	promotion.CategorySlug = strings.TrimSpace(promotion.CategorySlug)
	promotion.ProductSlug = strings.TrimSpace(promotion.ProductSlug)
	promotion.CampaignSlug = strings.TrimSpace(promotion.CampaignSlug)
	promotion.Headline = strings.TrimSpace(promotion.Headline)
	promotion.Description = strings.TrimSpace(promotion.Description)
	promotion.ImageURL = strings.TrimSpace(promotion.ImageURL)
	promotion.ImageAlt = strings.TrimSpace(promotion.ImageAlt)
	promotion.CTALabel = strings.TrimSpace(promotion.CTALabel)
	if err := validateMarketingPromotion(promotion); err != nil {
		return err
	}

	_, err := s.db.ExecContext(ctx, `INSERT INTO promotions (
		code, name, kind, percent_bps, amount_cents, min_subtotal_cents, category_slug, product_slug,
		active, starts_at, ends_at, max_redemptions, per_email_limit, public_campaign, auto_apply, campaign_slug,
		headline, description, image_url, image_alt, cta_label, featured, demo_only
	) VALUES (
		$1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''),
		$9, $10, $11, $12, $13, $14, $15, NULLIF($16, ''),
		$17, $18, $19, $20, $21, $22, $23
	) ON CONFLICT (code) DO UPDATE SET
		name = EXCLUDED.name, kind = EXCLUDED.kind, percent_bps = EXCLUDED.percent_bps,
		amount_cents = EXCLUDED.amount_cents, min_subtotal_cents = EXCLUDED.min_subtotal_cents,
		category_slug = EXCLUDED.category_slug, product_slug = EXCLUDED.product_slug,
		active = EXCLUDED.active, starts_at = EXCLUDED.starts_at, ends_at = EXCLUDED.ends_at,
		max_redemptions = EXCLUDED.max_redemptions, per_email_limit = EXCLUDED.per_email_limit,
		public_campaign = EXCLUDED.public_campaign, auto_apply = EXCLUDED.auto_apply,
		campaign_slug = EXCLUDED.campaign_slug, headline = EXCLUDED.headline,
		description = EXCLUDED.description, image_url = EXCLUDED.image_url,
		image_alt = EXCLUDED.image_alt, cta_label = EXCLUDED.cta_label,
		featured = EXCLUDED.featured, demo_only = EXCLUDED.demo_only`,
		promotion.Code, promotion.Name, promotion.Kind, promotion.PercentBPS, promotion.AmountCents,
		promotion.MinSubtotalCents, promotion.CategorySlug, promotion.ProductSlug, promotion.Active,
		promotion.StartsAt, promotion.EndsAt, promotion.MaxRedemptions, promotion.PerEmailLimit,
		promotion.PublicCampaign, promotion.AutoApply, promotion.CampaignSlug, promotion.Headline,
		promotion.Description, promotion.ImageURL,
		promotion.ImageAlt, promotion.CTALabel, promotion.Featured, promotion.DemoOnly)
	return err
}

func validateMarketingPromotion(promotion MarketingPromotion) error {
	if !promotionCodePattern.MatchString(promotion.Code) ||
		len(promotion.Name) < 3 || len(promotion.Name) > 120 {
		return ErrInvalidPromotion
	}
	if promotion.Kind != commerce.PromotionPercent && promotion.Kind != commerce.PromotionFixed &&
		promotion.Kind != commerce.PromotionFreeShipping {
		return ErrInvalidPromotion
	}
	if promotion.PercentBPS < 0 || promotion.PercentBPS > 10000 ||
		promotion.AmountCents < 0 || promotion.AmountCents > 100000000 ||
		promotion.MinSubtotalCents < 0 || promotion.MinSubtotalCents > 100000000 ||
		promotion.PerEmailLimit < 0 || (promotion.MaxRedemptions != nil && *promotion.MaxRedemptions < 1) {
		return ErrInvalidPromotion
	}
	switch promotion.Kind {
	case commerce.PromotionPercent:
		if promotion.PercentBPS == 0 || promotion.AmountCents != 0 {
			return ErrInvalidPromotion
		}
	case commerce.PromotionFixed:
		if promotion.AmountCents == 0 || promotion.PercentBPS != 0 {
			return ErrInvalidPromotion
		}
	case commerce.PromotionFreeShipping:
		if promotion.AmountCents != 0 || promotion.PercentBPS != 0 ||
			promotion.CategorySlug != "" || promotion.ProductSlug != "" {
			return ErrInvalidPromotion
		}
	}
	if (promotion.CategorySlug != "" && !campaignSlugPattern.MatchString(promotion.CategorySlug)) ||
		(promotion.ProductSlug != "" && !campaignSlugPattern.MatchString(promotion.ProductSlug)) {
		return ErrInvalidPromotion
	}
	if promotion.StartsAt != nil && promotion.EndsAt != nil && !promotion.StartsAt.Before(*promotion.EndsAt) {
		return ErrInvalidPromotion
	}
	if promotion.PublicCampaign {
		if !campaignSlugPattern.MatchString(promotion.CampaignSlug) ||
			len(promotion.Headline) < 3 || len(promotion.Headline) > 120 ||
			len(promotion.Description) < 5 || len(promotion.Description) > 500 ||
			len(promotion.CTALabel) < 2 || len(promotion.CTALabel) > 40 {
			return ErrInvalidPromotion
		}
		if promotion.ImageURL != "" {
			parsed, err := url.ParseRequestURI(promotion.ImageURL)
			if err != nil || parsed.Scheme != "https" || parsed.Host == "" || promotion.ImageAlt == "" {
				return ErrInvalidPromotion
			}
		} else if promotion.ImageAlt != "" {
			return ErrInvalidPromotion
		}
	}
	if promotion.AutoApply && (!promotion.PublicCampaign || promotion.PerEmailLimit > 0) {
		return ErrInvalidPromotion
	}
	return nil
}

func scanMarketingPromotion(s scanner) (MarketingPromotion, error) {
	var promotion MarketingPromotion
	var kind string
	var startsAt, endsAt sql.NullTime
	err := s.Scan(
		&promotion.Code, &promotion.Name, &kind, &promotion.PercentBPS, &promotion.AmountCents,
		&promotion.MinSubtotalCents, &promotion.CategorySlug, &promotion.ProductSlug, &promotion.Active,
		&startsAt, &endsAt, &promotion.MaxRedemptions, &promotion.PerEmailLimit,
		&promotion.PublicCampaign, &promotion.AutoApply, &promotion.CampaignSlug,
		&promotion.Headline, &promotion.Description, &promotion.ImageURL, &promotion.ImageAlt,
		&promotion.CTALabel, &promotion.Featured, &promotion.DemoOnly,
	)
	if err != nil {
		return MarketingPromotion{}, err
	}
	promotion.Kind = commerce.PromotionKind(kind)
	if startsAt.Valid {
		promotion.StartsAt = &startsAt.Time
	}
	if endsAt.Valid {
		promotion.EndsAt = &endsAt.Time
	}
	return promotion, nil
}

func (s *Store) bestAutomaticPromotion(ctx context.Context, q queryer, lines []commerce.Line) (*commerce.Promotion, error) {
	if len(lines) == 0 {
		return nil, nil
	}
	const query = `SELECT code, name, kind, percent_bps, amount_cents, min_subtotal_cents,
		COALESCE(category_slug, ''), COALESCE(product_slug, ''), active
		FROM promotions
		WHERE active = TRUE AND public_campaign = TRUE AND auto_apply = TRUE
			AND (starts_at IS NULL OR starts_at <= NOW())
			AND (ends_at IS NULL OR ends_at >= NOW())
			AND (max_redemptions IS NULL OR
				(SELECT COUNT(*) FROM promotion_redemptions redemption WHERE redemption.code = promotions.code) < max_redemptions)
		ORDER BY code`
	rows, err := q.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bestTotal := int(^uint(0) >> 1)
	var best *commerce.Promotion
	for rows.Next() {
		var candidate commerce.Promotion
		var kind string
		if err := rows.Scan(
			&candidate.Code, &candidate.Name, &kind, &candidate.PercentBPS, &candidate.AmountCents,
			&candidate.MinSubtotalCents, &candidate.CategorySlug, &candidate.ProductSlug, &candidate.Active,
		); err != nil {
			return nil, err
		}
		candidate.Kind = commerce.PromotionKind(kind)
		quote, err := commerce.ComputeQuote(lines, &candidate)
		if err != nil {
			return nil, err
		}
		if (quote.PromotionStatus == "applied" || quote.PromotionStatus == "applied_shipping") && quote.TotalCents < bestTotal {
			copy := candidate
			best = &copy
			bestTotal = quote.TotalCents
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return best, nil
}

func (s *Store) PublicMarketingPromotion(ctx context.Context, slug string) (MarketingPromotion, error) {
	promotions, err := s.PublicMarketingPromotions(ctx)
	if err != nil {
		return MarketingPromotion{}, err
	}
	for _, promotion := range promotions {
		if promotion.CampaignSlug == slug {
			return promotion, nil
		}
	}
	return MarketingPromotion{}, ErrNotFound
}
