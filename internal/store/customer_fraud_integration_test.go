//go:build integration

package store

import (
	"context"
	"errors"
	"testing"
)

func TestCustomerFraudFlagBlocksCheckoutAndFulfillmentUntilCleared(t *testing.T) {
	repository, productID := integrationStore(t, 3)
	ctx := context.Background()
	customer, err := repository.CreateUser(ctx, "fraud-buyer@example.test", "integration-password-hash", "b2c")
	if err != nil {
		t.Fatal(err)
	}

	session := "fraud-customer-session"
	if _, err := repository.AddCartItem(ctx, session, productID, 1); err != nil {
		t.Fatal(err)
	}
	order, err := repository.CheckoutForCustomer(ctx, session, "fraud-checkout-idempotency", customer.ID,
		integrationCheckoutInput("Fraud Buyer", customer.Email))
	if err != nil {
		t.Fatal(err)
	}

	status, err := repository.SetCustomerFraudFlag(ctx, customer.ID, true, "Payment details require review", "staff:admin")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Flagged || status.Reason != "Payment details require review" || status.FlaggedBy != "staff:admin" || status.FlaggedAt == nil {
		t.Fatalf("fraud status was not recorded: %#v", status)
	}

	if _, err := repository.CheckoutForCustomer(ctx, session, "fraud-checkout-idempotency", customer.ID,
		integrationCheckoutInput("Fraud Buyer", customer.Email)); !errors.Is(err, ErrCustomerFraudBlocked) {
		t.Fatalf("idempotent checkout retry error = %v, want fraud block", err)
	}
	if _, err := repository.AddCartItem(ctx, session, productID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CheckoutForCustomer(ctx, session, "fraud-checkout-retry-key", customer.ID,
		integrationCheckoutInput("Fraud Buyer", customer.Email)); !errors.Is(err, ErrCustomerFraudBlocked) {
		t.Fatalf("authenticated checkout error = %v, want fraud block", err)
	}

	guestSession := "fraud-guest-alias-session"
	if _, err := repository.AddCartItem(ctx, guestSession, productID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Checkout(ctx, guestSession, "fraud-guest-checkout-key",
		integrationCheckoutInput("Fraud Buyer", customer.Email)); !errors.Is(err, ErrCustomerFraudBlocked) {
		t.Fatalf("guest checkout with a flagged account email error = %v, want fraud block", err)
	}

	if _, err := repository.db.ExecContext(ctx, `UPDATE orders SET status='paid' WHERE id=$1`, order.OrderID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.ExecContext(ctx, `INSERT INTO warehouse_fulfillments(order_id,status) VALUES($1,'ready_to_pick')`, order.OrderID); err != nil {
		t.Fatal(err)
	}
	queue, _, _, err := repository.WarehouseQueuePage(ctx, "all", "", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 0 {
		t.Fatalf("fraud-blocked orders remained in warehouse queue: %#v", queue)
	}
	if err := repository.TransitionWarehouseFulfillment(ctx, order.OrderID, "staff:warehouse", "picking", "start"); !errors.Is(err, ErrCustomerFraudBlocked) {
		t.Fatalf("warehouse transition error = %v, want fraud block", err)
	}

	status, err = repository.SetCustomerFraudFlag(ctx, customer.ID, false, "Review completed", "staff:admin")
	if err != nil {
		t.Fatal(err)
	}
	if status.Flagged {
		t.Fatalf("fraud status stayed active after clearing: %#v", status)
	}
	var auditEntries int
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM customer_fraud_audit WHERE customer_id=$1`, customer.ID).Scan(&auditEntries); err != nil {
		t.Fatal(err)
	}
	if auditEntries != 2 {
		t.Fatalf("fraud audit entries = %d, want flag and unflag entries", auditEntries)
	}
	queue, _, _, err = repository.WarehouseQueuePage(ctx, "all", "", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 1 || queue[0].OrderID != order.OrderID {
		t.Fatalf("warehouse order was not released after clearing the flag: %#v", queue)
	}
}
