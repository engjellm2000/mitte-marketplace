//go:build integration

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// These tests exercise PostgreSQL row locks, unique indexes, and atomic
// inventory updates. Run only against a disposable database.
func TestConcurrentCheckoutNeverOversells(t *testing.T) {
	repository, productID := integrationStore(t, 10)
	ctx := context.Background()

	const buyers = 40
	results := make(chan error, buyers)
	var ready sync.WaitGroup
	ready.Add(buyers)
	start := make(chan struct{})
	for i := 0; i < buyers; i++ {
		session := fmt.Sprintf("checkout-session-%03d", i)
		if _, err := repository.AddCartItem(ctx, session, productID, 1); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < buyers; i++ {
		session := fmt.Sprintf("checkout-session-%03d", i)
		go func() {
			ready.Done()
			<-start
			_, err := repository.Checkout(ctx, session, fmt.Sprintf("checkout-idempotency-key-%03d", i), integrationCheckoutInput("Concurrent Buyer", fmt.Sprintf("buyer-%d@example.test", i)))
			results <- err
		}()
	}
	ready.Wait()
	close(start)

	successes := 0
	for i := 0; i < buyers; i++ {
		if err := <-results; err == nil {
			successes++
		} else if !errors.Is(err, ErrOutOfStock) {
			t.Fatalf("checkout error = %v, want nil or ErrOutOfStock", err)
		}
	}
	if successes != 10 {
		t.Fatalf("successful checkouts = %d, want 10", successes)
	}
	var inventory, orders int
	if err := repository.db.QueryRowContext(ctx, `SELECT inventory FROM products WHERE id = $1`, productID).Scan(&inventory); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE status = 'pending_payment'`).Scan(&orders); err != nil {
		t.Fatal(err)
	}
	if inventory != 0 || orders != 10 {
		t.Fatalf("inventory=%d orders=%d, want inventory=0 orders=10", inventory, orders)
	}
}

func TestConcurrentRetryCreatesOneOrder(t *testing.T) {
	repository, productID := integrationStore(t, 100)
	ctx := context.Background()
	const callers = 32
	session, key := "single-retry-session", "single-retry-idempotency-key"
	if _, err := repository.AddCartItem(ctx, session, productID, 1); err != nil {
		t.Fatal(err)
	}

	results := make(chan CheckoutResult, callers)
	errorsOut := make(chan error, callers)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < callers; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			result, err := repository.Checkout(ctx, session, key, integrationCheckoutInput("Retry Buyer", "retry@example.test"))
			results <- result
			errorsOut <- err
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	close(errorsOut)

	var orderID string
	for result := range results {
		if orderID == "" {
			orderID = result.OrderID
		}
		if result.OrderID != orderID {
			t.Fatalf("order IDs differ: %q and %q", orderID, result.OrderID)
		}
	}
	for err := range errorsOut {
		if err != nil {
			t.Fatalf("duplicate checkout error = %v", err)
		}
	}
	var orders, inventory int
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders`).Scan(&orders); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT inventory FROM products WHERE id = $1`, productID).Scan(&inventory); err != nil {
		t.Fatal(err)
	}
	if orders != 1 || inventory != 99 {
		t.Fatalf("orders=%d inventory=%d, want orders=1 inventory=99", orders, inventory)
	}
}

func TestCustomerEmailVerificationAndPasswordRecovery(t *testing.T) {
	repository, _ := integrationStore(t, 1)
	ctx := context.Background()
	now := time.Now().UTC()
	verifyHash := sessionTokenHash("verification-token")
	user, err := repository.CreateUserWithVerification(ctx, "buyer@example.test", "initial-password-hash", "b2c",
		verifyHash, now.Add(time.Hour), "Verify email", "Verification link")
	if err != nil {
		t.Fatalf("CreateUserWithVerification() error = %v", err)
	}
	if user.EmailVerified {
		t.Fatal("new customer email is verified before confirmation")
	}
	var queuedEmails, tokenRows int
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_outbox_events
		WHERE order_id IS NULL AND recipient = 'buyer@example.test' AND subject = 'Verify email'`).Scan(&queuedEmails); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_auth_tokens
		WHERE user_id = $1 AND purpose = 'verify_email' AND token_hash = $2`, user.ID, verifyHash).Scan(&tokenRows); err != nil {
		t.Fatal(err)
	}
	if queuedEmails != 1 || tokenRows != 1 {
		t.Fatalf("queued emails=%d tokens=%d, want 1 each", queuedEmails, tokenRows)
	}
	deliveries, total, err := repository.EmailDeliveriesForOperationsPage(ctx, "", "all", 50, 0)
	if err != nil || total != 1 || len(deliveries) != 1 || deliveries[0].OrderID != "" {
		t.Fatalf("account email delivery page = (%d, %#v, %v), want one account email", total, deliveries, err)
	}
	if worked, err := repository.DispatchNextEmail(ctx, integrationRecordingEmailSender{}); err != nil || !worked {
		t.Fatalf("DispatchNextEmail() = (%v, %v), want delivered message", worked, err)
	}
	var deliveredBody string
	if err := repository.db.QueryRowContext(ctx, `SELECT body FROM email_outbox_events WHERE id = $1`, deliveries[0].ID).Scan(&deliveredBody); err != nil {
		t.Fatal(err)
	}
	if deliveredBody != "" {
		t.Fatal("delivered authentication email body was retained with its one-time token")
	}
	if err := repository.VerifyUserEmail(ctx, verifyHash, now); err != nil {
		t.Fatalf("VerifyUserEmail() error = %v", err)
	}
	if err := repository.VerifyUserEmail(ctx, verifyHash, now); !errors.Is(err, ErrInvalidAuthToken) {
		t.Fatalf("reusing verification token error = %v, want ErrInvalidAuthToken", err)
	}
	authenticated, _, err := repository.AuthenticateUser(ctx, user.Email)
	if err != nil || !authenticated.EmailVerified {
		t.Fatalf("AuthenticateUser() verified = %v, error = %v", authenticated.EmailVerified, err)
	}
	if err := repository.CreateUserSession(ctx, user.ID, "session-token", now); err != nil {
		t.Fatal(err)
	}
	resetHash := sessionTokenHash("password-reset-token")
	if _, err := repository.IssueUserAuthToken(ctx, user.Email, "password_reset", resetHash,
		now.Add(time.Hour), "Reset password", "Reset link"); err != nil {
		t.Fatalf("IssueUserAuthToken() error = %v", err)
	}
	if err := repository.ResetUserPassword(ctx, resetHash, "replacement-password-hash", now); err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}
	_, passwordHash, err := repository.AuthenticateUser(ctx, user.Email)
	if err != nil || passwordHash != "replacement-password-hash" {
		t.Fatalf("password hash after reset = %q, error = %v", passwordHash, err)
	}
	if _, err := repository.UserBySession(ctx, "session-token", now); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("session after password reset error = %v, want ErrInvalidSession", err)
	}
	if err := repository.ResetUserPassword(ctx, resetHash, "another-password-hash", now); !errors.Is(err, ErrInvalidAuthToken) {
		t.Fatalf("reusing password reset token error = %v, want ErrInvalidAuthToken", err)
	}
	if err := repository.CreateUserSession(ctx, user.ID, "session-before-change", now); err != nil {
		t.Fatal(err)
	}
	pendingResetHash := sessionTokenHash("pending-password-reset-token")
	if _, err := repository.IssueUserAuthToken(ctx, user.Email, "password_reset", pendingResetHash,
		now.Add(time.Hour), "Reset password", "Reset link"); err != nil {
		t.Fatalf("IssueUserAuthToken() before password change error = %v", err)
	}
	if err := repository.ChangeUserPassword(ctx, user.ID, "user-changed-password-hash", now); err != nil {
		t.Fatalf("ChangeUserPassword() error = %v", err)
	}
	if err := repository.ResetUserPassword(ctx, pendingResetHash, "stale-reset-password-hash", now); !errors.Is(err, ErrInvalidAuthToken) {
		t.Fatalf("pending reset after password change error = %v, want ErrInvalidAuthToken", err)
	}
	if _, err := repository.UserBySession(ctx, "session-before-change", now); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("session after password change error = %v, want ErrInvalidSession", err)
	}
}

func TestExpiredReservationRestoresInventoryOnce(t *testing.T) {
	repository, productID := integrationStore(t, 1)
	ctx := context.Background()
	if _, err := repository.AddCartItem(ctx, "expiry-session", productID, 1); err != nil {
		t.Fatal(err)
	}
	result, err := repository.Checkout(ctx, "expiry-session", "expiry-idempotency-key", integrationCheckoutInput("Expiry Buyer", "expiry@example.test"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.ExecContext(ctx, `UPDATE orders SET checkout_expires_at = NOW() - INTERVAL '1 second' WHERE id = $1`, result.OrderID); err != nil {
		t.Fatal(err)
	}
	expired, err := repository.ExpirePendingOrders(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if expired != 1 {
		t.Fatalf("expired = %d, want 1", expired)
	}
	var inventory int
	var status string
	if err := repository.db.QueryRowContext(ctx, `SELECT inventory FROM products WHERE id = $1`, productID).Scan(&inventory); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT status FROM orders WHERE id = $1`, result.OrderID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if inventory != 1 || status != "cancelled" {
		t.Fatalf("inventory=%d status=%q, want inventory=1 status=cancelled", inventory, status)
	}
}

func TestInventoryLedgerTracksReservationsReleasesAndReceipts(t *testing.T) {
	repository, productID := integrationStore(t, 5)
	ctx := context.Background()
	if _, err := repository.AddCartItem(ctx, "inventory-ledger-session", productID, 1); err != nil {
		t.Fatal(err)
	}
	checkout, err := repository.Checkout(ctx, "inventory-ledger-session", "inventory-ledger-idempotency-key", integrationCheckoutInput("Ledger Buyer", "ledger@example.test"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.ExecContext(ctx, `UPDATE orders SET checkout_expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, checkout.OrderID); err != nil {
		t.Fatal(err)
	}
	if expired, err := repository.ExpirePendingOrders(ctx, 10); err != nil || expired != 1 {
		t.Fatalf("ExpirePendingOrders() = (%d, %v), want (1, nil)", expired, err)
	}
	if _, err := repository.ReceiveCatalogInventory(ctx, productID, 3, "Supplier delivery", "DEL-42", "staff@example.test"); err != nil {
		t.Fatal(err)
	}

	movements, err := repository.InventoryMovements(ctx, productID, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(movements) != 3 {
		t.Fatalf("movement count = %d, want 3: %#v", len(movements), movements)
	}
	want := []struct {
		kind                 string
		delta, before, after int
	}{
		{kind: "receipt", delta: 3, before: 5, after: 8},
		{kind: "reservation_release", delta: 1, before: 4, after: 5},
		{kind: "reservation", delta: -1, before: 5, after: 4},
	}
	for index, expected := range want {
		actual := movements[index]
		if actual.Type != expected.kind || actual.QuantityDelta != expected.delta ||
			actual.InventoryBefore != expected.before || actual.InventoryAfter != expected.after {
			t.Errorf("movement[%d] = %#v, want %#v", index, actual, expected)
		}
	}
}

func TestShipmentIssuesAndEmailsInvoiceExactlyOnce(t *testing.T) {
	repository, productID := integrationStore(t, 5)
	ctx := context.Background()
	if _, err := repository.AddCartItem(ctx, "invoice-shipment-session", productID, 1); err != nil {
		t.Fatal(err)
	}
	checkout, err := repository.Checkout(ctx, "invoice-shipment-session", "invoice-shipment-idempotency-key", CheckoutInput{
		Name: "Invoice Buyer", Email: "invoice-buyer@example.test", CustomerType: "b2c", UseBillingForShipping: true,
		BillingAddress: Address{RecipientName: "Invoice Buyer", AddressLine1: "Buyerstrasse 1", PostalCode: "10115", City: "Berlin", CountryCode: "DE"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.ExecContext(ctx, `UPDATE orders SET status='paid' WHERE id=$1`, checkout.OrderID); err != nil {
		t.Fatal(err)
	}
	if invoice, err := invoiceForOrder(ctx, repository.db, checkout.OrderID); err != nil || invoice != nil {
		t.Fatalf("invoice before shipment = (%#v, %v), want nil", invoice, err)
	}
	seller := InvoiceSeller{
		Name: "Mitte GmbH", Street: "Werkstrasse 1", PostalCode: "20095", City: "Hamburg",
		CountryCode: "DE", VATID: "DE123456789", Email: "invoice@example.test", IBAN: "DE02120300000000202051",
	}
	const actor = "warehouse@example.test"
	if err := repository.MarkOrderShippedBy(ctx, checkout.OrderID, actor, "Carrier", "TRACK-42", "https://carrier.example.test/TRACK-42", "https://shop.example.test", seller); err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkOrderShippedBy(ctx, checkout.OrderID, actor, "Carrier", "TRACK-42", "https://carrier.example.test/TRACK-42", "https://shop.example.test", seller); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("duplicate shipment error = %v, want ErrInvalidTransition", err)
	}
	var invoiceCount, documentCount, emailCount, eventCount, handoverCount int
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices WHERE order_id=$1`, checkout.OrderID).Scan(&invoiceCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoice_documents WHERE order_id=$1 AND document_type='invoice'`, checkout.OrderID).Scan(&documentCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_outbox_events WHERE order_id=$1 AND subject LIKE '%Rechnung%'`, checkout.OrderID).Scan(&emailCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM order_outbox_events WHERE order_id=$1 AND event_type='invoice.issued'`, checkout.OrderID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM warehouse_events WHERE order_id=$1 AND actor_email=$2 AND event_type='fulfillment.handed_over' AND detail='Carrier TRACK-42'`, checkout.OrderID, actor).Scan(&handoverCount); err != nil {
		t.Fatal(err)
	}
	if invoiceCount != 1 || documentCount != 1 || emailCount != 1 || eventCount != 1 || handoverCount != 1 {
		t.Fatalf("invoice=%d document=%d email=%d invoice_event=%d handover_event=%d, want exactly one of each", invoiceCount, documentCount, emailCount, eventCount, handoverCount)
	}
	var emailSubject, emailBody string
	if err := repository.db.QueryRowContext(ctx, `SELECT subject, body FROM email_outbox_events WHERE order_id=$1 AND subject LIKE '%Rechnung%'`, checkout.OrderID).Scan(&emailSubject, &emailBody); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(emailSubject, "vollständig versendet – Rechnung") || !strings.Contains(emailBody, "verfügbar") ||
		strings.Contains(emailSubject+emailBody, "Ã") || strings.Contains(emailSubject+emailBody, "â") {
		t.Fatalf("shipment email contains invalid text encoding: subject=%q body=%q", emailSubject, emailBody)
	}
	prefix := "https://shop.example.test/api/invoices/"
	linkStart := strings.Index(emailBody, prefix)
	if linkStart < 0 {
		t.Fatalf("shipment email has no invoice link: %s", emailBody)
	}
	token := strings.SplitN(strings.TrimPrefix(emailBody[linkStart:], prefix), "?", 2)[0]
	order, persistedSeller, err := repository.InvoiceByAccessToken(ctx, token)
	if err != nil {
		t.Fatalf("InvoiceByAccessToken() error = %v", err)
	}
	if order.ID != checkout.OrderID || order.Invoice == nil || !strings.EqualFold(persistedSeller.VATID, seller.VATID) {
		t.Fatalf("invoice access returned order %#v and seller %#v", order, persistedSeller)
	}
	customer, err := repository.CreateUser(ctx, "invoice-buyer@example.test", "integration-password-hash", "b2b")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if customer.Profile.CustomerType != "b2b" {
		t.Fatalf("new customer type = %q, want b2b", customer.Profile.CustomerType)
	}
	if _, err := repository.db.ExecContext(ctx, `UPDATE orders SET customer_id=$1 WHERE id=$2`, customer.ID, checkout.OrderID); err != nil {
		t.Fatal(err)
	}
	customerOrder, err := repository.OrderByIDForCustomer(ctx, checkout.OrderID, customer.ID)
	if err != nil {
		t.Fatalf("OrderByIDForCustomer() error = %v", err)
	}
	if customerOrder.Invoice == nil || customerOrder.Shipment == nil ||
		customerOrder.Invoice.Number != order.Invoice.Number {
		t.Fatalf("customer invoice lookup returned incomplete order: %#v", customerOrder)
	}
}

func TestFulfillmentHoldBlocksDirectShipmentUntilReleased(t *testing.T) {
	repository, productID := integrationStore(t, 5)
	ctx := context.Background()
	if _, err := repository.AddCartItem(ctx, "held-shipment-session", productID, 1); err != nil {
		t.Fatal(err)
	}
	checkout, err := repository.Checkout(ctx, "held-shipment-session", "held-shipment-checkout", integrationCheckoutInput("Held Buyer", "held@example.test"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.ExecContext(ctx, `UPDATE orders SET status='paid' WHERE id=$1`, checkout.OrderID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.ExecContext(ctx, `INSERT INTO warehouse_fulfillments (order_id,status) VALUES ($1,'ready_to_pick')`, checkout.OrderID); err != nil {
		t.Fatal(err)
	}
	if err := repository.HoldWarehouseFulfillment(ctx, checkout.OrderID, "staff:operations", "Address confirmation required"); err != nil {
		t.Fatal(err)
	}
	fulfillment, events, _, err := repository.WarehouseFulfillment(ctx, checkout.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if fulfillment.Status != "held" || fulfillment.ResumeStatus != "ready_to_pick" || fulfillment.HoldReason != "Address confirmation required" || len(events) == 0 || events[0].EventType != "fulfillment.held" {
		t.Fatalf("held fulfillment = %#v, events=%#v", fulfillment, events)
	}
	seller := InvoiceSeller{Name: "Mitte GmbH", Street: "Werkstrasse 1", PostalCode: "20095", City: "Hamburg", CountryCode: "DE", VATID: "DE123456789", Email: "invoice@example.test", IBAN: "DE02120300000000202051"}
	if err := repository.MarkOrderShippedBy(ctx, checkout.OrderID, "staff:operations", "Carrier", "HELD-42", "https://carrier.example.test/HELD-42", "https://shop.example.test", seller); !errors.Is(err, ErrInvalidFulfillmentHold) {
		t.Fatalf("shipment while held = %v, want ErrInvalidFulfillmentHold", err)
	}
	if err := repository.ReleaseWarehouseFulfillment(ctx, checkout.OrderID, "staff:operations"); err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkOrderShippedBy(ctx, checkout.OrderID, "staff:operations", "Carrier", "HELD-42", "https://carrier.example.test/HELD-42", "https://shop.example.test", seller); err != nil {
		t.Fatal(err)
	}
}

func TestWarehouseQueuePaginationAndBoundedHistory(t *testing.T) {
	repository, productID := integrationStore(t, 5)
	ctx := context.Background()
	if _, err := repository.AddCartItem(ctx, "warehouse-page-session", productID, 1); err != nil {
		t.Fatal(err)
	}
	checkout, err := repository.Checkout(ctx, "warehouse-page-session", "warehouse-page-checkout", integrationCheckoutInput("Paginated Buyer", "paginated@example.test"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.ExecContext(ctx, `INSERT INTO warehouse_fulfillments (order_id,status) VALUES ($1,'ready_to_pick')`, checkout.OrderID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < warehouseEventHistoryLimit+5; i++ {
		if _, err := repository.db.ExecContext(ctx, `INSERT INTO warehouse_events (order_id,actor_email,event_type,detail) VALUES ($1,'warehouse@example.test','test.event',$2)`, checkout.OrderID, fmt.Sprintf("event-%03d", i)); err != nil {
			t.Fatal(err)
		}
	}

	items, total, stats, err := repository.WarehouseQueuePage(ctx, "active", "Paginated Buyer", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || stats.Active != 1 || len(items) != 1 || items[0].OrderID != checkout.OrderID ||
		items[0].CustomerName != "Paginated Buyer" || items[0].TotalCents <= 0 {
		t.Fatalf("queue page = (%#v, total %d, stats %#v), want matched summary and one active result", items, total, stats)
	}
	items, nextTotal, _, err := repository.WarehouseQueuePage(ctx, "active", "Paginated Buyer", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 || nextTotal != total {
		t.Fatalf("second queue page = (%#v, total %d), want no items and total %d", items, nextTotal, total)
	}
	fulfillments, err := repository.WarehouseFulfillmentsByOrderIDs(ctx, []string{checkout.OrderID, "missing-order"})
	if err != nil || len(fulfillments) != 1 || fulfillments[0].OrderID != checkout.OrderID {
		t.Fatalf("fulfillments by order IDs = (%#v, %v), want the matching fulfillment", fulfillments, err)
	}

	_, events, eventCount, err := repository.WarehouseFulfillment(ctx, checkout.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if eventCount != warehouseEventHistoryLimit+5 || len(events) != warehouseEventHistoryLimit || events[0].Detail != "event-104" {
		t.Fatalf("event history = %d returned of %d total, newest %q", len(events), eventCount, events[0].Detail)
	}
}

func TestOrderOutboxLeaseOrderingRetryAndRecovery(t *testing.T) {
	repository, productID := integrationStore(t, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := repository.AddCartItem(ctx, "outbox-lease-session", productID, 1); err != nil {
		t.Fatal(err)
	}
	checkout, err := repository.Checkout(ctx, "outbox-lease-session", "outbox-lease-checkout", integrationCheckoutInput("Outbox Buyer", "outbox@example.test"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.ExecContext(ctx, `DELETE FROM order_outbox_events WHERE order_id = $1`, checkout.OrderID); err != nil {
		t.Fatal(err)
	}
	for _, eventType := range []string{"test.first", "test.second", "test.retry"} {
		if _, err := repository.db.ExecContext(ctx, `INSERT INTO order_outbox_events (order_id, event_type, payload) VALUES ($1, $2, '{}'::jsonb)`, checkout.OrderID, eventType); err != nil {
			t.Fatal(err)
		}
	}

	release := make(chan struct{})
	blocking := &integrationBlockingPublisher{entered: make(chan OutboxEvent, 1), release: release}
	result := make(chan error, 1)
	go func() {
		_, err := repository.DispatchNextOutboxEvent(ctx, blocking)
		result <- err
	}()
	var first OutboxEvent
	select {
	case first = <-blocking.entered:
	case err := <-result:
		t.Fatalf("first dispatch ended before publishing: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first event claim")
	}
	if first.Type != "test.first" {
		t.Fatalf("first claimed event = %q, want test.first", first.Type)
	}
	var claimToken string
	var claimExpiry sql.NullTime
	if err := repository.db.QueryRowContext(ctx, `SELECT claimed_by, claimed_until FROM order_outbox_events WHERE id = $1`, first.ID).Scan(&claimToken, &claimExpiry); err != nil {
		t.Fatal(err)
	}
	if claimToken == "" || !claimExpiry.Valid {
		t.Fatalf("event lease was not committed before publish: token=%q expiry=%v", claimToken, claimExpiry)
	}
	if didWork, err := repository.DispatchNextOutboxEvent(ctx, &integrationRecordingPublisher{}); err != nil || didWork {
		t.Fatalf("competing dispatch = (%t, %v), want no eligible event", didWork, err)
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatalf("first dispatch error: %v", err)
	}

	secondPublisher := &integrationRecordingPublisher{}
	if didWork, err := repository.DispatchNextOutboxEvent(ctx, secondPublisher); err != nil || !didWork {
		t.Fatalf("second dispatch = (%t, %v), want published event", didWork, err)
	}
	if len(secondPublisher.events) != 1 || secondPublisher.events[0].Type != "test.second" {
		t.Fatalf("second published events = %#v, want test.second", secondPublisher.events)
	}

	failure := errors.New("temporary broker failure")
	if didWork, err := repository.DispatchNextOutboxEvent(ctx, &integrationRecordingPublisher{err: failure}); !didWork || !errors.Is(err, failure) {
		t.Fatalf("failed dispatch = (%t, %v), want broker error", didWork, err)
	}
	var attempts int
	var lastError string
	if err := repository.db.QueryRowContext(ctx, `SELECT publish_attempts, last_error FROM order_outbox_events WHERE order_id = $1 AND event_type = 'test.retry'`, checkout.OrderID).Scan(&attempts, &lastError); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || lastError != failure.Error() {
		t.Fatalf("retry state = attempts %d, error %q", attempts, lastError)
	}
	if _, err := repository.db.ExecContext(ctx, `UPDATE order_outbox_events
		SET available_at = NOW(), claimed_by = 'expired-worker', claimed_until = NOW() - INTERVAL '1 second'
		WHERE order_id = $1 AND event_type = 'test.retry'`, checkout.OrderID); err != nil {
		t.Fatal(err)
	}
	recoveryPublisher := &integrationRecordingPublisher{}
	if didWork, err := repository.DispatchNextOutboxEvent(ctx, recoveryPublisher); err != nil || !didWork {
		t.Fatalf("recovery dispatch = (%t, %v), want published event", didWork, err)
	}
	if len(recoveryPublisher.events) != 1 || recoveryPublisher.events[0].Type != "test.retry" {
		t.Fatalf("recovery published events = %#v, want test.retry", recoveryPublisher.events)
	}
}

func TestEmailOutboxLeaseIsCommittedBeforeSend(t *testing.T) {
	repository, productID := integrationStore(t, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := repository.AddCartItem(ctx, "email-lease-session", productID, 1); err != nil {
		t.Fatal(err)
	}
	checkout, err := repository.Checkout(ctx, "email-lease-session", "email-lease-checkout", integrationCheckoutInput("Email Buyer", "email@example.test"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.ExecContext(ctx, `DELETE FROM email_outbox_events WHERE order_id = $1`, checkout.OrderID); err != nil {
		t.Fatal(err)
	}
	var emailID int64
	if err := repository.db.QueryRowContext(ctx, `INSERT INTO email_outbox_events (order_id, recipient, subject, body)
		VALUES ($1, 'email@example.test', 'Lease test', 'Message') RETURNING id`, checkout.OrderID).Scan(&emailID); err != nil {
		t.Fatal(err)
	}

	release := make(chan struct{})
	sender := &integrationBlockingEmailSender{entered: make(chan EmailMessage, 1), release: release}
	result := make(chan error, 1)
	go func() {
		_, err := repository.DispatchNextEmail(ctx, sender)
		result <- err
	}()
	var message EmailMessage
	select {
	case message = <-sender.entered:
	case err := <-result:
		t.Fatalf("email dispatch ended before sending: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for email claim")
	}
	if message.ID != emailID {
		t.Fatalf("claimed email ID = %d, want %d", message.ID, emailID)
	}
	var claimToken string
	var claimExpiry sql.NullTime
	if err := repository.db.QueryRowContext(ctx, `SELECT claimed_by, claimed_until FROM email_outbox_events WHERE id = $1`, emailID).Scan(&claimToken, &claimExpiry); err != nil {
		t.Fatal(err)
	}
	if claimToken == "" || !claimExpiry.Valid {
		t.Fatalf("email lease was not committed before send: token=%q expiry=%v", claimToken, claimExpiry)
	}
	if didWork, err := repository.DispatchNextEmail(ctx, &integrationRecordingEmailSender{}); err != nil || didWork {
		t.Fatalf("competing email dispatch = (%t, %v), want no eligible email", didWork, err)
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatalf("email dispatch error: %v", err)
	}
	var sentAt sql.NullTime
	if err := repository.db.QueryRowContext(ctx, `SELECT sent_at FROM email_outbox_events WHERE id = $1`, emailID).Scan(&sentAt); err != nil {
		t.Fatal(err)
	}
	if !sentAt.Valid {
		t.Fatal("email was not marked sent after delivery")
	}
}

func TestProductsPageBoundsAndSortsResultsAcrossPages(t *testing.T) {
	repository, productID := integrationStore(t, 1)
	ctx := context.Background()
	var categoryID int64
	if err := repository.db.QueryRowContext(ctx, `SELECT category_id FROM products WHERE id = $1`, productID).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 25; i++ {
		_, err := repository.db.ExecContext(ctx, `INSERT INTO products (category_id, slug, name, short_description, description, price_cents, image_url, image_alt, inventory)
			VALUES ($1, $2, $3, '', '', $4, 'https://example.test/product', 'Product', 1)`,
			categoryID, fmt.Sprintf("pagination-product-%02d", i), fmt.Sprintf("Pagination product %02d", i), 10000-i*100)
		if err != nil {
			t.Fatal(err)
		}
	}

	firstPage, hasMore, err := repository.ProductsPage(ctx, "", "", false, "price-low", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage) != 10 || !hasMore || firstPage[0].PriceCents != 1000 {
		t.Fatalf("first page length=%d hasMore=%t first price=%d", len(firstPage), hasMore, firstPage[0].PriceCents)
	}
	secondPage, hasMore, err := repository.ProductsPage(ctx, "", "", false, "price-low", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondPage) != 10 || !hasMore {
		t.Fatalf("second page length=%d hasMore=%t, want 10 and true", len(secondPage), hasMore)
	}
	thirdPage, hasMore, err := repository.ProductsPage(ctx, "", "", false, "price-low", 10, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(thirdPage) != 6 || hasMore {
		t.Fatalf("last page length=%d hasMore=%t, want 6 and false", len(thirdPage), hasMore)
	}
	allPages := append(append(firstPage, secondPage...), thirdPage...)
	seen := make(map[int64]struct{}, len(allPages))
	for index, product := range allPages {
		if _, duplicate := seen[product.ID]; duplicate {
			t.Fatalf("product ID %d appeared on multiple pages", product.ID)
		}
		seen[product.ID] = struct{}{}
		if index > 0 && allPages[index-1].PriceCents > product.PriceCents {
			t.Fatalf("price order breaks between indices %d and %d", index-1, index)
		}
	}
}

func TestSupplierPartialShipmentsIssueOneInvoiceAfterFinalQuantity(t *testing.T) {
	repository, productID := integrationStore(t, 5)
	ctx := context.Background()
	if _, err := repository.db.ExecContext(ctx, `UPDATE products SET supplier_name='Supplier A' WHERE id=$1`, productID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.AddCartItem(ctx, "supplier-partial-session", productID, 3); err != nil {
		t.Fatal(err)
	}
	checkout, err := repository.Checkout(ctx, "supplier-partial-session", "supplier-partial-idempotency-key",
		integrationCheckoutInput("Supplier Buyer", "supplier-buyer@example.test"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.ExecContext(ctx, `UPDATE orders SET status='paid' WHERE id=$1`, checkout.OrderID); err != nil {
		t.Fatal(err)
	}
	seller := InvoiceSeller{
		Name: "Mitte GmbH", Street: "Werkstrasse 1", PostalCode: "20095", City: "Hamburg",
		CountryCode: "DE", VATID: "DE123456789", Email: "invoice@example.test", IBAN: "DE02120300000000202051",
	}
	recordShipment := func(quantity int, trackingNumber string) error {
		return repository.RecordSupplierShipment(ctx, checkout.OrderID, "Supplier A", "supplier-a",
			"Teillieferung", "Carrier", trackingNumber, "https://carrier.example.test/"+trackingNumber,
			"https://shop.example.test", seller, []SupplierShipmentLine{{ProductID: productID, Quantity: quantity}})
	}
	if err := recordShipment(1, "PART-1"); err != nil {
		t.Fatalf("first partial shipment: %v", err)
	}
	if err := recordShipment(1, "PART-1"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("duplicate tracking number error = %v, want ErrInvalidTransition", err)
	}
	if invoice, err := invoiceForOrder(ctx, repository.db, checkout.OrderID); err != nil || invoice != nil {
		t.Fatalf("invoice after partial shipment = (%#v, %v), want no invoice", invoice, err)
	}
	var status string
	if err := repository.db.QueryRowContext(ctx, `SELECT status FROM orders WHERE id=$1`, checkout.OrderID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "paid" {
		t.Fatalf("status after partial shipment = %q, want paid", status)
	}
	if err := recordShipment(3, "OVER-SHIP"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("over-shipment error = %v, want ErrInvalidTransition", err)
	}
	if err := recordShipment(2, "PART-2"); err != nil {
		t.Fatalf("final shipment: %v", err)
	}
	if invoice, err := invoiceForOrder(ctx, repository.db, checkout.OrderID); err != nil || invoice == nil {
		t.Fatalf("invoice after final shipment = (%#v, %v), want one invoice", invoice, err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT status FROM orders WHERE id=$1`, checkout.OrderID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "fulfilled" {
		t.Fatalf("status after final shipment = %q, want fulfilled", status)
	}
	var shipmentCount, shippedQuantity, emailCount, shipmentEmailCount, invoiceEventCount int
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM shipments WHERE order_id=$1`, checkout.OrderID).Scan(&shipmentCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity),0) FROM shipment_items WHERE order_id=$1`, checkout.OrderID).Scan(&shippedQuantity); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_outbox_events WHERE order_id=$1`, checkout.OrderID).Scan(&emailCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_outbox_events
		WHERE order_id=$1 AND (subject LIKE 'Eine Sendung%' OR subject LIKE 'Deine Bestellung ist vollständig versendet%')`,
		checkout.OrderID).Scan(&shipmentEmailCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM order_outbox_events WHERE order_id=$1 AND event_type='invoice.issued'`, checkout.OrderID).Scan(&invoiceEventCount); err != nil {
		t.Fatal(err)
	}
	if shipmentCount != 2 || shippedQuantity != 3 || emailCount != 2 || shipmentEmailCount != 2 || invoiceEventCount != 1 {
		t.Fatalf("shipments=%d quantity=%d emails=%d shipment emails=%d invoice events=%d, want 2, 3, 2, 2, 1", shipmentCount, shippedQuantity, emailCount, shipmentEmailCount, invoiceEventCount)
	}
	var firstEmailBody string
	if err := repository.db.QueryRowContext(ctx, `SELECT body FROM email_outbox_events
		WHERE order_id=$1 AND subject LIKE 'Eine Sendung%' ORDER BY id LIMIT 1`, checkout.OrderID).Scan(&firstEmailBody); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(firstEmailBody, "/api/invoices/") {
		t.Fatalf("partial-shipment email unexpectedly includes an invoice link: %s", firstEmailBody)
	}
	var finalEmailBody string
	if err := repository.db.QueryRowContext(ctx, `SELECT body FROM email_outbox_events
		WHERE order_id=$1 AND subject LIKE 'Deine Bestellung ist vollständig versendet%' ORDER BY id DESC LIMIT 1`, checkout.OrderID).Scan(&finalEmailBody); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(finalEmailBody, "/api/invoices/") {
		t.Fatalf("final shipment email has no invoice link: %s", finalEmailBody)
	}
}

func TestLegacySupplierShipmentMigrationDeduplicatesProductReports(t *testing.T) {
	repository, productID := integrationStore(t, 5)
	ctx := context.Background()
	if _, err := repository.db.ExecContext(ctx, `UPDATE products SET supplier_name='Supplier A' WHERE id=$1`, productID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.AddCartItem(ctx, "legacy-supplier-session", productID, 3); err != nil {
		t.Fatal(err)
	}
	checkout, err := repository.Checkout(ctx, "legacy-supplier-session", "legacy-supplier-idempotency-key",
		integrationCheckoutInput("Legacy Buyer", "legacy-buyer@example.test"))
	if err != nil {
		t.Fatal(err)
	}
	var firstEventID, duplicateEventID int64
	if err := repository.db.QueryRowContext(ctx, `INSERT INTO supplier_order_events
		(order_id,product_id,product_name,supplier_name,event_type,carrier,tracking_number,tracking_url,quantity,actor_username,created_at)
		SELECT $1,$2,oi.product_name,'Supplier A','shipment_reported','Carrier','LEGACY-TRACK','https://carrier.example/LEGACY-TRACK',1,'supplier-a',NOW()-INTERVAL '3 minutes'
		FROM order_items oi WHERE oi.order_id=$1 AND oi.product_id=$2 RETURNING id`, checkout.OrderID, productID).Scan(&firstEventID); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `INSERT INTO supplier_order_events
		(order_id,product_id,product_name,supplier_name,event_type,carrier,tracking_number,tracking_url,quantity,actor_username,created_at)
		SELECT $1,$2,oi.product_name,'Supplier A','shipment_reported','Carrier','LEGACY-TRACK','https://carrier.example/LEGACY-TRACK',1,'supplier-a',NOW()-INTERVAL '2 minutes'
		FROM order_items oi WHERE oi.order_id=$1 AND oi.product_id=$2 RETURNING id`, checkout.OrderID, productID).Scan(&duplicateEventID); err != nil {
		t.Fatal(err)
	}
	var secondShipmentEventID int64
	if err := repository.db.QueryRowContext(ctx, `INSERT INTO supplier_order_events
		(order_id,product_id,product_name,supplier_name,event_type,carrier,tracking_number,tracking_url,quantity,actor_username,created_at)
		SELECT $1,$2,oi.product_name,'Supplier A','shipment_reported','Carrier','LEGACY-TRACK-2','https://carrier.example/LEGACY-TRACK-2',2,'supplier-a',NOW()-INTERVAL '1 minute'
		FROM order_items oi WHERE oi.order_id=$1 AND oi.product_id=$2 RETURNING id`, checkout.OrderID, productID).Scan(&secondShipmentEventID); err != nil {
		t.Fatal(err)
	}
	var excessShipmentEventID int64
	if err := repository.db.QueryRowContext(ctx, `INSERT INTO supplier_order_events
		(order_id,product_id,product_name,supplier_name,event_type,carrier,tracking_number,tracking_url,quantity,actor_username,created_at)
		SELECT $1,$2,oi.product_name,'Supplier A','shipment_reported','Carrier','LEGACY-TRACK-3','https://carrier.example/LEGACY-TRACK-3',3,'supplier-a',NOW()
		FROM order_items oi WHERE oi.order_id=$1 AND oi.product_id=$2 RETURNING id`, checkout.OrderID, productID).Scan(&excessShipmentEventID); err != nil {
		t.Fatal(err)
	}
	for _, report := range []struct {
		eventID  int64
		quantity int
	}{
		{eventID: firstEventID, quantity: 1},
		{eventID: duplicateEventID, quantity: 1},
		{eventID: secondShipmentEventID, quantity: 2},
		{eventID: excessShipmentEventID, quantity: 3},
	} {
		if _, err := repository.db.ExecContext(ctx, `INSERT INTO shipments
			(order_id,carrier,tracking_number,tracking_url,source,supplier_name,created_by,source_event_id,shipped_at)
			SELECT $1,carrier,tracking_number,tracking_url,'supplier','Supplier A','supplier:supplier-a',$2,NOW()
			FROM supplier_order_events WHERE id=$2`, checkout.OrderID, report.eventID); err != nil {
			t.Fatal(err)
		}
		var shipmentID int64
		if err := repository.db.QueryRowContext(ctx, `SELECT id FROM shipments WHERE source_event_id=$1`, report.eventID).Scan(&shipmentID); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.db.ExecContext(ctx, `INSERT INTO shipment_items(shipment_id,order_id,product_name,quantity)
			SELECT $1,order_id,product_name,$2 FROM supplier_order_events WHERE id=$3`,
			shipmentID, report.quantity, report.eventID); err != nil {
			t.Fatal(err)
		}
	}
	var deduplicationMigration string
	for _, migration := range appMigrations {
		if migration.Version == 47 {
			deduplicationMigration = migration.SQL
			break
		}
	}
	if deduplicationMigration == "" {
		t.Fatal("migration 47 is not registered")
	}
	for range 2 {
		tx, err := repository.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, deduplicationMigration); err != nil {
			_ = tx.Rollback()
			t.Fatalf("deduplicate legacy shipment rows: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	var shipmentCount, shipmentItemCount, shippedQuantity, uniqueTrackingCount int
	var sourceEventID int64
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*),MIN(source_event_id),COUNT(DISTINCT tracking_number) FROM shipments
		WHERE order_id=$1 AND source='supplier'`, checkout.OrderID).Scan(&shipmentCount, &sourceEventID, &uniqueTrackingCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(quantity),0) FROM shipment_items WHERE order_id=$1`,
		checkout.OrderID).Scan(&shipmentItemCount, &shippedQuantity); err != nil {
		t.Fatal(err)
	}
	var retainedEvents int
	if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM supplier_order_events
		WHERE order_id=$1 AND event_type='shipment_reported'`, checkout.OrderID).Scan(&retainedEvents); err != nil {
		t.Fatal(err)
	}
	if shipmentCount != 2 || shipmentItemCount != 2 || shippedQuantity != 3 || uniqueTrackingCount != 2 ||
		sourceEventID != firstEventID || retainedEvents != 4 {
		t.Fatalf("shipments=%d shipment items=%d shipped quantity=%d tracking numbers=%d minimum source event=%d retained events=%d; want 2, 2, 3, 2, %d, 4",
			shipmentCount, shipmentItemCount, shippedQuantity, uniqueTrackingCount, sourceEventID, retainedEvents, firstEventID)
	}
}

func TestMixedSupplierAndWarehouseShipmentsCompleteTogether(t *testing.T) {
	for _, supplierFirst := range []bool{false, true} {
		name := "warehouse-first"
		if supplierFirst {
			name = "supplier-first"
		}
		t.Run(name, func(t *testing.T) {
			repository, supplierProductID := integrationStore(t, 5)
			ctx := context.Background()
			if _, err := repository.db.ExecContext(ctx, `UPDATE products SET supplier_name='Supplier A' WHERE id=$1`, supplierProductID); err != nil {
				t.Fatal(err)
			}
			var categoryID, warehouseProductID int64
			if err := repository.db.QueryRowContext(ctx, `SELECT category_id FROM products WHERE id=$1`, supplierProductID).Scan(&categoryID); err != nil {
				t.Fatal(err)
			}
			if err := repository.db.QueryRowContext(ctx, `INSERT INTO products
				(category_id,slug,name,short_description,description,price_cents,image_url,image_alt,inventory)
				VALUES($1,'warehouse-product','Warehouse product','','',1500,'https://example.test/warehouse','Warehouse product',5)
				RETURNING id`, categoryID).Scan(&warehouseProductID); err != nil {
				t.Fatal(err)
			}
			sessionID := "mixed-shipment-" + name
			if _, err := repository.AddCartItem(ctx, sessionID, supplierProductID, 2); err != nil {
				t.Fatal(err)
			}
			if _, err := repository.AddCartItem(ctx, sessionID, warehouseProductID, 1); err != nil {
				t.Fatal(err)
			}
			checkout, err := repository.Checkout(ctx, sessionID, "mixed-shipment-key-"+name,
				integrationCheckoutInput("Mixed Buyer", "mixed-buyer@example.test"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := repository.db.ExecContext(ctx, `UPDATE orders SET status='paid' WHERE id=$1`, checkout.OrderID); err != nil {
				t.Fatal(err)
			}
			seller := InvoiceSeller{
				Name: "Mitte GmbH", Street: "Werkstrasse 1", PostalCode: "20095", City: "Hamburg",
				CountryCode: "DE", VATID: "DE123456789", Email: "invoice@example.test", IBAN: "DE02120300000000202051",
			}
			if err := repository.RecordSupplierShipment(ctx, checkout.OrderID, "Supplier A", "supplier-a",
				"Falsche Position", "Supplier Carrier", "WRONG-SOURCE-"+name, "https://carrier.example/WRONG-SOURCE-"+name,
				"https://shop.example.test", seller, []SupplierShipmentLine{{ProductID: warehouseProductID, Quantity: 1}}); !errors.Is(err, ErrNotFound) {
				t.Fatalf("supplier shipment for warehouse item error = %v, want ErrNotFound", err)
			}
			sendSupplier := func() error {
				return repository.RecordSupplierShipment(ctx, checkout.OrderID, "Supplier A", "supplier-a",
					"Teillieferung", "Supplier Carrier", "SUPPLIER-"+name, "https://carrier.example/SUPPLIER-"+name,
					"https://shop.example.test", seller, []SupplierShipmentLine{{ProductID: supplierProductID, Quantity: 2}})
			}
			sendWarehouse := func() error {
				return repository.MarkWarehouseShipmentBy(ctx, checkout.OrderID, "warehouse@example.test",
					"Warehouse Carrier", "WAREHOUSE-"+name, "https://carrier.example/WAREHOUSE-"+name,
					"https://shop.example.test", seller)
			}
			firstShipment, finalShipment := sendWarehouse, sendSupplier
			if supplierFirst {
				firstShipment, finalShipment = sendSupplier, sendWarehouse
			}
			if err := firstShipment(); err != nil {
				t.Fatalf("first shipment: %v", err)
			}
			var status string
			if err := repository.db.QueryRowContext(ctx, `SELECT status FROM orders WHERE id=$1`, checkout.OrderID).Scan(&status); err != nil {
				t.Fatal(err)
			}
			if status != "paid" {
				t.Fatalf("status after first shipment = %q, want paid", status)
			}
			if invoice, err := invoiceForOrder(ctx, repository.db, checkout.OrderID); err != nil || invoice != nil {
				t.Fatalf("invoice after first shipment = (%#v, %v), want no invoice", invoice, err)
			}
			if err := finalShipment(); err != nil {
				t.Fatalf("final shipment: %v", err)
			}
			if err := repository.db.QueryRowContext(ctx, `SELECT status FROM orders WHERE id=$1`, checkout.OrderID).Scan(&status); err != nil {
				t.Fatal(err)
			}
			if status != "fulfilled" {
				t.Fatalf("status after final shipment = %q, want fulfilled", status)
			}
			if invoice, err := invoiceForOrder(ctx, repository.db, checkout.OrderID); err != nil || invoice == nil {
				t.Fatalf("invoice after final shipment = (%#v, %v), want one invoice", invoice, err)
			}
			var shipmentCount, supplierShipmentCount, warehouseShipmentCount, shippedQuantity, invalidSourceItems, invoiceEventCount, shipmentEmailCount int
			if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*),
				COUNT(*) FILTER (WHERE source='supplier'),COUNT(*) FILTER (WHERE source='warehouse')
				FROM shipments WHERE order_id=$1`, checkout.OrderID).Scan(
				&shipmentCount, &supplierShipmentCount, &warehouseShipmentCount); err != nil {
				t.Fatal(err)
			}
			if err := repository.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity),0) FROM shipment_items WHERE order_id=$1`,
				checkout.OrderID).Scan(&shippedQuantity); err != nil {
				t.Fatal(err)
			}
			if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*)
				FROM shipment_items si
				JOIN shipments s ON s.id=si.shipment_id
				JOIN order_items oi ON oi.order_id=si.order_id AND oi.product_name=si.product_name
				WHERE si.order_id=$1 AND (
					(s.source='supplier' AND BTRIM(COALESCE(oi.supplier_name,''))='') OR
					(s.source='warehouse' AND BTRIM(COALESCE(oi.supplier_name,''))<>'')
				)`, checkout.OrderID).Scan(&invalidSourceItems); err != nil {
				t.Fatal(err)
			}
			if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM order_outbox_events
				WHERE order_id=$1 AND event_type='invoice.issued'`, checkout.OrderID).Scan(&invoiceEventCount); err != nil {
				t.Fatal(err)
			}
			if err := repository.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_outbox_events
				WHERE order_id=$1 AND (subject LIKE 'Eine Sendung%' OR subject LIKE 'Deine Bestellung ist vollständig versendet%')`,
				checkout.OrderID).Scan(&shipmentEmailCount); err != nil {
				t.Fatal(err)
			}
			if shipmentCount != 2 || supplierShipmentCount != 1 || warehouseShipmentCount != 1 ||
				shippedQuantity != 3 || invalidSourceItems != 0 || invoiceEventCount != 1 || shipmentEmailCount != 2 {
				t.Fatalf("shipments=%d supplier=%d warehouse=%d quantity=%d source mismatches=%d invoice events=%d shipment emails=%d; want 2, 1, 1, 3, 0, 1, 2",
					shipmentCount, supplierShipmentCount, warehouseShipmentCount, shippedQuantity, invalidSourceItems, invoiceEventCount, shipmentEmailCount)
			}
		})
	}
}

func integrationStore(t *testing.T, inventory int) (*Store, int64) {
	t.Helper()
	databaseURL := os.Getenv("INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set INTEGRATION_DATABASE_URL to run PostgreSQL concurrency tests")
	}
	ctx := context.Background()
	repository, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	if err := repository.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.ExecContext(ctx, `TRUNCATE email_outbox_events, order_outbox_events, payment_events, promotion_redemptions, order_status_history, order_items, orders, cart_items, carts, products, categories, user_sessions, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	var categoryID, productID int64
	if err := repository.db.QueryRowContext(ctx, `INSERT INTO categories (slug, name) VALUES ('concurrency', 'Concurrency') RETURNING id`).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRowContext(ctx, `INSERT INTO products (category_id, slug, name, short_description, description, price_cents, image_url, image_alt, inventory)
		VALUES ($1, 'concurrency-product', 'Concurrency product', '', '', 1000, 'https://example.test/product', 'Product', $2) RETURNING id`, categoryID, inventory).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	return repository, productID
}

func TestCatalogSpecificationsRoundTrip(t *testing.T) {
	repository, _ := integrationStore(t, 10)
	ctx := context.Background()
	var categoryID int64
	if err := repository.db.QueryRowContext(ctx, `SELECT id FROM categories WHERE slug = 'concurrency'`).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}

	input := CatalogProductInput{
		SKU: "MF-SPEC-001", Slug: "spec-product", Name: "Specification product",
		ShortDescription: "A short description", Description: "Detailed product description",
		PriceCents: 11900, ImageURL: "https://example.test/spec.jpg", ImageAlt: "Specification product",
		CategoryID: categoryID, Manufacturer: "Example manufacturer", ManufacturerSKU: "EX-MFR-9",
		GTIN: "4006381333931", Material: "Steel", Dimensions: "10 x 20 x 30 cm",
		WeightGrams: 1250, TechnicalDetails: map[string]string{"Finish": "Matte", "Voltage": "230 V"},
		InitialStock: 5,
	}
	created, err := repository.CreateCatalogProduct(ctx, input, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	if created.Manufacturer != input.Manufacturer || created.ManufacturerSKU != input.ManufacturerSKU ||
		created.GTIN != input.GTIN || created.Material != input.Material || created.Dimensions != input.Dimensions ||
		created.WeightGrams != input.WeightGrams || created.TechnicalDetails["Finish"] != "Matte" ||
		created.PriceGrossCents != 11900 || created.PriceNetCents != 10000 {
		t.Fatalf("created catalog product lost specifications or price breakdown: %#v", created)
	}

	input.ManufacturerSKU = "EX-MFR-10"
	input.TechnicalDetails = map[string]string{"Finish": "Brushed"}
	updated, err := repository.UpdateCatalogProduct(ctx, created.ID, input, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ManufacturerSKU != "EX-MFR-10" || updated.TechnicalDetails["Finish"] != "Brushed" {
		t.Fatalf("updated catalog specifications = %#v", updated)
	}

	if err := repository.SetCatalogStatus(ctx, created.ID, "active", "integration-test"); err != nil {
		t.Fatal(err)
	}
	product, err := repository.ProductBySlug(ctx, input.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if product.SKU != input.SKU || product.ManufacturerSKU != input.ManufacturerSKU ||
		product.GTIN != input.GTIN || product.TechnicalDetails["Finish"] != "Brushed" ||
		product.PriceNetCents != 10000 || product.PriceGrossCents != 11900 {
		t.Fatalf("public product query lost specifications or price breakdown: %#v", product)
	}

	cart, err := repository.AddCartItem(ctx, "specification-cart", created.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart.Items) != 1 || cart.Items[0].Product.GTIN != input.GTIN ||
		cart.Items[0].Product.TechnicalDetails["Finish"] != "Brushed" ||
		cart.Items[0].Product.PriceNetCents != 10000 || cart.Items[0].Quantity != 2 {
		t.Fatalf("cart product query lost specifications or price breakdown: %#v", cart.Items)
	}
}

func integrationCheckoutInput(name, email string) CheckoutInput {
	return CheckoutInput{
		Name: name, Email: email, CustomerType: "b2c", UseBillingForShipping: true,
		BillingAddress: Address{
			RecipientName: name, AddressLine1: "Teststrasse 1", PostalCode: "10115", City: "Berlin", CountryCode: "DE",
		},
	}
}

type integrationRecordingPublisher struct {
	events []OutboxEvent
	err    error
}

func (p *integrationRecordingPublisher) Publish(_ context.Context, event OutboxEvent) error {
	p.events = append(p.events, event)
	return p.err
}

type integrationBlockingPublisher struct {
	entered chan OutboxEvent
	release <-chan struct{}
}

func (p *integrationBlockingPublisher) Publish(ctx context.Context, event OutboxEvent) error {
	p.entered <- event
	select {
	case <-p.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type integrationRecordingEmailSender struct{}

func (integrationRecordingEmailSender) Send(context.Context, EmailMessage) error { return nil }

type integrationBlockingEmailSender struct {
	entered chan EmailMessage
	release <-chan struct{}
}

func (s *integrationBlockingEmailSender) Send(ctx context.Context, message EmailMessage) error {
	s.entered <- message
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
