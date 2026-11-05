package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"mitte/internal/payments"
	"mitte/internal/store"
)

func (a *App) omsConsole(w http.ResponseWriter, r *http.Request) {
	a.renderApp(w, r, "OMS - Mitte")
}

func (a *App) operations(w http.ResponseWriter, r *http.Request) {
	a.renderApp(w, r, "Lager & Versand - Mitte")
}

func (a *App) inventoryConsole(w http.ResponseWriter, r *http.Request) {
	a.renderApp(w, r, "Bestandsverwaltung - Mitte")
}

func (a *App) paymentPortal(w http.ResponseWriter, r *http.Request) {
	a.renderApp(w, r, "Zahlungseingänge - Mitte")
}

func (a *App) paymentTransactions(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	transactions, err := a.store.PaymentTransactions(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	recipient := strings.TrimSpace(a.invoiceSeller.Name)
	if recipient == "" {
		recipient = a.mitteTransferRecipient
	}
	iban := strings.TrimSpace(a.invoiceSeller.IBAN)
	if iban == "" {
		iban = a.mitteTransferIBAN
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"transactions": transactions,
		"bank": map[string]string{
			"recipient": recipient,
			"iban":      iban,
			"terms":     a.invoiceSeller.PaymentTerms,
		},
	})
}

func (a *App) paymentReceipts(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	receipts, err := a.store.PaymentReceipts(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"receipts": receipts})
}

func (a *App) createPaymentReceipt(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	var input struct {
		AmountCents int    `json:"amountCents"`
		PayerName   string `json:"payerName"`
		IBANLast4   string `json:"ibanLast4"`
		Note        string `json:"note"`
	}
	if err := decodeJSON(w, r, &input, 2048); err != nil || !validTransferReceipt(&struct {
		PayerName string `json:"payerName"`
		IBANLast4 string `json:"ibanLast4"`
		Note      string `json:"note"`
	}{input.PayerName, input.IBANLast4, input.Note}) || input.AmountCents < 1 || input.AmountCents > 10_000_000 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe Betrag, Zahler und die letzten vier IBAN-Ziffern."})
		return
	}
	reference := "ZE-" + time.Now().UTC().Format("20060102-150405.000000000")
	receipt, err := a.store.CreatePaymentReceipt(r.Context(), reference, input.AmountCents, strings.TrimSpace(input.PayerName), strings.TrimSpace(input.IBANLast4), strings.TrimSpace(input.Note), user.Email)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"receipt": receipt})
}

func (a *App) replacePaymentReceiptAllocations(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	receiptID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || receiptID < 1 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültiger Zahlungseingang."})
		return
	}
	var input struct {
		Allocations []struct {
			OrderID     string `json:"orderId"`
			AmountCents int    `json:"amountCents"`
		} `json:"allocations"`
	}
	if err := decodeJSON(w, r, &input, 8192); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe die Zuordnungen."})
		return
	}
	allocations := make([]store.PaymentReceiptAllocation, 0, len(input.Allocations))
	for _, allocation := range input.Allocations {
		allocations = append(allocations, store.PaymentReceiptAllocation{OrderID: strings.TrimSpace(allocation.OrderID), AmountCents: allocation.AmountCents})
	}
	if err := a.store.ReplacePaymentReceiptAllocations(r.Context(), receiptID, allocations); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "Zahlungseingang oder Rechnung nicht gefunden."})
			return
		}
		if errors.Is(err, store.ErrInvalidCheckout) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Die Zuordnung überschreitet den Zahlungseingang oder Rechnungsbetrag."})
			return
		}
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "Zahlung wurde neu zugeordnet."})
}

func (a *App) receiveTransferPayment(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	var input struct {
		PayerName string `json:"payerName"`
		IBANLast4 string `json:"ibanLast4"`
		Note      string `json:"note"`
	}
	if err := decodeJSON(w, r, &input, 2048); err != nil || !validTransferReceipt(&input) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe Zahler, die letzten vier IBAN-Ziffern und die Notiz."})
		return
	}
	if err := a.store.SetTransferPaymentReceived(r.Context(), r.PathValue("id"), input.PayerName, input.IBANLast4, input.Note, user.Email); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "Zahlung nicht gefunden."})
			return
		}
		if errors.Is(err, store.ErrInvalidTransition) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Diese Zahlung kann nicht bestätigt werden."})
			return
		}
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "Zahlungseingang wurde bestätigt."})
}

func validTransferReceipt(input *struct {
	PayerName string `json:"payerName"`
	IBANLast4 string `json:"ibanLast4"`
	Note      string `json:"note"`
}) bool {
	input.PayerName = strings.TrimSpace(input.PayerName)
	input.IBANLast4 = strings.TrimSpace(input.IBANLast4)
	input.Note = strings.TrimSpace(input.Note)
	if input.PayerName == "" || len([]rune(input.PayerName)) > 160 || len([]rune(input.Note)) > 1000 {
		return false
	}
	if len(input.IBANLast4) != 4 {
		return false
	}
	for _, r := range input.IBANLast4 {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (a *App) operationsOrders(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	query := r.URL.Query()
	// Order records include customer and payment information; browsers and shared
	// intermediaries must never retain an operations response.
	w.Header().Set("Cache-Control", "private, no-store")
	paged := query.Has("q") || query.Has("status") || query.Has("page") || query.Has("limit")
	if paged {
		search := strings.TrimSpace(query.Get("q"))
		status := query.Get("status")
		if status == "" {
			status = "all"
		}
		if utf8.RuneCountInString(search) > maxOperationsOrderSearchLength || !validOperationsOrderStatus(status) {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Auftragsabfrage."})
			return
		}
		page, limit, offset, err := parseOperationsOrderPage(query.Get("page"), query.Get("limit"))
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Auftragsseite."})
			return
		}
		orders, total, err := a.store.OrdersForOperationsPage(r.Context(), search, status, limit, offset)
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		summary, err := a.store.OperationsOrderSummary(r.Context())
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		pages := (total + limit - 1) / limit
		if pages == 0 {
			pages = 1
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"orders": orders, "total": total, "page": page, "pageSize": limit, "pages": pages, "summary": summary,
		})
		return
	}
	orders, err := a.store.OrdersForOperations(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"orders": orders})
}

func (a *App) operationsBacklog(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	query := r.URL.Query()
	search := strings.TrimSpace(query.Get("q"))
	stage := query.Get("stage")
	if stage == "" {
		stage = "all"
	}
	if utf8.RuneCountInString(search) > maxOperationsOrderSearchLength || !validOperationsBacklogStage(stage) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Rückstandsabfrage."})
		return
	}
	page, limit, offset, err := parseOperationsOrderPage(query.Get("page"), query.Get("limit"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Rückstandsseite."})
		return
	}
	items, total, summary, err := a.store.OperationsBacklogPage(r.Context(), search, stage, limit, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	pages := (total + limit - 1) / limit
	if pages < 1 {
		pages = 1
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "page": page, "pageSize": limit, "pages": pages, "summary": summary,
	})
}

func (a *App) operationsOrderMatrix(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	query := r.URL.Query()
	search := strings.TrimSpace(query.Get("q"))
	kind := query.Get("kind")
	if utf8.RuneCountInString(search) > maxOperationsOrderSearchLength || !validOperationsMatrixKind(kind) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Auftragsmatrix."})
		return
	}
	page, limit, offset, err := parseOperationsOrderPage(query.Get("page"), query.Get("limit"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Auftragsseite."})
		return
	}
	orders, total, err := a.store.OrdersForOperationsMatrixPage(r.Context(), search, kind, limit, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	pages := (total + limit - 1) / limit
	if pages == 0 {
		pages = 1
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{
		"orders": orders, "total": total, "page": page, "pageSize": limit, "pages": pages,
	})
}

func (a *App) operationsOrderItems(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	query := r.URL.Query()
	search := strings.TrimSpace(query.Get("q"))
	status := query.Get("status")
	if status == "" {
		status = "all"
	}
	if utf8.RuneCountInString(search) > maxOperationsOrderSearchLength || !validOperationsOrderStatus(status) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Positionsabfrage."})
		return
	}
	page, limit, offset, err := parseOperationsOrderPage(query.Get("page"), query.Get("limit"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Positionsseite."})
		return
	}
	items, total, err := a.store.OrderItemsForOperationsPage(r.Context(), search, status, limit, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	pages := (total + limit - 1) / limit
	if pages == 0 {
		pages = 1
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "page": page, "pageSize": limit, "pages": pages,
	})
}

func (a *App) operationsReturnRequests(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		return
	}
	query := r.URL.Query()
	search := strings.TrimSpace(query.Get("q"))
	status := query.Get("status")
	if status == "" {
		status = "all"
	}
	if utf8.RuneCountInString(search) > maxOperationsOrderSearchLength || !validOperationsReturnRequestStatus(status) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Anfrageabfrage."})
		return
	}
	page, limit, offset, err := parseOperationsOrderPage(query.Get("page"), query.Get("limit"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Anfrageseite."})
		return
	}
	requests, total, err := a.store.OrderItemRequestsForOperationsPage(r.Context(), search, status, limit, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	pages := (total + limit - 1) / limit
	if pages == 0 {
		pages = 1
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{
		"requests": requests, "total": total, "page": page, "pageSize": limit, "pages": pages,
	})
}

func (a *App) operationsShipmentDelays(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		return
	}
	query := r.URL.Query()
	search := strings.TrimSpace(query.Get("q"))
	if utf8.RuneCountInString(search) > maxOperationsOrderSearchLength {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Verzögerungssuche."})
		return
	}
	page, limit, offset, err := parseOperationsOrderPage(query.Get("page"), query.Get("limit"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Verzögerungsseite."})
		return
	}
	delays, total, err := a.store.ShipmentDelaysForOperationsPage(r.Context(), search, limit, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	pages := (total + limit - 1) / limit
	if pages == 0 {
		pages = 1
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{
		"delays": delays, "total": total, "page": page, "pageSize": limit, "pages": pages,
	})
}

func validOperationsReturnRequestStatus(status string) bool {
	switch status {
	case "all", "return_open", "replacement_requested":
		return true
	default:
		return false
	}
}

func validOperationsMatrixKind(kind string) bool {
	switch kind {
	case "fulfillment", "fulfillment-clarify", "payment", "payment-clarify", "old-orders", "old-ops-clarify":
		return true
	default:
		return false
	}
}

func (a *App) operationsCustomers(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		return
	}
	query := r.URL.Query()
	search := strings.TrimSpace(query.Get("q"))
	if utf8.RuneCountInString(search) > maxOperationsCustomerSearchLength {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Kundensuche."})
		return
	}
	page, limit, offset, err := parseOperationsCustomerPage(query.Get("page"), query.Get("limit"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Kundenseite."})
		return
	}
	customers, total, err := a.store.CustomersForOperationsPage(r.Context(), search, limit, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	pages := (total + limit - 1) / limit
	if pages == 0 {
		pages = 1
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{
		"customers": customers, "total": total, "page": page, "pageSize": limit, "pages": pages,
	})
}

func (a *App) setCustomerFraudFlag(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	staff, ok := a.currentStaff(w, r)
	if !ok {
		return
	}
	username := strings.TrimPrefix(staff.Email, "staff:")
	if a.staffRoles[username] != "admin" {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Nur Admins dürfen Fraud-Markierungen verwalten."})
		return
	}
	customerID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || customerID < 1 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültiges Kundenkonto."})
		return
	}
	var input struct {
		Flagged *bool  `json:"flagged"`
		Reason  string `json:"reason"`
	}
	if err := decodeJSON(w, r, &input, 4096); err != nil || input.Flagged == nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe Status und Begründung."})
		return
	}
	status, err := a.store.SetCustomerFraudFlag(r.Context(), customerID, *input.Flagged, input.Reason, staff.Email)
	switch {
	case errors.Is(err, store.ErrNotFound):
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Kundenkonto nicht gefunden."})
	case errors.Is(err, store.ErrInvalidCheckout):
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte gib eine Begründung mit 5 bis 500 Zeichen ein."})
	case err != nil:
		a.serverError(w, r, err)
	default:
		respondJSON(w, http.StatusOK, map[string]any{"fraud": status})
	}
}

func (a *App) operationsEmailDeliveries(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		return
	}
	query := r.URL.Query()
	search := strings.TrimSpace(query.Get("q"))
	status := query.Get("status")
	if status == "" {
		status = "all"
	}
	if utf8.RuneCountInString(search) > maxOperationsOrderSearchLength || !validOperationsEmailDeliveryStatus(status) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige E-Mail-Abfrage."})
		return
	}
	page, limit, offset, err := parseOperationsCustomerPage(query.Get("page"), query.Get("limit"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige E-Mail-Seite."})
		return
	}
	deliveries, total, err := a.store.EmailDeliveriesForOperationsPage(r.Context(), search, status, limit, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	pages := (total + limit - 1) / limit
	if pages == 0 {
		pages = 1
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{
		"deliveries": deliveries, "total": total, "page": page, "pageSize": limit, "pages": pages,
	})
}

func validOperationsEmailDeliveryStatus(status string) bool {
	switch status {
	case "all", "pending", "sending", "retrying", "sent", "cancelled":
		return true
	default:
		return false
	}
}

func (a *App) operationsCustomerOrders(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		return
	}
	customerID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || customerID < 1 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültiges Kundenkonto."})
		return
	}
	page, limit, offset, err := parseOperationsCustomerPage(r.URL.Query().Get("page"), r.URL.Query().Get("limit"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Auftragsseite."})
		return
	}
	orders, total, err := a.store.OrdersForCustomerOperationsPage(r.Context(), customerID, limit, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	pages := (total + limit - 1) / limit
	if pages == 0 {
		pages = 1
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{
		"orders": orders, "total": total, "page": page, "pageSize": limit, "pages": pages,
	})
}

const (
	defaultOperationsCustomerPageSize = 25
	maxOperationsCustomerPageSize     = 100
	maxOperationsCustomerPage         = 10000
	maxOperationsCustomerOffset       = 100000
	maxOperationsCustomerSearchLength = 120
)

func parseOperationsCustomerPage(pageValue, limitValue string) (page, limit, offset int, err error) {
	page = 1
	if pageValue != "" {
		page, err = strconv.Atoi(pageValue)
		if err != nil || page < 1 || page > maxOperationsCustomerPage {
			return 0, 0, 0, errors.New("invalid operations customer page")
		}
	}
	limit = defaultOperationsCustomerPageSize
	if limitValue != "" {
		limit, err = strconv.Atoi(limitValue)
		if err != nil || limit < 1 || limit > maxOperationsCustomerPageSize {
			return 0, 0, 0, errors.New("invalid operations customer page size")
		}
	}
	if page-1 > maxOperationsCustomerOffset/limit {
		return 0, 0, 0, errors.New("operations customer offset exceeds maximum")
	}
	return page, limit, (page - 1) * limit, nil
}

const (
	defaultOperationsOrderPageSize = 25
	maxOperationsOrderPageSize     = 100
	maxOperationsOrderPage         = 10000
	maxOperationsOrderOffset       = 100000
	maxOperationsOrderSearchLength = 120
)

func parseOperationsOrderPage(pageValue, limitValue string) (page, limit, offset int, err error) {
	page = 1
	if pageValue != "" {
		page, err = strconv.Atoi(pageValue)
		if err != nil || page < 1 || page > maxOperationsOrderPage {
			return 0, 0, 0, errors.New("invalid operations order page")
		}
	}
	limit = defaultOperationsOrderPageSize
	if limitValue != "" {
		limit, err = strconv.Atoi(limitValue)
		if err != nil || limit < 1 || limit > maxOperationsOrderPageSize {
			return 0, 0, 0, errors.New("invalid operations order page size")
		}
	}
	if page-1 > maxOperationsOrderOffset/limit {
		return 0, 0, 0, errors.New("operations order offset exceeds maximum")
	}
	offset = (page - 1) * limit
	return page, limit, offset, nil
}

func validOperationsOrderStatus(status string) bool {
	switch status {
	case "all", "open", "paid", "pending_payment", "invoice_approved", "fulfilled", "cancelled":
		return true
	default:
		return false
	}
}

func validOperationsBacklogStage(stage string) bool {
	switch stage {
	case "all", "payment", "needs_fulfillment", "in_progress", "held":
		return true
	default:
		return false
	}
}

func (a *App) operationOrderDetail(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	detail, err := a.store.OrderManagementDetail(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Bestellung nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"detail": detail})
}

func (a *App) addOperationNote(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	var input struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(w, r, &input, 4096); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe die interne Notiz."})
		return
	}
	note, err := a.store.AddInternalOrderNote(r.Context(), r.PathValue("id"), user.Email, input.Body)
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Bestellung nicht gefunden."})
		return
	}
	if errors.Is(err, store.ErrInvalidCheckout) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Die interne Notiz muss zwischen 1 und 2.000 Zeichen lang sein."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	a.logger.Info("order internal note added", "request_id", requestID(r.Context()), "order_id", r.PathValue("id"), "author", user.Email)
	respondJSON(w, http.StatusCreated, map[string]any{"note": note})
}

func (a *App) updateOrderItemOperation(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	var input struct {
		ProductName string `json:"productName"`
		Action      string `json:"action"`
		Detail      string `json:"detail"`
	}
	if err := decodeJSON(w, r, &input, 2048); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe die Positionsaktion."})
		return
	}
	if err := a.store.UpdateOrderItemOperation(r.Context(), r.PathValue("id"), strings.TrimSpace(input.ProductName), strings.TrimSpace(input.Action), strings.TrimSpace(input.Detail), user.Email); err != nil {
		if errors.Is(err, store.ErrCustomerFraudBlocked) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Dieses Kundenkonto ist für eine Fraud-Prüfung gesperrt; die Auftragsbearbeitung ist angehalten."})
			return
		}
		if errors.Is(err, store.ErrInvalidCheckout) {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Positionsaktion."})
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "Bestellung oder Position nicht gefunden."})
			return
		}
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "Position aktualisiert."})
}

func (a *App) applyOrderItemAction(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	var input struct {
		Action       string   `json:"action"`
		ProductNames []string `json:"productNames"`
		Reason       string   `json:"reason"`
	}
	if err := decodeJSON(w, r, &input, 8192); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe die ausgewählten Artikel und den Grund."})
		return
	}
	input.Action = strings.TrimSpace(input.Action)
	input.Reason = strings.TrimSpace(input.Reason)
	orderID := r.PathValue("id")
	detail, err := a.store.OrderManagementDetail(r.Context(), orderID)
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Bestellung nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if input.Action == "return" || input.Action == "replacement" || input.Action == "replacement_with_return" {
		if err := a.store.ApplyOrderItemAction(r.Context(), orderID, input.ProductNames, input.Action, input.Reason, user.Email); err != nil {
			if errors.Is(err, store.ErrCustomerFraudBlocked) {
				respondJSON(w, http.StatusConflict, map[string]string{"error": "Dieses Kundenkonto ist für eine Fraud-Prüfung gesperrt; die Auftragsbearbeitung ist angehalten."})
				return
			}
			if errors.Is(err, store.ErrReturnAlreadyOpen) {
				respondJSON(w, http.StatusConflict, map[string]string{"error": "Für mindestens einen Artikel wurde bereits ein Retourenavis angelegt."})
				return
			}
			if errors.Is(err, store.ErrInvalidReturnProduct) {
				respondJSON(w, http.StatusConflict, map[string]string{"error": "Für mindestens einen ausgewählten Artikel fehlt die Produktverknüpfung für den Lagerzugang."})
				return
			}
			if errors.Is(err, store.ErrInvalidCheckout) {
				respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte wähle gültige Artikel und gib einen Grund mit höchstens 500 Zeichen an."})
				return
			}
			if errors.Is(err, store.ErrInvalidTransition) {
				respondJSON(w, http.StatusConflict, map[string]string{"error": "Rückgabe- und Ersatzanfragen sind erst nach der Versandübergabe möglich."})
				return
			}
			if errors.Is(err, store.ErrNotFound) {
				respondJSON(w, http.StatusNotFound, map[string]string{"error": "Mindestens eine ausgewählte Position wurde nicht gefunden."})
				return
			}
			a.serverError(w, r, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "Anfrage wurde protokolliert und die Bestell-E-Mail in die Versandwarteschlange gestellt."})
		return
	}
	if input.Action != "cancel" && input.Action != "refund" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Diese Aktion wird nicht unterstützt."})
		return
	}
	if input.Reason == "" || len([]rune(input.Reason)) > 500 || !selectedAllOrderItems(input.ProductNames, detail.Order.Items) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Stornierung und Erstattung gelten für die gesamte Bestellung. Wähle alle Positionen und gib einen Grund an."})
		return
	}
	if input.Action == "cancel" && detail.Order.Status == "pending_payment" {
		err = a.store.CancelPendingOrderForOperations(r.Context(), orderID, input.Reason)
		if errors.Is(err, store.ErrInvalidTransition) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Diese offene Bestellung kann nicht mehr storniert werden."})
			return
		}
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "Offene Bestellung wurde storniert; reservierter Bestand wurde freigegeben."})
		return
	}
	if detail.Order.Status != "paid" || detail.Order.StripePaymentIntentID == "" {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Eine vollständige Erstattung ist nur für bezahlte, noch nicht versendete Stripe-Bestellungen verfügbar."})
		return
	}
	username := strings.TrimPrefix(user.Email, "staff:")
	if !staffRoleAllowsFinancialReversal(a.staffRoles[username]) {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Nur Admins dürfen bezahlte Bestellungen stornieren oder erstatten."})
		return
	}
	refunder, ok := a.stripe.(payments.Refunder)
	if !ok {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Stripe-Erstattungen sind nicht konfiguriert."})
		return
	}
	refundID, err := refunder.RefundPaymentIntent(r.Context(), detail.Order.StripePaymentIntentID, "oms_refund_"+orderID)
	if err != nil {
		a.logger.Error("OMS Stripe refund failed", "request_id", requestID(r.Context()), "order_id", orderID, "error", err)
		respondJSON(w, http.StatusBadGateway, map[string]string{"error": "Stripe hat die Erstattung nicht bestätigt. Bitte prüfe den Zahlungsstatus, bevor du es erneut versuchst."})
		return
	}
	if err := a.store.CancelPaidOrderForOperations(r.Context(), orderID, refundID, input.Reason); err != nil {
		a.serverError(w, r, err)
		return
	}
	documentType := "credit_note"
	if input.Action == "cancel" {
		documentType = "cancellation"
	}
	if _, err := a.store.CreateInvoiceDocument(r.Context(), orderID, documentType, input.Reason, user.Email); err != nil {
		a.logger.Error("OMS refund document creation failed", "request_id", requestID(r.Context()), "order_id", orderID, "error", err)
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "Vollständige Stripe-Erstattung wurde ausgelöst; Bestellung und Kundenbenachrichtigung wurden aktualisiert."})
}

func selectedAllOrderItems(selected []string, items []store.OrderItem) bool {
	if len(selected) != len(items) || len(items) == 0 {
		return false
	}
	names := make(map[string]struct{}, len(selected))
	for _, name := range selected {
		names[strings.TrimSpace(name)] = struct{}{}
	}
	if len(names) != len(items) {
		return false
	}
	for _, item := range items {
		if _, ok := names[item.Name]; !ok {
			return false
		}
	}
	return true
}

func (a *App) sendOrderSummaryEmail(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	detail, err := a.store.OrderManagementDetail(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Bestellung nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	var body strings.Builder
	fmt.Fprintf(&body, "Bestellübersicht %s\n\nKunde: %s\nE-Mail: %s\nStatus: %s\n",
		detail.Order.ID, detail.Order.CustomerName, detail.Order.CustomerEmail, detail.Order.Status)
	if detail.Order.CompanyName != "" {
		fmt.Fprintf(&body, "Unternehmen: %s\n", detail.Order.CompanyName)
	}
	if detail.Order.VATID != "" {
		fmt.Fprintf(&body, "USt-IdNr.: %s\n", detail.Order.VATID)
	}
	if detail.Order.BuyerReference != "" {
		fmt.Fprintf(&body, "Bestellreferenz: %s\n", detail.Order.BuyerReference)
	}
	fmt.Fprintf(&body, "\nRechnungsadresse:\n%s\n\nLieferadresse:\n%s\n\nArtikel:\n",
		orderAddressText(detail.Order.BillingAddress), orderAddressText(detail.Order.ShippingAddress))
	for _, item := range detail.Order.Items {
		fmt.Fprintf(&body, "- %d × %s (%s) · %s\n", item.Quantity, item.Name, item.SKU, euroString(item.UnitPriceCents*item.Quantity))
	}
	fmt.Fprintf(&body, "\nZwischensumme: %s\nVersand: %s\nSteuern: %s\nGesamt: %s\n",
		euroString(detail.Order.SubtotalCents), euroString(detail.Order.ShippingCents), euroString(detail.Order.TaxCents), euroString(detail.Order.TotalCents))
	recipient, err := a.store.QueueOrderSummaryEmail(r.Context(), detail.Order.ID, body.String())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]string{"message": "Bestellübersicht wurde für " + recipient + " in die E-Mail-Warteschlange gestellt."})
}

func orderAddressText(address store.Address) string {
	lines := []string{address.RecipientName, address.CompanyName, address.AddressLine1, address.AddressLine2,
		strings.TrimSpace(address.PostalCode + " " + address.City), address.CountryCode}
	filtered := lines[:0]
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

func euroString(cents int) string {
	return fmt.Sprintf("€%.2f", float64(cents)/100)
}

func (a *App) shipOrder(w http.ResponseWriter, r *http.Request) {
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
	}
	if err := decodeJSON(w, r, &input, 2048); err != nil || !validShipmentInput(&input) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe Versanddienstleister, Sendungsnummer und Tracking-Link."})
		return
	}
	if err := a.store.MarkOrderShippedBy(r.Context(), r.PathValue("id"), user.Email, input.Carrier, input.TrackingNumber, input.TrackingURL, a.invoicePublicBaseURL, a.invoiceSeller); err != nil {
		if errors.Is(err, store.ErrCustomerFraudBlocked) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Dieses Kundenkonto ist für eine Fraud-Prüfung gesperrt; der Versand ist angehalten."})
			return
		}
		if errors.Is(err, store.ErrInvalidFulfillmentHold) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Dieses Fulfillment ist angehalten und muss vor dem Versand fortgesetzt werden."})
			return
		}
		if errors.Is(err, store.ErrInvalidTransition) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Diese Bestellung wurde bereits bearbeitet und kann nicht versendet werden."})
			return
		}
		a.serverError(w, r, err)
		return
	}
	a.logger.Info("order marked shipped", "request_id", requestID(r.Context()), "order_id", r.PathValue("id"), "carrier", input.Carrier)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) currentStaff(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	username, err := a.currentStaffUsername(r)
	if errors.Is(err, store.ErrInvalidSession) {
		return store.User{}, false
	}
	if err != nil {
		a.serverError(w, r, err)
		return store.User{}, false
	}
	if !staffRoleAllows(a.staffRoles[username], staffPermissionForPath(r.URL.Path)) {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "You do not have permission to access this operation."})
		return store.User{}, false
	}
	// Operations audit fields are textual actor labels. Deliberately do not
	// create or look up a customer account for an internal credential.
	return store.User{Email: "staff:" + username}, true
}

func validShipmentInput(input *struct {
	Carrier        string `json:"carrier"`
	TrackingNumber string `json:"trackingNumber"`
	TrackingURL    string `json:"trackingUrl"`
}) bool {
	input.Carrier = strings.TrimSpace(input.Carrier)
	input.TrackingNumber = strings.TrimSpace(input.TrackingNumber)
	input.TrackingURL = strings.TrimSpace(input.TrackingURL)
	if input.Carrier == "" || input.TrackingNumber == "" || utf8.RuneCountInString(input.Carrier) > 100 || utf8.RuneCountInString(input.TrackingNumber) > 160 {
		return false
	}
	if input.TrackingURL == "" {
		return true
	}
	u, err := url.Parse(input.TrackingURL)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Fragment == "" && len(input.TrackingURL) <= 2048
}
