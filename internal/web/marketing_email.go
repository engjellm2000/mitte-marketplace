package web

import (
	"errors"
	"net/http"
	"strconv"

	"mitte/internal/store"
)

func (a *App) adminMarketingEmailCampaigns(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	campaigns, audienceSize, err := a.store.MarketingEmailCampaignsForAdmin(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{
		"campaigns":    campaigns,
		"audienceSize": audienceSize,
	})
}

func (a *App) saveAdminMarketingEmailCampaign(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	var campaign store.MarketingEmailCampaign
	if err := decodeJSON(w, r, &campaign, 16*1024); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige E-Mail-Kampagne."})
		return
	}
	if id := r.PathValue("id"); id != "" {
		parsedID, err := strconv.ParseInt(id, 10, 64)
		if err != nil || parsedID < 1 {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Kampagnen-ID."})
			return
		}
		campaign.ID = parsedID
	}
	saved, err := a.store.SaveMarketingEmailCampaign(r.Context(), campaign, user.Email)
	if errors.Is(err, store.ErrInvalidMarketingEmailCampaign) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe Name, Betreff, Vorschautext und Nachrichtentext."})
		return
	}
	if errors.Is(err, store.ErrMarketingEmailCampaignLocked) {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Diese Kampagne ist nicht mehr als Entwurf bearbeitbar."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{"campaign": saved})
}

func (a *App) sendAdminMarketingEmailCampaign(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Kampagnen-ID."})
		return
	}
	var input struct {
		ExpectedAudienceSize int `json:"expectedAudienceSize"`
	}
	if err := decodeJSON(w, r, &input, 256); err != nil || input.ExpectedAudienceSize < 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte bestätige die angezeigte Empfängerzahl."})
		return
	}
	queued, err := a.store.QueueMarketingEmailCampaign(r.Context(), id, a.invoicePublicBaseURL, input.ExpectedAudienceSize)
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Kampagne nicht gefunden."})
		return
	}
	if errors.Is(err, store.ErrMarketingEmailCampaignLocked) {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Diese Kampagne wurde bereits in die Warteschlange gestellt."})
		return
	}
	if errors.Is(err, store.ErrMarketingEmailAudienceChanged) {
		respondJSON(w, http.StatusConflict, map[string]any{
			"error":        "Die Empfängerliste hat sich geändert. Prüfe die neue Zahl und bestätige den Versand erneut.",
			"audienceSize": queued,
		})
		return
	}
	if errors.Is(err, store.ErrInvalidMarketingEmailCampaign) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte ergänze Betreff und Nachricht, bevor du versendest."})
		return
	}
	if errors.Is(err, store.ErrInvalidEmailPublicURL) {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Für E-Mail-Abmeldelinks ist keine gültige öffentliche Shop-Adresse eingerichtet."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusAccepted, map[string]any{
		"queued":  queued,
		"message": strconv.Itoa(queued) + " E-Mails wurden in die Zustellwarteschlange gestellt.",
	})
}
