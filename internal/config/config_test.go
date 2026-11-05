package config

import (
	"strings"
	"testing"
	"time"
)

const testStaffPasswordHash = "$argon2id$v=19$m=65536,t=3,p=2$MDEyMzQ1Njc4OWFiY2RlZg$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func TestDurationValueRejectsInvalidConfiguration(t *testing.T) {
	for _, test := range []struct {
		name    string
		value   string
		want    time.Duration
		wantErr bool
	}{
		{name: "default", want: 10 * time.Second},
		{name: "valid", value: "2m", want: 2 * time.Minute},
		{name: "invalid", value: "soon", wantErr: true},
		{name: "non-positive", value: "0s", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TEST_DURATION", test.value)
			got, err := durationValue("TEST_DURATION", 10*time.Second)
			if (err != nil) != test.wantErr {
				t.Fatalf("durationValue() error = %v, wantErr %v", err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("durationValue() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestLoadProductionDefaults(t *testing.T) {
	if !validArgon2idHash(testStaffPasswordHash) {
		t.Fatal("test staff credential must be a valid Argon2id hash")
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_COMPONENT", "web")
	t.Setenv("DATABASE_URL", "postgres://user:pass@example.test/mitte?sslmode=verify-full")
	t.Setenv("TRUSTED_HOSTS", "shop.example.de, www.shop.example.de")
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8")
	t.Setenv("METRICS_BEARER_TOKEN", "a-long-random-metrics-token-for-tests")
	t.Setenv("STAFF_CREDENTIALS", "operator:"+testStaffPasswordHash)
	t.Setenv("STAFF_ROLES", "operator:admin")
	t.Setenv("RUN_MIGRATIONS", "true")
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_example")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_example")
	t.Setenv("STRIPE_SUCCESS_URL", "https://shop.example.de/bestellung/erfolg?session_id={CHECKOUT_SESSION_ID}")
	t.Setenv("STRIPE_CANCEL_URL", "https://shop.example.de/warenkorb")
	t.Setenv("KAFKA_BROKERS", "kafka-1.example.de:9092,kafka-2.example.de:9092")
	t.Setenv("PUBLIC_BASE_URL", "https://shop.example.de")
	t.Setenv("INVOICE_SELLER_NAME", "Example GmbH")
	t.Setenv("INVOICE_SELLER_STREET", "Musterstrasse 1")
	t.Setenv("INVOICE_SELLER_POSTAL_CODE", "20095")
	t.Setenv("INVOICE_SELLER_CITY", "Hamburg")
	t.Setenv("INVOICE_SELLER_COUNTRY_CODE", "DE")
	t.Setenv("INVOICE_SELLER_VAT_ID", "DE123456789")
	t.Setenv("INVOICE_SELLER_EMAIL", "invoice@example.de")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !config.SecureCookies || len(config.TrustedHosts) != 2 || !config.RunMigrations {
		t.Fatalf("unexpected config: %#v", config)
	}
	if config.StripeSecretKey == "" || config.StripeWebhookSecret == "" || len(config.KafkaBrokers) != 2 {
		t.Fatalf("expected payment and Kafka config in production: %#v", config)
	}
	if config.GRPCListenAddr != ":9090" {
		t.Fatalf("GRPCListenAddr = %q, want default :9090", config.GRPCListenAddr)
	}
}

func TestStaffCredentialsPreserveArgon2ParameterCommas(t *testing.T) {
	credentials, err := staffCredentialsValue(
		"operator:"+testStaffPasswordHash+",warehouse:"+testStaffPasswordHash,
		true,
	)
	if err != nil {
		t.Fatalf("staffCredentialsValue() error = %v", err)
	}
	if len(credentials) != 2 || credentials["operator"] != testStaffPasswordHash || credentials["warehouse"] != testStaffPasswordHash {
		t.Fatalf("parsed staff credentials = %#v, want both Argon2id hashes intact", credentials)
	}
}

func TestSupplierCredentialsPreserveArgon2ParameterCommas(t *testing.T) {
	credentials, err := supplierCredentialsValue(
		"alice:Acme Wholesale:"+testStaffPasswordHash+",bob:North Supply:local-development-password",
		false,
	)
	if err != nil {
		t.Fatalf("supplierCredentialsValue() error = %v", err)
	}
	if len(credentials) != 2 || credentials["alice"].SupplierName != "Acme Wholesale" ||
		credentials["alice"].Password != testStaffPasswordHash || credentials["bob"].SupplierName != "North Supply" {
		t.Fatalf("parsed supplier credentials = %#v, want both suppliers and intact Argon2id hash", credentials)
	}
}

func TestSupplierCredentialsRequireArgon2idInProduction(t *testing.T) {
	if _, err := supplierCredentialsValue("alice:Acme Wholesale:local-development-password", true); err == nil {
		t.Fatal("supplierCredentialsValue() error = nil, want plaintext production password rejection")
	}
	credentials, err := supplierCredentialsValue("alice:Acme Wholesale:"+testStaffPasswordHash, true)
	if err != nil || credentials["alice"].Password != testStaffPasswordHash {
		t.Fatalf("supplierCredentialsValue() = %#v, %v; want validated Argon2id credential", credentials, err)
	}
}

func TestLoadProductionWebRequiresStaffCredentials(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://user:pass@example.test/mitte?sslmode=verify-full")
	t.Setenv("STAFF_CREDENTIALS", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing production staff credentials rejection")
	}
}

func TestLoadProductionWebRequiresStaffRoles(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://user:pass@example.test/mitte?sslmode=verify-full")
	t.Setenv("STAFF_CREDENTIALS", "operator:"+testStaffPasswordHash)
	t.Setenv("STAFF_ROLES", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing production staff roles rejection")
	}
}

func TestStaffRolesRequireAValidCompleteAssignment(t *testing.T) {
	credentials := map[string]string{"orders": "secret", "warehouse": "secret"}
	for _, test := range []struct {
		name string
		raw  string
		ok   bool
	}{
		{name: "all accounts scoped", raw: "orders:cs,warehouse:warehouse", ok: true},
		{name: "missing account", raw: "orders:orders"},
		{name: "unknown account", raw: "orders:orders,other:admin"},
		{name: "unknown role", raw: "orders:root,warehouse:warehouse"},
		{name: "duplicate account", raw: "orders:orders,orders:admin"},
	} {
		t.Run(test.name, func(t *testing.T) {
			roles, err := staffRolesValue(test.raw, credentials, true)
			if (err == nil) != test.ok {
				t.Fatalf("staffRolesValue() error = %v, want success %t", err, test.ok)
			}
			if err == nil && (roles["orders"] != "cs" || roles["warehouse"] != "warehouse") {
				t.Fatalf("roles = %#v, want configured least-privilege roles", roles)
			}
		})
	}
}

func TestDevelopmentDoesNotInventStaffCredentials(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/mitte?sslmode=disable")
	t.Setenv("STAFF_CREDENTIALS", "")
	t.Setenv("STAFF_ROLES", "")
	t.Setenv("MITTE_PAY_SANDBOX_ENABLED", "false")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(config.StaffCredentials) != 0 || len(config.StaffRoles) != 0 {
		t.Fatalf("development config invented staff credentials or roles: %#v / %#v", config.StaffCredentials, config.StaffRoles)
	}
	if config.GeminiModel != "gemini-3.8-flash" {
		t.Fatalf("GeminiModel = %q, want gemini-3.8-flash by default", config.GeminiModel)
	}
}

func TestGeminiModelMustBeAPathSafeModelName(t *testing.T) {
	for _, test := range []struct {
		model string
		valid bool
	}{
		{model: "gemini-2.5-flash", valid: true},
		{model: "models/gemini-2.5-flash"},
		{model: "gemini-2.5-flash?key=secret"},
		{model: strings.Repeat("a", 81)},
	} {
		t.Run(test.model, func(t *testing.T) {
			if got := validGeminiModel(test.model); got != test.valid {
				t.Fatalf("validGeminiModel(%q) = %t, want %t", test.model, got, test.valid)
			}
		})
	}
}

func TestPaymentSandboxRequiresConfiguredOTPSecret(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/mitte?sslmode=disable")
	t.Setenv("MITTE_PAY_SANDBOX_ENABLED", "true")
	t.Setenv("MITTE_PAY_OTP_SECRET", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing OTP secret rejection")
	}
}

func TestLoadProductionRequiresStripe(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8")
	t.Setenv("METRICS_BEARER_TOKEN", "a-long-random-metrics-token-for-tests")
	t.Setenv("STAFF_CREDENTIALS", "operator:"+testStaffPasswordHash)
	t.Setenv("STAFF_ROLES", "operator:admin")
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://user:pass@example.test/mitte?sslmode=verify-full")
	t.Setenv("TRUSTED_HOSTS", "shop.example.de")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want production stripe requirement")
	}
}

func TestLoadProductionRejectsInvoiceSandbox(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8")
	t.Setenv("METRICS_BEARER_TOKEN", "a-long-random-metrics-token-for-tests")
	t.Setenv("STAFF_CREDENTIALS", "operator:"+testStaffPasswordHash)
	t.Setenv("STAFF_ROLES", "operator:admin")
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://user:pass@example.test/mitte?sslmode=verify-full")
	t.Setenv("TRUSTED_HOSTS", "shop.example.de")
	t.Setenv("MITTE_INVOICE_SANDBOX_ENABLED", "true")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want production invoice sandbox rejection")
	}
}

func TestLoadProductionRejectsUnencryptedDatabase(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://user:pass@example.test/mitte?sslmode=disable")
	t.Setenv("TRUSTED_HOSTS", "shop.example.de")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want verified TLS requirement")
	}
}

func TestLoadProductionRejectsDatabaseTLSWithoutHostnameVerification(t *testing.T) {
	for _, databaseURL := range []string{
		"postgres://user:pass@example.test/mitte?sslmode=require",
		"postgres://user:pass@example.test/mitte?sslmode=disable",
		"postgres://user:pass@example.test/mitte",
		"postgres://user:pass@example.test/mitte?sslmode=verify-full&sslmode=disable",
	} {
		if err := validateProductionDatabaseURL(databaseURL); err == nil || !strings.Contains(err.Error(), "sslmode=verify-full") {
			t.Errorf("validateProductionDatabaseURL(%q) error = %v, want sslmode=verify-full requirement", databaseURL, err)
		}
	}
}

func TestLoadProductionRejectsUnsafeStripeURLs(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8")
	t.Setenv("METRICS_BEARER_TOKEN", "a-long-random-metrics-token-for-tests")
	t.Setenv("STAFF_CREDENTIALS", "operator:"+testStaffPasswordHash)
	t.Setenv("STAFF_ROLES", "operator:admin")
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://user:pass@example.test/mitte?sslmode=verify-full")
	t.Setenv("TRUSTED_HOSTS", "shop.example.de")
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_example")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_example")
	t.Setenv("STRIPE_SUCCESS_URL", "http://shop.example.de/bestellung/erfolg?session_id={CHECKOUT_SESSION_ID}")
	t.Setenv("STRIPE_CANCEL_URL", "https://shop.example.de/warenkorb")
	t.Setenv("PUBLIC_BASE_URL", "https://shop.example.de")
	t.Setenv("INVOICE_SELLER_NAME", "Example GmbH")
	t.Setenv("INVOICE_SELLER_STREET", "Musterstrasse 1")
	t.Setenv("INVOICE_SELLER_POSTAL_CODE", "20095")
	t.Setenv("INVOICE_SELLER_CITY", "Hamburg")
	t.Setenv("INVOICE_SELLER_COUNTRY_CODE", "DE")
	t.Setenv("INVOICE_SELLER_VAT_ID", "DE123456789")
	t.Setenv("INVOICE_SELLER_EMAIL", "invoice@example.de")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want non-HTTPS Stripe URL rejection")
	}
}

func TestLoadProductionWorkerDoesNotRequireWebCredentials(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_COMPONENT", "email-relay")
	t.Setenv("METRICS_BEARER_TOKEN", "a-long-random-metrics-token-for-tests")
	t.Setenv("DATABASE_URL", "postgres://user:pass@example.test/mitte?sslmode=verify-full")
	t.Setenv("MITTE_INVOICE_SANDBOX_ENABLED", "false")
	if _, err := Load(); err != nil {
		t.Fatalf("Load() for email relay error = %v", err)
	}
}

func TestProductionOrderRelayRequiresTLSAndSASL(t *testing.T) {
	for _, test := range []struct {
		name       string
		tlsEnabled string
		mechanism  string
		username   string
		password   string
		wantError  bool
	}{
		{name: "missing TLS", mechanism: "scram-sha-512", username: "relay", password: "secret", wantError: true},
		{name: "missing SASL", tlsEnabled: "true", wantError: true},
		{name: "valid SCRAM over TLS", tlsEnabled: "true", mechanism: "scram-sha-512", username: "relay", password: "secret"},
		{name: "credentials without mechanism", tlsEnabled: "true", username: "relay", password: "secret", wantError: true},
		{name: "mechanism without password", tlsEnabled: "true", mechanism: "plain", username: "relay", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("APP_ENV", "production")
			t.Setenv("APP_COMPONENT", "order-relay")
			t.Setenv("DATABASE_URL", "postgres://user:password@db.example.test/mitte?sslmode=verify-full")
			t.Setenv("METRICS_BEARER_TOKEN", "a-long-random-metrics-token-for-tests")
			t.Setenv("ORDER_RELAY_ENABLED", "true")
			t.Setenv("KAFKA_BROKERS", "kafka.example.test:9093")
			t.Setenv("KAFKA_TLS_ENABLED", test.tlsEnabled)
			t.Setenv("KAFKA_SASL_MECHANISM", test.mechanism)
			t.Setenv("KAFKA_SASL_USERNAME", test.username)
			t.Setenv("KAFKA_SASL_PASSWORD", test.password)

			_, err := Load()
			if (err != nil) != test.wantError {
				t.Fatalf("Load() error = %v, want error %t", err, test.wantError)
			}
		})
	}
}

func TestLoadProductionStripeReconcileRequiresStripeWithoutWebCredentials(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_COMPONENT", "stripe-reconcile")
	t.Setenv("DATABASE_URL", "postgres://app:secret@example.test/mitte?sslmode=verify-full")
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_example")
	t.Setenv("MITTE_INVOICE_SANDBOX_ENABLED", "false")

	if _, err := Load(); err != nil {
		t.Fatalf("Load() for stripe reconciliation error = %v", err)
	}

	t.Setenv("STRIPE_SECRET_KEY", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing Stripe secret rejection")
	}
}

func TestLoadProductionRequiresInvoiceSellerAndPublicURL(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8")
	t.Setenv("METRICS_BEARER_TOKEN", "a-long-random-metrics-token-for-tests")
	t.Setenv("STAFF_CREDENTIALS", "operator:"+testStaffPasswordHash)
	t.Setenv("STAFF_ROLES", "operator:admin")
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_COMPONENT", "web")
	t.Setenv("DATABASE_URL", "******example.test/mitte?sslmode=verify-full")
	t.Setenv("TRUSTED_HOSTS", "shop.example.de")
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_example")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_example")
	t.Setenv("STRIPE_SUCCESS_URL", "https://shop.example.de/bestellung/erfolg?session_id={CHECKOUT_SESSION_ID}")
	t.Setenv("STRIPE_CANCEL_URL", "https://shop.example.de/warenkorb")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing invoice seller/public URL rejection")
	}
}

func TestLoadRejectsInvalidDatabasePool(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@example.test/mitte?sslmode=verify-full")
	t.Setenv("DB_MAX_OPEN_CONNS", "5")
	t.Setenv("DB_MAX_IDLE_CONNS", "6")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid pool rejection")
	}
}

func TestLoadMigrationComponentNeedsOnlyDatabaseConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_COMPONENT", "migrate")
	t.Setenv("DATABASE_URL", "******example.test/mitte?sslmode=verify-full")
	t.Setenv("RUN_MIGRATIONS", "true")
	t.Setenv("DATABASE_URL", "postgres"+"://user:pass@example.test/mitte?sslmode=verify-full")
	t.Setenv("STRIPE_SECRET_KEY", "")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")
	t.Setenv("PUBLIC_BASE_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() migration configuration error = %v", err)
	}
	if cfg.Component != "migrate" || !cfg.RunMigrations {
		t.Fatalf("migration configuration = %#v, want migrate component with migrations enabled", cfg)
	}
}

func TestLoadRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	t.Setenv("DATABASE_URL", "******example.test/mitte?sslmode=verify-full")
	t.Setenv("TRUSTED_PROXY_CIDRS", "not-a-cidr")
	t.Setenv("DATABASE_URL", "postgres"+"://user:pass@example.test/mitte?sslmode=verify-full")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid trusted proxy CIDR rejection")
	}
}

func TestLoadRejectsInvalidStaffCredentials(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres"+"://user:pass@example.test/mitte?sslmode=verify-full")
	t.Setenv("STAFF_CREDENTIALS", "bad credential")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid staff credential rejection")
	}
}

func TestLoadProductionWebRequiresTrustedProxyCIDRs(t *testing.T) {
	t.Setenv("STAFF_CREDENTIALS", "operator:"+testStaffPasswordHash)
	t.Setenv("STAFF_ROLES", "operator:admin")
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_COMPONENT", "web")
	t.Setenv("DATABASE_URL", "postgres"+"://user:pass@example.test/mitte?sslmode=verify-full")
	t.Setenv("TRUSTED_HOSTS", "shop.example.de")
	t.Setenv("TRUSTED_PROXY_CIDRS", "")
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_example")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_example")
	t.Setenv("STRIPE_SUCCESS_URL", "https://shop.example.de/bestellung/erfolg?session_id={CHECKOUT_SESSION_ID}")
	t.Setenv("STRIPE_CANCEL_URL", "https://shop.example.de/warenkorb")
	t.Setenv("PUBLIC_BASE_URL", "https://shop.example.de")
	t.Setenv("INVOICE_SELLER_NAME", "Example GmbH")
	t.Setenv("INVOICE_SELLER_STREET", "Musterstrasse 1")
	t.Setenv("INVOICE_SELLER_POSTAL_CODE", "20095")
	t.Setenv("INVOICE_SELLER_CITY", "Hamburg")
	t.Setenv("INVOICE_SELLER_COUNTRY_CODE", "DE")
	t.Setenv("INVOICE_SELLER_VAT_ID", "DE123456789")
	t.Setenv("INVOICE_SELLER_EMAIL", "invoice@example.de")
	t.Setenv("DATABASE_URL", "postgres"+"://user:pass@example.test/mitte?sslmode=verify-full")
	t.Setenv("METRICS_BEARER_TOKEN", "a-long-random-metrics-token-for-tests")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing trusted proxy CIDR rejection")
	}
}

func TestLoadProductionRejectsPlaintextStaffPassword(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_COMPONENT", "web")
	t.Setenv("DATABASE_URL", "postgres://user:pass@example.test/mitte?sslmode=verify-full")
	t.Setenv("STAFF_CREDENTIALS", "operator:another-long-test-password")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "Argon2id") {
		t.Fatalf("Load() error = %v, want plaintext staff-password rejection", err)
	}
}
