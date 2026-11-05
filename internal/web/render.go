package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"strings"

	"mitte/internal/commerce"
	"mitte/internal/store"
)

func (a *App) allowMutation(w http.ResponseWriter, r *http.Request) bool {
	if !a.limiter.allow(a.clientIP(r)) {
		w.Header().Set("Retry-After", "60")
		respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Zu viele Anfragen. Bitte warte kurz."})
		return false
	}
	if !a.validCSRF(r) {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Sicherheitsprüfung fehlgeschlagen. Bitte lade die Seite neu."})
		return false
	}
	return true
}

func (a *App) baseData(w http.ResponseWriter, r *http.Request) (viewData, error) {
	categories, err := a.store.Categories(r.Context())
	if err != nil {
		return viewData{}, err
	}
	cart, err := a.store.Cart(r.Context(), a.sessionToken(w, r))
	if err != nil {
		return viewData{}, err
	}
	return viewData{Categories: categories, Cart: cart, CartCount: cartCount(cart), CSRFToken: a.csrfToken(w, r)}, nil
}

func (a *App) render(w http.ResponseWriter, r *http.Request, page string, data viewData) {
	tmpl, err := template.New("base.html").Funcs(template.FuncMap{"euro": formatEuro, "cartCount": cartCount, "multiply": func(left, right int) int { return left * right }}).ParseFS(assets, "templates/base.html", "templates/"+page)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		a.serverError(w, r, err)
	}
}

func (a *App) renderApp(w http.ResponseWriter, r *http.Request, title string) {
	data, err := a.baseData(w, r)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	data.PageTitle = title
	tmpl, err := template.ParseFS(assets, "templates/app.html")
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "app", data); err != nil {
		a.serverError(w, r, err)
	}
}

func (a *App) sessionToken(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie("mitte_session"); err == nil && len(cookie.Value) == 36 {
		return cookie.Value
	}
	token := randomToken(18)
	http.SetCookie(w, &http.Cookie{Name: "mitte_session", Value: token, Path: "/", MaxAge: 60 * 60 * 24 * 30, HttpOnly: true, Secure: a.secureCookies, SameSite: http.SameSiteLaxMode})
	return token
}

func (a *App) csrfToken(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie("mitte_csrf"); err == nil && len(cookie.Value) == 64 {
		return cookie.Value
	}
	token := randomToken(32)
	http.SetCookie(w, &http.Cookie{Name: "mitte_csrf", Value: token, Path: "/", MaxAge: 60 * 60 * 24, HttpOnly: true, Secure: a.secureCookies, SameSite: http.SameSiteStrictMode})
	return token
}

func (a *App) validCSRF(r *http.Request) bool {
	cookie, err := r.Cookie("mitte_csrf")
	if err != nil || len(cookie.Value) != 64 {
		return false
	}
	provided := r.Header.Get("X-CSRF-Token")
	return len(provided) == len(cookie.Value) && subtle.ConstantTimeCompare([]byte(provided), []byte(cookie.Value)) == 1
}

func (a *App) serverError(w http.ResponseWriter, r *http.Request, err error) {
	a.logger.Error("request failed", "request_id", requestID(r.Context()), "path", r.URL.Path, "error", err)
	status := http.StatusInternalServerError
	if store.IsRetryableTransactionError(err) {
		// The storage layer already made bounded retries. Tell clients and
		// webhook providers that a later idempotent retry is appropriate.
		w.Header().Set("Retry-After", "1")
		status = http.StatusServiceUnavailable
	}
	http.Error(w, "Es ist etwas schiefgelaufen. Bitte versuche es erneut.", status)
}

func cartCount(cart store.Cart) int {
	total := 0
	for _, item := range cart.Items {
		total += item.Quantity
	}
	return total
}
func formatEuro(cents int) string { return fmt.Sprintf("%d,%02d €", cents/100, cents%100) }

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any, maxBytes int64) error {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		return errors.New("Content-Type must be application/json")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain exactly one JSON value")
	}
	return nil
}

func validCheckoutInput(input store.CheckoutInput) bool {
	email := strings.TrimSpace(input.Email)
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && commerce.ValidCheckoutInput(input)
}

func respondJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(true)
	_ = encoder.Encode(value)
}

func cartErrorStatus(err error) int {
	if errors.Is(err, store.ErrCustomerFraudBlocked) {
		return http.StatusConflict
	}
	if errors.Is(err, store.ErrNotFound) {
		return http.StatusNotFound
	}
	if errors.Is(err, store.ErrOutOfStock) {
		return http.StatusConflict
	}
	if errors.Is(err, store.ErrInvalidQuantity) || errors.Is(err, store.ErrEmptyCart) || errors.Is(err, store.ErrInvalidCheckout) {
		return http.StatusBadRequest
	}
	if errors.Is(err, store.ErrUnknownPromotion) || errors.Is(err, store.ErrPromotionInactive) {
		return http.StatusNotFound
	}
	if errors.Is(err, store.ErrPromotionExhausted) || errors.Is(err, store.ErrPromotionLimit) {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

func cartErrorMessage(err error) string {
	switch {
	case errors.Is(err, store.ErrCustomerFraudBlocked):
		return "Dieses Kundenkonto ist für eine manuelle Fraud-Prüfung gesperrt. Bitte kontaktiere den Kundenservice."
	case errors.Is(err, store.ErrNotFound):
		return "Dieser Artikel ist nicht mehr verfügbar."
	case errors.Is(err, store.ErrOutOfStock):
		return "Die gewünschte Menge ist nicht mehr verfügbar."
	case errors.Is(err, store.ErrInvalidQuantity):
		return "Bitte wähle eine gültige Menge."
	case errors.Is(err, store.ErrEmptyCart):
		return "Dein Warenkorb ist leer."
	case errors.Is(err, store.ErrInvalidCheckout):
		return "Bitte prüfe deine Angaben."
	case errors.Is(err, store.ErrUnknownPromotion), errors.Is(err, store.ErrPromotionInactive):
		return "Dieser Gutscheincode gilt gerade nicht."
	case errors.Is(err, store.ErrPromotionExhausted):
		return "Dieser Gutschein ist ausgeschöpft."
	case errors.Is(err, store.ErrPromotionLimit):
		return "Diesen Willkommensrabatt hast du bereits eingelöst."
	default:
		return "Es ist etwas schiefgelaufen. Bitte versuche es erneut."
	}
}

func randomToken(bytesLen int) string {
	bytes := make([]byte, bytesLen)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}
