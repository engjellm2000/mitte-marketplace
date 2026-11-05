package store

import (
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsRetryableTransactionError(t *testing.T) {
	if !IsRetryableTransactionError(driver.ErrBadConn) {
		t.Fatal("driver.ErrBadConn should be retryable")
	}
	for _, code := range []string{"40P01", "40001", "55P03"} {
		t.Run(code, func(t *testing.T) {
			if !IsRetryableTransactionError(&pgconn.PgError{Code: code}) {
				t.Fatalf("code %s should be retryable", code)
			}
		})
	}
	if IsRetryableTransactionError(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("unique violation must not be retried")
	}
	if IsRetryableTransactionError(errors.New("ordinary error")) {
		t.Fatal("ordinary errors must not be retried")
	}
}
