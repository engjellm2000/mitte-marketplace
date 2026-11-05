// Command reconcile-stripe repairs a paid Stripe Checkout session whose webhook
// could not be delivered. It verifies the session with Stripe before making any
// local order change, so it is safe to run again for the same session.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"mitte/internal/config"
	"mitte/internal/payments"
	"mitte/internal/store"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: reconcile-stripe <checkout-session-id>")
	}
	if os.Getenv("APP_COMPONENT") == "" {
		if err := os.Setenv("APP_COMPONENT", "stripe-reconcile"); err != nil {
			log.Fatalf("set component role: %v", err)
		}
	}
	if err := config.LoadDevelopmentEnvFile(".env"); err != nil {
		log.Fatalf("load environment: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}
	if cfg.StripeSecretKey == "" {
		log.Fatal("STRIPE_SECRET_KEY is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repository, err := store.OpenWithPool(ctx, cfg.DatabaseURL, store.PoolConfig{MaxOpenConns: 2, MaxIdleConns: 1})
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer repository.Close()
	stripeClient := payments.NewStripeClient(cfg.StripeSecretKey)
	session, err := stripeClient.RetrieveCheckoutSession(ctx, os.Args[1])
	if err != nil {
		log.Fatalf("retrieve checkout session: %v", err)
	}
	if !session.Paid || session.OrderID == "" {
		log.Fatal("Stripe session is not a paid order")
	}
	if err := repository.FulfillStripeCheckout(ctx, session.OrderID, session.ID, session.PaymentIntentID, session.AmountTotal, session.Currency); err != nil {
		if !errors.Is(err, store.ErrInvalidTransition) {
			log.Fatalf("fulfill order: %v", err)
		}
		if err := repository.ReconcileLateStripeCheckout(ctx, session.OrderID, session.ID, session.PaymentIntentID, session.AmountTotal, session.Currency); err != nil {
			log.Fatalf("reconcile late payment: %v", err)
		}
	}
	fmt.Printf("Order %s reconciled as paid.\n", session.OrderID)
}
