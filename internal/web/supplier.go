package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"mitte/internal/store"
)

const supplierCookieName = "mitte_supplier"

type supplierShipmentInput struct {
	Note           string                       `json:"note"`
	Carrier        string                       `json:"carrier"`
	TrackingNumber string                       `json:"trackingNumber"`
	TrackingURL    string                       `json:"trackingUrl"`
	Items          []store.SupplierShipmentLine `json:"items"`
}

type supplierOrderActionInput struct {
	Action         string `json:"action"`
	Note           string `json:"note"`
	DelayedDate    string `json:"delayedDate"`
	Carrier        string `json:"carrier"`
	TrackingNumber string `json:"trackingNumber"`
	TrackingURL    string `json:"trackingUrl"`
}

func (a *App) supplierPortal(w http.ResponseWriter, r *http.Request) {
	a.renderApp(w, r, "Lieferantenportal – MITTE")
}

func (a *App) supplierLogin(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthMutation(w, r) {
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &input, 1024); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Anfrage."})
		return
	}
	username := strings.ToLower(strings.TrimSpace(input.Username))
	if !a.loginLimiter.allow("supplier:" + username) {
		w.Header().Set("Retry-After", "60")
		respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Zu viele Anmeldeversuche. Bitte warte kurz."})
		return
	}
	credential, exists := a.supplierCredentials[username]
	if !exists || !staffPasswordMatches(input.Password, credential.Password) {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Benutzername oder Passwort ist nicht korrekt."})
		return
	}
	token, err := store.NewOperatorSessionToken()
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	expiresAt := time.Now().UTC().Add(8 * time.Hour)
	if err := a.sessions.CreateOperatorSession(r.Context(), token, "supplier:"+username, expiresAt); err != nil {
		a.serverError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: supplierCookieName, Value: token, Path: "/", Expires: expiresAt, MaxAge: int((8 * time.Hour).Seconds()), HttpOnly: true, Secure: a.secureCookies, SameSite: http.SameSiteStrictMode})
	respondJSON(w, http.StatusOK, map[string]any{"supplier": a.supplierIdentity(username)})
}

func (a *App) supplierLogout(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	if token := a.supplierSessionToken(r); token != "" {
		if err := a.sessions.RevokeOperatorSession(r.Context(), token, time.Now().UTC()); err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	a.clearSupplierCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) supplierMe(w http.ResponseWriter, r *http.Request) {
	username, ok := a.currentSupplier(w, r)
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"supplier": a.supplierIdentity(username)})
}

func (a *App) supplierOrders(w http.ResponseWriter, r *http.Request) {
	username, ok := a.currentSupplier(w, r)
	if !ok {
		return
	}
	items, err := a.store.SupplierOrderItems(r.Context(), a.supplierCredentials[username].SupplierName)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) supplierOrderAction(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	username, ok := a.currentSupplier(w, r)
	if !ok {
		return
	}
	productID, err := strconv.ParseInt(r.PathValue("productID"), 10, 64)
	if err != nil || productID < 1 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Bestellposition."})
		return
	}
	var input supplierOrderActionInput
	if err := decodeJSON(w, r, &input, 4096); err != nil || !validSupplierAction(&input) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe Aktion, Begründung und Versanddaten."})
		return
	}
	credential := a.supplierCredentials[username]
	if err := a.store.RecordSupplierOrderAction(r.Context(), r.PathValue("id"), productID,
		credential.SupplierName, username, input.Action, input.Note, input.DelayedDate,
		input.Carrier, input.TrackingNumber, input.TrackingURL); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "Bestellposition nicht gefunden oder nicht diesem Lieferanten zugeordnet."})
		case errors.Is(err, store.ErrInvalidTransition):
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Diese Bestellposition wurde bereits bearbeitet oder kann nicht mehr geändert werden."})
		case errors.Is(err, store.ErrInvalidCheckout):
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Die Aktion ist ungültig."})
		default:
			a.serverError(w, r, err)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) supplierShipment(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	username, ok := a.currentSupplier(w, r)
	if !ok {
		return
	}
	var input supplierShipmentInput
	if err := decodeJSON(w, r, &input, 8192); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe Sendungsdaten und Mengen."})
		return
	}
	if !validSupplierShipment(&input) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe Versanddienstleister, Sendungsnummer, Tracking-Link und Mengen."})
		return
	}
	credential := a.supplierCredentials[username]
	if err := a.store.RecordSupplierShipment(r.Context(), r.PathValue("id"), credential.SupplierName,
		username, input.Note, input.Carrier, input.TrackingNumber, input.TrackingURL,
		a.invoicePublicBaseURL, a.invoiceSeller, input.Items); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "Bestellung oder Position nicht gefunden oder nicht diesem Lieferanten zugeordnet."})
		case errors.Is(err, store.ErrInvalidTransition):
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Prüfe offene Mengen und Trackingnummer; die Sendung wurde möglicherweise bereits gemeldet oder die Bestellung geändert."})
		case errors.Is(err, store.ErrInvalidCheckout):
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Die Sendung ist ungültig."})
		default:
			a.serverError(w, r, err)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) supplierCancellationRequests(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		return
	}
	requests, err := a.store.SupplierCancellationRequests(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"requests": requests})
}

func (a *App) reviewSupplierCancellationRequest(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	staff, ok := a.currentStaff(w, r)
	if !ok {
		return
	}
	eventID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || eventID < 1 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Stornierungsanfrage."})
		return
	}
	var input struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := decodeJSON(w, r, &input, 2048); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe die Rückmeldung."})
		return
	}
	if err := a.store.ReviewSupplierCancellationRequest(r.Context(), eventID, input.Status, input.Note, staff.Email); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "Offene Stornierungsanfrage nicht gefunden."})
		case errors.Is(err, store.ErrInvalidCheckout):
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte wähle eine Entscheidung und gib eine interne Notiz ein."})
		default:
			a.serverError(w, r, err)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validSupplierShipment(input *supplierShipmentInput) bool {
	input.Note = strings.TrimSpace(input.Note)
	input.Carrier = strings.TrimSpace(input.Carrier)
	input.TrackingNumber = strings.TrimSpace(input.TrackingNumber)
	input.TrackingURL = strings.TrimSpace(input.TrackingURL)
	if utf8.RuneCountInString(input.Note) > 1000 ||
		!validSupplierTracking(input.Carrier, input.TrackingNumber, input.TrackingURL) ||
		len(input.Items) == 0 || len(input.Items) > 50 {
		return false
	}
	seen := make(map[int64]bool, len(input.Items))
	for _, item := range input.Items {
		if item.ProductID < 1 || item.Quantity < 1 || seen[item.ProductID] {
			return false
		}
		seen[item.ProductID] = true
	}
	return true
}

func validSupplierAction(input *supplierOrderActionInput) bool {
	input.Action = strings.TrimSpace(input.Action)
	input.Note = strings.TrimSpace(input.Note)
	input.DelayedDate = strings.TrimSpace(input.DelayedDate)
	input.Carrier = strings.TrimSpace(input.Carrier)
	input.TrackingNumber = strings.TrimSpace(input.TrackingNumber)
	input.TrackingURL = strings.TrimSpace(input.TrackingURL)
	if utf8.RuneCountInString(input.Note) > 1000 {
		return false
	}
	switch input.Action {
	case "accept":
		return utf8.RuneCountInString(input.Note) <= 1000
	case "delay":
		_, err := time.Parse("2006-01-02", input.DelayedDate)
		today := time.Now().UTC().Format("2006-01-02")
		return err == nil && input.DelayedDate > today && input.Note != ""
	case "cancellation_request":
		return utf8.RuneCountInString(input.Note) >= 5
	default:
		return false
	}
}

func validSupplierTracking(carrier, trackingNumber, trackingURL string) bool {
	if carrier == "" || trackingNumber == "" ||
		utf8.RuneCountInString(carrier) > 100 || utf8.RuneCountInString(trackingNumber) > 160 {
		return false
	}
	if trackingURL == "" {
		return true
	}
	parsed, err := url.Parse(trackingURL)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == "" && len(trackingURL) <= 2048
}

func (a *App) currentSupplier(w http.ResponseWriter, r *http.Request) (string, bool) {
	token := a.supplierSessionToken(r)
	if token == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Bitte melde dich im Lieferantenportal an."})
		return "", false
	}
	principal, err := a.sessions.OperatorSessionPrincipal(r.Context(), token, time.Now().UTC())
	if errors.Is(err, store.ErrInvalidSession) {
		a.clearSupplierCookie(w)
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Deine Sitzung ist abgelaufen. Bitte melde dich erneut an."})
		return "", false
	}
	if err != nil {
		a.serverError(w, r, err)
		return "", false
	}
	username, found := strings.CutPrefix(principal, "supplier:")
	if !found || username == "" {
		a.clearSupplierCookie(w)
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Ungültige Lieferantensitzung."})
		return "", false
	}
	if _, exists := a.supplierCredentials[username]; !exists {
		a.clearSupplierCookie(w)
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Dieses Lieferantenkonto ist nicht mehr aktiv."})
		return "", false
	}
	return username, true
}

func (a *App) supplierIdentity(username string) map[string]string {
	return map[string]string{"username": username, "supplierName": a.supplierCredentials[username].SupplierName}
}

func (a *App) supplierSessionToken(r *http.Request) string {
	cookie, err := r.Cookie(supplierCookieName)
	if err != nil || len(cookie.Value) != 64 {
		return ""
	}
	return cookie.Value
}

func (a *App) clearSupplierCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: supplierCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.secureCookies, SameSite: http.SameSiteStrictMode})
}
