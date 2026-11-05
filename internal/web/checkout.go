package web

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v82"

	"mitte/internal/checkout"
	"mitte/internal/commerce"
	"mitte/internal/payments"
	"mitte/internal/store"
)

func (a *App) checkout(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	var input store.CheckoutInput
	if err := decodeJSON(w, r, &input, 8192); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe deine Angaben."})
		return
	}
	commerce.NormalizeCheckoutInput(&input)
	if input.ShippingAddress.CountryCode != "DE" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Der Versand und die Umsatzsteuer sind derzeit nur für Lieferadressen in Deutschland eingerichtet."})
		return
	}
	if !validCheckoutInput(input) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe deine Kontakt- und Adressangaben."})
		return
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	var customerID int64
	if user, ok := a.currentUser(w, r); ok {
		if !strings.EqualFold(strings.TrimSpace(input.Email), user.Email) {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte verwende die E-Mail-Adresse deines Kontos."})
			return
		}
		customerID = user.ID
	}
	result, err := a.checkoutService.Execute(r.Context(), a.sessionToken(w, r), idempotencyKey, customerID, input)
	if err != nil {
		if errors.Is(err, checkout.ErrInvalidInput) {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe deine Kontakt- und Adressangaben."})
			return
		}
		if errors.Is(err, checkout.ErrPaymentUnavailable) {
			respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Die gewählte Zahlungsart ist derzeit nicht verfügbar."})
			return
		}
		if errors.Is(err, checkout.ErrPaymentMethodChanged) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Für diesen Checkout wurde bereits eine andere Zahlungsart gewählt."})
			return
		}
		var stripeErr *stripe.Error
		if errors.As(err, &stripeErr) {
			a.logger.Error("create Stripe checkout session", "request_id", requestID(r.Context()), "error", err)
			respondJSON(w, http.StatusBadGateway, map[string]string{"error": "Die Zahlung konnte nicht vorbereitet werden. Bitte versuche es erneut."})
			return
		}
		a.logger.Error(
			"checkout creation failed",
			"request_id", requestID(r.Context()),
			"error", err,
		)

		respondJSON(w, cartErrorStatus(err), map[string]string{
			"error": cartErrorMessage(err),
		})
		return
	}
	if result.CheckoutURL == "" {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Die Zahlung konnte nicht vorbereitet werden."})
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]string{
		"orderId": result.OrderID, "checkoutUrl": result.CheckoutURL, "status": "pending_payment",
	})
}

type stripeCheckoutGateway struct {
	provider payments.CheckoutCreator
}

func (g stripeCheckoutGateway) CreateCheckoutSession(ctx context.Context, request checkout.PaymentRequest) (string, string, error) {
	lineItems := make([]payments.LineItem, len(request.LineItems))
	for i, item := range request.LineItems {
		lineItems[i] = payments.LineItem{Name: item.Name, AmountCents: item.AmountCents, Quantity: item.Quantity}
	}
	return g.provider.CreateCheckoutSession(payments.CheckoutRequest{
		Context: ctx, OrderID: request.OrderID, CustomerEmail: request.CustomerEmail,
		LineItems: lineItems, SuccessURL: request.SuccessURL, CancelURL: request.CancelURL,
	})
}

func paymentLineItems(items []commerce.CheckoutLineItem) []payments.LineItem {
	converted := make([]payments.LineItem, len(items))
	for i, item := range items {
		converted[i] = payments.LineItem{Name: item.Name, AmountCents: item.AmountCents, Quantity: item.Quantity}
	}
	return converted
}

func (a *App) checkoutSuccess(w http.ResponseWriter, r *http.Request) {
	// The payment return screen is a React route. Go owns order state and exposes
	// it through /api/orders/status; React handles the pending webhook state
	// without falling back to a second server-rendered storefront.
	a.renderApp(w, r, "Bestellung - Mitte")
}

func (a *App) orderStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	orderID := strings.TrimSpace(r.URL.Query().Get("order_id"))
	if sessionID == "" && orderID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "session_id or order_id required"})
		return
	}
	var order store.Order
	var err error
	if sessionID != "" {
		order, err = a.store.OrderByStripeSessionID(r.Context(), sessionID)
	} else {
		user, ok := a.currentUser(w, r)
		if !ok {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Bitte melde dich an."})
			return
		}
		order, err = a.store.OrderByIDForCustomer(r.Context(), orderID, user.ID)
	}
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if order.Status == "pending_payment" {
		a.confirmReturnedStripePayment(r.Context(), order)
		// A successful confirmation updates the order transactionally. Reload it
		// before responding so the browser receives the final status immediately.
		order, err = a.store.OrderByStripeSessionID(r.Context(), order.StripeCheckoutSessionID)
		if err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	payload := map[string]any{"orderId": order.ID, "status": order.Status, "totalCents": order.TotalCents,
		"subtotalCents": order.SubtotalCents, "discountCents": order.DiscountCents, "shippingCents": order.ShippingCents, "taxCents": order.TaxCents}
	if order.PromotionCode != "" {
		payload["promotionCode"] = order.PromotionCode
	}
	if order.PaidAt != nil {
		decision := commerce.ReturnEligibility(*order.PaidAt, time.Now())
		payload["paidAt"] = order.PaidAt.UTC().Format(time.RFC3339)
		payload["returns"] = decision
	}
	respondJSON(w, http.StatusOK, payload)
}

func (a *App) confirmReturnedStripePayment(ctx context.Context, order store.Order) {
	retriever, ok := a.stripe.(payments.CheckoutSessionRetriever)
	if !ok || order.StripeCheckoutSessionID == "" {
		return
	}

	session, err := retriever.RetrieveCheckoutSession(ctx, order.StripeCheckoutSessionID)
	if err != nil {
		a.logger.Warn("retrieve Stripe checkout session after return failed", "order_id", order.ID, "error", err)
		return
	}
	if !session.Paid {
		return
	}
	// FulfillStripeCheckout repeats the session, currency, and amount checks
	// under the order lock. Checking metadata here additionally makes sure this
	// Stripe response belongs to the order we are about to fulfill.
	if session.ID != order.StripeCheckoutSessionID || session.OrderID != order.ID {
		a.logger.Error("returned Stripe session does not belong to order", "order_id", order.ID, "session_id", session.ID)
		return
	}
	if err := a.fulfillVerifiedStripeCheckout(ctx, order.ID, session.ID, session.PaymentIntentID, session.AmountTotal, session.Currency); err != nil {
		a.logger.Error("fulfill returned Stripe payment failed", "order_id", order.ID, "error", err)
		return
	}
	if err := a.store.RecordCheckoutPaymentConfirmation(ctx, order.ID, session.ID); err != nil {
		a.logger.Error("record returned Stripe payment event failed", "order_id", order.ID, "error", err)
	}
}

func (a *App) fulfillVerifiedStripeCheckout(ctx context.Context, orderID, checkoutSessionID, paymentIntentID string, amountTotal int64, currency string) error {
	err := a.store.FulfillStripeCheckout(ctx, orderID, checkoutSessionID, paymentIntentID, amountTotal, currency)
	if !errors.Is(err, store.ErrInvalidTransition) {
		return err
	}
	return a.store.ReconcileLateStripeCheckout(ctx, orderID, checkoutSessionID, paymentIntentID, amountTotal, currency)
}
