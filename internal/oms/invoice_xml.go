package oms

import (
	"encoding/xml"
	"fmt"
	"math"
	"strings"
	"time"

	"mitte/internal/store"
)

const (
	ublInvoiceNamespace = "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
	ublCACNamespace     = "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
	ublCBCNamespace     = "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
)

type ublAmount struct {
	Currency string `xml:"currencyID,attr"`
	Value    string `xml:",chardata"`
}

type ublInvoice struct {
	XMLName            xml.Name         `xml:"Invoice"`
	XMLNS              string           `xml:"xmlns,attr"`
	XMLNSCAC           string           `xml:"xmlns:cac,attr"`
	XMLNSCBC           string           `xml:"xmlns:cbc,attr"`
	CustomizationID    string           `xml:"cbc:CustomizationID"`
	ProfileID          string           `xml:"cbc:ProfileID"`
	ID                 string           `xml:"cbc:ID"`
	IssueDate          string           `xml:"cbc:IssueDate"`
	DueDate            string           `xml:"cbc:DueDate"`
	InvoiceTypeCode    string           `xml:"cbc:InvoiceTypeCode"`
	DocumentCurrency   string           `xml:"cbc:DocumentCurrencyCode"`
	BuyerReference     string           `xml:"cbc:BuyerReference"`
	Supplier           ublSupplier      `xml:"cac:AccountingSupplierParty"`
	Customer           ublCustomer      `xml:"cac:AccountingCustomerParty"`
	PaymentMeans       *ublPaymentMeans `xml:"cac:PaymentMeans,omitempty"`
	PaymentTerms       *ublPaymentTerms `xml:"cac:PaymentTerms,omitempty"`
	TaxTotal           ublTaxTotal      `xml:"cac:TaxTotal"`
	LegalMonetaryTotal ublLegalTotal    `xml:"cac:LegalMonetaryTotal"`
	InvoiceLines       []ublInvoiceLine `xml:"cac:InvoiceLine"`
}

type ublSupplier struct {
	Party ublParty `xml:"cac:Party"`
}

type ublCustomer struct {
	Party ublParty `xml:"cac:Party"`
}

type ublParty struct {
	EndpointID  *ublEndpointID   `xml:"cbc:EndpointID,omitempty"`
	PartyName   ublPartyName     `xml:"cac:PartyName"`
	Postal      ublPostalAddress `xml:"cac:PostalAddress"`
	TaxScheme   *ublPartyTax     `xml:"cac:PartyTaxScheme,omitempty"`
	LegalEntity ublLegalEntity   `xml:"cac:PartyLegalEntity"`
	Contact     *ublContact      `xml:"cac:Contact,omitempty"`
}

type ublEndpointID struct {
	SchemeID string `xml:"schemeID,attr"`
	Value    string `xml:",chardata"`
}

type ublPartyName struct {
	Name string `xml:"cbc:Name"`
}

type ublPostalAddress struct {
	StreetName string     `xml:"cbc:StreetName"`
	CityName   string     `xml:"cbc:CityName"`
	PostalZone string     `xml:"cbc:PostalZone"`
	Country    ublCountry `xml:"cac:Country"`
}

type ublCountry struct {
	IdentificationCode string `xml:"cbc:IdentificationCode"`
}

type ublPartyTax struct {
	CompanyID string       `xml:"cbc:CompanyID"`
	Scheme    ublTaxScheme `xml:"cac:TaxScheme"`
}

type ublTaxScheme struct {
	ID string `xml:"cbc:ID"`
}

type ublLegalEntity struct {
	RegistrationName string `xml:"cbc:RegistrationName"`
}

type ublContact struct {
	ElectronicMail string `xml:"cbc:ElectronicMail"`
}

type ublPaymentMeans struct {
	Code         string              `xml:"cbc:PaymentMeansCode"`
	PayeeAccount ublFinancialAccount `xml:"cac:PayeeFinancialAccount"`
}

type ublFinancialAccount struct {
	ID string `xml:"cbc:ID"`
}

type ublPaymentTerms struct {
	Note string `xml:"cbc:Note"`
}

type ublTaxTotal struct {
	TaxAmount ublAmount     `xml:"cbc:TaxAmount"`
	Subtotals []ublTaxBreak `xml:"cac:TaxSubtotal"`
}

type ublTaxBreak struct {
	TaxableAmount ublAmount      `xml:"cbc:TaxableAmount"`
	TaxAmount     ublAmount      `xml:"cbc:TaxAmount"`
	Category      ublTaxCategory `xml:"cac:TaxCategory"`
}

type ublTaxCategory struct {
	ID        string       `xml:"cbc:ID"`
	Percent   string       `xml:"cbc:Percent"`
	TaxScheme ublTaxScheme `xml:"cac:TaxScheme"`
}

type ublLegalTotal struct {
	LineExtensionAmount ublAmount  `xml:"cbc:LineExtensionAmount"`
	TaxExclusiveAmount  ublAmount  `xml:"cbc:TaxExclusiveAmount"`
	TaxInclusiveAmount  ublAmount  `xml:"cbc:TaxInclusiveAmount"`
	PrepaidAmount       *ublAmount `xml:"cbc:PrepaidAmount,omitempty"`
	PayableAmount       ublAmount  `xml:"cbc:PayableAmount"`
}

type ublInvoiceLine struct {
	ID            string      `xml:"cbc:ID"`
	Quantity      ublQuantity `xml:"cbc:InvoicedQuantity"`
	LineExtension ublAmount   `xml:"cbc:LineExtensionAmount"`
	Item          ublItem     `xml:"cac:Item"`
	Price         ublPrice    `xml:"cac:Price"`
}

type ublQuantity struct {
	UnitCode string `xml:"unitCode,attr"`
	Value    string `xml:",chardata"`
}

type ublItem struct {
	Name        string         `xml:"cbc:Name"`
	TaxCategory ublTaxCategory `xml:"cac:ClassifiedTaxCategory"`
}

type ublPrice struct {
	PriceAmount ublAmount `xml:"cbc:PriceAmount"`
}

// InvoiceXML emits a UBL 2.1 invoice from the immutable order snapshot.
// This is not an XRechnung-conformance claim; production deployments must
// validate and map the output against their target e-invoice profile.
func InvoiceXML(detail store.OrderManagementDetail, seller SellerProfile) ([]byte, error) {
	order := detail.Order
	if !isValidInvoiceSeller(seller) || order.Invoice == nil {
		return nil, fmt.Errorf("invoice seller or issued invoice is incomplete")
	}
	if order.TotalCents < 0 || order.TaxCents < 0 || order.TaxCents > order.TotalCents {
		return nil, fmt.Errorf("invoice totals are inconsistent")
	}
	taxable := order.TotalCents - order.TaxCents
	invoice := ublInvoice{
		XMLNS:            ublInvoiceNamespace,
		XMLNSCAC:         ublCACNamespace,
		XMLNSCBC:         ublCBCNamespace,
		CustomizationID:  "urn:cen.eu:en16931:2017",
		ProfileID:        "urn:cen.eu:en16931:2017",
		ID:               order.Invoice.Number,
		IssueDate:        order.Invoice.IssuedAt.UTC().Format(time.DateOnly),
		DueDate:          order.Invoice.DueAt.UTC().Format(time.DateOnly),
		InvoiceTypeCode:  "380",
		DocumentCurrency: "EUR",
		BuyerReference:   strings.TrimSpace(order.BuyerReference),
		Supplier:         ublSupplier{Party: partyForSeller(seller)},
		Customer:         ublCustomer{Party: partyForBuyer(order)},
		TaxTotal: ublTaxTotal{
			TaxAmount: ublAmount{Currency: "EUR", Value: amountText(order.TaxCents)},
			Subtotals: []ublTaxBreak{{
				TaxableAmount: ublAmount{Currency: "EUR", Value: amountText(taxable)},
				TaxAmount:     ublAmount{Currency: "EUR", Value: amountText(order.TaxCents)},
				Category:      ublTaxCategory{ID: "S", Percent: "19", TaxScheme: ublTaxScheme{ID: "VAT"}},
			}},
		},
		LegalMonetaryTotal: ublLegalTotal{
			LineExtensionAmount: ublAmount{Currency: "EUR", Value: amountText(taxable)},
			TaxExclusiveAmount:  ublAmount{Currency: "EUR", Value: amountText(taxable)},
			TaxInclusiveAmount:  ublAmount{Currency: "EUR", Value: amountText(order.TotalCents)},
			PayableAmount:       ublAmount{Currency: "EUR", Value: amountText(order.TotalCents)},
		},
		InvoiceLines: make([]ublInvoiceLine, 0, len(order.Items)+1),
	}
	if invoice.BuyerReference == "" {
		invoice.BuyerReference = order.ID
	}
	unpaid := order.Status != "paid" && !(order.Status == "fulfilled" && order.PaymentProvider != "mitte_invoice")
	if seller.IBAN != "" && unpaid {
		invoice.PaymentMeans = &ublPaymentMeans{Code: "58", PayeeAccount: ublFinancialAccount{ID: strings.ReplaceAll(seller.IBAN, " ", "")}}
	}
	if seller.PaymentTerms != "" {
		invoice.PaymentTerms = &ublPaymentTerms{Note: seller.PaymentTerms}
	}
	if !unpaid {
		invoice.LegalMonetaryTotal.PrepaidAmount = &ublAmount{Currency: "EUR", Value: amountText(order.TotalCents)}
		invoice.LegalMonetaryTotal.PayableAmount.Value = amountText(0)
	}
	lineAmounts, err := allocateInvoiceLineAmounts(order, taxable)
	if err != nil {
		return nil, err
	}
	for i, item := range order.Items {
		if item.Quantity < 1 {
			return nil, fmt.Errorf("invoice contains an invalid quantity")
		}
		invoice.InvoiceLines = append(invoice.InvoiceLines, newUBLLine(fmt.Sprint(i+1), item.Name, item.Quantity, lineAmounts[i]))
	}
	if order.ShippingCents > 0 {
		invoice.InvoiceLines = append(invoice.InvoiceLines, newUBLLine("SHIPPING", "Versand", 1, lineAmounts[len(lineAmounts)-1]))
	}
	output, err := xml.MarshalIndent(invoice, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode UBL invoice: %w", err)
	}
	return append([]byte(xml.Header), output...), nil
}

func partyForSeller(seller SellerProfile) ublParty {
	party := ublParty{
		EndpointID:  &ublEndpointID{SchemeID: "EM", Value: seller.Email},
		PartyName:   ublPartyName{Name: seller.Name},
		Postal:      ublPostalAddress{StreetName: seller.Street, CityName: seller.City, PostalZone: seller.PostalCode, Country: ublCountry{IdentificationCode: seller.CountryCode}},
		TaxScheme:   &ublPartyTax{CompanyID: seller.VATID, Scheme: ublTaxScheme{ID: "VAT"}},
		LegalEntity: ublLegalEntity{RegistrationName: seller.Name},
		Contact:     &ublContact{ElectronicMail: seller.Email},
	}
	return party
}

func partyForBuyer(order store.Order) ublParty {
	name := strings.TrimSpace(order.CompanyName)
	if name == "" {
		name = strings.TrimSpace(order.CustomerName)
	}
	party := ublParty{
		PartyName:   ublPartyName{Name: name},
		Postal:      ublPostalAddress{StreetName: order.BillingAddress.AddressLine1, CityName: order.BillingAddress.City, PostalZone: order.BillingAddress.PostalCode, Country: ublCountry{IdentificationCode: order.BillingAddress.CountryCode}},
		LegalEntity: ublLegalEntity{RegistrationName: name},
	}
	if strings.TrimSpace(order.VATID) != "" {
		party.TaxScheme = &ublPartyTax{CompanyID: strings.TrimSpace(order.VATID), Scheme: ublTaxScheme{ID: "VAT"}}
	}
	return party
}

func newUBLLine(id, name string, quantity, cents int) ublInvoiceLine {
	return ublInvoiceLine{
		ID:            id,
		Quantity:      ublQuantity{UnitCode: "C62", Value: fmt.Sprint(quantity)},
		LineExtension: ublAmount{Currency: "EUR", Value: amountText(cents)},
		Item: ublItem{
			Name:        name,
			TaxCategory: ublTaxCategory{ID: "S", Percent: "19", TaxScheme: ublTaxScheme{ID: "VAT"}},
		},
		Price: ublPrice{PriceAmount: ublAmount{Currency: "EUR", Value: unitAmountText(cents, quantity)}},
	}
}

func allocateInvoiceLineAmounts(order store.Order, target int) ([]int, error) {
	count := len(order.Items)
	if order.ShippingCents > 0 {
		count++
	}
	if count == 0 {
		return nil, fmt.Errorf("invoice has no lines")
	}
	gross := make([]int, count)
	for i, item := range order.Items {
		gross[i] = item.UnitPriceCents * item.Quantity
	}
	if order.ShippingCents > 0 {
		gross[count-1] = order.ShippingCents
	}
	grossTotal := 0
	for _, value := range gross {
		grossTotal += value
	}
	if grossTotal == 0 {
		if target != 0 {
			return nil, fmt.Errorf("invoice line amounts do not reconcile")
		}
		return make([]int, count), nil
	}
	amounts := make([]int, count)
	remainders := make([]int, count)
	allocated := 0
	for i, value := range gross {
		product := value * target
		amounts[i] = product / grossTotal
		remainders[i] = product % grossTotal
		allocated += amounts[i]
	}
	for allocated < target {
		best := 0
		for i := 1; i < len(remainders); i++ {
			if remainders[i] > remainders[best] {
				best = i
			}
		}
		amounts[best]++
		remainders[best] = -1
		allocated++
	}
	return amounts, nil
}

func amountText(cents int) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

func unitAmountText(cents, quantity int) string {
	scaled := int(math.Round(float64(cents*100) / float64(quantity)))
	return fmt.Sprintf("%d.%04d", scaled/10000, scaled%10000)
}

func isValidInvoiceSeller(seller SellerProfile) bool {
	return seller.Valid()
}
