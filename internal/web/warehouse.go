package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"mitte/internal/store"
)

func (a *App) warehouseQueue(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	if values, present := r.URL.Query()["orderIds"]; present {
		if len(values) != 1 {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe die Bestellnummern."})
			return
		}
		orderIDs, err := parseWarehouseOrderIDs(values[0])
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe die Bestellnummern."})
			return
		}
		items, err := a.store.WarehouseFulfillmentsByOrderIDs(r.Context(), orderIDs)
		if errors.Is(err, store.ErrInvalidWarehouseOrderIDs) {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Zu viele Bestellnummern angefragt."})
			return
		}
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"fulfillments": items})
		return
	}
	page, limit, offset, err := parseWarehouseQueuePage(r.URL.Query().Get("page"), r.URL.Query().Get("limit"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	if !validWarehouseQueueStatus(status) || len(search) > maxWarehouseQueueSearchLength {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe Versandstatus und Suchbegriff."})
		return
	}
	items, total, stats, err := a.store.WarehouseQueuePage(r.Context(), status, search, limit, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	pages := (total + limit - 1) / limit
	if pages < 1 {
		pages = 1
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"fulfillments": items, "total": total, "page": page, "limit": limit, "pages": pages,
		"summary": map[string]int{"active": stats.Active, "handedOver": stats.HandedOver, "processing": stats.Processing},
	})
}

func (a *App) warehouseDetail(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	fulfillment, events, eventCount, err := a.store.WarehouseFulfillment(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Fulfillment nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"fulfillment": fulfillment, "events": events, "eventCount": eventCount})
}

func (a *App) warehouseClaim(w http.ResponseWriter, r *http.Request) {
	a.warehouseTransition(w, r, "picking", "Kommissionierung gestartet")
}
func (a *App) warehousePack(w http.ResponseWriter, r *http.Request) {
	a.warehouseTransition(w, r, "packed", "Sendung verpackt")
}
func (a *App) warehouseHold(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(w, r, &input, 2048); err != nil || len([]rune(strings.TrimSpace(input.Reason))) < 3 || len([]rune(strings.TrimSpace(input.Reason))) > 500 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte gib einen Grund mit 3 bis 500 Zeichen an."})
		return
	}
	err := a.store.HoldWarehouseFulfillment(r.Context(), r.PathValue("id"), user.Email, input.Reason)
	a.warehouseHoldResponse(w, r, err, "Fulfillment kann nur vor der Übergabe angehalten werden.")
}

func (a *App) warehouseRelease(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	err := a.store.ReleaseWarehouseFulfillment(r.Context(), r.PathValue("id"), user.Email)
	a.warehouseHoldResponse(w, r, err, "Dieses Fulfillment ist nicht angehalten.")
}

func (a *App) warehouseHoldResponse(w http.ResponseWriter, r *http.Request, err error, invalid string) {
	if errors.Is(err, store.ErrInvalidFulfillmentHold) {
		respondJSON(w, http.StatusConflict, map[string]string{"error": invalid})
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Fulfillment nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *App) warehouseTransition(w http.ResponseWriter, r *http.Request, next, detail string) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	err := a.store.TransitionWarehouseFulfillment(r.Context(), r.PathValue("id"), user.Email, next, detail)
	if errors.Is(err, store.ErrCustomerFraudBlocked) {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Dieses Kundenkonto ist für eine Fraud-Prüfung gesperrt; Fulfillment ist angehalten."})
		return
	}
	if errors.Is(err, store.ErrInvalidFulfillmentTransition) {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Dieser Arbeitsschritt ist für den aktuellen Versandstatus nicht möglich."})
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Fulfillment nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) warehouseLabel(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	var input struct {
		Carrier        string `json:"carrier"`
		TrackingNumber string `json:"trackingNumber"`
		TrackingURL    string `json:"trackingUrl"`
		LabelURL       string `json:"labelUrl"`
	}
	if err := decodeJSON(w, r, &input, 4096); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe Versanddienstleister, Tracking- und Label-Link."})
		return
	}
	input.Carrier = strings.TrimSpace(input.Carrier)
	input.TrackingNumber = strings.TrimSpace(input.TrackingNumber)
	input.TrackingURL = strings.TrimSpace(input.TrackingURL)
	input.LabelURL = strings.TrimSpace(input.LabelURL)
	if input.Carrier == "" || input.TrackingNumber == "" || len(input.Carrier) > 100 || len(input.TrackingNumber) > 160 || !validHTTPSURL(input.TrackingURL) || !validHTTPSURL(input.LabelURL) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe Versanddienstleister, Tracking- und Label-Link."})
		return
	}
	err := a.store.AttachWarehouseLabel(r.Context(), r.PathValue("id"), user.Email, input.Carrier, input.TrackingNumber, input.TrackingURL, input.LabelURL)
	if errors.Is(err, store.ErrCustomerFraudBlocked) {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Dieses Kundenkonto ist für eine Fraud-Prüfung gesperrt; Fulfillment ist angehalten."})
		return
	}
	if errors.Is(err, store.ErrInvalidFulfillmentTransition) {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Ein Label kann erst nach dem Verpacken hinterlegt werden."})
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Fulfillment nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) warehouseHandover(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	item, _, _, err := a.store.WarehouseFulfillment(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Fulfillment nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if item.Status != "label_ready" {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Die Sendung benötigt zuerst ein hinterlegtes Label."})
		return
	}
	if err := a.store.MarkWarehouseShipmentBy(r.Context(), item.OrderID, user.Email, item.Carrier, item.TrackingNumber, item.TrackingURL, a.invoicePublicBaseURL, a.invoiceSeller); err != nil {
		if errors.Is(err, store.ErrCustomerFraudBlocked) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Dieses Kundenkonto ist für eine Fraud-Prüfung gesperrt; Fulfillment ist angehalten."})
			return
		}
		if errors.Is(err, store.ErrInvalidTransition) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Diese Bestellung wurde bereits bearbeitet."})
			return
		}
		a.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

const (
	defaultWarehouseQueuePageSize = 50
	maxWarehouseQueuePageSize     = 100
	maxWarehouseQueuePage         = 10000
	maxWarehouseQueueOffset       = 100000
	maxWarehouseQueueSearchLength = 120
	maxWarehouseOrderIDs          = 100
	maxWarehouseOrderIDLength     = 128
)

func parseWarehouseQueuePage(pageValue, limitValue string) (page, limit, offset int, err error) {
	page = 1
	if pageValue != "" {
		page, err = strconv.Atoi(pageValue)
		if err != nil || page < 1 || page > maxWarehouseQueuePage {
			return 0, 0, 0, errors.New("invalid warehouse queue page")
		}
	}
	limit = defaultWarehouseQueuePageSize
	if limitValue != "" {
		limit, err = strconv.Atoi(limitValue)
		if err != nil || limit < 1 || limit > maxWarehouseQueuePageSize {
			return 0, 0, 0, errors.New("invalid warehouse queue page size")
		}
	}
	if page-1 > maxWarehouseQueueOffset/limit {
		return 0, 0, 0, errors.New("warehouse queue offset exceeds maximum")
	}
	return page, limit, (page - 1) * limit, nil
}

func validWarehouseQueueStatus(status string) bool {
	switch status {
	case "", "all", "active", "ready_to_pick", "picking", "packed", "label_ready", "held", "handed_over", "cancelled":
		return true
	default:
		return false
	}
}

func parseWarehouseOrderIDs(value string) ([]string, error) {
	if value == "" {
		return nil, errors.New("warehouse order IDs cannot be empty")
	}
	values := strings.Split(value, ",")
	if len(values) > maxWarehouseOrderIDs {
		return nil, errors.New("too many warehouse order IDs")
	}
	ids := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, id := range values {
		id = strings.TrimSpace(id)
		if id == "" || len(id) > maxWarehouseOrderIDLength {
			return nil, errors.New("invalid warehouse order ID")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func validHTTPSURL(value string) bool {
	if value == "" {
		return true
	}
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Fragment == "" && len(value) <= 2048
}
