package web

import (
	"errors"
	"net/http"
	"strings"

	"mitte/internal/invoiceprofile"
	"mitte/internal/oms"
	"mitte/internal/store"
)

func (a *App) publicInvoice(w http.ResponseWriter, r *http.Request) {
	order, seller, err := a.store.InvoiceByAccessToken(r.Context(), r.PathValue("token"))
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Rechnung nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if order.Invoice == nil || order.Shipment == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Rechnung nicht gefunden."})
		return
	}
	if !validPublicInvoiceSeller(seller) {
		seller = a.invoiceSeller
	}
	if !validPublicInvoiceSeller(seller) {
		a.serverError(w, r, errors.New("invoice seller snapshot is incomplete"))
		return
	}
	detail := store.OrderManagementDetail{Order: order}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format"))) {
	case "", "pdf":
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="`+order.Invoice.Number+`.pdf"`)
		_, _ = w.Write(oms.InvoicePDF(detail, seller))
	case "xml":
		output, err := oms.InvoiceXML(detail, seller)
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+order.Invoice.Number+`.xml"`)
		_, _ = w.Write(output)
	default:
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Format muss pdf oder xml sein."})
	}
}

func validPublicInvoiceSeller(seller invoiceprofile.Profile) bool {
	return seller.Valid()
}
