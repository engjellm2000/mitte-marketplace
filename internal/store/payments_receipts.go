package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type PaymentTransaction struct {
	OrderID, Provider, Reference, Status, PayerName, PayerIBANLast4, Note string
	ReceivableStatus                                                      string
	AmountCents                                                           int
	ReceivedAt                                                            *time.Time
	CreatedAt                                                             time.Time
	InvoiceNumber                                                         string
	DueAt                                                                 *time.Time
	AppliedCents, OutstandingCents                                        int
}

// PaymentReceipt is one observed bank-credit. Allocations are maintained
// separately, so a payment can be split or moved without changing the
// immutable invoice/receivable record.
type PaymentReceipt struct {
	ID                                                     int64
	Reference, PayerName, PayerIBANLast4, Note, ReceivedBy string
	AmountCents, AllocatedCents, UnallocatedCents          int
	ReceivedAt, CreatedAt                                  time.Time
	Allocations                                            []PaymentReceiptAllocation
}

type PaymentReceiptAllocation struct {
	OrderID, InvoiceNumber string
	AmountCents            int
}

// CreateTransferPayment creates the first-party payment record after checkout.
// It is deliberately a bank-transfer ledger, not a card processor: card details
// must continue to be collected by PCI-compliant Stripe Checkout.
func (s *Store) CreateTransferPayment(ctx context.Context, orderID, reference string, amountCents int) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `UPDATE orders SET payment_provider = 'mitte_transfer' WHERE id = $1 AND status = 'pending_payment'`, orderID)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows != 1 {
			return ErrNotFound
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO payment_transactions (order_id, provider, reference, amount_cents) VALUES ($1, 'mitte_transfer', $2, $3)`, orderID, reference, amountCents)
		return err
	})
}

func (s *Store) PaymentTransactions(ctx context.Context) ([]PaymentTransaction, error) {
	if _, err := s.db.ExecContext(ctx, `WITH changed AS (
		UPDATE payment_transactions p SET receivable_status='overdue', updated_at=NOW()
		FROM invoices i WHERE i.order_id=p.order_id AND p.provider='mitte_invoice' AND p.receivable_status='sent' AND i.due_at < NOW()
		RETURNING p.order_id
	) INSERT INTO receivable_status_history (order_id, from_status, to_status, reason)
	SELECT order_id, 'sent', 'overdue', 'payment_term_expired' FROM changed`); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT p.order_id, p.provider, p.reference, p.status, p.amount_cents, p.payer_name, p.payer_iban_last4, p.note, p.received_at, p.created_at,
		COALESCE(i.invoice_number, ''), i.due_at, COALESCE(a.applied_cents, 0), p.receivable_status
		FROM payment_transactions p
		LEFT JOIN invoices i ON i.order_id = p.order_id
		LEFT JOIN LATERAL (SELECT COALESCE(SUM(amount_cents), 0) AS applied_cents FROM payment_receipt_allocations WHERE order_id = p.order_id) a ON TRUE
		ORDER BY CASE WHEN p.provider = 'mitte_invoice' AND COALESCE(a.applied_cents, 0) < p.amount_cents THEN 0 WHEN p.status = 'awaiting_payment' THEN 0 ELSE 1 END, p.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	transactions := []PaymentTransaction{}
	for rows.Next() {
		var p PaymentTransaction
		if err := rows.Scan(&p.OrderID, &p.Provider, &p.Reference, &p.Status, &p.AmountCents, &p.PayerName, &p.PayerIBANLast4, &p.Note, &p.ReceivedAt, &p.CreatedAt, &p.InvoiceNumber, &p.DueAt, &p.AppliedCents, &p.ReceivableStatus); err != nil {
			return nil, err
		}
		p.OutstandingCents = max(0, p.AmountCents-p.AppliedCents)
		transactions = append(transactions, p)
	}
	return transactions, rows.Err()
}

func (s *Store) PaymentReceipts(ctx context.Context) ([]PaymentReceipt, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.id, r.receipt_reference, r.amount_cents, r.payer_name, r.payer_iban_last4, r.note, r.received_by, r.received_at, r.created_at, COALESCE(SUM(a.amount_cents), 0)
		FROM payment_receipts r LEFT JOIN payment_receipt_allocations a ON a.receipt_id = r.id
		GROUP BY r.id ORDER BY r.received_at DESC, r.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var receipts []PaymentReceipt
	for rows.Next() {
		var receipt PaymentReceipt
		if err := rows.Scan(&receipt.ID, &receipt.Reference, &receipt.AmountCents, &receipt.PayerName, &receipt.PayerIBANLast4, &receipt.Note, &receipt.ReceivedBy, &receipt.ReceivedAt, &receipt.CreatedAt, &receipt.AllocatedCents); err != nil {
			return nil, err
		}
		receipt.UnallocatedCents = receipt.AmountCents - receipt.AllocatedCents
		if receipt.Allocations, err = paymentReceiptAllocations(ctx, s.db, receipt.ID); err != nil {
			return nil, err
		}
		receipts = append(receipts, receipt)
	}
	return receipts, rows.Err()
}

func paymentReceiptAllocations(ctx context.Context, q queryer, receiptID int64) ([]PaymentReceiptAllocation, error) {
	rows, err := q.QueryContext(ctx, `SELECT a.order_id, i.invoice_number, a.amount_cents FROM payment_receipt_allocations a JOIN invoices i ON i.order_id=a.order_id WHERE a.receipt_id=$1 ORDER BY i.invoice_number`, receiptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var allocations []PaymentReceiptAllocation
	for rows.Next() {
		var allocation PaymentReceiptAllocation
		if err := rows.Scan(&allocation.OrderID, &allocation.InvoiceNumber, &allocation.AmountCents); err != nil {
			return nil, err
		}
		allocations = append(allocations, allocation)
	}
	return allocations, rows.Err()
}

func (s *Store) CreatePaymentReceipt(ctx context.Context, reference string, amountCents int, payerName, ibanLast4, note, receivedBy string) (PaymentReceipt, error) {
	var receipt PaymentReceipt
	err := s.db.QueryRowContext(ctx, `INSERT INTO payment_receipts (receipt_reference, amount_cents, payer_name, payer_iban_last4, note, received_by) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, receipt_reference, amount_cents, payer_name, payer_iban_last4, note, received_by, received_at, created_at`, reference, amountCents, payerName, ibanLast4, note, receivedBy).
		Scan(&receipt.ID, &receipt.Reference, &receipt.AmountCents, &receipt.PayerName, &receipt.PayerIBANLast4, &receipt.Note, &receipt.ReceivedBy, &receipt.ReceivedAt, &receipt.CreatedAt)
	if err != nil {
		return PaymentReceipt{}, err
	}
	receipt.UnallocatedCents = receipt.AmountCents
	return receipt, nil
}

// ReplacePaymentReceiptAllocations performs a complete, auditable allocation
// update in one transaction. A receipt may remain partly unallocated, but an
// invoice may never be overpaid.
func (s *Store) ReplacePaymentReceiptAllocations(ctx context.Context, receiptID int64, allocations []PaymentReceiptAllocation) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var receiptAmount int
		if err := tx.QueryRowContext(ctx, `SELECT amount_cents FROM payment_receipts WHERE id=$1 FOR UPDATE`, receiptID).Scan(&receiptAmount); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		oldRows, err := tx.QueryContext(ctx, `SELECT order_id FROM payment_receipt_allocations WHERE receipt_id=$1`, receiptID)
		if err != nil {
			return err
		}
		affected := map[string]struct{}{}
		for oldRows.Next() {
			var orderID string
			if err := oldRows.Scan(&orderID); err != nil {
				oldRows.Close()
				return err
			}
			affected[orderID] = struct{}{}
		}
		if err := oldRows.Close(); err != nil {
			return err
		}
		sum := 0
		seen := map[string]struct{}{}
		for _, allocation := range allocations {
			if allocation.OrderID == "" || allocation.AmountCents <= 0 {
				return ErrInvalidCheckout
			}
			if _, exists := seen[allocation.OrderID]; exists {
				return ErrInvalidCheckout
			}
			seen[allocation.OrderID] = struct{}{}
			sum += allocation.AmountCents
			affected[allocation.OrderID] = struct{}{}
		}
		if sum > receiptAmount {
			return ErrInvalidCheckout
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM payment_receipt_allocations WHERE receipt_id=$1`, receiptID); err != nil {
			return err
		}
		for _, allocation := range allocations {
			var total, previouslyApplied int
			var provider string
			if err := tx.QueryRowContext(ctx, `SELECT o.payment_provider, i.total_cents, COALESCE((SELECT SUM(amount_cents) FROM payment_receipt_allocations WHERE order_id=o.id),0) FROM orders o JOIN invoices i ON i.order_id=o.id WHERE o.id=$1 FOR UPDATE`, allocation.OrderID).Scan(&provider, &total, &previouslyApplied); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrNotFound
				}
				return err
			}
			if provider != "mitte_invoice" || previouslyApplied+allocation.AmountCents > total {
				return ErrInvalidCheckout
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO payment_receipt_allocations (receipt_id,order_id,amount_cents) VALUES ($1,$2,$3)`, receiptID, allocation.OrderID, allocation.AmountCents); err != nil {
				return err
			}
		}
		for orderID := range affected {
			if err := reconcileInvoiceSettlement(ctx, tx, orderID); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE payment_receipts SET updated_at=NOW() WHERE id=$1`, receiptID)
		return err
	})
}

func reconcileInvoiceSettlement(ctx context.Context, tx *sql.Tx, orderID string) error {
	var status string
	var total, applied int
	if err := tx.QueryRowContext(ctx, `SELECT o.status, i.total_cents, COALESCE((SELECT SUM(amount_cents) FROM payment_receipt_allocations WHERE order_id=o.id),0) FROM orders o JOIN invoices i ON i.order_id=o.id WHERE o.id=$1 FOR UPDATE`, orderID).Scan(&status, &total, &applied); err != nil {
		return err
	}
	if applied == total && status == "fulfilled" {
		if err := transitionOrderStatus(ctx, tx, orderID, "fulfilled", "paid", "invoice_receipt_fully_allocated"); err != nil {
			return err
		}
		if err := enqueueOrderEmail(ctx, tx, orderID, "Zahlung bestätigt", "Wir haben deine Überweisung vollständig deiner Rechnung zugeordnet."); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE payment_transactions SET status='received', received_at=COALESCE(received_at, NOW()), updated_at=NOW() WHERE order_id=$1`, orderID); err != nil {
			return err
		}
		if err := setReceivableStatus(ctx, tx, orderID, "paid", "invoice_receipt_fully_allocated"); err != nil {
			return err
		}
		return enqueueOrderEvent(ctx, tx, orderID, "invoice.paid", map[string]string{"order_id": orderID})
	}
	if applied < total && status == "paid" {
		if _, err := tx.ExecContext(ctx, `UPDATE orders SET status='fulfilled', paid_at=NULL WHERE id=$1`, orderID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_status_history (order_id, from_status, to_status, reason) VALUES ($1,'paid','fulfilled','invoice_receipt_reallocated')`, orderID); err != nil {
			return err
		}
		if err := setReceivableStatus(ctx, tx, orderID, "sent", "invoice_receipt_reallocated"); err != nil {
			return err
		}
		return enqueueOrderEvent(ctx, tx, orderID, "order.status_changed", map[string]string{"order_id": orderID, "from_status": "paid", "to_status": "fulfilled", "reason": "invoice_receipt_reallocated"})
	}
	return nil
}

func setReceivableStatus(ctx context.Context, tx *sql.Tx, orderID, next, reason string) error {
	var current string
	if err := tx.QueryRowContext(ctx, `SELECT receivable_status FROM payment_transactions WHERE order_id=$1 FOR UPDATE`, orderID).Scan(&current); err != nil {
		return err
	}
	if current == next {
		return nil
	}
	if !allowedReceivableTransitions[current][next] {
		return ErrInvalidTransition
	}
	if _, err := tx.ExecContext(ctx, `UPDATE payment_transactions SET receivable_status=$2, updated_at=NOW() WHERE order_id=$1`, orderID, next); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO receivable_status_history (order_id, from_status, to_status, reason) VALUES ($1,$2,$3,$4)`, orderID, current, next, reason)
	return err
}

func (s *Store) TransferPaymentByOrderID(ctx context.Context, orderID string) (PaymentTransaction, error) {
	var p PaymentTransaction
	err := s.db.QueryRowContext(ctx, `SELECT order_id, provider, reference, status, amount_cents, payer_name, payer_iban_last4, note, received_at, created_at FROM payment_transactions WHERE order_id = $1`, orderID).
		Scan(&p.OrderID, &p.Provider, &p.Reference, &p.Status, &p.AmountCents, &p.PayerName, &p.PayerIBANLast4, &p.Note, &p.ReceivedAt, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PaymentTransaction{}, ErrNotFound
	}
	return p, err
}

// SetTransferPaymentReceived records reconciliation data and only then advances
// the order. The transaction lock prevents duplicate inbound bank statements
// from fulfilling an order twice.
func (s *Store) SetTransferPaymentReceived(ctx context.Context, orderID, payerName, ibanLast4, note, receivedBy string) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM payment_transactions WHERE order_id = $1 FOR UPDATE`, orderID).Scan(&status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status == "received" {
			return nil
		}
		if status != "awaiting_payment" {
			return ErrInvalidTransition
		}
		var orderStatus string
		var reserved bool
		if err := tx.QueryRowContext(ctx, `SELECT status, inventory_reserved FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&orderStatus, &reserved); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if orderStatus != "pending_payment" && orderStatus != "fulfilled" {
			return ErrInvalidTransition
		}
		if orderStatus == "pending_payment" && !reserved {
			if err := reserveInventoryForOrder(ctx, tx, orderID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE orders SET inventory_reserved = TRUE WHERE id = $1`, orderID); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE payment_transactions SET status='received', payer_name=$2, payer_iban_last4=$3, note=$4, received_by=$5, received_at=NOW(), updated_at=NOW() WHERE order_id=$1`, orderID, payerName, ibanLast4, note, receivedBy); err != nil {
			return err
		}
		if err := transitionOrderStatus(ctx, tx, orderID, orderStatus, "paid", "mitte_transfer_received"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE checkout_sagas SET status = 'completed', last_error = '', updated_at = NOW()
			WHERE order_id = $1`, orderID); err != nil {
			return err
		}
		if orderStatus == "fulfilled" {
			return enqueueOrderEvent(ctx, tx, orderID, "invoice.paid", map[string]string{"order_id": orderID})
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO warehouse_fulfillments (order_id, status)
			SELECT $1, 'ready_to_pick'
			WHERE EXISTS (SELECT 1 FROM order_items oi WHERE oi.order_id=$1 AND BTRIM(COALESCE(oi.supplier_name,''))='')
			ON CONFLICT (order_id) DO NOTHING`, orderID); err != nil {
			return err
		}
		if err := enqueueOrderEmail(ctx, tx, orderID, "Zahlung bestätigt", "Wir haben deine Überweisung erhalten und bereiten deine Bestellung vor. Die Gesamtrechnung folgt nach dem vollständigen Versand aller Positionen."); err != nil {
			return err
		}
		return nil
	})
}
