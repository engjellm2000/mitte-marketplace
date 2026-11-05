package commerce

import "testing"

func TestValidCheckoutInputRejectsMalformedEmailAndControlCharacters(t *testing.T) {
	input := validCheckoutInput()
	for _, field := range []struct {
		name  string
		apply func(*CheckoutInput)
	}{
		{name: "display-name email", apply: func(in *CheckoutInput) { in.Email = "Buyer <buyer@example.test>" }},
		{name: "missing email domain", apply: func(in *CheckoutInput) { in.Email = "buyer@" }},
		{name: "control character in name", apply: func(in *CheckoutInput) { in.Name = "Buyer\x00Name" }},
		{name: "control character in address", apply: func(in *CheckoutInput) { in.BillingAddress.AddressLine1 = "Example\x1fStreet" }},
		{name: "invalid country code", apply: func(in *CheckoutInput) { in.BillingAddress.CountryCode = "D1" }},
	} {
		t.Run(field.name, func(t *testing.T) {
			candidate := input
			field.apply(&candidate)
			if ValidCheckoutInput(candidate) {
				t.Fatal("ValidCheckoutInput accepted invalid input")
			}
		})
	}
}

func TestValidCheckoutInputAcceptsInternationalTextAndMultilineNotes(t *testing.T) {
	input := validCheckoutInput()
	input.Name = "Élise Müller"
	input.BillingAddress.City = "Köln"
	input.ShippingAddress.City = "Köln"
	input.Note = "Bitte bei der Rezeption abgeben.\nVielen Dank."
	if !ValidCheckoutInput(input) {
		t.Fatal("ValidCheckoutInput rejected valid international checkout input")
	}
}

func validCheckoutInput() CheckoutInput {
	address := Address{RecipientName: "Buyer", AddressLine1: "Example Street 1", PostalCode: "12345", City: "Hamburg", CountryCode: "DE"}
	return CheckoutInput{
		Name: "Buyer", Email: "buyer@example.test", PaymentMethod: "stripe", CustomerType: "b2c", ProcurementMethod: "online_payment",
		BillingAddress: address, ShippingAddress: address,
	}
}
