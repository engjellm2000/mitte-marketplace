package web

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
)

func (a *App) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if a.stripeWebhookSecret == "" {
		http.Error(w, "webhooks not configured", http.StatusServiceUnavailable)
		return
	}

	const maxBody = 65536
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), a.stripeWebhookSecret)
	if err != nil {
		a.logger.Warn("stripe webhook signature invalid", "request_id", requestID(r.Context()), "error", err)
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	orderID := ""
	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted:
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			http.Error(w, "invalid event payload", http.StatusBadRequest)
			return
		}
		orderID = session.Metadata["order_id"]
		if orderID == "" {
			a.logger.Error("stripe checkout completed without order_id", "request_id", requestID(r.Context()), "session_id", session.ID)
			w.WriteHeader(http.StatusOK)
			return
		}
		paymentIntentID := ""
		if session.PaymentIntent != nil {
			paymentIntentID = session.PaymentIntent.ID
		}
		if session.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
			a.logger.Warn("checkout completion without settled payment", "request_id", requestID(r.Context()), "session_id", session.ID, "payment_status", session.PaymentStatus)
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := a.fulfillVerifiedStripeCheckout(r.Context(), orderID, session.ID, paymentIntentID, session.AmountTotal, string(session.Currency)); err != nil {
			a.logger.Error("fulfill paid order failed", "request_id", requestID(r.Context()), "order_id", orderID, "error", err)
			a.serverError(w, r, err)
			return
		}
	case stripe.EventTypeCheckoutSessionExpired:
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			http.Error(w, "invalid event payload", http.StatusBadRequest)
			return
		}
		orderID = session.Metadata["order_id"]
		if orderID == "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := a.store.CancelPendingOrder(r.Context(), orderID, "checkout_session_expired"); err != nil {
			a.logger.Error("cancel pending order failed", "request_id", requestID(r.Context()), "order_id", orderID, "error", err)
			a.serverError(w, r, err)
			return
		}
	}

	// Record only after the side effect succeeded. If the process dies after a
	// database commit, a retried delivery is safe because order transitions are
	// idempotent; recording before processing would permanently lose that retry.
	if _, err := a.store.RecordPaymentEvent(r.Context(), event.ID, string(event.Type), orderID, payload); err != nil {
		a.serverError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
