package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"mitte/internal/store"
)

func (a *App) warehouseReturns(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	query := r.URL.Query()
	status := query.Get("status")
	if status == "" {
		status = "all"
	}
	search := strings.TrimSpace(query.Get("q"))
	if !validWarehouseReturnStatus(status) || len([]rune(search)) > 120 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe Status und Suchbegriff."})
		return
	}
	page, limit, offset, err := parseWarehouseQueuePage(query.Get("page"), query.Get("limit"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Retourenseite."})
		return
	}
	cases, total, err := a.store.WarehouseReturns(r.Context(), status, search, limit, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	pages := (total + limit - 1) / limit
	if pages < 1 {
		pages = 1
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{"returns": cases, "total": total, "page": page, "pages": pages})
}

func (a *App) receiveWarehouseReturn(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var input struct {
		Quantity        int    `json:"quantity"`
		ParcelReference string `json:"parcelReference"`
	}
	if err != nil || decodeJSON(w, r, &input, 2048) != nil || len([]rune(input.ParcelReference)) > 120 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe die Retourennummer und Paketreferenz."})
		return
	}
	if err := a.store.ReceiveWarehouseReturn(r.Context(), id, input.Quantity, input.ParcelReference, user.Email); err != nil {
		writeWarehouseReturnError(w, r, a, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) inspectWarehouseReturn(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var input struct {
		Disposition string `json:"disposition"`
	}
	if err != nil || decodeJSON(w, r, &input, 2048) != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe das Prüfergebnis."})
		return
	}
	if err := a.store.InspectWarehouseReturn(r.Context(), id, input.Disposition, user.Email); err != nil {
		writeWarehouseReturnError(w, r, a, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeWarehouseReturnError(w http.ResponseWriter, r *http.Request, a *App, err error) {
	switch {
	case errors.Is(err, store.ErrInvalidCheckout):
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Retourenaktion."})
	case errors.Is(err, store.ErrInvalidTransition):
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Der Retourenstatus erlaubt diesen Arbeitsschritt nicht."})
	case errors.Is(err, store.ErrNotFound):
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Retoure oder Artikel nicht gefunden."})
	case errors.Is(err, store.ErrInvalidQuantity):
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Die Bestandsmenge konnte nicht gebucht werden."})
	default:
		a.serverError(w, r, err)
	}
}

func validWarehouseReturnStatus(status string) bool {
	switch status {
	case "all", "expected", "received", "restocked", "damaged", "not_restockable":
		return true
	default:
		return false
	}
}
