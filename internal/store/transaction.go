package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// transactionAttempts is intentionally small. Retrying a deadlock is useful;
// turning a saturated database into a retry storm is not.
const transactionAttempts = 3

// withTransaction retries only PostgreSQL errors for which the failed
// transaction was rolled back by the server. Callers must keep side effects
// (HTTP calls, message publication, email) outside fn: a retry can execute fn
// more than once. Checkout and webhook transitions are safe because they are
// idempotent database operations and external effects happen after commit.
func (s *Store) withTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	var err error
	for attempt := 0; attempt < transactionAttempts; attempt++ {
		tx, beginErr := s.db.BeginTx(ctx, nil)
		if beginErr != nil {
			err = beginErr
		} else {
			// Bound lock waits so a hot row cannot consume the entire HTTP or
			// webhook deadline. PostgreSQL reports a lock timeout as 55P03,
			// which is retried below with backoff.
			err = setTransactionGuards(ctx, tx)
			if err == nil {
				err = fn(tx)
			}
			if err == nil {
				err = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
		}
		if !IsRetryableTransactionError(err) || attempt == transactionAttempts-1 || ctx.Err() != nil {
			return err
		}
		if err := waitForRetry(ctx, attempt); err != nil {
			return err
		}
	}
	return err
}

func setTransactionGuards(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, "SET LOCAL lock_timeout = '1s'"); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "SET LOCAL idle_in_transaction_session_timeout = '5s'")
	return err
}

// IsRetryableTransactionError reports PostgreSQL failures for which a caller
// may safely retry a complete idempotent operation.
func IsRetryableTransactionError(err error) bool {
	// database/sql returns ErrBadConn when a pooled connection has been closed
	// underneath a transaction. No work from that transaction was committed, so
	// retrying the complete idempotent database operation is safe.
	if errors.Is(err, driver.ErrBadConn) {
		return true
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "40P01", // deadlock_detected
		"40001", // serialization_failure
		"55P03": // lock_not_available (including a configured lock_timeout)
		return true
	default:
		return false
	}
}

func waitForRetry(ctx context.Context, attempt int) error {
	// A bounded, non-zero jitter prevents synchronized retry waves without
	// making a user-facing mutation wait longer than 35 ms across all retries.
	delay := time.Duration(5*(1<<attempt)+int(time.Now().UnixNano()%5)) * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
