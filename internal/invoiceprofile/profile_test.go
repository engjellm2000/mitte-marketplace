package invoiceprofile

import "testing"

func TestProfileValid(t *testing.T) {
	profile := Profile{
		Name:        "Example GmbH",
		Street:      "Musterstrasse 1",
		PostalCode:  "20095",
		City:        "Hamburg",
		CountryCode: "DE",
		VATID:       "DE123456789",
		Email:       "invoice@example.de",
	}
	if !profile.Valid() {
		t.Fatal("Valid() = false for complete seller profile")
	}

	profile.Name = " "
	if profile.Valid() {
		t.Fatal("Valid() = true with blank seller name")
	}
}
