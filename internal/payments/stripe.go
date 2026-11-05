package payments

import (
	"context"
	"fmt"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/refund"
	"time"
)

type LineItem struct {
	Name        string
	AmountCents int64
	Quantity    int64
}

type CheckoutRequest struct {
	Context       context.Context
	OrderID       string
	CustomerEmail string
	LineItems     []LineItem
	SuccessURL    string
	CancelURL     string
}

type CheckoutCreator interface {
	CreateCheckoutSession(req CheckoutRequest) (sessionID, url string, err error)
}

// CheckoutSessionRetriever lets the return page independently verify a payment
// with Stripe. Webhooks remain the primary confirmation mechanism, but a
// customer must not be left waiting forever when a webhook cannot reach the
// application (for example during local development or a temporary ingress
// outage).
type CheckoutSessionRetriever interface {
	RetrieveCheckoutSession(ctx context.Context, sessionID string) (VerifiedCheckoutSession, error)
}

// Refunder is intentionally separate from CheckoutCreator so checkout tests
// and local development do not need payment credentials to exercise orders.
type Refunder interface {
	RefundPaymentIntent(ctx context.Context, paymentIntentID, idempotencyKey string) (string, error)
}

type VerifiedCheckoutSession struct {
	ID              string
	OrderID         string
	PaymentIntentID string
	AmountTotal     int64
	Currency        string
	Paid            bool
}

type StripeClient struct{}

var _ CheckoutCreator = (*StripeClient)(nil)

func NewStripeClient(secretKey string) *StripeClient {
	stripe.Key = secretKey
	return &StripeClient{}
}

func (c *StripeClient) CreateCheckoutSession(req CheckoutRequest) (sessionID, url string, err error) {
	if req.OrderID == "" || req.CustomerEmail == "" || len(req.LineItems) == 0 {
		return "", "", fmt.Errorf("invalid checkout request")
	}
	items := make([]*stripe.CheckoutSessionLineItemParams, len(req.LineItems))
	for i, lineItem := range req.LineItems {
		items[i] = &stripe.CheckoutSessionLineItemParams{
			Quantity: stripe.Int64(lineItem.Quantity),
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency:   stripe.String(string(stripe.CurrencyEUR)),
				UnitAmount: stripe.Int64(lineItem.AmountCents),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name: stripe.String(lineItem.Name),
				},
			},
		}
	}

	params := &stripe.CheckoutSessionParams{
		Params:                   stripe.Params{Context: req.Context, IdempotencyKey: stripe.String("checkout_" + req.OrderID)},
		Mode:                     stripe.String(string(stripe.CheckoutSessionModePayment)),
		CustomerEmail:            stripe.String(req.CustomerEmail),
		ClientReferenceID:        stripe.String(req.OrderID),
		BillingAddressCollection: stripe.String("required"),
		LineItems:                items,
		SuccessURL:               stripe.String(req.SuccessURL),
		CancelURL:                stripe.String(req.CancelURL),
		// Stripe requires Checkout sessions to expire at least 30 minutes in the
		// future. Do not use the boundary: transit time can otherwise make the
		// request invalid by the time Stripe validates it.
		ExpiresAt: stripe.Int64(time.Now().Add(45 * time.Minute).Unix()),
		Metadata: map[string]string{
			"order_id": req.OrderID,
		},
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{Metadata: map[string]string{"order_id": req.OrderID}},
	}

	sess, err := session.New(params)
	if err != nil {
		return "", "", fmt.Errorf("stripe checkout session creation failed: %w", err)
	}
	return sess.ID, sess.URL, nil
}

func (c *StripeClient) RetrieveCheckoutSession(ctx context.Context, sessionID string) (VerifiedCheckoutSession, error) {
	if sessionID == "" {
		return VerifiedCheckoutSession{}, fmt.Errorf("checkout session ID is required")
	}

	sess, err := session.Get(sessionID, &stripe.CheckoutSessionParams{Params: stripe.Params{Context: ctx}})
	if err != nil {
		return VerifiedCheckoutSession{}, fmt.Errorf("retrieve Stripe checkout session: %w", err)
	}

	paymentIntentID := ""
	if sess.PaymentIntent != nil {
		paymentIntentID = sess.PaymentIntent.ID
	}
	return VerifiedCheckoutSession{
		ID:              sess.ID,
		OrderID:         sess.Metadata["order_id"],
		PaymentIntentID: paymentIntentID,
		AmountTotal:     sess.AmountTotal,
		Currency:        string(sess.Currency),
		Paid:            sess.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid,
	}, nil
}

func (c *StripeClient) RefundPaymentIntent(ctx context.Context, paymentIntentID, idempotencyKey string) (string, error) {
	if paymentIntentID == "" {
		return "", fmt.Errorf("payment intent ID is required")
	}
	params := &stripe.RefundParams{PaymentIntent: stripe.String(paymentIntentID)}
	params.Context = ctx
	params.IdempotencyKey = stripe.String(idempotencyKey)
	result, err := refund.New(params)
	if err != nil {
		return "", fmt.Errorf("refund Stripe payment: %w", err)
	}
	return result.ID, nil
}
