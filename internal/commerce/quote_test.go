package commerce

import "testing"

func TestComputeQuoteFreeShippingThreshold(t *testing.T) {
	quote, err := ComputeQuote([]Line{{Name: "Mori", UnitPriceCents: 12900, Quantity: 1, CategorySlug: "wohnen"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if quote.ShippingCents != 0 || quote.TotalCents != 12900 || !quote.FreeShippingUnlocked {
		t.Fatalf("quote = %#v", quote)
	}
	if quote.TaxCents != VATIncludedCents(12900) {
		t.Fatalf("tax = %d", quote.TaxCents)
	}
}

func TestComputeQuoteStandardShipping(t *testing.T) {
	quote, err := ComputeQuote([]Line{{Name: "Tasse", UnitPriceCents: 4200, Quantity: 1, CategorySlug: "tisch"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if quote.ShippingCents != StandardShippingCents || quote.TotalCents != 4200+StandardShippingCents {
		t.Fatalf("quote = %#v", quote)
	}
	if quote.RemainingForFreeShippingCents != FreeShippingThresholdCents-4200 {
		t.Fatalf("remaining = %d", quote.RemainingForFreeShippingCents)
	}
}

func TestPercentPromotionRoundsAndAllocatesExactly(t *testing.T) {
	promo := &Promotion{Code: "MORGEN10", Name: "10% auf alles", Kind: PromotionPercent, PercentBPS: 1000, Active: true}
	quote, err := ComputeQuote([]Line{
		{Name: "A", ProductID: 1, UnitPriceCents: 1999, Quantity: 1, CategorySlug: "tisch"},
		{Name: "B", ProductID: 2, UnitPriceCents: 3300, Quantity: 1, CategorySlug: "wohnen"},
	}, promo)
	if err != nil {
		t.Fatal(err)
	}
	if quote.DiscountCents != 530 {
		t.Fatalf("discount = %d, want 530", quote.DiscountCents)
	}
	allocated := 0
	for _, line := range quote.Lines {
		allocated += line.DiscountCents
		if line.NetCents != line.LineTotalCents-line.DiscountCents {
			t.Fatalf("line net mismatch: %#v", line)
		}
	}
	if allocated != quote.DiscountCents {
		t.Fatalf("allocated %d, discount %d", allocated, quote.DiscountCents)
	}
	if quote.PromotionStatus != "applied" {
		t.Fatalf("status = %s", quote.PromotionStatus)
	}
}

func TestFixedPromotionRespectsMinimumAndCap(t *testing.T) {
	promo := &Promotion{Code: "WILLKOMMEN", Name: "8 Euro", Kind: PromotionFixed, AmountCents: 800, MinSubtotalCents: 4000, Active: true}
	tooSmall, err := ComputeQuote([]Line{{Name: "Tasche", UnitPriceCents: 3900, Quantity: 1}}, promo)
	if err != nil {
		t.Fatal(err)
	}
	if tooSmall.DiscountCents != 0 || tooSmall.PromotionStatus != "below_minimum" {
		t.Fatalf("small cart quote = %#v", tooSmall)
	}

	applied, err := ComputeQuote([]Line{{Name: "Brett", UnitPriceCents: 5600, Quantity: 1}}, promo)
	if err != nil {
		t.Fatal(err)
	}
	if applied.DiscountCents != 800 || applied.Lines[0].NetCents != 4800 {
		t.Fatalf("applied = %#v", applied)
	}
}

func TestCategoryPromotionDoesNotDiscountOtherLines(t *testing.T) {
	promo := &Promotion{Code: "TISCH15", Name: "15% Tisch", Kind: PromotionPercent, PercentBPS: 1500, CategorySlug: "tisch", Active: true}
	quote, err := ComputeQuote([]Line{
		{Name: "Brett", UnitPriceCents: 5600, Quantity: 1, CategorySlug: "tisch", ProductSlug: "forma-schneidebrett"},
		{Name: "Decke", UnitPriceCents: 9800, Quantity: 1, CategorySlug: "textil", ProductSlug: "fjord-wolldecke"},
	}, promo)
	if err != nil {
		t.Fatal(err)
	}
	if quote.DiscountCents != 840 {
		t.Fatalf("discount = %d", quote.DiscountCents)
	}
	if quote.Lines[0].DiscountCents != 840 || quote.Lines[1].DiscountCents != 0 {
		t.Fatalf("lines = %#v", quote.Lines)
	}
}

func TestCategoryPromotionIncludesDescendants(t *testing.T) {
	lines := []Line{{
		ProductID: 1, Name: "Beauty product", CategorySlug: "hair-care",
		CategorySlugs: []string{"hair-care", "beauty"}, ProductSlug: "shampoo",
		UnitPriceCents: 1000, Quantity: 1,
	}}
	promotion := &Promotion{
		Code: "BEAUTY15", Name: "Beauty campaign", Kind: PromotionPercent,
		PercentBPS: 1500, CategorySlug: "beauty", Active: true,
	}
	quote, err := ComputeQuote(lines, promotion)
	if err != nil {
		t.Fatal(err)
	}
	if quote.DiscountCents != 150 {
		t.Fatalf("discount = %d, want 150", quote.DiscountCents)
	}
}

func TestFreeShippingCouponWaivesFeeBelowThreshold(t *testing.T) {
	promo := &Promotion{Code: "FREIVERSAND", Name: "Gratis Versand", Kind: PromotionFreeShipping, Active: true}
	quote, err := ComputeQuote([]Line{{Name: "Tasse", UnitPriceCents: 4200, Quantity: 1}}, promo)
	if err != nil {
		t.Fatal(err)
	}
	if quote.ShippingCents != 0 || quote.TotalCents != 4200 || quote.PromotionStatus != "applied_shipping" {
		t.Fatalf("quote = %#v", quote)
	}
}

func TestPercentDiscountCanUnlockFreeShipping(t *testing.T) {
	lines := []Line{{Name: "Mix", UnitPriceCents: 7800, Quantity: 1}}
	before, _ := ComputeQuote(lines, nil)
	if before.ShippingCents != 0 {
		t.Fatalf("expected free shipping before discount, got %d", before.ShippingCents)
	}
	cut := &Promotion{Code: "BIG", Name: "20%", Kind: PromotionPercent, PercentBPS: 2000, Active: true}
	after, err := ComputeQuote(lines, cut)
	if err != nil {
		t.Fatal(err)
	}
	if after.DiscountCents != 1560 || after.ShippingCents != StandardShippingCents {
		t.Fatalf("discount should drop cart below free-shipping threshold: %#v", after)
	}
}

func TestStripeItemsBakeInDiscountAndShipping(t *testing.T) {
	quote, err := ComputeQuote([]Line{{Name: "Vase", UnitPriceCents: 6800, Quantity: 2}}, &Promotion{
		Code: "WILLKOMMEN", Name: "8 Euro", Kind: PromotionFixed, AmountCents: 800, MinSubtotalCents: 4000, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	items := StripeItems(quote)
	sum := int64(0)
	for _, item := range items {
		sum += item.AmountCents * item.Quantity
	}
	if sum != int64(quote.TotalCents) {
		t.Fatalf("stripe sum %d != total %d (%#v)", sum, quote.TotalCents, items)
	}
}

func TestReconstructQuotePreservesSnapshot(t *testing.T) {
	original, _ := ComputeQuote([]Line{{Name: "Vase", UnitPriceCents: 6800, Quantity: 1, ProductID: 4}}, &Promotion{
		Code: "MORGEN10", Name: "10%", Kind: PromotionPercent, PercentBPS: 1000, Active: true,
	})
	restored := ReconstructQuote(original.Lines, original.DiscountCents, original.ShippingCents, original.PromotionCode, original.DiscountLabel)
	if restored.TotalCents != original.TotalCents || restored.TaxCents != original.TaxCents || restored.Lines[0].NetCents != original.Lines[0].NetCents {
		t.Fatalf("restored %#v vs original %#v", restored, original)
	}
}

func TestVATIncludedKnownAmounts(t *testing.T) {
	if got := VATIncludedCents(11900); got != 1900 {
		t.Fatalf("119€ VAT = %d", got)
	}
	if VATIncludedCents(0) != 0 {
		t.Fatal("zero VAT")
	}
}
