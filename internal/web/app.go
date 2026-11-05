package web

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mitte/internal/checkout"
	"mitte/internal/config"
	"mitte/internal/invoiceprofile"
	"mitte/internal/oms"
	"mitte/internal/payments"
	"mitte/internal/store"
)

//go:embed templates/*.html static/*
var assets embed.FS

type Options struct {
	SecureCookies              bool
	TrustedHosts               map[string]struct{}
	TrustedProxyCIDRs          []*net.IPNet
	MetricsBearerToken         string
	RequestTimeout             time.Duration
	Production                 bool
	Logger                     *slog.Logger
	Stripe                     payments.CheckoutCreator
	StripeWebhookSecret        string
	StripeSuccessURL           string
	StripeCancelURL            string
	MittePaySandboxEnabled     bool
	MittePayOTPSecret          string
	MitteTransferEnabled       bool
	MitteTransferIBAN          string
	MitteTransferRecipient     string
	MitteInvoiceSandboxEnabled bool
	StaffCredentials           map[string]string
	StaffRoles                 map[string]string
	SupplierCredentials        map[string]config.SupplierCredential
	InvoiceSeller              invoiceprofile.Profile
	InvoicePublicBaseURL       string
	GeminiAPIKey               string
	GeminiModel                string
}

type App struct {
	store                      *store.Store
	sessions                   operatorSessionStore
	checkoutService            *checkout.Service
	stripe                     payments.CheckoutCreator
	stripeWebhookSecret        string
	mittePaySandboxEnabled     bool
	mittePayOTPSecret          string
	mitteTransferEnabled       bool
	mitteTransferIBAN          string
	mitteTransferRecipient     string
	mitteInvoiceSandboxEnabled bool
	secureCookies              bool
	trustedHosts               map[string]struct{}
	trustedProxyCIDRs          []*net.IPNet
	metricsBearerToken         string
	requestTimeout             time.Duration
	production                 bool
	logger                     *slog.Logger
	metrics                    *metrics
	limiter                    *clientLimiter
	authLimiter                *clientLimiter
	loginLimiter               *clientLimiter
	apiLimiter                 *clientLimiter
	staffCredentials           map[string]string
	staffRoles                 map[string]string
	supplierCredentials        map[string]config.SupplierCredential
	invoiceSeller              invoiceprofile.Profile
	invoicePublicBaseURL       string
	assistant                  *geminiAssistant
	assistantLimiter           *clientLimiter
}

type viewData struct {
	PageTitle  string
	ActivePage string
	Categories []store.Category
	Products   []store.Product
	Product    store.Product
	Cart       store.Cart
	CartCount  int
	Search     string
	Category   string
	CSRFToken  string
	Order      store.Order
	SessionID  string
}

func New(repository *store.Store, options Options) http.Handler {
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.RequestTimeout <= 0 {
		options.RequestTimeout = 10 * time.Second
	}
	app := &App{
		store: repository, sessions: repository, stripe: options.Stripe, stripeWebhookSecret: options.StripeWebhookSecret,
		mittePaySandboxEnabled:     options.MittePaySandboxEnabled,
		mittePayOTPSecret:          options.MittePayOTPSecret,
		mitteTransferEnabled:       options.MitteTransferEnabled,
		mitteTransferIBAN:          options.MitteTransferIBAN,
		mitteTransferRecipient:     options.MitteTransferRecipient,
		mitteInvoiceSandboxEnabled: options.MitteInvoiceSandboxEnabled,
		secureCookies:              options.SecureCookies, trustedHosts: options.TrustedHosts,
		trustedProxyCIDRs: options.TrustedProxyCIDRs, metricsBearerToken: options.MetricsBearerToken,
		requestTimeout: options.RequestTimeout, production: options.Production,
		staffCredentials:     options.StaffCredentials,
		staffRoles:           options.StaffRoles,
		supplierCredentials:  options.SupplierCredentials,
		invoiceSeller:        options.InvoiceSeller,
		invoicePublicBaseURL: strings.TrimRight(options.InvoicePublicBaseURL, "/"),
		logger:               options.Logger, metrics: newMetrics(), limiter: newClientLimiter(120, time.Minute), authLimiter: newClientLimiter(10, time.Minute), loginLimiter: newClientLimiter(5, time.Minute), apiLimiter: newClientLimiter(300, time.Minute),
		assistantLimiter: newClientLimiter(12, time.Minute),
	}
	var checkoutGateway checkout.PaymentGateway
	if options.Stripe != nil {
		checkoutGateway = stripeCheckoutGateway{provider: options.Stripe}
	}
	app.checkoutService = checkout.NewService(repository, checkout.Settings{
		Stripe: checkoutGateway, StripeSuccessURL: options.StripeSuccessURL, StripeCancelURL: options.StripeCancelURL,
		MittePayEnabled:      options.MittePaySandboxEnabled,
		MitteTransferEnabled: options.MitteTransferEnabled,
		MitteInvoiceEnabled:  options.MitteInvoiceSandboxEnabled,
	})
	if repository != nil {
		app.assistant = newGeminiAssistant(options.GeminiAPIKey, options.GeminiModel, repository)
	}
	mux := http.NewServeMux()
	app.registerRoutes(mux)

	static, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", withStaticCaching(http.FileServer(http.FS(static)))))
	return app.securityHeaders(app.logRequests(app.recover(app.hostGuard(app.requestLimits(app.timeout(mux))))))
}

func (a *App) customerInvoicePDF(w http.ResponseWriter, r *http.Request) {
	user, ok := a.currentUser(w, r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Bitte melde dich an."})
		return
	}
	order, err := a.store.OrderByIDForCustomer(r.Context(), r.PathValue("id"), user.ID)
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Rechnung nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if order.Invoice == nil || order.Shipment == nil {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Die Rechnung wird nach vollständigem Versand bereitgestellt."})
		return
	}
	detail, err := a.store.OrderManagementDetail(r.Context(), order.ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	seller, err := a.store.InvoiceSellerForOrder(r.Context(), order.ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if !validPublicInvoiceSeller(seller) {
		seller = a.invoiceSeller
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+order.Invoice.Number+`.pdf"`)
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(oms.InvoicePDF(detail, seller))
}

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/konto" && (r.URL.Query().Has("verify_email") || r.URL.Query().Has("reset_password")) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
	}
	a.renderApp(w, r, "Mitte – Dinge für den Alltag")
}

func (a *App) shop(w http.ResponseWriter, r *http.Request) {
	a.renderApp(w, r, "Shop – Mitte")
}

func (a *App) product(w http.ResponseWriter, r *http.Request) {
	a.renderApp(w, r, "Produkt – Mitte")
}

func (a *App) cart(w http.ResponseWriter, r *http.Request) {
	a.renderApp(w, r, "Warenkorb – Mitte")
}

func (a *App) getCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := a.store.Categories(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	setPublicCatalogCache(w)
	respondJSON(w, http.StatusOK, map[string]any{"categories": categories})
}

func (a *App) getProducts(w http.ResponseWriter, r *http.Request) {
	category, search := strings.TrimSpace(r.URL.Query().Get("category")), strings.TrimSpace(r.URL.Query().Get("search"))
	if len(category) > 64 || len(search) > 100 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Suche."})
		return
	}
	page, limit, offset, sort, err := parseProductPageQuery(r)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Produktseiten- oder Sortierangabe."})
		return
	}
	products, hasMore, err := a.store.ProductsPage(r.Context(), category, search, false, sort, limit, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	setPublicCatalogCache(w)
	respondJSON(w, http.StatusOK, map[string]any{"products": products, "hasMore": hasMore, "page": page, "limit": limit})
}

func (a *App) getProduct(w http.ResponseWriter, r *http.Request) {
	product, err := a.store.ProductBySlug(r.Context(), r.PathValue("slug"))
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Produkt nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	setPublicCatalogCache(w)
	respondJSON(w, http.StatusOK, map[string]any{"product": product})
}

const (
	defaultProductPageSize = 100
	maxProductPageSize     = 100
	maxProductPage         = 10000
	maxProductOffset       = 100000
)

func parseProductPageQuery(r *http.Request) (page, limit, offset int, sort string, err error) {
	query := r.URL.Query()
	page, sort, err = parseProductListOptions(query.Get("page"), query.Get("sort"))
	if err != nil {
		return 0, 0, 0, "", err
	}
	limit = defaultProductPageSize
	if value := query.Get("limit"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil || limit < 1 || limit > maxProductPageSize {
			return 0, 0, 0, "", errors.New("invalid product page size")
		}
	}
	if page-1 > maxProductOffset/limit {
		return 0, 0, 0, "", errors.New("product page offset exceeds maximum")
	}
	offset = (page - 1) * limit
	return page, limit, offset, sort, nil
}

func parseProductListOptions(pageValue, sortValue string) (int, string, error) {
	page := 1
	if pageValue != "" {
		parsed, err := strconv.Atoi(pageValue)
		if err != nil || parsed < 1 || parsed > maxProductPage {
			return 0, "", errors.New("invalid product page")
		}
		page = parsed
	}
	if sortValue == "" {
		sortValue = "featured"
	}
	if sortValue != "featured" && sortValue != "price-low" && sortValue != "price-high" && sortValue != "bestselling" {
		return 0, "", errors.New("unsupported product sort")
	}
	return page, sortValue, nil
}

func setPublicCatalogCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
}

func (a *App) live(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.store.Ping(ctx); err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *App) getCart(w http.ResponseWriter, r *http.Request) {
	cart, err := a.store.Cart(r.Context(), a.sessionToken(w, r))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"cart": cart, "count": cartCount(cart)})
}

func (a *App) addCartItem(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	var input struct {
		ProductID int64 `json:"productId"`
		Quantity  int   `json:"quantity"`
	}
	if err := decodeJSON(w, r, &input, 1024); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Anfrage."})
		return
	}
	cart, err := a.store.AddCartItem(r.Context(), a.sessionToken(w, r), input.ProductID, input.Quantity)
	if err != nil {
		a.logger.Error(
			"checkout creation failed",
			"request_id", requestID(r.Context()),
			"error", err,
		)

		respondJSON(w, cartErrorStatus(err), map[string]string{
			"error": cartErrorMessage(err),
		})
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"cart": cart, "count": cartCount(cart)})
}

func (a *App) setCartItem(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	var input struct {
		ProductID int64 `json:"productId"`
		Quantity  int   `json:"quantity"`
	}
	if err := decodeJSON(w, r, &input, 1024); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Anfrage."})
		return
	}
	cart, err := a.store.SetCartItem(r.Context(), a.sessionToken(w, r), input.ProductID, input.Quantity)
	if err != nil {
		a.logger.Error(
			"checkout creation failed",
			"request_id", requestID(r.Context()),
			"error", err,
		)

		respondJSON(w, cartErrorStatus(err), map[string]string{
			"error": cartErrorMessage(err),
		})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"cart": cart, "count": cartCount(cart)})
}

func (a *App) applyPromotion(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(w, r, &input, 1024); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Anfrage."})
		return
	}
	cart, err := a.store.ApplyPromotion(r.Context(), a.sessionToken(w, r), input.Code)
	if err != nil {
		a.logger.Error(
			"checkout creation failed",
			"request_id", requestID(r.Context()),
			"error", err,
		)

		respondJSON(w, cartErrorStatus(err), map[string]string{
			"error": cartErrorMessage(err),
		})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"cart": cart, "count": cartCount(cart)})
}
