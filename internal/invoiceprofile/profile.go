package invoiceprofile

import "strings"

type Profile struct {
	Name         string `json:"name"`
	Address      string `json:"address"`
	Street       string `json:"street"`
	PostalCode   string `json:"postalCode"`
	City         string `json:"city"`
	CountryCode  string `json:"countryCode"`
	VATID        string `json:"vatId"`
	Email        string `json:"email"`
	IBAN         string `json:"iban"`
	PaymentTerms string `json:"paymentTerms"`
	Demo         bool   `json:"demo"`
}

func (p Profile) Valid() bool {
	return strings.TrimSpace(p.Name) != "" &&
		strings.TrimSpace(p.Street) != "" &&
		strings.TrimSpace(p.PostalCode) != "" &&
		strings.TrimSpace(p.City) != "" &&
		len(p.CountryCode) == 2 &&
		strings.TrimSpace(p.VATID) != "" &&
		strings.TrimSpace(p.Email) != ""
}
