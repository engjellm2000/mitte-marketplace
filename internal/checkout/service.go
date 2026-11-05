package checkout

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"mitte/internal/commerce"
)

var (
	ErrPaymentUnavailable   = errors.New("payment method unavailable")
	ErrPaymentMethodChanged = errors.New("payment method differs from the original checkout")
	ErrInvalidInput         = errors.New("invalid checkout input")
)

type OrderRepository interface {
	CheckoutForCustomer(context.Context, string, string, int64, commerce.CheckoutInput) (commerce.CheckoutResult, error)
	AttachCheckoutSession(context.Context, string, string, string) error
	ConfigureTransferCheckout(context.Context, string, string, string, string, int) error
	ConfigureInvoiceSandbox(context.Context, string, string, string) error
	MarkCheckoutSagaRetryable(context.Context, string, string) error
}

type PaymentGateway interface {
	CreateCheckoutSession(context.Context, PaymentRequest) (sessionID, checkoutURL string, err error)
}

type PaymentRequest struct {
	OrderID       string
	CustomerEmail string
	LineItems     []commerce.CheckoutLineItem
	SuccessURL    string
	CancelURL     string
}

type Settings struct {
	Stripe                    PaymentGateway
	StripeSuccessURL          string
	StripeCancelURL           string
	MittePayEnabled      bool
	MitteTransferEnabled bool
	MitteInvoiceEnabled  bool
}

type Service struct {
	orders   OrderRepository
	settings Settings
	newID    func(string) (string, error)
}

func NewService(orders OrderRepository, settings Settings) *Service {
	return &Service{orders: orders, settings: settings, newID: randomSessionID}
}

func (s *Service) Execute(ctx context.Context, sessionToken, idempotencyKey string, customerID int64, input commerce.CheckoutInput) (commerce.CheckoutResult, error) {
	commerce.NormalizeCheckoutInput(&input)
	if input.PaymentMethod == "" {
		input.PaymentMethod = "stripe"
	}
	if !commerce.ValidCheckoutInput(input) {
		return commerce.CheckoutResult{}, ErrInvalidInput
	}
	if err := s.validatePaymentMethod(input.PaymentMethod); err != nil {
		return commerce.CheckoutResult{}, err
	}
	result, err := s.orders.CheckoutForCustomer(ctx, sessionToken, idempotencyKey, customerID, input)
	if err != nil {
		return commerce.CheckoutResult{}, err
	}
	if result.PaymentMethod != "" && result.PaymentMethod != input.PaymentMethod {
		return commerce.CheckoutResult{}, ErrPaymentMethodChanged
	}
	if result.CheckoutURL != "" {
		return result, nil
	}

	var operationErr error
	switch input.PaymentMethod {
	case "mitte_pay":
		sessionID, err := s.newID("mfpay_")
		if err == nil {
			url := "/zahlung/mitte?session_id=" + sessionID
			err = s.orders.AttachCheckoutSession(ctx, result.OrderID, sessionID, url)
			result.CheckoutSessionID, result.CheckoutURL = sessionID, url
		}
		operationErr = err
	case "mitte_transfer":
		sessionID, err := s.newID("mftransfer_")
		if err == nil {
			url := "/zahlung/mitte?session_id=" + sessionID
			reference := "MF-" + strings.ToUpper(result.OrderID)
			err = s.orders.ConfigureTransferCheckout(ctx, result.OrderID, sessionID, url, reference, result.TotalCents)
			result.CheckoutSessionID, result.CheckoutURL = sessionID, url
		}
		operationErr = err
	case "mitte_invoice":
		sessionID, err := s.newID("mfinvoice_")
		if err == nil {
			url := "/zahlung/rechnung?session_id=" + sessionID
			err = s.orders.ConfigureInvoiceSandbox(ctx, result.OrderID, sessionID, url)
			result.CheckoutSessionID, result.CheckoutURL = sessionID, url
		}
		operationErr = err
	default:
		sessionID, checkoutURL, err := s.settings.Stripe.CreateCheckoutSession(ctx, PaymentRequest{
			OrderID: result.OrderID, CustomerEmail: input.Email, LineItems: result.LineItems,
			SuccessURL: s.settings.StripeSuccessURL, CancelURL: s.settings.StripeCancelURL,
		})
		if err == nil {
			err = s.orders.AttachCheckoutSession(ctx, result.OrderID, sessionID, checkoutURL)
			result.CheckoutSessionID, result.CheckoutURL = sessionID, checkoutURL
		}
		operationErr = err
	}
	if operationErr != nil {
		if markErr := s.orders.MarkCheckoutSagaRetryable(ctx, result.OrderID, "checkout preparation failed"); markErr != nil {
			return commerce.CheckoutResult{}, errors.Join(operationErr, fmt.Errorf("persist checkout retry state: %w", markErr))
		}
		return commerce.CheckoutResult{}, operationErr
	}
	result.SagaStatus = "awaiting_payment"
	return result, nil
}

func (s *Service) validatePaymentMethod(method string) error {
	switch method {
	case "stripe":
		if s.settings.Stripe == nil {
			return ErrPaymentUnavailable
		}
	case "mitte_pay":
		if !s.settings.MittePayEnabled {
			return ErrPaymentUnavailable
		}
	case "mitte_transfer":
		if !s.settings.MitteTransferEnabled {
			return ErrPaymentUnavailable
		}
	case "mitte_invoice":
		if !s.settings.MitteInvoiceEnabled {
			return ErrPaymentUnavailable
		}
	default:
		return fmt.Errorf("unsupported payment method %q", method)
	}
	return nil
}

func randomSessionID(prefix string) (string, error) {
	token := make([]byte, 18)
	if _, err := rand.Read(token); err != nil {
		return "", fmt.Errorf("generate payment session ID: %w", err)
	}
	return prefix + hex.EncodeToString(token), nil
}
