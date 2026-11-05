package web

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"mitte/internal/commerce"
	"mitte/internal/store"
)

type publicOffer struct {
	Code             string                 `json:"code,omitempty"`
	Name             string                 `json:"name"`
	Kind             commerce.PromotionKind `json:"kind"`
	PercentBPS       int                    `json:"percentBps"`
	AmountCents      int                    `json:"amountCents"`
	MinSubtotalCents int                    `json:"minSubtotalCents"`
	CategorySlug     string                 `json:"categorySlug,omitempty"`
	ProductSlug      string                 `json:"productSlug,omitempty"`
	AutoApply        bool                   `json:"autoApply"`
	CampaignSlug     string                 `json:"campaignSlug"`
	Headline         string                 `json:"headline"`
	Description      string                 `json:"description"`
	ImageURL         string                 `json:"imageUrl,omitempty"`
	ImageAlt         string                 `json:"imageAlt,omitempty"`
	CTALabel         string                 `json:"ctaLabel"`
	Featured         bool                   `json:"featured"`
	DemoOnly         bool                   `json:"demoOnly"`
	StartsAt         *time.Time             `json:"startsAt,omitempty"`
	EndsAt           *time.Time             `json:"endsAt,omitempty"`
}

func toPublicOffer(promotion store.MarketingPromotion) publicOffer {
	offer := publicOffer{
		Name: promotion.Name, Kind: promotion.Kind, PercentBPS: promotion.PercentBPS,
		AmountCents: promotion.AmountCents, MinSubtotalCents: promotion.MinSubtotalCents,
		CategorySlug: promotion.CategorySlug, ProductSlug: promotion.ProductSlug,
		AutoApply: promotion.AutoApply, CampaignSlug: promotion.CampaignSlug,
		Headline: promotion.Headline, Description: promotion.Description, ImageURL: promotion.ImageURL,
		ImageAlt: promotion.ImageAlt, CTALabel: promotion.CTALabel, Featured: promotion.Featured,
		DemoOnly: promotion.DemoOnly,
	}
	if !promotion.AutoApply {
		offer.Code = promotion.Code
	}
	if promotion.StartsAt != nil {
		offer.StartsAt = promotion.StartsAt
	}
	if promotion.EndsAt != nil {
		offer.EndsAt = promotion.EndsAt
	}
	return offer
}

func (a *App) publicOffers(w http.ResponseWriter, r *http.Request) {
	promotions, err := a.store.PublicMarketingPromotions(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	offers := make([]publicOffer, 0, len(promotions))
	for _, promotion := range promotions {
		offers = append(offers, toPublicOffer(promotion))
	}
	setPublicCatalogCache(w)
	respondJSON(w, http.StatusOK, map[string]any{"offers": offers})
}

func (a *App) publicOffer(w http.ResponseWriter, r *http.Request) {
	promotion, err := a.store.PublicMarketingPromotion(r.Context(), r.PathValue("slug"))
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Angebot nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	setPublicCatalogCache(w)
	respondJSON(w, http.StatusOK, map[string]any{"offer": toPublicOffer(promotion)})
}

func (a *App) adminPromotions(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	promotions, err := a.store.AdminMarketingPromotions(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{"promotions": promotions})
}

func (a *App) createAdminPromotion(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	a.saveAdminPromotion(w, r, "")
}

func (a *App) updateAdminPromotion(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	a.saveAdminPromotion(w, r, strings.ToUpper(strings.TrimSpace(r.PathValue("code"))))
}

func (a *App) saveAdminPromotion(w http.ResponseWriter, r *http.Request, code string) {
	var promotion store.MarketingPromotion
	if err := decodeJSON(w, r, &promotion, 16*1024); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Angebotsdaten."})
		return
	}
	if code != "" {
		if promotion.Code != "" && !strings.EqualFold(promotion.Code, code) {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Der Angebotscode kann nicht geändert werden."})
			return
		}
		promotion.Code = code
	}
	if err := a.store.SaveMarketingPromotion(r.Context(), promotion); errors.Is(err, store.ErrInvalidPromotion) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe Angebotscode, Rabatt, Laufzeit und Kampagnenangaben."})
		return
	} else if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{"promotion": promotion})
}
