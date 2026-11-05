package web

import (
	"errors"
	"net/http"
	"testing"

	"mitte/internal/store"
)

func TestFraudBlockedCheckoutReturnsConflict(t *testing.T) {
	err := store.ErrCustomerFraudBlocked
	if status := cartErrorStatus(err); status != http.StatusConflict {
		t.Fatalf("cartErrorStatus(fraud block) = %d, want %d", status, http.StatusConflict)
	}
	if message := cartErrorMessage(errors.Join(err, errors.New("detail"))); message != cartErrorMessage(err) {
		t.Fatalf("cartErrorMessage should preserve the fraud-block response: %q", message)
	}
}
