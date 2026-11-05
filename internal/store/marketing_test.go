package store

import (
	"testing"
	"time"

	"mitte/internal/commerce"
)

func TestValidateMarketingPromotion(t *testing.T) {
	start := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	valid := MarketingPromotion{
		Code: "SUMMER15", Name: "Summer beauty", Kind: commerce.PromotionPercent,
		PercentBPS: 1500, Active: true, StartsAt: &start, EndsAt: &end,
		PublicCampaign: true, AutoApply: true, CampaignSlug: "summer-beauty",
		Headline: "A little more beauty", Description: "A local development campaign.",
		CTALabel: "Shop beauty", CategorySlug: "beauty",
	}

	if err := validateMarketingPromotion(valid); err != nil {
		t.Fatalf("valid campaign rejected: %v", err)
	}

	tests := []struct {
		name   string
		change func(*MarketingPromotion)
	}{
		{name: "invalid code", change: func(p *MarketingPromotion) { p.Code = "bad code" }},
		{name: "invalid discount", change: func(p *MarketingPromotion) { p.PercentBPS = 10001 }},
		{name: "reversed schedule", change: func(p *MarketingPromotion) { p.StartsAt, p.EndsAt = &end, &start }},
		{name: "missing campaign content", change: func(p *MarketingPromotion) { p.Headline = "" }},
		{name: "automatic without public campaign", change: func(p *MarketingPromotion) { p.PublicCampaign = false }},
		{name: "automatic with email limit", change: func(p *MarketingPromotion) { p.PerEmailLimit = 1 }},
		{name: "non-https campaign image", change: func(p *MarketingPromotion) {
			p.ImageURL = "http://example.test/image.jpg"
			p.ImageAlt = "Campaign image"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			promotion := valid
			test.change(&promotion)
			if err := validateMarketingPromotion(promotion); err != ErrInvalidPromotion {
				t.Fatalf("validateMarketingPromotion() error = %v, want ErrInvalidPromotion", err)
			}
		})
	}
}
