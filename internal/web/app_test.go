package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mitte/internal/store"
)

func TestValidCSRF(t *testing.T) {
	app := &App{}
	token := strings.Repeat("a", 64)
	request := httptest.NewRequest(http.MethodPost, "/api/cart/items", nil)
	request.AddCookie(&http.Cookie{Name: "mitte_csrf", Value: token})
	request.Header.Set("X-CSRF-Token", token)

	if !app.validCSRF(request) {
		t.Fatal("validCSRF() = false, want true")
	}
}

func TestGuestSessionAndCSRFCookiesAreProtected(t *testing.T) {
	app := &App{secureCookies: true}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	app.sessionToken(recorder, request)
	app.csrfToken(recorder, request)

	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("got %d cookies, want 2", len(cookies))
	}
	for _, cookie := range cookies {
		if cookie.Path != "/" || cookie.MaxAge <= 0 || !cookie.HttpOnly || !cookie.Secure {
			t.Errorf("cookie %q lacks expected path, lifetime, HttpOnly, or Secure attributes: %#v", cookie.Name, cookie)
		}
		switch cookie.Name {
		case "mitte_session":
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("session cookie SameSite = %v, want Lax", cookie.SameSite)
			}
		case "mitte_csrf":
			if cookie.SameSite != http.SameSiteStrictMode {
				t.Errorf("CSRF cookie SameSite = %v, want Strict", cookie.SameSite)
			}
		default:
			t.Errorf("unexpected cookie %q", cookie.Name)
		}
	}
}

func TestStaticAssetsUseBoundedRevalidatingCache(t *testing.T) {
	handler := New(nil, Options{})
	request := httptest.NewRequest(http.MethodGet, "/static/styles.css", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("static asset status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=300, must-revalidate" {
		t.Fatalf("static asset Cache-Control = %q", got)
	}
}

func TestPageRoutesIncludeAllClientNavigableStorefrontPages(t *testing.T) {
	mux := http.NewServeMux()
	(&App{}).registerPageRoutes(mux)

	for _, path := range []string{"/konto", "/merkliste"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		_, pattern := mux.Handler(request)
		if pattern == "" {
			t.Errorf("GET %s is not registered; direct navigation must load the storefront application", path)
		}
	}
}

func TestParseProductPageQueryBoundsAndDefaults(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantPage   int
		wantLimit  int
		wantOffset int
		wantSort   string
		wantErr    bool
	}{
		{name: "defaults", target: "/api/products", wantPage: 1, wantLimit: 100, wantSort: "featured"},
		{name: "bounded page", target: "/api/products?page=3&limit=24&sort=price-low", wantPage: 3, wantLimit: 24, wantOffset: 48, wantSort: "price-low"},
		{name: "invalid page", target: "/api/products?page=0", wantErr: true},
		{name: "invalid page size", target: "/api/products?limit=101", wantErr: true},
		{name: "invalid sort", target: "/api/products?sort=random", wantErr: true},
		{name: "excessive offset", target: "/api/products?page=2000&limit=100", wantErr: true},
		{name: "overflow-safe offset", target: "/api/products?page=9223372036854775807&limit=100", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			page, limit, offset, sort, err := parseProductPageQuery(request)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseProductPageQuery() error = %v, wantErr %t", err, test.wantErr)
			}
			if err == nil && (page != test.wantPage || limit != test.wantLimit || offset != test.wantOffset || sort != test.wantSort) {
				t.Fatalf("parseProductPageQuery() = (%d, %d, %d, %q), want (%d, %d, %d, %q)",
					page, limit, offset, sort, test.wantPage, test.wantLimit, test.wantOffset, test.wantSort)
			}
		})
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"allowed": true, "unexpected": true}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	var input struct {
		Allowed bool `json:"allowed"`
	}

	if err := decodeJSON(recorder, request, &input, 1024); err == nil {
		t.Fatal("decodeJSON() error = nil, want unknown field error")
	}
}

func TestDecodeJSONRejectsTrailingValues(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"allowed": true} {"allowed": false}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	var input struct {
		Allowed bool `json:"allowed"`
	}

	if err := decodeJSON(recorder, request, &input, 1024); err == nil {
		t.Fatal("decodeJSON() error = nil, want trailing JSON rejection")
	}
}

func TestDecodeJSONRejectsUnexpectedContentType(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"allowed": true}`))
	request.Header.Set("Content-Type", "text/plain")
	recorder := httptest.NewRecorder()
	var input struct{ Allowed bool }
	if err := decodeJSON(recorder, request, &input, 1024); err == nil {
		t.Fatal("decodeJSON() error = nil, want content-type rejection")
	}
}

func TestClientLimiterEnforcesWindowLimit(t *testing.T) {
	limiter := newClientLimiter(1, time.Minute)
	if !limiter.allow("127.0.0.1") || limiter.allow("127.0.0.1") {
		t.Fatal("clientLimiter did not enforce request limit")
	}
}

func TestPaymentLineItemsPreserveServerCalculatedAmounts(t *testing.T) {
	items := paymentLineItems([]store.CheckoutLineItem{{Name: "Vase", AmountCents: 4900, Quantity: 2}})
	if len(items) != 1 || items[0].Name != "Vase" || items[0].AmountCents != 4900 || items[0].Quantity != 2 {
		t.Fatalf("paymentLineItems() = %#v", items)
	}
}

func TestSelectedAllOrderItemsRequiresEveryUniqueItem(t *testing.T) {
	items := []store.OrderItem{{Name: "Lamp"}, {Name: "Vase"}}
	tests := []struct {
		name     string
		selected []string
		want     bool
	}{
		{name: "all selected", selected: []string{"Lamp", "Vase"}, want: true},
		{name: "partial selection", selected: []string{"Lamp"}},
		{name: "duplicate selection", selected: []string{"Lamp", "Lamp"}},
		{name: "unknown item", selected: []string{"Lamp", "Other"}},
		{name: "empty order", selected: []string{}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := selectedAllOrderItems(test.selected, items); got != test.want {
				t.Fatalf("selectedAllOrderItems() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestValidCatalogInputNormalizesAndBoundsInventoryFields(t *testing.T) {
	input := &store.CatalogProductInput{
		SKU: " mf-123 ", Slug: "  MORI-LAMP ", Name: "Mori Lamp", ShortDescription: "Lamp",
		Description: "A product description", PriceCents: 12000, ImageURL: "https://example.test/lamp.jpg",
		ImageAlt: "Lamp", CategoryID: 1, Barcode: "  4001234567890 ", SupplierName: " Atelier ",
		UnitCostCents: 4000, ReorderPoint: 5, LeadTimeDays: 30, InitialStock: 12,
	}
	if !validCatalogInput(input) {
		t.Fatal("validCatalogInput rejected a complete product and inventory record")
	}
	if input.SKU != "MF-123" || input.Slug != "mori-lamp" || input.Barcode != "4001234567890" || input.SupplierName != "Atelier" {
		t.Fatalf("validCatalogInput did not normalize inventory identifiers: %#v", input)
	}

	input.InitialStock = -1
	if validCatalogInput(input) {
		t.Fatal("validCatalogInput accepted a negative initial stock")
	}
	input.InitialStock = 0
	input.ReorderPoint = 1000001
	if validCatalogInput(input) {
		t.Fatal("validCatalogInput accepted an excessive reorder point")
	}
}

func TestValidCatalogInputValidatesProductSpecifications(t *testing.T) {
	base := store.CatalogProductInput{
		Slug: "product", Name: "Product", ShortDescription: "Short", Description: "Description",
		PriceCents: 1000, ImageURL: "https://example.test/product.jpg", ImageAlt: "Product",
		CategoryID: 1, TechnicalDetails: map[string]string{"Finish": "Matte"},
	}
	tests := []struct {
		name string
		gtin string
		want bool
	}{
		{name: "empty GTIN is optional", want: true},
		{name: "valid EAN-8", gtin: "96385074", want: true},
		{name: "valid UPC-A", gtin: "036000291452", want: true},
		{name: "valid EAN-13", gtin: "4006381333931", want: true},
		{name: "valid GTIN-14", gtin: "10012345000017", want: true},
		{name: "bad check digit", gtin: "4006381333932"},
		{name: "unsupported length", gtin: "123456789"},
		{name: "non-digit", gtin: "40063813339x1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := base
			input.GTIN = test.gtin
			if got := validCatalogInput(&input); got != test.want {
				t.Fatalf("validCatalogInput() = %t, want %t", got, test.want)
			}
			if input.TechnicalDetails == nil {
				t.Fatal("validCatalogInput did not initialize technical details")
			}
		})
	}

	input := base
	input.TechnicalDetails = map[string]string{"": "value"}
	if validCatalogInput(&input) {
		t.Fatal("validCatalogInput accepted a blank technical-details key")
	}
}

func TestMittePayCodeValidation(t *testing.T) {
	for _, code := range []string{"000000", "123456", "999999"} {
		if !validMittePayCode(code) {
			t.Fatalf("validMittePayCode(%q) = false, want true", code)
		}
	}
	for _, code := range []string{"12345", "1234567", "12 456", "abcdef"} {
		if validMittePayCode(code) {
			t.Fatalf("validMittePayCode(%q) = true, want false", code)
		}
	}
	if !validMittePaySessionID("mfpay_"+strings.Repeat("a", 36)) || validMittePaySessionID("mfpay_short") {
		t.Fatal("validMittePaySessionID did not enforce the generated session format")
	}
}

func TestMitteInvoiceSessionValidation(t *testing.T) {
	if !validMitteInvoiceSessionID("mfinvoice_" + strings.Repeat("a", 36)) {
		t.Fatal("validMitteInvoiceSessionID rejected a generated session")
	}
	if validMitteInvoiceSessionID("mfinvoice_short") {
		t.Fatal("validMitteInvoiceSessionID accepted a malformed session")
	}
}

func TestMittePayCodeHashBindsEveryInput(t *testing.T) {
	app := &App{mittePayOTPSecret: "test-secret"}
	expected := app.mittePayCodeHash("mfpay_session", "person@example.test", "123456")
	for _, candidate := range [][]byte{
		app.mittePayCodeHash("mfpay_other", "person@example.test", "123456"),
		app.mittePayCodeHash("mfpay_session", "other@example.test", "123456"),
		app.mittePayCodeHash("mfpay_session", "person@example.test", "654321"),
		(&App{mittePayOTPSecret: "other-secret"}).mittePayCodeHash("mfpay_session", "person@example.test", "123456"),
	} {
		if string(expected) == string(candidate) {
			t.Fatal("mittePayCodeHash did not bind all security inputs")
		}
	}
}

func TestMittePayCodeIsSixDigits(t *testing.T) {
	code, err := mittePayCode()
	if err != nil {
		t.Fatalf("mittePayCode() error = %v", err)
	}
	if !validMittePayCode(code) {
		t.Fatalf("mittePayCode() = %q, want six digits", code)
	}
}

func TestPasswordHashVerification(t *testing.T) {
	hash, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}
	if !verifyPassword("correct horse battery staple", hash) {
		t.Fatal("verifyPassword() = false, want true")
	}
	if verifyPassword("incorrect password", hash) {
		t.Fatal("verifyPassword() = true for incorrect password")
	}
}

func TestAuthEmailTokensAreOpaqueAndOnlyStoredAsDigests(t *testing.T) {
	token, digest, err := newAuthToken()
	if err != nil {
		t.Fatalf("newAuthToken() error = %v", err)
	}
	if len(token) != 64 || len(digest) != 32 {
		t.Fatalf("newAuthToken() lengths = %d and %d, want 64 and 32", len(token), len(digest))
	}
	parsed, ok := authTokenHash(token)
	if !ok || string(parsed) != string(digest) {
		t.Fatal("authTokenHash() does not match the stored token digest")
	}
	if _, ok := authTokenHash("not-a-token"); ok {
		t.Fatal("authTokenHash() accepted a malformed token")
	}
}

func TestAuthEmailLinkUsesConfiguredPublicBaseURL(t *testing.T) {
	app := &App{invoicePublicBaseURL: "https://shop.example.test/store/"}
	got := app.authEmailLink("/konto", "verify_email", strings.Repeat("a", 64))
	want := "https://shop.example.test/store/konto?verify_email=" + strings.Repeat("a", 64)
	if got != want {
		t.Fatalf("authEmailLink() = %q, want %q", got, want)
	}
}

func TestAuthInputValidation(t *testing.T) {
	if !validAuthEmail("person@example.com") || validAuthEmail("Person <person@example.com>") {
		t.Fatal("validAuthEmail did not enforce a plain normalized address")
	}
	if !validPassword("twelve-chars") || validPassword("too-short") {
		t.Fatal("validPassword did not enforce the password length policy")
	}
	if !validCustomerType("b2c") || !validCustomerType("b2b") || validCustomerType("admin") {
		t.Fatal("validCustomerType did not restrict customer account types")
	}
}

func TestCustomerProfileValidationRejectsMalformedText(t *testing.T) {
	profile := store.CustomerProfile{FullName: "Élise Müller", CustomerType: "b2c", Address: store.Address{CountryCode: "DE"}}
	if !validCustomerProfile(&profile) {
		t.Fatal("validCustomerProfile rejected a valid profile")
	}

	for _, name := range []struct {
		name  string
		apply func(*store.CustomerProfile)
	}{
		{name: "short name", apply: func(profile *store.CustomerProfile) { profile.FullName = "A" }},
		{name: "control character", apply: func(profile *store.CustomerProfile) { profile.Phone = "123\x00" }},
		{name: "invalid country", apply: func(profile *store.CustomerProfile) { profile.Address.CountryCode = "D1" }},
	} {
		t.Run(name.name, func(t *testing.T) {
			candidate := profile
			name.apply(&candidate)
			if validCustomerProfile(&candidate) {
				t.Fatal("validCustomerProfile accepted malformed profile data")
			}
		})
	}
}

func TestCheckoutInputUsesBillingAddressForShipping(t *testing.T) {
	input := store.CheckoutInput{
		Name: "Erika Muster", Email: "ERIKA@EXAMPLE.TEST", UseBillingForShipping: true,
		BillingAddress: store.Address{RecipientName: "Erika Muster", AddressLine1: "Musterstraße 12", PostalCode: "20095", City: "Hamburg"},
	}
	store.NormalizeCheckoutInput(&input)
	if !validCheckoutInput(input) || input.ShippingAddress.City != "Hamburg" || input.Email != "erika@example.test" {
		t.Fatalf("checkout input was not normalized correctly: %#v", input)
	}
	input.Email = "Erika Muster <erika@example.test>"
	if validCheckoutInput(input) {
		t.Fatal("validCheckoutInput accepted a display-name email address")
	}
}

func TestCheckoutRestrictsShippingToConfiguredTaxJurisdiction(t *testing.T) {
	input := store.CheckoutInput{
		Name: "Erika Muster", Email: "erika@example.test", UseBillingForShipping: false,
		BillingAddress:  store.Address{RecipientName: "Erika Muster", AddressLine1: "Musterstraße 12", PostalCode: "20095", City: "Hamburg", CountryCode: "AT"},
		ShippingAddress: store.Address{RecipientName: "Erika Muster", AddressLine1: "Musterstraße 12", PostalCode: "20095", City: "Hamburg", CountryCode: "DE"},
	}
	store.NormalizeCheckoutInput(&input)
	if !validCheckoutInput(input) {
		t.Fatal("valid DE shipping was rejected solely due to the foreign billing address")
	}
	input.UseBillingForShipping = false
	input.ShippingAddress = store.Address{RecipientName: "Erika Muster", AddressLine1: "Musterstraße 12", PostalCode: "10115", City: "Berlin", CountryCode: "AT"}
	store.NormalizeCheckoutInput(&input)
	if validCheckoutInput(input) {
		t.Fatal("validCheckoutInput accepted shipping outside the configured German tax/shipping jurisdiction")
	}
}

func TestCheckoutInputAcceptsDevelopmentInvoiceProvider(t *testing.T) {
	input := store.CheckoutInput{
		Name: "Erika Muster", Email: "erika@example.test", PaymentMethod: "mitte_invoice", UseBillingForShipping: true,
		BillingAddress: store.Address{RecipientName: "Erika Muster", AddressLine1: "Musterstraße 12", PostalCode: "20095", City: "Hamburg"},
	}
	store.NormalizeCheckoutInput(&input)
	if !validCheckoutInput(input) {
		t.Fatal("validCheckoutInput rejected the development invoice provider")
	}
}
