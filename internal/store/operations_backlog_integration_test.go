//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

func TestOperationsBacklogPageClassifiesAndOrdersOpenWork(t *testing.T) {
	repository, _ := integrationStore(t, 5)
	ctx := context.Background()

	createdAt := time.Now().UTC().Add(-3 * time.Hour)
	insertOrder := func(id, status, name string, created time.Time) {
		t.Helper()
		cartID := id + "-cart"
		sessionToken := id + "-session"
		if _, err := repository.db.ExecContext(ctx, `INSERT INTO carts (id, session_token, status)
			VALUES ($1, $2, 'checked_out')`, cartID, sessionToken); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.db.ExecContext(ctx, `INSERT INTO orders
			(id, cart_id, checkout_session_token, customer_name, customer_email, total_cents, status, created_at)
			VALUES ($1, $2, $3, $4, $5, 5000, $6, $7)`, id, cartID, sessionToken, name, id+"@example.test", status, created); err != nil {
			t.Fatal(err)
		}
	}

	insertOrder("backlog-payment", "pending_payment", "Payment waiting", createdAt)
	insertOrder("backlog-missing", "paid", "Missing fulfillment", createdAt.Add(time.Hour))
	insertOrder("backlog-active", "paid", "Active fulfillment", createdAt.Add(2*time.Hour))
	insertOrder("backlog-held", "paid", "Held fulfillment", createdAt.Add(2*time.Hour))
	insertOrder("backlog-done", "fulfilled", "Completed order", createdAt.Add(3*time.Hour))

	if _, err := repository.db.ExecContext(ctx, `INSERT INTO warehouse_fulfillments (order_id, status)
		VALUES ('backlog-active', 'ready_to_pick'), ('backlog-held', 'held'), ('backlog-done', 'handed_over')`); err != nil {
		t.Fatal(err)
	}

	items, total, summary, err := repository.OperationsBacklogPage(ctx, "", "all", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || summary != (OperationsBacklogSummary{Total: 4, Payment: 1, NeedsFulfillment: 1, InProgress: 1, Held: 1}) {
		t.Fatalf("backlog total and summary = (%d, %#v), want 3 actionable rows split across the three stages", total, summary)
	}
	if len(items) != 4 || items[0].ID != "backlog-payment" || items[1].ID != "backlog-missing" ||
		items[2].ID != "backlog-active" || items[3].ID != "backlog-held" || items[1].Stage != "needs_fulfillment" ||
		items[2].FulfillmentStatus != "ready_to_pick" || items[3].Stage != "held" {
		t.Fatalf("backlog rows are not correctly classified and oldest-first: %#v", items)
	}

	items, total, _, err = repository.OperationsBacklogPage(ctx, "", "held", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != "backlog-held" {
		t.Fatalf("held backlog = (%#v, %d), want only the held fulfillment", items, total)
	}

	items, total, _, err = repository.OperationsBacklogPage(ctx, "Missing", "needs_fulfillment", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != "backlog-missing" {
		t.Fatalf("filtered backlog = (%#v, %d), want only the missing fulfillment record", items, total)
	}
}
