package store

import (
	"testing"
	"unicode/utf8"
)

func TestMarketingEmailContentAllowsIncompleteDraftsButRequiresSendableCopy(t *testing.T) {
	draft := MarketingEmailCampaign{Name: "Autumn update"}
	if !validMarketingEmailContent(draft, false) {
		t.Fatal("validMarketingEmailContent() rejected a draft that can be completed later")
	}
	if validMarketingEmailContent(draft, true) {
		t.Fatal("validMarketingEmailContent() accepted an incomplete campaign for sending")
	}
	draft.Subject = "Neu bei MITTE"
	draft.Body = "Entdecke unsere aktuellen Neuigkeiten."
	if !validMarketingEmailContent(draft, true) {
		t.Fatal("validMarketingEmailContent() rejected complete campaign copy")
	}
	draft.Subject = "Betreff\r\nBcc: customer@example.test"
	if validMarketingEmailContent(draft, false) {
		t.Fatal("validMarketingEmailContent() accepted a subject with header injection")
	}
}

func TestEmailUnsubscribeTokensAreOpaqueAndStoredAsDigests(t *testing.T) {
	token, digest, err := newEmailUnsubscribeToken()
	if err != nil {
		t.Fatalf("newEmailUnsubscribeToken() error = %v", err)
	}
	if utf8.RuneCountInString(token) != 64 || len(digest) != 32 {
		t.Fatalf("unsubscribe token lengths = %d/%d, want 64/32", len(token), len(digest))
	}
	parsed, ok := EmailUnsubscribeTokenHash(token)
	if !ok || string(parsed) != string(digest) {
		t.Fatal("EmailUnsubscribeTokenHash() did not return the stored digest")
	}
	if _, ok := EmailUnsubscribeTokenHash("not-a-token"); ok {
		t.Fatal("EmailUnsubscribeTokenHash() accepted an invalid token")
	}
	second, _, err := newEmailUnsubscribeToken()
	if err != nil {
		t.Fatalf("second newEmailUnsubscribeToken() error = %v", err)
	}
	if token == second {
		t.Fatal("unsubscribe tokens repeated")
	}
}
