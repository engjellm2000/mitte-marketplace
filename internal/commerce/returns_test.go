package commerce

import (
	"testing"
	"time"
)

func TestReturnEligibilityWindow(t *testing.T) {
	paid := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	now := paid.Add(10 * 24 * time.Hour)
	decision := ReturnEligibility(paid, now)
	if !decision.Eligible || decision.DaysRemaining != 20 {
		t.Fatalf("%#v", decision)
	}

	closed := ReturnEligibility(paid, paid.Add(ReturnWindow+time.Hour))
	if closed.Eligible || closed.Reason != "window_closed" {
		t.Fatalf("%#v", closed)
	}

	unpaid := ReturnEligibility(time.Time{}, now)
	if unpaid.Eligible || unpaid.Reason != "not_paid" {
		t.Fatalf("%#v", unpaid)
	}
}
