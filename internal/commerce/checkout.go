package commerce

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

type CheckoutInput struct {
	Name                  string  `json:"name"`
	Email                 string  `json:"email"`
	PaymentMethod         string  `json:"paymentMethod"`
	CustomerType          string  `json:"customerType"`
	CompanyName           string  `json:"companyName"`
	VATID                 string  `json:"vatId"`
	BuyerReference        string  `json:"buyerReference"`
	ProcurementMethod     string  `json:"procurementMethod"`
	Note                  string  `json:"note"`
	BillingAddress        Address `json:"billingAddress"`
	ShippingAddress       Address `json:"shippingAddress"`
	UseBillingForShipping bool    `json:"useBillingForShipping"`
}

// Address is an immutable order-time snapshot, not a mutable customer profile.
type Address struct {
	RecipientName string `json:"recipientName"`
	CompanyName   string `json:"companyName"`
	AddressLine1  string `json:"addressLine1"`
	AddressLine2  string `json:"addressLine2"`
	PostalCode    string `json:"postalCode"`
	City          string `json:"city"`
	CountryCode   string `json:"countryCode"`
}

type CheckoutLineItem struct {
	Name        string
	AmountCents int64
	Quantity    int64
}

type CheckoutResult struct {
	OrderID           string
	TotalCents        int
	LineItems         []CheckoutLineItem
	CheckoutSessionID string
	CheckoutURL       string
	PaymentMethod     string
	SagaStatus        string
}

func NormalizeCheckoutInput(input *CheckoutInput) {
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	input.PaymentMethod = strings.TrimSpace(input.PaymentMethod)
	input.CustomerType = strings.ToLower(strings.TrimSpace(input.CustomerType))
	if input.CustomerType == "" {
		input.CustomerType = "b2c"
	}
	input.CompanyName = strings.TrimSpace(input.CompanyName)
	input.VATID = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(input.VATID), " ", ""))
	input.BuyerReference = strings.TrimSpace(input.BuyerReference)
	input.ProcurementMethod = strings.ToLower(strings.TrimSpace(input.ProcurementMethod))
	if input.ProcurementMethod == "" {
		input.ProcurementMethod = "online_payment"
	}
	input.Note = strings.TrimSpace(input.Note)
	normalizeAddress(&input.BillingAddress)
	if input.UseBillingForShipping {
		input.ShippingAddress = input.BillingAddress
	}
	normalizeAddress(&input.ShippingAddress)
}

func normalizeAddress(address *Address) {
	address.RecipientName = strings.TrimSpace(address.RecipientName)
	address.CompanyName = strings.TrimSpace(address.CompanyName)
	address.AddressLine1 = strings.TrimSpace(address.AddressLine1)
	address.AddressLine2 = strings.TrimSpace(address.AddressLine2)
	address.PostalCode = strings.TrimSpace(address.PostalCode)
	address.City = strings.TrimSpace(address.City)
	address.CountryCode = strings.ToUpper(strings.TrimSpace(address.CountryCode))
	if address.CountryCode == "" {
		address.CountryCode = "DE"
	}
}

func ValidCheckoutInput(input CheckoutInput) bool {
	if !validText(input.Name, 2, 120, false) || !validEmail(input.Email) ||
		(input.PaymentMethod != "" && input.PaymentMethod != "stripe" && input.PaymentMethod != "mitte_pay" && input.PaymentMethod != "mitte_transfer" && input.PaymentMethod != "mitte_invoice") ||
		(input.CustomerType != "b2c" && input.CustomerType != "b2b") ||
		(input.ProcurementMethod != "online_payment" && input.ProcurementMethod != "purchase_order" && input.ProcurementMethod != "eprocurement") ||
		!validText(input.CompanyName, 0, 160, false) || !validText(input.VATID, 0, 32, false) ||
		!validText(input.BuyerReference, 0, 120, false) || !validText(input.Note, 0, 1000, true) ||
		!validAddress(input.BillingAddress) || !validAddress(input.ShippingAddress) {
		return false
	}
	return input.ShippingAddress.CountryCode == "DE" && (input.CustomerType != "b2b" || utf8.RuneCountInString(input.CompanyName) >= 2)
}

func validAddress(address Address) bool {
	return validText(address.RecipientName, 2, 120, false) && validText(address.CompanyName, 0, 160, false) &&
		validText(address.AddressLine1, 3, 160, false) && validText(address.AddressLine2, 0, 160, false) &&
		validText(address.PostalCode, 3, 20, false) && validText(address.City, 2, 120, false) && validCountryCode(address.CountryCode)
}

func validEmail(value string) bool {
	if !utf8.ValidString(value) || len(value) > 254 {
		return false
	}
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}

func validCountryCode(value string) bool {
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func validText(value string, minimum, maximum int, multiline bool) bool {
	if !utf8.ValidString(value) {
		return false
	}
	length := utf8.RuneCountInString(value)
	if length < minimum || length > maximum {
		return false
	}
	for _, character := range value {
		if character == 0 || character == 127 || (character < 32 && (!multiline || (character != '\n' && character != '\r' && character != '\t'))) {
			return false
		}
	}
	return true
}
