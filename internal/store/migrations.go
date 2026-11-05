package store

import _ "embed"

//go:embed migrations/001_initial_schema.sql
var schemaSQL string

//go:embed migrations/002_order_status.sql
var orderStatusSQL string

//go:embed migrations/003_payments.sql
var paymentsSQL string

//go:embed migrations/004_checkout_reliability.sql
var checkoutReliabilitySQL string

//go:embed migrations/005_commerce.sql
var commerceSQL string

//go:embed migrations/006_integrity_guards.sql
var integrityGuardsSQL string

//go:embed migrations/007_schema_hardening.sql
var schemaHardeningSQL string

//go:embed migrations/008_customer_authentication.sql
var customerAuthenticationSQL string

//go:embed migrations/009_order_outbox.sql
var orderOutboxSQL string

//go:embed migrations/010_payment_expiry.sql
var paymentExpirySQL string

//go:embed migrations/011_outbox_parallelism.sql
var outboxParallelismSQL string

//go:embed migrations/012_b2b_b2c_checkout.sql
var b2bB2CCheckoutSQL string

//go:embed migrations/013_customer_profiles.sql
var customerProfilesSQL string

//go:embed migrations/014_late_stripe_payment_reconciliation.sql
var lateStripePaymentReconciliationSQL string

//go:embed migrations/015_order_operations.sql
var orderOperationsSQL string

//go:embed migrations/016_order_management.sql
var orderManagementSQL string

//go:embed migrations/017_warehouse_operations.sql
var warehouseOperationsSQL string

//go:embed migrations/018_invoice_documents.sql
var invoiceDocumentsSQL string

//go:embed migrations/019_morgenform_pay_challenges.sql
var mittePayChallengesSQL string

//go:embed migrations/020_payment_provider_ledger.sql
var paymentProviderLedgerSQL string

//go:embed migrations/021_development_invoice_provider.sql
var developmentInvoiceProviderSQL string

//go:embed migrations/022_invoice_settlement_after_shipment.sql
var invoiceSettlementAfterShipmentSQL string

//go:embed migrations/023_invoice_receivables.sql
var invoiceReceivablesSQL string

//go:embed migrations/024_invoice_order_transitions.sql
var invoiceOrderTransitionsSQL string

//go:embed migrations/025_backfill_invoice_receivables.sql
var backfillInvoiceReceivablesSQL string

//go:embed migrations/026_receivable_statuses.sql
var receivableStatusesSQL string

//go:embed migrations/027_order_item_operations.sql
var orderItemOperationsSQL string

//go:embed migrations/028_catalog_management.sql
var catalogManagementSQL string

//go:embed migrations/029_order_item_sku_and_replacement_requests.sql
var orderItemSKUAndReplacementRequestsSQL string

//go:embed migrations/030_inventory_control.sql
var inventoryControlSQL string

//go:embed migrations/031_shipment_invoice_delivery.sql
var shipmentInvoiceDeliverySQL string

//go:embed migrations/032_order_items_product_lookup.sql
var orderItemsProductLookupSQL string

//go:embed migrations/033_outbox_delivery_leases.sql
var outboxDeliveryLeasesSQL string

//go:embed migrations/034_product_specifications.sql
var productSpecificationsSQL string

//go:embed migrations/036_remove_demo_seed_data.sql
var removeDemoSeedDataSQL string

//go:embed migrations/037_product_feed_indexes.sql
var productFeedIndexesSQL string

//go:embed migrations/038_operator_sessions.sql
var operatorSessionsSQL string

//go:embed migrations/039_marketplace_categories.sql
var marketplaceCategoriesSQL string

//go:embed migrations/040_recommendation_consent.sql
var recommendationConsentSQL string

//go:embed migrations/041_warehouse_returns.sql
var warehouseReturnsSQL string

//go:embed migrations/042_customer_email_authentication.sql
var customerEmailAuthenticationSQL string

//go:embed migrations/043_marketing_campaigns.sql
var marketingCampaignsSQL string

//go:embed migrations/044_email_marketing.sql
var emailMarketingSQL string

//go:embed migrations/045_supplier_portal.sql
var supplierPortalSQL string

//go:embed migrations/046_partial_shipments.sql
var partialShipmentsSQL string

//go:embed migrations/047_legacy_supplier_shipment_deduplication.sql
var legacySupplierShipmentDeduplicationSQL string

//go:embed migrations/048_customer_fraud_flags.sql
var customerFraudFlagsSQL string

//go:embed migrations/049_numeric_order_numbers.sql
var numericOrderNumbersSQL string

//go:embed migrations/050_checkout_sagas.sql
var checkoutSagasSQL string

//go:embed migrations/051_warehouse_fulfillment_holds.sql
var warehouseFulfillmentHoldsSQL string

//go:embed migrations/052_mitte_branding.sql
var mitteBrandingSQL string

//go:embed migrations/035_category_hierarchy.sql
var categoryHierarchySQL string

// migration is an append-only database change. Versions are recorded in
// schema_migrations, so deployed databases never re-run an applied change.
type migration struct {
	Version int
	Name    string
	SQL     string
}

// appMigrations is intentionally ordered and versioned in code. Add new
// migrations at the end; never edit SQL that may already be recorded in a
// production schema_migrations table.
var appMigrations = []migration{
	{Version: 1, Name: "initial_schema", SQL: schemaSQL},
	{Version: 2, Name: "order_status", SQL: orderStatusSQL},
	{Version: 3, Name: "payments", SQL: paymentsSQL},
	{Version: 4, Name: "checkout_reliability", SQL: checkoutReliabilitySQL},
	{Version: 5, Name: "commerce", SQL: commerceSQL},
	{Version: 6, Name: "integrity_guards", SQL: integrityGuardsSQL},
	{Version: 7, Name: "schema_hardening", SQL: schemaHardeningSQL},
	{Version: 8, Name: "customer_authentication", SQL: customerAuthenticationSQL},
	{Version: 9, Name: "order_outbox", SQL: orderOutboxSQL},
	{Version: 10, Name: "payment_expiry", SQL: paymentExpirySQL},
	{Version: 11, Name: "outbox_parallelism", SQL: outboxParallelismSQL},
	{Version: 12, Name: "b2b_b2c_checkout", SQL: b2bB2CCheckoutSQL},
	{Version: 13, Name: "customer_profiles", SQL: customerProfilesSQL},
	{Version: 14, Name: "late_stripe_payment_reconciliation", SQL: lateStripePaymentReconciliationSQL},
	{Version: 15, Name: "order_operations", SQL: orderOperationsSQL},
	{Version: 16, Name: "order_management", SQL: orderManagementSQL},
	{Version: 17, Name: "warehouse_operations", SQL: warehouseOperationsSQL},
	{Version: 18, Name: "invoice_documents", SQL: invoiceDocumentsSQL},
	{Version: 19, Name: "morgenform_pay_challenges", SQL: mittePayChallengesSQL},
	{Version: 20, Name: "payment_provider_ledger", SQL: paymentProviderLedgerSQL},
	{Version: 21, Name: "development_invoice_provider", SQL: developmentInvoiceProviderSQL},
	{Version: 22, Name: "invoice_settlement_after_shipment", SQL: invoiceSettlementAfterShipmentSQL},
	{Version: 23, Name: "invoice_receivables", SQL: invoiceReceivablesSQL},
	{Version: 24, Name: "invoice_order_transitions", SQL: invoiceOrderTransitionsSQL},
	{Version: 25, Name: "backfill_invoice_receivables", SQL: backfillInvoiceReceivablesSQL},
	{Version: 26, Name: "receivable_statuses", SQL: receivableStatusesSQL},
	{Version: 27, Name: "order_item_operations", SQL: orderItemOperationsSQL},
	{Version: 28, Name: "catalog_management", SQL: catalogManagementSQL},
	{Version: 29, Name: "order_item_sku_and_replacement_requests", SQL: orderItemSKUAndReplacementRequestsSQL},
	{Version: 30, Name: "inventory_control", SQL: inventoryControlSQL},
	{Version: 31, Name: "shipment_invoice_delivery", SQL: shipmentInvoiceDeliverySQL},
	{Version: 32, Name: "order_items_product_lookup", SQL: orderItemsProductLookupSQL},
	{Version: 33, Name: "outbox_delivery_leases", SQL: outboxDeliveryLeasesSQL},
	{Version: 34, Name: "product_specifications", SQL: productSpecificationsSQL},
	{Version: 35, Name: "category_hierarchy", SQL: categoryHierarchySQL},
	{Version: 36, Name: "remove_demo_seed_data", SQL: removeDemoSeedDataSQL},
	{Version: 37, Name: "product_feed_indexes", SQL: productFeedIndexesSQL},
	{Version: 38, Name: "operator_sessions", SQL: operatorSessionsSQL},
	{Version: 39, Name: "marketplace_categories", SQL: marketplaceCategoriesSQL},
	{Version: 40, Name: "recommendation_consent", SQL: recommendationConsentSQL},
	{Version: 41, Name: "warehouse_returns", SQL: warehouseReturnsSQL},
	{Version: 42, Name: "customer_email_authentication", SQL: customerEmailAuthenticationSQL},
	{Version: 43, Name: "marketing_campaigns", SQL: marketingCampaignsSQL},
	{Version: 44, Name: "email_marketing", SQL: emailMarketingSQL},
	{Version: 45, Name: "supplier_portal", SQL: supplierPortalSQL},
	{Version: 46, Name: "partial_shipments", SQL: partialShipmentsSQL},
	{Version: 47, Name: "legacy_supplier_shipment_deduplication", SQL: legacySupplierShipmentDeduplicationSQL},
	{Version: 48, Name: "customer_fraud_flags", SQL: customerFraudFlagsSQL},
	{Version: 49, Name: "numeric_order_numbers", SQL: numericOrderNumbersSQL},
	{Version: 50, Name: "checkout_sagas", SQL: checkoutSagasSQL},
	{Version: 51, Name: "warehouse_fulfillment_holds", SQL: warehouseFulfillmentHoldsSQL},
	{Version: 52, Name: "mitte_branding", SQL: mitteBrandingSQL},
}
