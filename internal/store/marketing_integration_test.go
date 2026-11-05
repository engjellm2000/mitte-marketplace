package store

import (
	"context"
	"os"
	"testing"

	"mitte/internal/commerce"
)

func TestMarketingCatalogQueriesWhenConfigured(t *testing.T) {
	databaseURL := os.Getenv("INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set INTEGRATION_DATABASE_URL to run read-only marketing query tests")
	}
	ctx := context.Background()
	repository, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	if err := repository.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	offers, err := repository.PublicMarketingPromotions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	targetedOffer := false
	for _, offer := range offers {
		if !offer.AutoApply || offer.CategorySlug != "beauty" {
			continue
		}
		targetedOffer = true
		for offset := 0; ; offset += 100 {
			products, hasMore, err := repository.ProductsPage(ctx, "beauty", "", false, "featured", 100, offset)
			if err != nil {
				t.Fatal(err)
			}
			for _, product := range products {
				if product.SalePriceCents <= 0 || product.OfferCode != offer.Code {
					continue
				}
				detail, err := repository.ProductBySlug(ctx, product.Slug)
				if err != nil {
					t.Fatal(err)
				}
				if detail.SalePriceCents != product.SalePriceCents || detail.OfferCode != offer.Code {
					t.Fatalf("product listing and detail offer prices differ: listing=%#v detail=%#v", product, detail)
				}
				lines := []commerce.Line{{
					ProductID: product.ID, Name: product.Name, CategorySlug: product.Category.Slug,
					CategorySlugs: []string{product.Category.Slug, offer.CategorySlug},
					ProductSlug:   product.Slug, UnitPriceCents: product.PriceCents, Quantity: 1,
				}}
				promotion, err := repository.bestAutomaticPromotion(ctx, repository.db, lines)
				if err != nil {
					t.Fatal(err)
				}
				if promotion == nil || promotion.Code != offer.Code {
					t.Fatalf("expected automatic campaign %q for %s, got %#v", offer.Code, product.Slug, promotion)
				}
				return
			}
			if !hasMore {
				break
			}
		}
	}
	if targetedOffer {
		t.Fatal("active beauty campaign did not produce a price offer on any beauty product")
	}
	t.Skip("no active auto-applied beauty campaign is configured")
}
