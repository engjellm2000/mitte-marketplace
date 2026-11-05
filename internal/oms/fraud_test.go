package oms

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"mitte/internal/store"
)

func TestFraudBlockedFulfillmentReturnsConflict(t *testing.T) {
	recorder := httptest.NewRecorder()
	failure(recorder, store.ErrCustomerFraudBlocked)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("fraud-blocked fulfillment status = %d, want %d", recorder.Code, http.StatusConflict)
	}
}
