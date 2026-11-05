package store

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var ErrPaymentChallengeInvalid = errors.New("invalid payment challenge")
var ErrPaymentChallengeRateLimited = errors.New("payment challenge rate limited")

// IssueMittePayChallenge atomically replaces a previous unused code and
// queues its email. The payment session and the payer email must belong to the
// same pending order.
func (s *Store) IssueMittePayChallenge(ctx context.Context, orderID, sessionID, email string, codeHash []byte, expiresAt time.Time, subject, body string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var status, expectedSessionID, customerEmail string
		if err := tx.QueryRowContext(ctx, `SELECT status, COALESCE(stripe_checkout_session_id, ''), customer_email FROM orders WHERE id = $1 FOR UPDATE`, orderID).
			Scan(&status, &expectedSessionID, &customerEmail); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status != "pending_payment" || expectedSessionID != sessionID || !strings.EqualFold(customerEmail, email) {
			return ErrPaymentChallengeInvalid
		}
		var recent bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM mitte_pay_challenges
			WHERE session_id = $1 AND email = $2 AND last_sent_at > NOW() - INTERVAL '60 seconds')`, sessionID, email).Scan(&recent); err != nil {
			return err
		}
		if recent {
			return ErrPaymentChallengeRateLimited
		}
		if _, err := tx.ExecContext(ctx, `UPDATE mitte_pay_challenges SET consumed_at = NOW()
			WHERE session_id = $1 AND email = $2 AND consumed_at IS NULL`, sessionID, email); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO mitte_pay_challenges (order_id, session_id, email, code_hash, expires_at)
			VALUES ($1, $2, $3, $4, $5)`, orderID, sessionID, email, codeHash, expiresAt); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO email_outbox_events (order_id, recipient, subject, body) VALUES ($1, $2, $3, $4)`, orderID, email, subject, body)
		return err
	})
}

// ConsumeMittePayChallenge verifies a short-lived code under a row lock.
// An attempt is consumed before the order can transition, so concurrent submits
// cannot approve the same session twice.
func (s *Store) ConsumeMittePayChallenge(ctx context.Context, sessionID, email string, codeHash []byte) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var orderID string
	err := s.withTransaction(ctx, func(tx *sql.Tx) error {
		var id int64
		var storedHash []byte
		var expiresAt time.Time
		var consumedAt *time.Time
		var attempts int
		err := tx.QueryRowContext(ctx, `SELECT id, order_id, code_hash, expires_at, consumed_at, attempts
			FROM mitte_pay_challenges WHERE session_id = $1 AND email = $2
			ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, sessionID, email).
			Scan(&id, &orderID, &storedHash, &expiresAt, &consumedAt, &attempts)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPaymentChallengeInvalid
		}
		if err != nil {
			return err
		}
		if consumedAt != nil || attempts >= 5 || !time.Now().Before(expiresAt) {
			return ErrPaymentChallengeInvalid
		}
		if subtle.ConstantTimeCompare(storedHash, codeHash) != 1 {
			_, err := tx.ExecContext(ctx, `UPDATE mitte_pay_challenges SET attempts = attempts + 1 WHERE id = $1`, id)
			if err != nil {
				return err
			}
			return ErrPaymentChallengeInvalid
		}
		_, err = tx.ExecContext(ctx, `UPDATE mitte_pay_challenges SET consumed_at = NOW() WHERE id = $1 AND consumed_at IS NULL`, id)
		return err
	})
	return orderID, err
}
