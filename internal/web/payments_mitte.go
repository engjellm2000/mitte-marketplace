package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"mitte/internal/store"
)

func (a *App) mittePay(w http.ResponseWriter, r *http.Request) {
	// The React screen is a sandbox-hosted payment page. It intentionally has
	// no credential fields: this service is not a PCI card processor.
	a.renderApp(w, r, "Mitte Pay")
}

func (a *App) mitteInvoice(w http.ResponseWriter, r *http.Request) {
	a.renderApp(w, r, "Mitte Rechnung · Demo")
}

func (a *App) approveMitteInvoice(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	if !a.mitteInvoiceSandboxEnabled {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Mitte Rechnung ist nur in der Entwicklungsumgebung verfügbar."})
		return
	}
	var input struct {
		SessionID string `json:"sessionId"`
	}
	if err := decodeJSON(w, r, &input, 1024); err != nil || !validMitteInvoiceSessionID(input.SessionID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Rechnungsreferenz."})
		return
	}
	order, err := a.store.OrderByStripeSessionID(r.Context(), input.SessionID)
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Rechnung nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if err := a.store.ApproveMitteInvoice(r.Context(), order.ID, input.SessionID); err != nil {
		if errors.Is(err, store.ErrInvalidTransition) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Die Rechnungsfreigabe kann für diese Bestellung nicht abgeschlossen werden."})
			return
		}
		a.serverError(w, r, err)
		return
	}
	if _, err := a.store.RecordPaymentEvent(r.Context(), "mitte_invoice:"+input.SessionID, "mitte_invoice.approved", order.ID, []byte(`{"source":"development_invoice_sandbox"}`)); err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"orderId": order.ID, "status": "invoice_approved"})
}

// simulateMitteInvoiceTransfer stands in for the provider's bank-feed
// webhook in local development. Production must receive an authenticated bank
// or provider event instead.
func (a *App) simulateMitteInvoiceTransfer(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) || !a.mitteInvoiceSandboxEnabled {
		return
	}
	var input struct {
		SessionID string `json:"sessionId"`
	}
	if err := decodeJSON(w, r, &input, 1024); err != nil || !validMitteInvoiceSessionID(input.SessionID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Rechnungsreferenz."})
		return
	}
	order, err := a.store.OrderByStripeSessionID(r.Context(), input.SessionID)
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Rechnung nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if order.Status == "paid" {
		respondJSON(w, http.StatusOK, map[string]string{"orderId": order.ID, "status": "paid"})
		return
	}
	receipt, err := a.store.CreatePaymentReceipt(r.Context(), "SIM-"+strings.ToUpper(input.SessionID), order.TotalCents, order.CustomerName, "0000", "Simulierter Bankeingang (Entwicklung)", "invoice-sandbox")
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if err := a.store.ReplacePaymentReceiptAllocations(r.Context(), receipt.ID, []store.PaymentReceiptAllocation{{OrderID: order.ID, AmountCents: order.TotalCents}}); err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"orderId": order.ID, "status": "paid"})
}

func (a *App) transferInstructions(w http.ResponseWriter, r *http.Request) {
	if !a.mitteTransferEnabled {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Mitte Überweisung ist derzeit nicht verfügbar."})
		return
	}
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if !strings.HasPrefix(sessionID, "mftransfer_") || len(sessionID) != len("mftransfer_")+36 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Zahlungsreferenz."})
		return
	}
	order, err := a.store.OrderByStripeSessionID(r.Context(), sessionID)
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Zahlung nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	payment, err := a.store.TransferPaymentByOrderID(r.Context(), order.ID)
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Zahlung nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"orderId": order.ID, "amountCents": payment.AmountCents, "reference": payment.Reference, "status": payment.Status, "recipient": a.mitteTransferRecipient, "iban": a.mitteTransferIBAN})
}

func (a *App) confirmMittePay(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	if !a.mittePaySandboxEnabled {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Mitte Pay ist in dieser Umgebung nicht verfügbar."})
		return
	}
	var input struct {
		SessionID string `json:"sessionId"`
		Code      string `json:"code"`
	}
	if err := decodeJSON(w, r, &input, 1024); err != nil || !validMittePaySessionID(input.SessionID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe E-Mail-Adresse und Bestätigungscode."})
		return
	}
	if !validMittePayCode(input.Code) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe E-Mail-Adresse und Bestätigungscode."})
		return
	}
	order, err := a.store.OrderByStripeSessionID(r.Context(), input.SessionID)
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Zahlungssitzung nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	orderID, err := a.store.ConsumeMittePayChallenge(r.Context(), input.SessionID, order.CustomerEmail, a.mittePayCodeHash(input.SessionID, order.CustomerEmail, input.Code))
	if errors.Is(err, store.ErrPaymentChallengeInvalid) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Der Bestätigungscode ist ungültig oder abgelaufen."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	order, err = a.store.OrderByID(r.Context(), orderID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if order.StripeCheckoutSessionID != input.SessionID {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Der Bestätigungscode passt nicht zu dieser Zahlung."})
		return
	}
	if order.Status == "cancelled" || order.Status == "fulfilled" {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Diese Zahlungssitzung ist nicht mehr aktiv."})
		return
	}
	if err := a.store.FulfillMittePay(r.Context(), order.ID, input.SessionID, int64(order.TotalCents)); err != nil && !errors.Is(err, store.ErrInvalidTransition) {
		a.serverError(w, r, err)
		return
	}
	if _, err := a.store.RecordPaymentEvent(r.Context(), "mitte_pay:"+input.SessionID, "mitte_pay.confirmed", order.ID, []byte(`{"source":"mitte_pay_sandbox"}`)); err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"orderId": order.ID, "status": "paid"})
}

func (a *App) requestMittePayCode(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	if !a.mittePaySandboxEnabled {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Mitte Pay ist in dieser Umgebung nicht verfügbar."})
		return
	}
	var input struct {
		SessionID string `json:"sessionId"`
	}
	if err := decodeJSON(w, r, &input, 1024); err != nil || !validMittePaySessionID(input.SessionID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Zahlungsreferenz."})
		return
	}
	code, err := mittePayCode()
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	expiresAt := time.Now().Add(10 * time.Minute)
	subject := "Dein Mitte Pay Bestätigungscode"
	body := fmt.Sprintf("Dein Bestätigungscode lautet: %s\n\nEr ist zehn Minuten gültig. Teile ihn mit niemandem.", code)
	order, err := a.store.OrderByStripeSessionID(r.Context(), input.SessionID)
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Zahlungssitzung nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if err := a.store.IssueMittePayChallenge(r.Context(), order.ID, input.SessionID, order.CustomerEmail, a.mittePayCodeHash(input.SessionID, order.CustomerEmail, code), expiresAt, subject, body); err != nil {
		switch {
		case errors.Is(err, store.ErrPaymentChallengeRateLimited):
			w.Header().Set("Retry-After", "60")
			respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Ein Code wurde bereits gesendet. Bitte warte kurz."})
		case errors.Is(err, store.ErrPaymentChallengeInvalid), errors.Is(err, store.ErrNotFound):
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Diese Zahlung kann nicht bestätigt werden."})
		default:
			a.serverError(w, r, err)
		}
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]string{"status": "code_sent", "expiresAt": expiresAt.UTC().Format(time.RFC3339)})
}

func validMittePaySessionID(sessionID string) bool {
	return strings.HasPrefix(sessionID, "mfpay_") && len(sessionID) == len("mfpay_")+36
}

func validMitteInvoiceSessionID(sessionID string) bool {
	return strings.HasPrefix(sessionID, "mfinvoice_") && len(sessionID) == len("mfinvoice_")+36
}

func validMittePayCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, character := range code {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func mittePayCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

func (a *App) mittePayCodeHash(sessionID, email, code string) []byte {
	mac := hmac.New(sha256.New, []byte(a.mittePayOTPSecret))
	_, _ = mac.Write([]byte(sessionID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(strings.ToLower(strings.TrimSpace(email))))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(code))
	return mac.Sum(nil)
}
