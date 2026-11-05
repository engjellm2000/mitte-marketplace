package commerce

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

const (
	VATRateBPS                 = 1900
	VATDivisor                 = 119
	FreeShippingThresholdCents = 7500
	StandardShippingCents      = 490
	MaxLineQuantity            = 10
	MinChargeableTotalCents    = 50
)

type PromotionKind string

const (
	PromotionPercent      PromotionKind = "percent"
	PromotionFixed        PromotionKind = "fixed"
	PromotionFreeShipping PromotionKind = "free_shipping"
)

type Promotion struct {
	Code             string
	Name             string
	Kind             PromotionKind
	PercentBPS       int
	AmountCents      int
	MinSubtotalCents int
	CategorySlug     string
	ProductSlug      string
	Active           bool
}

type Line struct {
	ProductID      int64
	Name           string
	CategorySlug   string
	CategorySlugs  []string `json:"-"`
	ProductSlug    string
	UnitPriceCents int
	Quantity       int
}

type QuotedLine struct {
	ProductID      int64
	Name           string
	CategorySlug   string
	CategorySlugs  []string `json:"-"`
	ProductSlug    string
	UnitPriceCents int
	Quantity       int
	LineTotalCents int
	DiscountCents  int
	NetCents       int
}

type Quote struct {
	Lines                         []QuotedLine
	SubtotalCents                 int
	DiscountCents                 int
	ShippingCents                 int
	TaxCents                      int
	TotalCents                    int
	PromotionCode                 string
	DiscountLabel                 string
	ShippingLabel                 string
	PromotionStatus               string
	FreeShippingUnlocked          bool
	RemainingForFreeShippingCents int
}

func EmptyQuote() Quote {
	return Quote{
		Lines:           make([]QuotedLine, 0),
		ShippingLabel:   "Warenkorb leer",
		PromotionStatus: "none",
	}
}

func ComputeQuote(lines []Line, promo *Promotion) (Quote, error) {
	if len(lines) == 0 {
		return EmptyQuote(), nil
	}

	quoted := make([]QuotedLine, len(lines))
	subtotal := 0
	for i, line := range lines {
		if line.Quantity < 1 || line.Quantity > MaxLineQuantity {
			return Quote{}, fmt.Errorf("invalid quantity for %s", line.Name)
		}
		if line.UnitPriceCents < 0 {
			return Quote{}, fmt.Errorf("invalid price for %s", line.Name)
		}
		lineTotal := line.UnitPriceCents * line.Quantity
		quoted[i] = QuotedLine{
			ProductID:      line.ProductID,
			Name:           line.Name,
			CategorySlug:   line.CategorySlug,
			CategorySlugs:  slices.Clone(line.CategorySlugs),
			ProductSlug:    line.ProductSlug,
			UnitPriceCents: line.UnitPriceCents,
			Quantity:       line.Quantity,
			LineTotalCents: lineTotal,
			NetCents:       lineTotal,
		}
		subtotal += lineTotal
	}

	quote := Quote{
		Lines:                         quoted,
		SubtotalCents:                 subtotal,
		PromotionStatus:               "none",
		ShippingLabel:                 "Standardversand",
		RemainingForFreeShippingCents: max(0, FreeShippingThresholdCents-subtotal),
	}

	freeShippingCoupon := false
	if promo != nil && strings.TrimSpace(promo.Code) != "" {
		applyPromotion(&quote, *promo)
		freeShippingCoupon = promo.Kind == PromotionFreeShipping && (quote.PromotionStatus == "applied" || quote.PromotionStatus == "applied_shipping")
	}

	merchandiseNet := quote.SubtotalCents - quote.DiscountCents
	shipping := StandardShippingCents
	if freeShippingCoupon || merchandiseNet >= FreeShippingThresholdCents {
		shipping = 0
		quote.ShippingLabel = "Versandkostenfrei"
		quote.FreeShippingUnlocked = true
		quote.RemainingForFreeShippingCents = 0
	} else {
		quote.RemainingForFreeShippingCents = FreeShippingThresholdCents - merchandiseNet
	}
	quote.ShippingCents = shipping
	quote.TotalCents = merchandiseNet + shipping
	quote.TaxCents = VATIncludedCents(quote.TotalCents)
	return quote, nil
}

func ReconstructQuote(lines []QuotedLine, discountCents, shippingCents int, promotionCode, discountLabel string) Quote {
	subtotal := 0
	copied := make([]QuotedLine, len(lines))
	for i, line := range lines {
		copied[i] = line
		copied[i].LineTotalCents = line.UnitPriceCents * line.Quantity
		copied[i].DiscountCents = 0
		copied[i].NetCents = copied[i].LineTotalCents
		subtotal += copied[i].LineTotalCents
	}
	if discountCents < 0 {
		discountCents = 0
	}
	if discountCents > subtotal {
		discountCents = subtotal
	}
	allocateDiscount(copied, discountCents, func(QuotedLine) bool { return true })
	if shippingCents < 0 {
		shippingCents = 0
	}
	quote := Quote{
		Lines:           copied,
		SubtotalCents:   subtotal,
		DiscountCents:   discountCents,
		ShippingCents:   shippingCents,
		TotalCents:      subtotal - discountCents + shippingCents,
		PromotionCode:   promotionCode,
		DiscountLabel:   discountLabel,
		PromotionStatus: "none",
		ShippingLabel:   "Standardversand",
	}
	if discountCents > 0 {
		quote.PromotionStatus = "applied"
	}
	if shippingCents == 0 {
		quote.ShippingLabel = "Versandkostenfrei"
		quote.FreeShippingUnlocked = true
	}
	quote.TaxCents = VATIncludedCents(quote.TotalCents)
	return quote
}

func VATIncludedCents(grossCents int) int {
	if grossCents <= 0 {
		return 0
	}
	return (grossCents*VATRateBPS/100 + VATDivisor/2) / VATDivisor
}

func StripeItems(quote Quote) []StripeItem {
	items := make([]StripeItem, 0, len(quote.Lines)+1)
	for _, line := range quote.Lines {
		if line.NetCents <= 0 {
			continue
		}
		if line.NetCents%line.Quantity == 0 {
			items = append(items, StripeItem{Name: line.Name, AmountCents: int64(line.NetCents / line.Quantity), Quantity: int64(line.Quantity)})
			continue
		}
		items = append(items, StripeItem{
			Name:        fmt.Sprintf("%d × %s", line.Quantity, line.Name),
			AmountCents: int64(line.NetCents),
			Quantity:    1,
		})
	}
	if quote.ShippingCents > 0 {
		items = append(items, StripeItem{Name: "Versand", AmountCents: int64(quote.ShippingCents), Quantity: 1})
	}
	return items
}

type StripeItem struct {
	Name        string
	AmountCents int64
	Quantity    int64
}

func Chargeable(quote Quote) bool {
	return quote.TotalCents >= MinChargeableTotalCents && len(StripeItems(quote)) > 0
}

func applyPromotion(quote *Quote, promo Promotion) {
	quote.PromotionCode = strings.ToUpper(strings.TrimSpace(promo.Code))
	quote.DiscountLabel = promo.Name
	if !promo.Active {
		quote.PromotionStatus = "inactive"
		return
	}

	eligible := 0
	for _, line := range quote.Lines {
		if lineEligible(line, promo) {
			eligible += line.LineTotalCents
		}
	}
	if eligible == 0 {
		quote.PromotionStatus = "ineligible"
		return
	}
	if eligible < promo.MinSubtotalCents {
		quote.PromotionStatus = "below_minimum"
		return
	}

	switch promo.Kind {
	case PromotionPercent:
		if promo.PercentBPS <= 0 || promo.PercentBPS > 10000 {
			quote.PromotionStatus = "inactive"
			return
		}
		discount := int(math.Round(float64(eligible) * float64(promo.PercentBPS) / 10000))
		if discount > eligible {
			discount = eligible
		}
		allocateDiscount(quote.Lines, discount, func(line QuotedLine) bool { return lineEligible(line, promo) })
		quote.DiscountCents = discount
		quote.PromotionStatus = "applied"
	case PromotionFixed:
		discount := promo.AmountCents
		if discount <= 0 {
			quote.PromotionStatus = "inactive"
			return
		}
		if discount > eligible {
			discount = eligible
		}
		allocateDiscount(quote.Lines, discount, func(line QuotedLine) bool { return lineEligible(line, promo) })
		quote.DiscountCents = discount
		quote.PromotionStatus = "applied"
	case PromotionFreeShipping:
		quote.PromotionStatus = "applied_shipping"
	default:
		quote.PromotionStatus = "inactive"
	}
}

func lineEligible(line QuotedLine, promo Promotion) bool {
	if promo.ProductSlug != "" && line.ProductSlug != promo.ProductSlug {
		return false
	}
	if promo.CategorySlug != "" && line.CategorySlug != promo.CategorySlug && !slices.Contains(line.CategorySlugs, promo.CategorySlug) {
		return false
	}
	return true
}

func allocateDiscount(lines []QuotedLine, discount int, eligible func(QuotedLine) bool) {
	if discount <= 0 {
		return
	}
	totalEligible := 0
	indexes := make([]int, 0, len(lines))
	for i, line := range lines {
		if eligible(line) && line.LineTotalCents > 0 {
			indexes = append(indexes, i)
			totalEligible += line.LineTotalCents
		}
	}
	if totalEligible == 0 {
		return
	}

	remaining := discount
	type remainder struct {
		index int
		frac  int
	}
	leftovers := make([]remainder, 0, len(indexes))
	for _, i := range indexes {
		share := discount * lines[i].LineTotalCents / totalEligible
		lines[i].DiscountCents = share
		remaining -= share
		leftovers = append(leftovers, remainder{index: i, frac: (discount * lines[i].LineTotalCents) % totalEligible})
	}
	for remaining > 0 {
		best := 0
		for i := 1; i < len(leftovers); i++ {
			if leftovers[i].frac > leftovers[best].frac {
				best = i
			}
		}
		lines[leftovers[best].index].DiscountCents++
		leftovers[best].frac = -1
		remaining--
	}
	for i := range lines {
		if lines[i].DiscountCents > lines[i].LineTotalCents {
			lines[i].DiscountCents = lines[i].LineTotalCents
		}
		lines[i].NetCents = lines[i].LineTotalCents - lines[i].DiscountCents
	}
}
