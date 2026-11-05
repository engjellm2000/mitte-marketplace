package store

import (
	"strings"
	"testing"
)

func TestMigrationsAreRegisteredInOrder(t *testing.T) {
	var foundLookup, foundLeases, foundDemoCleanup, foundProductFeed, foundOperatorSessions bool
	var foundMarketplaceCategories, foundRecommendationConsent, foundWarehouseReturns, foundCustomerEmailAuth bool
	var foundEmailMarketing, foundPartialShipments, foundLegacySupplierShipmentDeduplication bool
	var foundCustomerFraudFlags bool
	var foundNumericOrderNumbers bool
	var foundMitteBranding bool
	for index, item := range appMigrations {
		if index > 0 && appMigrations[index-1].Version >= item.Version {
			t.Fatalf("migration versions are not strictly increasing at %d", item.Version)
		}
		switch item.Version {
		case 32:
			foundLookup = item.Name == "order_items_product_lookup" && strings.Contains(item.SQL, "order_items_product_id_idx")
		case 33:
			foundLeases = item.Name == "outbox_delivery_leases" &&
				strings.Contains(item.SQL, "order_outbox_events") &&
				strings.Contains(item.SQL, "email_outbox_events") &&
				strings.Contains(item.SQL, "claimed_until")
		case 36:
			foundDemoCleanup = item.Name == "remove_demo_seed_data" &&
				strings.Contains(item.SQL, "legacy_demo_products") &&
				strings.Contains(item.SQL, "seed-order-%") &&
				strings.Contains(item.SQL, "carts.status = 'open'") &&
				strings.Contains(item.SQL, "carts.status = 'checked_out'") &&
				!strings.Contains(item.SQL, "INSERT INTO products")
		case 37:
			foundProductFeed = item.Name == "product_feed_indexes" &&
				strings.Contains(item.SQL, "products_active_featured_created_idx") &&
				strings.Contains(item.SQL, "products_active_price_low_idx") &&
				strings.Contains(item.SQL, "products_active_price_high_idx")
		case 38:
			foundOperatorSessions = item.Name == "operator_sessions" &&
				strings.Contains(item.SQL, "CREATE TABLE operator_sessions") &&
				strings.Contains(item.SQL, "token_hash BYTEA PRIMARY KEY") &&
				strings.Contains(item.SQL, "revoked_at")
		case 39:
			foundMarketplaceCategories = item.Name == "marketplace_categories" &&
				strings.Contains(item.SQL, "('technik', 'Technik'") &&
				strings.Contains(item.SQL, "('monitore', 'Monitore'") &&
				strings.Contains(item.SQL, "('elektrowerkzeuge', 'Elektrowerkzeuge'")
		case 40:
			foundRecommendationConsent = item.Name == "recommendation_consent" &&
				strings.Contains(item.SQL, "recommendation_consent BOOLEAN NOT NULL DEFAULT FALSE") &&
				strings.Contains(item.SQL, "CREATE TABLE IF NOT EXISTS customer_product_views") &&
				strings.Contains(item.SQL, "ON DELETE CASCADE")
		case 41:
			foundWarehouseReturns = item.Name == "warehouse_returns" &&
				strings.Contains(item.SQL, "CREATE TABLE warehouse_return_cases") &&
				strings.Contains(item.SQL, "replacement_with_return") &&
				strings.Contains(item.SQL, "return_restock")
		case 42:
			foundCustomerEmailAuth = item.Name == "customer_email_authentication" &&
				strings.Contains(item.SQL, "email_verified_at") &&
				strings.Contains(item.SQL, "UPDATE user_sessions") &&
				strings.Contains(item.SQL, "CREATE TABLE user_auth_tokens") &&
				strings.Contains(item.SQL, "ALTER COLUMN order_id DROP NOT NULL")
		case 44:
			foundEmailMarketing = item.Name == "email_marketing" &&
				strings.Contains(item.SQL, "marketing_email_consent BOOLEAN NOT NULL DEFAULT FALSE") &&
				strings.Contains(item.SQL, "CREATE TABLE marketing_email_campaigns") &&
				strings.Contains(item.SQL, "CREATE TABLE email_marketing_unsubscribe_tokens") &&
				strings.Contains(item.SQL, "cancelled_at TIMESTAMPTZ")
		case 46:
			foundPartialShipments = item.Name == "partial_shipments" &&
				strings.Contains(item.SQL, "shipments_order_id_key") &&
				strings.Contains(item.SQL, "CREATE TABLE IF NOT EXISTS shipment_items") &&
				strings.Contains(item.SQL, "source_event_id") &&
				strings.Contains(item.SQL, "ON CONFLICT (source_event_id) WHERE source_event_id IS NOT NULL DO NOTHING")
		case 47:
			foundLegacySupplierShipmentDeduplication = item.Name == "legacy_supplier_shipment_deduplication" &&
				strings.Contains(item.SQL, "PARTITION BY e.order_id, e.product_id") &&
				strings.Contains(item.SQL, "tracking_key") &&
				strings.Contains(item.SQL, "item_event_rank = 1") &&
				strings.Contains(item.SQL, "canonical_shipment_id")
		case 48:
			foundCustomerFraudFlags = item.Name == "customer_fraud_flags" &&
				strings.Contains(item.SQL, "fraud_flagged BOOLEAN NOT NULL DEFAULT FALSE") &&
				strings.Contains(item.SQL, "CREATE TABLE IF NOT EXISTS customer_fraud_audit") &&
				strings.Contains(item.SQL, "actor_email TEXT NOT NULL")
		case 49:
			foundNumericOrderNumbers = item.Name == "numeric_order_numbers" &&
				strings.Contains(item.SQL, "CREATE SEQUENCE IF NOT EXISTS order_number_seq") &&
				strings.Contains(item.SQL, "START WITH 10000000") &&
				strings.Contains(item.SQL, "MAXVALUE 99999999")
		case 52:
			foundMitteBranding = item.Name == "mitte_branding" &&
				strings.Contains(item.SQL, "RENAME TO mitte_pay_challenges") &&
				strings.Contains(item.SQL, "UPDATE orders SET payment_provider = 'mitte_transfer'") &&
				strings.Contains(item.SQL, "UPDATE checkout_sagas SET payment_method = 'mitte_pay'")
		}
	}
	if !foundLookup {
		t.Fatal("migration 32 does not register the order item product lookup index")
	}
	if !foundLeases {
		t.Fatal("migration 33 does not register both outbox delivery leases")
	}
	if !foundDemoCleanup {
		t.Fatal("migration 36 does not register demo fixture cleanup")
	}
	if !foundProductFeed {
		t.Fatal("migration 37 does not register product feed indexes")
	}
	if !foundOperatorSessions {
		t.Fatal("migration 38 does not register revocable operator sessions")
	}
	if !foundMarketplaceCategories {
		t.Fatal("migration 39 does not register the marketplace category hierarchy")
	}
	if !foundRecommendationConsent {
		t.Fatal("migration 40 does not register consent-gated product view history")
	}
	if !foundWarehouseReturns {
		t.Fatal("migration 41 does not register warehouse return cases and stock movement")
	}
	if !foundCustomerEmailAuth {
		t.Fatal("migration 42 does not register customer email verification and recovery")
	}
	if !foundEmailMarketing {
		t.Fatal("migration 44 does not register account consent and marketing email delivery")
	}
	if !foundPartialShipments {
		t.Fatal("migration 46 does not register partial shipment records and legacy shipment migration")
	}
	if !foundLegacySupplierShipmentDeduplication {
		t.Fatal("migration 47 does not register legacy supplier shipment deduplication")
	}
	if !foundCustomerFraudFlags {
		t.Fatal("migration 48 does not register customer fraud status and audit history")
	}
	if !foundNumericOrderNumbers {
		t.Fatal("migration 49 does not register the eight-digit order number sequence")
	}
	if !foundMitteBranding {
		t.Fatal("migration 52 does not register the Mitte branding data migration")
	}
}

func TestNewOutboxClaimTokenIsUniqueAndOpaque(t *testing.T) {
	first, err := newOutboxClaimToken()
	if err != nil {
		t.Fatalf("newOutboxClaimToken() error: %v", err)
	}
	second, err := newOutboxClaimToken()
	if err != nil {
		t.Fatalf("newOutboxClaimToken() error: %v", err)
	}
	if len(first) != 32 || first == second {
		t.Fatalf("claim tokens are malformed or repeated: %q and %q", first, second)
	}
}

func TestNewOperatorSessionTokenIsUniqueAndOpaque(t *testing.T) {
	first, err := NewOperatorSessionToken()
	if err != nil {
		t.Fatalf("NewOperatorSessionToken() error: %v", err)
	}
	second, err := NewOperatorSessionToken()
	if err != nil {
		t.Fatalf("NewOperatorSessionToken() error: %v", err)
	}
	if len(first) != 64 || first == second {
		t.Fatal("session tokens are malformed or repeated")
	}
}
