package oms

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"mitte/internal/store"
)

func TestInvoiceXMLIsWellFormedAndReconcilesTotals(t *testing.T) {
	detail := store.OrderManagementDetail{Order: store.Order{
		ID: "order-42", Status: "fulfilled", PaymentProvider: "mitte_invoice",
		TotalCents: 11900, SubtotalCents: 11410, ShippingCents: 490, TaxCents: 1900,
		CustomerName: "Buyer GmbH", CompanyName: "Buyer GmbH", VATID: "DE123456789",
		BuyerReference: "PO-2026-42",
		BillingAddress: store.Address{AddressLine1: "Buyerstrasse 8", PostalCode: "10115", City: "Berlin", CountryCode: "DE"},
		Items:          []store.OrderItem{{SKU: "MF-42", Name: "Desk lamp", UnitPriceCents: 11410, Quantity: 1}},
		Invoice:        &store.Invoice{Number: "MF-2026-order-42", IssuedAt: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), DueAt: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC), TotalCents: 11900},
	}}
	seller := SellerProfile{Name: "Mitte GmbH", Street: "Werkstrasse 1", PostalCode: "20095", City: "Hamburg", CountryCode: "DE", VATID: "DE987654321", Email: "invoice@example.test", IBAN: "DE02120300000000202051"}
	output, err := InvoiceXML(detail, seller)
	if err != nil {
		t.Fatalf("InvoiceXML() error = %v", err)
	}
	decoder := xml.NewDecoder(strings.NewReader(string(output)))
	for {
		if _, err := decoder.Token(); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("InvoiceXML is not well-formed: %v", err)
		}
	}
	text := string(output)
	for _, want := range []string{"MF-2026-order-42", "100.00", "19.00", "119.00", "Buyer GmbH", "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"} {
		if !strings.Contains(text, want) {
			t.Errorf("InvoiceXML output does not contain %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "PrepaidAmount") {
		t.Fatal("unpaid invoice unexpectedly reports a prepayment")
	}
}

func TestInvoiceXMLRequiresSellerAndIssuedInvoice(t *testing.T) {
	if _, err := InvoiceXML(store.OrderManagementDetail{}, SellerProfile{}); err == nil {
		t.Fatal("InvoiceXML() accepted a missing seller and invoice")
	}
}
