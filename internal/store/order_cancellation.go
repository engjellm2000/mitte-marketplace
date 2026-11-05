package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) CancelPendingOrder(ctx context.Context, orderID, reason string) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		_, err := cancelPendingOrderTx(ctx, tx, orderID, reason)
		return err
	})
}

// CancelPaidOrder is called only after the payment provider has accepted an
// idempotent refund. A customer cannot cancel an order once it has shipped.
func (s *Store) CancelPaidOrder(ctx context.Context, orderID string, customerID int64, refundID string) (Order, error) {
	err := s.withTransaction(ctx, func(tx *sql.Tx) error {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM orders WHERE id = $1 AND customer_id = $2 FOR UPDATE`, orderID, customerID).Scan(&status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status != "paid" {
			return ErrInvalidTransition
		}
		if err := restoreReservedInventory(ctx, tx, orderID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE orders SET refund_id = $2, refunded_at = NOW() WHERE id = $1`, orderID, refundID); err != nil {
			return err
		}
		if err := transitionOrderStatus(ctx, tx, orderID, "paid", "cancelled", "customer_cancelled_refunded"); err != nil {
			return err
		}
		return enqueueOrderEmail(ctx, tx, orderID, "Bestellung storniert", "Deine Bestellung wurde storniert. Die Erstattung wurde automatisch ausgelöst.")
	})
	if err != nil {
		return Order{}, err
	}
	return s.OrderByIDForCustomer(ctx, orderID, customerID)
}

// CancelPaidOrderForOperations records a refund that has already been accepted
// by the payment provider. It keeps the payment-provider action outside the DB
// transaction while retaining an auditable cancellation and notification.
func (s *Store) CancelPaidOrderForOperations(ctx context.Context, orderID, refundID, reason string) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status != "paid" {
			return ErrInvalidTransition
		}
		if err := restoreReservedInventory(ctx, tx, orderID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE orders SET refund_id=$2, refunded_at=NOW() WHERE id=$1`, orderID, refundID); err != nil {
			return err
		}
		if err := transitionOrderStatus(ctx, tx, orderID, "paid", "cancelled", "oms_refund:"+reason); err != nil {
			return err
		}
		return enqueueOrderEmail(ctx, tx, orderID, "Erstattung veranlasst", "Wir haben deine Erstattung veranlasst. Referenz: "+refundID)
	})
}

func (s *Store) CancelPendingOrderForOperations(ctx context.Context, orderID, reason string) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status != "pending_payment" {
			return ErrInvalidTransition
		}
		if err := restoreReservedInventory(ctx, tx, orderID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM promotion_redemptions WHERE order_id=$1`, orderID); err != nil {
			return err
		}
		if err := transitionOrderStatus(ctx, tx, orderID, status, "cancelled", "oms_cancelled:"+strings.TrimSpace(reason)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE checkout_sagas SET status = 'compensated', updated_at = NOW()
			WHERE order_id = $1 AND status <> 'completed'`, orderID); err != nil {
			return err
		}
		return enqueueOrderEmail(ctx, tx, orderID, "Bestellung storniert", "Deine offene Bestellung wurde storniert. Es wurde keine Zahlung eingezogen.")
	})
}

// MarkOrderShipped is deliberately a store operation, not a public customer
// API. A back-office process or Kafka fulfilment consumer calls it after label
// creation and supplies the carrier tracking details.

func cancelPendingOrderTx(ctx context.Context, tx *sql.Tx, orderID, reason string) (bool, error) {
	var status string
	var reserved bool
	if err := tx.QueryRowContext(ctx, `SELECT status, inventory_reserved FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&status, &reserved); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrNotFound
		}
		return false, err
	}
	if status == "cancelled" || status == "paid" || status == "fulfilled" {
		return false, nil
	}
	if reserved {
		if err := restoreReservedInventory(ctx, tx, orderID); err != nil {
			return false, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM promotion_redemptions WHERE order_id = $1`, orderID); err != nil {
		return false, err
	}
	if err := transitionOrderStatus(ctx, tx, orderID, "pending_payment", "cancelled", reason); err != nil {
		if errors.Is(err, ErrInvalidTransition) {
			return false, nil
		}
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE checkout_sagas SET status = 'compensated', updated_at = NOW()
		WHERE order_id = $1 AND status <> 'completed'`, orderID); err != nil {
		return false, err
	}
	return true, nil
}
