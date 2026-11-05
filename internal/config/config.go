package config

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Component                  string
	Environment                string
	DatabaseURL                string
	DatabaseMaxOpenConns       int
	DatabaseMaxIdleConns       int
	Port                       string
	GRPCListenAddr             string
	RunMigrations              bool
	SecureCookies              bool
	TrustedHosts               map[string]struct{}
	TrustedProxyCIDRs          []*net.IPNet
	MetricsBearerToken         string
	RequestTimeout             time.Duration
	ShutdownTimeout            time.Duration
	StripeSecretKey            string
	StripeWebhookSecret        string
	StripeSuccessURL           string
	StripeCancelURL            string
	GeminiAPIKey               string
	GeminiModel                string
	MittePaySandboxEnabled     bool
	MittePayOTPSecret          string
	MitteTransferEnabled       bool
	MitteTransferIBAN          string
	MitteTransferRecipient     string
	MitteInvoiceSandboxEnabled bool
	KafkaBrokers               []string
	KafkaTopic                 string
	KafkaTLSEnabled            bool
	KafkaTLSCACert             string
	KafkaSASLMechanism         string
	KafkaSASLUsername          string
	KafkaSASLPassword          string
	OrderRelayEnabled          bool
	StaffCredentials           map[string]string
	StaffRoles                 map[string]string
	SupplierCredentials        map[string]SupplierCredential
	PublicBaseURL              string
	InvoiceSellerName          string
	InvoiceSellerAddress       string
	InvoiceSellerStreet        string
	InvoiceSellerPostalCode    string
	InvoiceSellerCity          string
	InvoiceSellerCountryCode   string
	InvoiceSellerVATID         string
	InvoiceSellerEmail         string
	InvoiceSellerIBAN          string
	InvoicePaymentTerms        string
}

type SupplierCredential struct {
	Password     string
	SupplierName string
}

func Load() (Config, error) {
	environment := valueOr("APP_ENV", "development")
	if environment != "development" && environment != "production" {
		return Config{}, fmt.Errorf("APP_ENV must be development or production")
	}
	component := valueOr("APP_COMPONENT", "web")
	switch component {
	case "web", "oms", "order-relay", "email-relay", "payment-expiry", "stripe-reconcile", "migrate":
	default:
		return Config{}, fmt.Errorf("APP_COMPONENT must be web, oms, order-relay, email-relay, payment-expiry, stripe-reconcile, or migrate")
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if environment == "production" {
		if err := validateProductionDatabaseURL(databaseURL); err != nil {
			return Config{}, err
		}
	}
	databaseMaxOpenConns, err := positiveIntValue("DB_MAX_OPEN_CONNS", 20)
	if err != nil {
		return Config{}, err
	}
	databaseMaxIdleConns, err := nonNegativeIntValue("DB_MAX_IDLE_CONNS", 10)
	if err != nil {
		return Config{}, err
	}
	if databaseMaxIdleConns > databaseMaxOpenConns {
		return Config{}, fmt.Errorf("DB_MAX_IDLE_CONNS cannot exceed DB_MAX_OPEN_CONNS")
	}

	runMigrations, err := boolValue("RUN_MIGRATIONS", environment == "development")
	if err != nil {
		return Config{}, err
	}
	trustedHosts := make(map[string]struct{})
	for _, host := range strings.Split(os.Getenv("TRUSTED_HOSTS"), ",") {
		if host = strings.TrimSpace(host); host != "" {
			trustedHosts[host] = struct{}{}
		}
	}
	trustedProxyCIDRs := make([]*net.IPNet, 0)
	for _, value := range commaSeparated("TRUSTED_PROXY_CIDRS") {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return Config{}, fmt.Errorf("TRUSTED_PROXY_CIDRS contains an invalid CIDR: %q", value)
		}
		trustedProxyCIDRs = append(trustedProxyCIDRs, network)
	}
	staffCredentials, err := staffCredentialsValue(os.Getenv("STAFF_CREDENTIALS"), environment == "production" && component == "web")
	if err != nil {
		return Config{}, err
	}
	staffRoles, err := staffRolesValue(os.Getenv("STAFF_ROLES"), staffCredentials, environment == "production" && component == "web")
	if err != nil {
		return Config{}, err
	}
	if environment == "production" && component == "web" {
		if len(staffCredentials) == 0 {
			return Config{}, fmt.Errorf("STAFF_CREDENTIALS is required in production")
		}
	}
	supplierCredentials, err := supplierCredentialsValue(os.Getenv("SUPPLIER_CREDENTIALS"), environment == "production" && component == "web")
	if err != nil {
		return Config{}, err
	}
	if environment == "production" && component == "web" && len(trustedHosts) == 0 {
		return Config{}, fmt.Errorf("TRUSTED_HOSTS is required in production")
	}
	if environment == "production" && component == "web" && len(trustedProxyCIDRs) == 0 {
		return Config{}, fmt.Errorf("TRUSTED_PROXY_CIDRS is required for the production web component")
	}
	metricsBearerToken := strings.TrimSpace(os.Getenv("METRICS_BEARER_TOKEN"))
	if environment == "production" && (component == "web" || component == "oms" || component == "order-relay" || component == "email-relay" || component == "payment-expiry") && len(metricsBearerToken) < 32 {
		return Config{}, fmt.Errorf("METRICS_BEARER_TOKEN must contain at least 32 characters for production application and worker components")
	}
	publicBaseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")), "/")

	stripeSecretKey := strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY"))
	stripeWebhookSecret := strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET"))
	stripeSuccessURL := strings.TrimSpace(os.Getenv("STRIPE_SUCCESS_URL"))
	stripeCancelURL := strings.TrimSpace(os.Getenv("STRIPE_CANCEL_URL"))
	geminiAPIKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	geminiModel := valueOr("GEMINI_MODEL", "gemini-3.8-flash")
	if !validGeminiModel(geminiModel) {
		return Config{}, fmt.Errorf("GEMINI_MODEL must contain only letters, numbers, periods, underscores, or hyphens")
	}
	kafkaBrokers := commaSeparated("KAFKA_BROKERS")
	kafkaTopic := valueOr("KAFKA_ORDER_TOPIC", "mitte.orders.v1")
	kafkaTLSEnabled, err := boolValue("KAFKA_TLS_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	kafkaTLSCACert := os.Getenv("KAFKA_TLS_CA_CERT")
	kafkaSASLMechanism := strings.ToLower(strings.TrimSpace(os.Getenv("KAFKA_SASL_MECHANISM")))
	kafkaSASLUsername := strings.TrimSpace(os.Getenv("KAFKA_SASL_USERNAME"))
	kafkaSASLPassword := os.Getenv("KAFKA_SASL_PASSWORD")
	if kafkaSASLMechanism != "" && kafkaSASLMechanism != "plain" &&
		kafkaSASLMechanism != "scram-sha-256" && kafkaSASLMechanism != "scram-sha-512" {
		return Config{}, fmt.Errorf("KAFKA_SASL_MECHANISM must be plain, scram-sha-256, or scram-sha-512")
	}
	if kafkaSASLMechanism == "" && (kafkaSASLUsername != "" || kafkaSASLPassword != "") ||
		kafkaSASLMechanism != "" && (kafkaSASLUsername == "" || kafkaSASLPassword == "") {
		return Config{}, fmt.Errorf("KAFKA_SASL_MECHANISM, KAFKA_SASL_USERNAME, and KAFKA_SASL_PASSWORD must be configured together")
	}
	if kafkaSASLMechanism != "" && !kafkaTLSEnabled {
		return Config{}, fmt.Errorf("KAFKA_TLS_ENABLED must be true when Kafka SASL authentication is configured")
	}
	if strings.TrimSpace(kafkaTLSCACert) != "" && !kafkaTLSEnabled {
		return Config{}, fmt.Errorf("KAFKA_TLS_ENABLED must be true when KAFKA_TLS_CA_CERT is configured")
	}
	orderRelayEnabled, err := boolValue("ORDER_RELAY_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	mittePaySandboxEnabled, err := boolValue("MITTE_PAY_SANDBOX_ENABLED", environment == "development")
	if err != nil {
		return Config{}, err
	}
	if environment == "production" && mittePaySandboxEnabled {
		return Config{}, fmt.Errorf("MITTE_PAY_SANDBOX_ENABLED must be false in production")
	}
	mittePayOTPSecret := strings.TrimSpace(os.Getenv("MITTE_PAY_OTP_SECRET"))
	if component == "web" && mittePaySandboxEnabled && len(mittePayOTPSecret) < 32 {
		return Config{}, fmt.Errorf("MITTE_PAY_OTP_SECRET must contain at least 32 characters when the payment sandbox is enabled")
	}
	mitteTransferEnabled, err := boolValue("MITTE_TRANSFER_ENABLED", environment == "development")
	if err != nil {
		return Config{}, err
	}
	mitteTransferIBAN := strings.TrimSpace(os.Getenv("MITTE_TRANSFER_IBAN"))
	mitteTransferRecipient := strings.TrimSpace(os.Getenv("MITTE_TRANSFER_RECIPIENT"))
	if environment == "development" {
		if mitteTransferRecipient == "" {
			mitteTransferRecipient = "Mitte Demo Handelsgesellschaft mbH"
		}
		if mitteTransferIBAN == "" {
			mitteTransferIBAN = "DE00 0000 0000 0000 0000 00"
		}
	}
	if environment == "production" {
		if invoiceSandboxEnabled, err := boolValue("MITTE_INVOICE_SANDBOX_ENABLED", false); err != nil || invoiceSandboxEnabled {
			if err != nil {
				return Config{}, err
			}
			return Config{}, fmt.Errorf("MITTE_INVOICE_SANDBOX_ENABLED must be false in production")
		}
		if (component == "web" || component == "oms" || component == "stripe-reconcile") && stripeSecretKey == "" {
			return Config{}, fmt.Errorf("STRIPE_SECRET_KEY is required in production")
		}
		if component == "web" && stripeWebhookSecret == "" {
			return Config{}, fmt.Errorf("STRIPE_WEBHOOK_SECRET is required in production")
		}
		if component == "web" && stripeSuccessURL == "" {
			return Config{}, fmt.Errorf("STRIPE_SUCCESS_URL is required in production")
		}
		if component == "web" && stripeCancelURL == "" {
			return Config{}, fmt.Errorf("STRIPE_CANCEL_URL is required in production")
		}
		if mitteTransferEnabled && (mitteTransferIBAN == "" || mitteTransferRecipient == "") {
			return Config{}, fmt.Errorf("MITTE_TRANSFER_IBAN and MITTE_TRANSFER_RECIPIENT are required when MITTE_TRANSFER_ENABLED is true")
		}
		if component == "web" {
			if err := validateStripeURL("STRIPE_SUCCESS_URL", stripeSuccessURL, true); err != nil {
				return Config{}, err
			}
			if err := validateStripeURL("STRIPE_CANCEL_URL", stripeCancelURL, false); err != nil {
				return Config{}, err
			}
		}
		if orderRelayEnabled && len(kafkaBrokers) == 0 {
			return Config{}, fmt.Errorf("KAFKA_BROKERS is required when ORDER_RELAY_ENABLED is true in production")
		}
		if component == "order-relay" && orderRelayEnabled {
			if !kafkaTLSEnabled {
				return Config{}, fmt.Errorf("KAFKA_TLS_ENABLED must be true for the production order relay")
			}
			if kafkaSASLMechanism == "" {
				return Config{}, fmt.Errorf("KAFKA_SASL_MECHANISM, KAFKA_SASL_USERNAME, and KAFKA_SASL_PASSWORD are required for the production order relay")
			}
		}
	}
	mitteInvoiceSandboxEnabled, err := boolValue("MITTE_INVOICE_SANDBOX_ENABLED", environment == "development")
	if err != nil {
		return Config{}, err
	}
	invoiceSellerName := strings.TrimSpace(os.Getenv("INVOICE_SELLER_NAME"))
	invoiceSellerAddress := strings.TrimSpace(os.Getenv("INVOICE_SELLER_ADDRESS"))
	invoiceSellerStreet := strings.TrimSpace(os.Getenv("INVOICE_SELLER_STREET"))
	invoiceSellerPostalCode := strings.TrimSpace(os.Getenv("INVOICE_SELLER_POSTAL_CODE"))
	invoiceSellerCity := strings.TrimSpace(os.Getenv("INVOICE_SELLER_CITY"))
	invoiceSellerCountryCode := strings.ToUpper(strings.TrimSpace(os.Getenv("INVOICE_SELLER_COUNTRY_CODE")))
	invoiceSellerVATID := strings.TrimSpace(os.Getenv("INVOICE_SELLER_VAT_ID"))
	invoiceSellerEmail := strings.TrimSpace(os.Getenv("INVOICE_SELLER_EMAIL"))
	invoiceSellerIBAN := strings.TrimSpace(os.Getenv("INVOICE_SELLER_IBAN"))
	invoicePaymentTerms := strings.TrimSpace(os.Getenv("INVOICE_PAYMENT_TERMS"))
	if environment == "development" &&
		invoiceSellerName == "" && invoiceSellerAddress == "" && invoiceSellerStreet == "" &&
		invoiceSellerPostalCode == "" && invoiceSellerCity == "" && invoiceSellerCountryCode == "" &&
		invoiceSellerVATID == "" && invoiceSellerEmail == "" && invoiceSellerIBAN == "" {
		invoiceSellerName = "Mitte Demo Handelsgesellschaft mbH"
		invoiceSellerAddress = "Musterstrasse 12, 20095 Hamburg, Deutschland"
		invoiceSellerStreet = "Musterstrasse 12"
		invoiceSellerPostalCode = "20095"
		invoiceSellerCity = "Hamburg"
		invoiceSellerCountryCode = "DE"
		invoiceSellerVATID = "DE000000000"
		invoiceSellerEmail = "rechnung@example.test"
		invoiceSellerIBAN = "DE00000000000000000000"
		invoicePaymentTerms = "Bitte innerhalb von 30 Tagen nach Rechnungsdatum überweisen."
	}
	if environment == "development" && publicBaseURL == "" {
		publicBaseURL = "http://localhost:8080"
	}
	if environment == "production" && (component == "web" || component == "oms") {
		if err := validateInvoiceConfiguration(publicBaseURL, trustedHosts, invoiceSellerName, invoiceSellerStreet,
			invoiceSellerPostalCode, invoiceSellerCity, invoiceSellerCountryCode, invoiceSellerVATID, invoiceSellerEmail); err != nil {
			return Config{}, err
		}
	}
	requestTimeout, err := durationValue("REQUEST_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := durationValue("SHUTDOWN_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Component:                  component,
		Environment:                environment,
		DatabaseURL:                databaseURL,
		DatabaseMaxOpenConns:       databaseMaxOpenConns,
		DatabaseMaxIdleConns:       databaseMaxIdleConns,
		Port:                       valueOr("PORT", "8080"),
		GRPCListenAddr:             valueOr("GRPC_LISTEN_ADDR", ":9090"),
		RunMigrations:              runMigrations,
		SecureCookies:              environment == "production",
		TrustedHosts:               trustedHosts,
		TrustedProxyCIDRs:          trustedProxyCIDRs,
		MetricsBearerToken:         metricsBearerToken,
		RequestTimeout:             requestTimeout,
		ShutdownTimeout:            shutdownTimeout,
		StripeSecretKey:            stripeSecretKey,
		StripeWebhookSecret:        stripeWebhookSecret,
		StripeSuccessURL:           stripeSuccessURL,
		StripeCancelURL:            stripeCancelURL,
		GeminiAPIKey:               geminiAPIKey,
		GeminiModel:                geminiModel,
		MittePaySandboxEnabled:     mittePaySandboxEnabled,
		MittePayOTPSecret:          mittePayOTPSecret,
		MitteTransferEnabled:       mitteTransferEnabled,
		MitteTransferIBAN:          mitteTransferIBAN,
		MitteTransferRecipient:     mitteTransferRecipient,
		MitteInvoiceSandboxEnabled: mitteInvoiceSandboxEnabled,
		KafkaBrokers:               kafkaBrokers,
		KafkaTopic:                 kafkaTopic,
		KafkaTLSEnabled:            kafkaTLSEnabled,
		KafkaTLSCACert:             kafkaTLSCACert,
		KafkaSASLMechanism:         kafkaSASLMechanism,
		KafkaSASLUsername:          kafkaSASLUsername,
		KafkaSASLPassword:          kafkaSASLPassword,
		OrderRelayEnabled:          orderRelayEnabled,
		StaffCredentials:           staffCredentials,
		StaffRoles:                 staffRoles,
		SupplierCredentials:        supplierCredentials,
		PublicBaseURL:              publicBaseURL,
		InvoiceSellerName:          invoiceSellerName,
		InvoiceSellerAddress:       invoiceSellerAddress,
		InvoiceSellerStreet:        invoiceSellerStreet,
		InvoiceSellerPostalCode:    invoiceSellerPostalCode,
		InvoiceSellerCity:          invoiceSellerCity,
		InvoiceSellerCountryCode:   invoiceSellerCountryCode,
		InvoiceSellerVATID:         invoiceSellerVATID,
		InvoiceSellerEmail:         invoiceSellerEmail,
		InvoiceSellerIBAN:          invoiceSellerIBAN,
		InvoicePaymentTerms:        invoicePaymentTerms,
	}, nil
}

// supplierCredentialsValue parses comma-separated username:supplier-name:password entries.
// Passwords may contain colons, including the separators in an Argon2id hash.
func supplierCredentialsValue(raw string, requirePasswordHash bool) (map[string]SupplierCredential, error) {
	credentials := make(map[string]SupplierCredential)
	var currentUsername, currentSupplierName string
	var currentPassword strings.Builder
	hasCredential := false
	flush := func() error {
		if !hasCredential {
			return nil
		}
		username := strings.ToLower(strings.TrimSpace(currentUsername))
		supplierName := strings.TrimSpace(currentSupplierName)
		password := strings.TrimSpace(currentPassword.String())
		if len(username) < 3 || len(username) > 64 || len(supplierName) == 0 ||
			len(supplierName) > 200 || len(password) < 16 || len(password) > 512 {
			return fmt.Errorf("SUPPLIER_CREDENTIALS entries must be username:supplier-name:password pairs")
		}
		for _, character := range username {
			if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' || character == '_') {
				return fmt.Errorf("SUPPLIER_CREDENTIALS usernames may contain only lowercase letters, numbers, hyphens, and underscores")
			}
		}
		if strings.ContainsAny(supplierName, ",:") {
			return fmt.Errorf("SUPPLIER_CREDENTIALS supplier names cannot contain commas or colons")
		}
		if requirePasswordHash && !validArgon2idHash(password) {
			return fmt.Errorf("SUPPLIER_CREDENTIALS must contain Argon2id password hashes in production")
		}
		if _, exists := credentials[username]; exists {
			return fmt.Errorf("SUPPLIER_CREDENTIALS contains a duplicate username")
		}
		credentials[username] = SupplierCredential{Password: password, SupplierName: supplierName}
		hasCredential = false
		currentUsername, currentSupplierName = "", ""
		currentPassword.Reset()
		return nil
	}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		username, remainder, found := strings.Cut(entry, ":")
		supplierName, password, hasPassword := strings.Cut(remainder, ":")
		if found && hasPassword {
			if err := flush(); err != nil {
				return nil, err
			}
			currentUsername, currentSupplierName = username, supplierName
			currentPassword.WriteString(password)
			hasCredential = true
			continue
		}
		if !hasCredential {
			return nil, fmt.Errorf("SUPPLIER_CREDENTIALS entries must be username:supplier-name:password pairs")
		}
		currentPassword.WriteByte(',')
		currentPassword.WriteString(entry)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return credentials, nil
}

func validGeminiModel(model string) bool {
	if model == "" || len(model) > 80 {
		return false
	}
	for _, character := range model {
		if !(character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == '-') {
			return false
		}
	}
	return true
}

func staffRolesValue(raw string, credentials map[string]string, required bool) (map[string]string, error) {
	roles := make(map[string]string, len(credentials))
	for username := range credentials {
		roles[username] = "admin"
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if required && len(credentials) > 0 {
			return nil, fmt.Errorf("STAFF_ROLES is required in production and must assign every staff account a least-privilege role")
		}
		return roles, nil
	}

	roles = make(map[string]string, len(credentials))
	allowed := map[string]struct{}{
		"admin": {}, "cs": {}, "orders": {}, "operations": {}, "warehouse": {},
		"catalog": {}, "inventory": {}, "payments": {},
	}
	for _, entry := range strings.Split(raw, ",") {
		username, role, found := strings.Cut(strings.TrimSpace(entry), ":")
		username = strings.ToLower(strings.TrimSpace(username))
		role = strings.ToLower(strings.TrimSpace(role))
		if !found || username == "" || role == "" {
			return nil, fmt.Errorf("STAFF_ROLES entries must be username:role pairs")
		}
		if _, exists := credentials[username]; !exists {
			return nil, fmt.Errorf("STAFF_ROLES contains a username without matching STAFF_CREDENTIALS")
		}
		if _, exists := allowed[role]; !exists {
			return nil, fmt.Errorf("STAFF_ROLES contains an unsupported role")
		}
		if _, exists := roles[username]; exists {
			return nil, fmt.Errorf("STAFF_ROLES contains a duplicate username")
		}
		roles[username] = role
	}
	if len(roles) != len(credentials) {
		return nil, fmt.Errorf("STAFF_ROLES must assign a role to every STAFF_CREDENTIALS account")
	}
	return roles, nil
}

// staffCredentialsValue parses STAFF_CREDENTIALS as comma-separated
// username:argon2id-hash pairs. Plaintext values remain available only outside
// production to keep local development simple; production must never inject a
// reusable staff password into a running web process.
func staffCredentialsValue(raw string, requirePasswordHash bool) (map[string]string, error) {
	credentials := make(map[string]string)
	var currentUsername string
	var currentPassword strings.Builder
	hasPendingCredential := false
	flush := func() error {
		if !hasPendingCredential {
			return nil
		}
		username := strings.ToLower(strings.TrimSpace(currentUsername))
		password := strings.TrimSpace(currentPassword.String())
		if len(username) < 3 || len(username) > 64 || len(password) < 16 || len(password) > 512 {
			return fmt.Errorf("STAFF_CREDENTIALS entries must be username:password pairs (username 3-64 characters, password 16-512 characters)")
		}
		if requirePasswordHash && !validArgon2idHash(password) {
			return fmt.Errorf("STAFF_CREDENTIALS must contain Argon2id password hashes in production")
		}
		for _, character := range username {
			if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' || character == '_') {
				return fmt.Errorf("STAFF_CREDENTIALS usernames may contain only lowercase letters, numbers, hyphens, and underscores")
			}
		}
		if _, exists := credentials[username]; exists {
			return fmt.Errorf("STAFF_CREDENTIALS contains a duplicate username")
		}
		credentials[username] = password
		hasPendingCredential = false
		currentUsername = ""
		currentPassword.Reset()
		return nil
	}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		username, password, found := strings.Cut(entry, ":")
		if found {
			if err := flush(); err != nil {
				return nil, err
			}
			currentUsername = username
			currentPassword.WriteString(password)
			hasPendingCredential = true
			continue
		}
		if !hasPendingCredential {
			return nil, fmt.Errorf("STAFF_CREDENTIALS entries must be username:password pairs")
		}
		currentPassword.WriteByte(',')
		currentPassword.WriteString(entry)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return credentials, nil
}

func validArgon2idHash(encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=65536,t=3,p=2" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	return err == nil && len(hash) == 32
}

func validateInvoiceConfiguration(baseURL string, trustedHosts map[string]struct{}, name, street, postalCode, city, countryCode, vatID, email string) error {
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("PUBLIC_BASE_URL must be an HTTPS origin/path without credentials, query, or fragment in production")
	}
	hostname := strings.ToLower(parsed.Hostname())
	trusted := false
	for host := range trustedHosts {
		if strings.EqualFold(strings.TrimSpace(host), hostname) {
			trusted = true
			break
		}
	}
	if !trusted {
		return fmt.Errorf("PUBLIC_BASE_URL hostname must be included in TRUSTED_HOSTS")
	}
	if name == "" || street == "" || postalCode == "" || city == "" || len(countryCode) != 2 || vatID == "" || !strings.Contains(email, "@") {
		return fmt.Errorf("INVOICE_SELLER_NAME, INVOICE_SELLER_STREET, INVOICE_SELLER_POSTAL_CODE, INVOICE_SELLER_CITY, INVOICE_SELLER_COUNTRY_CODE, INVOICE_SELLER_VAT_ID, and INVOICE_SELLER_EMAIL are required in production")
	}
	return nil
}

func positiveIntValue(name string, fallback int) (int, error) {
	value, err := nonNegativeIntValue(name, fallback)
	if err != nil || value < 1 {
		if err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return value, nil
}

func nonNegativeIntValue(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return parsed, nil
}

func commaSeparated(name string) []string {
	values := make([]string, 0)
	for _, value := range strings.Split(os.Getenv(name), ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func validateStripeURL(name, value string, requireSessionPlaceholder bool) error {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("%s must be an absolute HTTPS URL without credentials or fragment", name)
	}
	if requireSessionPlaceholder && !strings.Contains(u.RawQuery, "session_id={CHECKOUT_SESSION_ID}") {
		return fmt.Errorf("%s must include session_id={CHECKOUT_SESSION_ID}", name)
	}
	return nil
}

func validateProductionDatabaseURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" || u.User == nil {
		return fmt.Errorf("DATABASE_URL must be an authenticated PostgreSQL URL in production")
	}
	sslModes := u.Query()["sslmode"]
	if len(sslModes) != 1 || !strings.EqualFold(sslModes[0], "verify-full") {
		return fmt.Errorf("DATABASE_URL must verify the PostgreSQL TLS certificate and hostname in production (sslmode=verify-full is required)")
	}
	return nil
}

func valueOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func boolValue(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
	}
	return parsed, nil
}

func durationValue(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", name, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return parsed, nil
}
