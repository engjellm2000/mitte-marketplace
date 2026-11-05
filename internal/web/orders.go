package web

import (
	"errors"
	"net/http"

	"mitte/internal/payments"
	"mitte/internal/store"
)

// cancelOrder refunds a paid, not-yet-shipped order. The authenticated owner
// is resolved before calling Stripe, and the Stripe idempotency key makes a
// retry safe if the browser loses its response.
func (a *App) cancelOrder(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentUser(w, r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Bitte melde dich an."})
		return
	}
	order, err := a.store.OrderByIDForCustomer(r.Context(), r.PathValue("id"), user.ID)
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Bestellung nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if order.Status != "paid" || order.PaymentProvider != "stripe" || order.StripePaymentIntentID == "" {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Diese Bestellung kann nicht mehr storniert werden."})
		return
	}
	refunder, ok := a.stripe.(payments.Refunder)
	if !ok {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Erstattungen sind derzeit nicht verfügbar."})
		return
	}
	refundID, err := refunder.RefundPaymentIntent(r.Context(), order.StripePaymentIntentID, "customer_cancel_"+order.ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	order, err = a.store.CancelPaidOrder(r.Context(), order.ID, user.ID, refundID)
	if errors.Is(err, store.ErrInvalidTransition) {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Die Bestellung wurde zwischenzeitlich versendet und kann nicht mehr storniert werden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"order": order, "message": "Bestellung storniert; die Erstattung wurde ausgelöst."})
}
