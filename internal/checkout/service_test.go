package checkout

import (
	"context"
	"errors"
	"testing"

	"mitte/internal/commerce"
)

type fakeOrders struct {
	result          commerce.CheckoutResult
	checkoutCalls   int
	retryableCalls  int
	attachedSession string
	attachedURL     string
}

func (f *fakeOrders) CheckoutForCustomer(_ context.Context, _, _ string, _ int64, input commerce.CheckoutInput) (commerce.CheckoutResult, error) {
	f.checkoutCalls++
	if f.result.OrderID == "" {
		f.result = commerce.CheckoutResult{
			OrderID: "123", TotalCents: 5000,
			LineItems:     []commerce.CheckoutLineItem{{Name: "Item", AmountCents: 5000, Quantity: 1}},
			PaymentMethod: input.PaymentMethod, SagaStatus: "started",
		}
	}
	return f.result, nil
}

func (f *fakeOrders) AttachCheckoutSession(_ context.Context, _, sessionID, checkoutURL string) error {
	f.attachedSession, f.attachedURL = sessionID, checkoutURL
	f.result.CheckoutSessionID, f.result.CheckoutURL = sessionID, checkoutURL
	f.result.SagaStatus = "awaiting_payment"
	return nil
}

func (f *fakeOrders) ConfigureTransferCheckout(_ context.Context, _, sessionID, checkoutURL, _ string, _ int) error {
	return f.AttachCheckoutSession(context.Background(), "", sessionID, checkoutURL)
}

func (f *fakeOrders) ConfigureInvoiceSandbox(_ context.Context, _, sessionID, checkoutURL string) error {
	return f.AttachCheckoutSession(context.Background(), "", sessionID, checkoutURL)
}

func (f *fakeOrders) MarkCheckoutSagaRetryable(_ context.Context, _, _ string) error {
	f.retryableCalls++
	f.result.SagaStatus = "retryable"
	return nil
}

type fakeGateway struct {
	calls int
	err   error
}

func (f *fakeGateway) CreateCheckoutSession(_ context.Context, request PaymentRequest) (string, string, error) {
	f.calls++
	if f.err != nil {
		err := f.err
		f.err = nil
		return "", "", err
	}
	if request.OrderID != "123" || len(request.LineItems) != 1 {
		return "", "", errors.New("invalid payment request")
	}
	return "cs_123", "https://payments.example/cs_123", nil
}

func TestStripeCheckoutRetryResumesSameSaga(t *testing.T) {
	orders := &fakeOrders{}
	gateway := &fakeGateway{err: errors.New("temporary provider failure")}
	service := NewService(orders, Settings{Stripe: gateway})
	service.newID = func(prefix string) (string, error) { return prefix + "fixed", nil }
	input := validInput("stripe")

	if _, err := service.Execute(context.Background(), "session", "idempotency-key-123456", 1, input); err == nil {
		t.Fatal("first provider failure was swallowed")
	}
	if orders.retryableCalls != 1 || orders.result.SagaStatus != "retryable" {
		t.Fatalf("failed saga was not persisted as retryable: %+v", orders)
	}

	result, err := service.Execute(context.Background(), "session", "idempotency-key-123456", 1, input)
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if orders.checkoutCalls != 2 || gateway.calls != 2 {
		t.Fatalf("expected same idempotent order to be retried, checkout calls=%d provider calls=%d", orders.checkoutCalls, gateway.calls)
	}
	if result.OrderID != "123" || result.CheckoutSessionID != "cs_123" || result.CheckoutURL != "https://payments.example/cs_123" {
		t.Fatalf("unexpected checkout result: %+v", result)
	}
	if orders.result.SagaStatus != "awaiting_payment" {
		t.Fatalf("successful retry did not advance saga: %s", orders.result.SagaStatus)
	}
}

func TestUnavailablePaymentDoesNotCreateOrder(t *testing.T) {
	orders := &fakeOrders{}
	service := NewService(orders, Settings{})
	_, err := service.Execute(context.Background(), "session", "idempotency-key-123456", 1,
		validInput("stripe"))
	if !errors.Is(err, ErrPaymentUnavailable) {
		t.Fatalf("expected unavailable payment error, got %v", err)
	}
	if orders.checkoutCalls != 0 {
		t.Fatalf("created order before checking provider availability: calls=%d", orders.checkoutCalls)
	}
}

func validInput(paymentMethod string) commerce.CheckoutInput {
	address := commerce.Address{
		RecipientName: "Buyer", AddressLine1: "Example Street 1",
		PostalCode: "12345", City: "Hamburg", CountryCode: "DE",
	}
	return commerce.CheckoutInput{
		Name: "Buyer", Email: "buyer@example.test", PaymentMethod: paymentMethod,
		BillingAddress: address, ShippingAddress: address,
	}
}
