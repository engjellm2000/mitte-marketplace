package commerce

import "time"

const ReturnWindow = 30 * 24 * time.Hour

type ReturnDecision struct {
	Eligible      bool
	Reason        string
	ExpiresAt     time.Time
	DaysRemaining int
}

func ReturnEligibility(paidAt time.Time, now time.Time) ReturnDecision {
	if paidAt.IsZero() {
		return ReturnDecision{Reason: "not_paid"}
	}
	expires := paidAt.Add(ReturnWindow)
	if now.After(expires) {
		return ReturnDecision{Reason: "window_closed", ExpiresAt: expires}
	}
	remaining := int(expires.Sub(now).Hours() / 24)
	if remaining < 1 && now.Before(expires) {
		remaining = 1
	}
	return ReturnDecision{Eligible: true, Reason: "within_window", ExpiresAt: expires, DaysRemaining: remaining}
}
