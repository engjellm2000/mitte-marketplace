// Package oms exposes the isolated order-management boundary. It is intended
// for the back-office UI, warehouse scanners, and vetted integrations only.
package oms

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"

	"mitte/internal/invoiceprofile"
	"mitte/internal/payments"
	"mitte/internal/store"
)

type Options struct {
	APIKey, Username, Password string
	MetricsBearerToken         string
	PublicBaseURL              string
	Seller                     SellerProfile
	Refunder                   payments.Refunder
	RequestTimeout             time.Duration
	SecureCookies              bool
	Logger                     *slog.Logger
}

func New(repo Repository, options Options) http.Handler {
	if options.RequestTimeout <= 0 {
		options.RequestTimeout = 10 * time.Second
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	s := &server{repo: repo, key: options.APIKey, metricsBearerToken: options.MetricsBearerToken, username: options.Username, password: options.Password, refunder: options.Refunder, seller: options.Seller, publicBaseURL: strings.TrimRight(options.PublicBaseURL, "/"), timeout: options.RequestTimeout, secureCookies: options.SecureCookies, logger: options.Logger, metrics: make(map[string]uint64), durations: make(map[string]float64), buckets: make(map[string][7]uint64)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /metrics", s.prometheusMetrics)
	mux.HandleFunc("GET /", http.NotFound)
	mux.HandleFunc("POST /auth/login", s.login)
	mux.HandleFunc("POST /auth/logout", s.logout)
	mux.HandleFunc("GET /v1/orders", s.orders)
	mux.HandleFunc("GET /v1/orders/{id}", s.order)
	mux.HandleFunc("GET /v1/orders/{id}/invoice.pdf", s.invoicePDF)
	mux.HandleFunc("GET /v1/orders/{id}/invoice.xml", s.invoiceXML)
	mux.HandleFunc("POST /v1/orders/{id}/notes", s.note)
	mux.HandleFunc("POST /v1/orders/{id}/credit-note", s.creditNote)
	mux.HandleFunc("POST /v1/orders/{id}/cancellation", s.cancellation)
	mux.HandleFunc("POST /v1/orders/{id}/refund", s.refund)
	mux.HandleFunc("GET /v1/fulfillments", s.fulfillments)
	mux.HandleFunc("GET /v1/fulfillments/{id}", s.fulfillment)
	mux.HandleFunc("POST /v1/fulfillments/{id}/claim", s.claim)
	mux.HandleFunc("POST /v1/fulfillments/{id}/pack", s.pack)
	mux.HandleFunc("POST /v1/fulfillments/{id}/hold", s.hold)
	mux.HandleFunc("POST /v1/fulfillments/{id}/release", s.release)
	mux.HandleFunc("POST /v1/fulfillments/{id}/label", s.label)
	mux.HandleFunc("POST /v1/fulfillments/{id}/handover", s.handover)
	return s.logging(s.timeoutMiddleware(s.auth(mux)))
}

type server struct {
	repo                    Repository
	key, username, password string
	metricsBearerToken      string
	refunder                payments.Refunder
	seller                  SellerProfile
	publicBaseURL           string
	timeout                 time.Duration
	secureCookies           bool
	logger                  *slog.Logger
	metricsMu               sync.Mutex
	metrics                 map[string]uint64
	durations               map[string]float64
	buckets                 map[string][7]uint64
}

var requestSequence atomic.Uint64
var omsDurationBuckets = [...]float64{0.01, 0.05, 0.1, 0.3, 1, 3, 10}

type actorContextKey struct{}

func (s *server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := fmt.Sprintf("%x-%x", time.Now().UnixNano(), requestSequence.Add(1))
		route := omsMetricRoute(r.URL.Path)
		parent := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := otel.Tracer("mitte/oms").Start(parent, r.Method+" "+route)
		defer span.End()
		span.SetAttributes(
			attribute.String("http.request.method", r.Method),
			attribute.String("http.route", route),
		)
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		recorder := &omsStatusRecorder{ResponseWriter: w, status: http.StatusOK}
		started := time.Now()
		next.ServeHTTP(recorder, r.WithContext(ctx))
		duration := time.Since(started)
		span.SetAttributes(attribute.Int("http.response.status_code", recorder.status))
		if recorder.status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(recorder.status))
		}
		key := r.Method + "\x00" + route + "\x00" + strconv.Itoa(recorder.status)
		s.metricsMu.Lock()
		s.metrics[key]++
		s.durations[key] += duration.Seconds()
		buckets := s.buckets[key]
		for i, bound := range omsDurationBuckets {
			if duration.Seconds() <= bound {
				buckets[i]++
			}
		}
		s.buckets[key] = buckets
		s.metricsMu.Unlock()
		if route == "/metrics" {
			return
		}
		attributes := []any{"request_id", id, "method", r.Method, "route", route, "status", recorder.status, "duration_ms", duration.Milliseconds()}
		spanContext := span.SpanContext()
		if spanContext.IsValid() {
			attributes = append(attributes, "trace_id", spanContext.TraceID().String(), "span_id", spanContext.SpanID().String())
		}
		switch {
		case recorder.status >= http.StatusInternalServerError:
			s.logger.Error("OMS request completed", attributes...)
		case recorder.status >= http.StatusBadRequest:
			s.logger.Warn("OMS request completed", attributes...)
		default:
			s.logger.Info("OMS request completed", attributes...)
		}
	})
}

type omsStatusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *omsStatusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *omsStatusRecorder) Write(body []byte) (int, error) {
	return w.ResponseWriter.Write(body)
}

func omsMetricRoute(path string) string {
	switch {
	case path == "/healthz", path == "/readyz", path == "/metrics", path == "/":
		return path
	case strings.HasPrefix(path, "/auth/"):
		return "/auth"
	case strings.HasPrefix(path, "/v1/orders"):
		return "/v1/orders"
	case strings.HasPrefix(path, "/v1/fulfillments"):
		return "/v1/fulfillments"
	default:
		return "/other"
	}
}

func (s *server) prometheusMetrics(w http.ResponseWriter, r *http.Request) {
	if s.metricsBearerToken != "" && !validMetricsBearerToken(r, s.metricsBearerToken) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
		write(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	s.metricsMu.Lock()
	keys := make([]string, 0, len(s.metrics))
	for key := range s.metrics {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var output strings.Builder
	output.WriteString("# HELP mitte_oms_http_requests_total Total completed OMS HTTP requests.\n# TYPE mitte_oms_http_requests_total counter\n")
	for _, key := range keys {
		parts := strings.Split(key, "\x00")
		fmt.Fprintf(&output, "mitte_oms_http_requests_total{method=%q,path=%q,status=%q} %d\n", parts[0], parts[1], parts[2], s.metrics[key])
	}
	output.WriteString("# HELP mitte_oms_http_request_duration_seconds OMS HTTP request duration in seconds.\n# TYPE mitte_oms_http_request_duration_seconds histogram\n")
	for _, key := range keys {
		parts := strings.Split(key, "\x00")
		labels := fmt.Sprintf("method=%q,path=%q,status=%q", parts[0], parts[1], parts[2])
		for i, bound := range omsDurationBuckets {
			fmt.Fprintf(&output, "mitte_oms_http_request_duration_seconds_bucket{%s,le=%q} %d\n", labels, fmt.Sprint(bound), s.buckets[key][i])
		}
		fmt.Fprintf(&output, "mitte_oms_http_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\n", labels, s.metrics[key])
		fmt.Fprintf(&output, "mitte_oms_http_request_duration_seconds_sum{%s} %.6f\n", labels, s.durations[key])
		fmt.Fprintf(&output, "mitte_oms_http_request_duration_seconds_count{%s} %d\n", labels, s.metrics[key])
	}
	s.metricsMu.Unlock()
	if s.repo != nil {
		stats := s.repo.DatabaseStats()
		fmt.Fprintf(&output, "# HELP mitte_oms_database_open_connections Open OMS database connections.\n# TYPE mitte_oms_database_open_connections gauge\nmitte_oms_database_open_connections %d\n", stats.OpenConnections)
		fmt.Fprintf(&output, "# HELP mitte_oms_database_in_use_connections In-use OMS database connections.\n# TYPE mitte_oms_database_in_use_connections gauge\nmitte_oms_database_in_use_connections %d\n", stats.InUse)
		fmt.Fprintf(&output, "# HELP mitte_oms_database_max_open_connections Configured maximum OMS database connections.\n# TYPE mitte_oms_database_max_open_connections gauge\nmitte_oms_database_max_open_connections %d\n", stats.MaxOpenConnections)
		fmt.Fprintf(&output, "# HELP mitte_oms_database_wait_count_total OMS database pool waits.\n# TYPE mitte_oms_database_wait_count_total counter\nmitte_oms_database_wait_count_total %d\n", stats.WaitCount)
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(output.String()))
}

func validMetricsBearerToken(r *http.Request, expected string) bool {
	const prefix = "Bearer "
	got := r.Header.Get("Authorization")
	return strings.HasPrefix(got, prefix) && subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(got, prefix)), []byte(expected)) == 1
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	write(w, http.StatusOK, map[string]string{"status": "ok"})
}
func (s *server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.repo.Ping(ctx); err != nil {
		write(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	write(w, http.StatusOK, map[string]string{"status": "ready"})
}
func (s *server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" || r.URL.Path == "/metrics" || r.URL.Path == "/" || r.URL.Path == "/auth/login" {
			next.ServeHTTP(w, r)
			return
		}
		got := r.Header.Get("Authorization")
		const prefix = "Bearer "
		api := s.key != "" && strings.HasPrefix(got, prefix) && subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(got, prefix)), []byte(s.key)) == 1
		if api {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorContextKey{}, "oms:service")))
			return
		}
		principal, err := s.sessionPrincipal(r)
		if err != nil {
			if errors.Is(err, store.ErrInvalidSession) {
				write(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			s.logger.Error("validate OMS session", "error", err)
			write(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		if principal != "oms:"+s.username {
			write(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions && !sameOriginRequest(r) {
			write(w, http.StatusForbidden, map[string]string{"error": "same-origin request required"})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorContextKey{}, principal)))
	})
}

func sameOriginRequest(r *http.Request) bool {
	rawOrigin := r.Header.Get("Origin")
	if rawOrigin == "" {
		return false
	}
	origin, err := url.Parse(rawOrigin)
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") ||
		origin.Host == "" || origin.User != nil || origin.Path != "" ||
		origin.RawQuery != "" || origin.Fragment != "" {
		return false
	}
	return strings.EqualFold(origin.Host, r.Host)
}
func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) || s.username == "" || s.password == "" || subtle.ConstantTimeCompare([]byte(in.Username), []byte(s.username)) != 1 || subtle.ConstantTimeCompare([]byte(in.Password), []byte(s.password)) != 1 {
		write(w, http.StatusUnauthorized, map[string]string{"error": "Invalid username or password."})
		return
	}
	token, err := store.NewOperatorSessionToken()
	if err != nil {
		s.logger.Error("generate OMS session token", "error", err)
		write(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	expiresAt := time.Now().UTC().Add(8 * time.Hour)
	if err := s.repo.CreateOperatorSession(r.Context(), token, "oms:"+s.username, expiresAt); err != nil {
		s.logger.Error("create OMS session", "error", err)
		write(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "mitte_oms", Value: token, Path: "/", Expires: expiresAt, HttpOnly: true, Secure: s.secureCookies, SameSite: http.SameSiteStrictMode, MaxAge: 28800})
	write(w, http.StatusOK, map[string]string{"status": "ok"})
}
func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("mitte_oms"); err == nil {
		if err := s.repo.RevokeOperatorSession(r.Context(), cookie.Value, time.Now().UTC()); err != nil {
			s.logger.Error("revoke OMS session", "error", err)
			write(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "mitte_oms", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.secureCookies, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}
func (s *server) sessionPrincipal(r *http.Request) (string, error) {
	c, err := r.Cookie("mitte_oms")
	if err != nil || len(c.Value) != 64 {
		return "", store.ErrInvalidSession
	}
	principal, err := s.repo.OperatorSessionPrincipal(r.Context(), c.Value, time.Now().UTC())
	if err != nil {
		return "", err
	}
	return principal, nil
}

func omsActor(r *http.Request) string {
	principal, ok := r.Context().Value(actorContextKey{}).(string)
	if !ok || principal == "" {
		return "oms:service"
	}
	return principal
}
func (s *server) timeoutMiddleware(next http.Handler) http.Handler {
	return http.TimeoutHandler(next, s.timeout, `{"error":"request timed out"}`)
}
func (s *server) orders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.repo.OrdersForOperations(r.Context())
	if err != nil {
		failure(w, err)
		return
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if query != "" {
		filtered := orders[:0]
		for _, order := range orders {
			candidate := strings.ToLower(strings.Join([]string{order.ID, order.CustomerName, order.CustomerEmail, order.CompanyName}, " "))
			if strings.Contains(candidate, query) {
				filtered = append(filtered, order)
			}
		}
		orders = filtered
	}
	write(w, http.StatusOK, map[string]any{"orders": orders})
}
func (s *server) order(w http.ResponseWriter, r *http.Request) {
	detail, err := s.repo.OrderManagementDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		failure(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"detail": detail})
}
func (s *server) invoicePDF(w http.ResponseWriter, r *http.Request) {
	detail, err := s.repo.OrderManagementDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		failure(w, err)
		return
	}
	if detail.Order.Invoice == nil || detail.Order.Shipment == nil {
		write(w, http.StatusConflict, map[string]string{"error": "invoice is available after shipment"})
		return
	}
	seller, err := s.repo.InvoiceSellerForOrder(r.Context(), detail.Order.ID)
	if err != nil {
		failure(w, err)
		return
	}
	if !isValidInvoiceSeller(seller) {
		seller = s.seller
	}
	pdf := InvoicePDF(detail, seller)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+detail.Order.Invoice.Number+`.pdf"`)
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(pdf)
}
func (s *server) invoiceXML(w http.ResponseWriter, r *http.Request) {
	detail, err := s.repo.OrderManagementDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		failure(w, err)
		return
	}
	if detail.Order.Invoice == nil || detail.Order.Shipment == nil {
		write(w, http.StatusConflict, map[string]string{"error": "invoice is available after shipment"})
		return
	}
	seller, err := s.repo.InvoiceSellerForOrder(r.Context(), detail.Order.ID)
	if err != nil {
		failure(w, err)
		return
	}
	if !isValidInvoiceSeller(seller) {
		seller = s.seller
	}
	output, err := InvoiceXML(detail, seller)
	if err != nil {
		failure(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+detail.Order.Invoice.Number+`.xml"`)
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(output)
}
func (s *server) note(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Body   string `json:"body"`
		Author string `json:"author"`
	}
	if !decode(w, r, &in) || !validText(in.Body, true, 4000) || !validText(in.Author, true, 128) {
		write(w, http.StatusBadRequest, map[string]string{"error": "body and author are required"})
		return
	}
	note, err := s.repo.AddInternalOrderNote(r.Context(), r.PathValue("id"), omsActor(r), strings.TrimSpace(in.Body))
	if err != nil {
		failure(w, err)
		return
	}
	write(w, http.StatusCreated, map[string]any{"note": note})
}
func (s *server) creditNote(w http.ResponseWriter, r *http.Request) {
	s.createDocument(w, r, "credit_note")
}
func (s *server) cancellation(w http.ResponseWriter, r *http.Request) {
	s.createDocument(w, r, "cancellation")
}
func (s *server) createDocument(w http.ResponseWriter, r *http.Request, kind string) {
	var in struct {
		Reason string `json:"reason"`
		Actor  string `json:"actor"`
	}
	if !decode(w, r, &in) || !validText(in.Actor, true, 128) || !validText(in.Reason, true, 1000) {
		write(w, http.StatusBadRequest, map[string]string{"error": "reason and actor are required"})
		return
	}
	doc, err := s.repo.CreateInvoiceDocument(r.Context(), r.PathValue("id"), kind, strings.TrimSpace(in.Reason), omsActor(r))
	if err != nil {
		failure(w, err)
		return
	}
	write(w, http.StatusCreated, map[string]any{"document": doc})
}
func (s *server) refund(w http.ResponseWriter, r *http.Request) {
	if s.refunder == nil {
		write(w, http.StatusServiceUnavailable, map[string]string{"error": "Stripe refunds are not configured for OMS"})
		return
	}
	var in struct {
		Reason string `json:"reason"`
		Actor  string `json:"actor"`
	}
	if !decode(w, r, &in) || !validText(in.Actor, true, 128) || !validText(in.Reason, true, 1000) {
		write(w, http.StatusBadRequest, map[string]string{"error": "reason and actor are required"})
		return
	}
	detail, err := s.repo.OrderManagementDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		failure(w, err)
		return
	}
	if detail.Order.Status != "paid" || detail.Order.StripePaymentIntentID == "" {
		write(w, http.StatusConflict, map[string]string{"error": "only paid Stripe orders can be refunded"})
		return
	}
	refundID, err := s.refunder.RefundPaymentIntent(r.Context(), detail.Order.StripePaymentIntentID, "oms_refund_"+detail.Order.ID)
	if err != nil {
		write(w, http.StatusBadGateway, map[string]string{"error": "Stripe refund failed"})
		return
	}
	if err = s.repo.CancelPaidOrderForOperations(r.Context(), detail.Order.ID, refundID, strings.TrimSpace(in.Reason)); err != nil {
		failure(w, err)
		return
	}
	doc, err := s.repo.CreateInvoiceDocument(r.Context(), detail.Order.ID, "cancellation", strings.TrimSpace(in.Reason), omsActor(r))
	if err != nil && !strings.Contains(err.Error(), "duplicate") {
		failure(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"refundId": refundID, "document": doc})
}
func (s *server) fulfillments(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if values, present := query["orderIds"]; present {
		if len(values) != 1 {
			write(w, http.StatusBadRequest, map[string]string{"error": "invalid orderIds"})
			return
		}
		orderIDs, err := parseFulfillmentOrderIDs(values[0])
		if err != nil {
			write(w, http.StatusBadRequest, map[string]string{"error": "invalid orderIds"})
			return
		}
		items, err := s.repo.WarehouseFulfillmentsByOrderIDs(r.Context(), orderIDs)
		if errors.Is(err, store.ErrInvalidWarehouseOrderIDs) {
			write(w, http.StatusBadRequest, map[string]string{"error": "too many orderIds"})
			return
		}
		if err != nil {
			failure(w, err)
			return
		}
		write(w, http.StatusOK, map[string]any{"fulfillments": items})
		return
	}
	page, limit, offset, err := parseFulfillmentPage(query.Get("page"), query.Get("limit"))
	if err != nil {
		write(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	status := strings.TrimSpace(query.Get("status"))
	search := strings.TrimSpace(query.Get("q"))
	if !validFulfillmentStatus(status) || len(search) > 120 {
		write(w, http.StatusBadRequest, map[string]string{"error": "invalid fulfillment filter"})
		return
	}
	items, total, stats, err := s.repo.WarehouseQueuePage(r.Context(), status, search, limit, offset)
	if err != nil {
		failure(w, err)
		return
	}
	pages := (total + limit - 1) / limit
	if pages < 1 {
		pages = 1
	}
	write(w, http.StatusOK, map[string]any{
		"fulfillments": items, "total": total, "page": page, "limit": limit, "pages": pages,
		"summary": map[string]int{"active": stats.Active, "handedOver": stats.HandedOver, "processing": stats.Processing},
	})
}

const (
	defaultFulfillmentPageSize = 50
	maxFulfillmentPageSize     = 100
	maxFulfillmentPage         = 10000
	maxFulfillmentOffset       = 100000
)

func parseFulfillmentPage(pageValue, limitValue string) (page, limit, offset int, err error) {
	page = 1
	if pageValue != "" {
		page, err = strconv.Atoi(pageValue)
		if err != nil || page < 1 || page > maxFulfillmentPage {
			return 0, 0, 0, errors.New("invalid fulfillment page")
		}
	}
	limit = defaultFulfillmentPageSize
	if limitValue != "" {
		limit, err = strconv.Atoi(limitValue)
		if err != nil || limit < 1 || limit > maxFulfillmentPageSize {
			return 0, 0, 0, errors.New("invalid fulfillment page size")
		}
	}
	if page-1 > maxFulfillmentOffset/limit {
		return 0, 0, 0, errors.New("fulfillment offset exceeds maximum")
	}
	return page, limit, (page - 1) * limit, nil
}

func validFulfillmentStatus(status string) bool {
	switch status {
	case "", "all", "active", "ready_to_pick", "picking", "packed", "label_ready", "handed_over", "cancelled":
		return true
	default:
		return false
	}
}

func parseFulfillmentOrderIDs(value string) ([]string, error) {
	if value == "" {
		return nil, errors.New("order IDs cannot be empty")
	}
	values := strings.Split(value, ",")
	if len(values) > 100 {
		return nil, errors.New("too many order IDs")
	}
	ids := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, id := range values {
		id = strings.TrimSpace(id)
		if id == "" || len(id) > 128 {
			return nil, errors.New("invalid order ID")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *server) fulfillment(w http.ResponseWriter, r *http.Request) {
	item, events, eventCount, err := s.repo.WarehouseFulfillment(r.Context(), r.PathValue("id"))
	if err != nil {
		failure(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"fulfillment": item, "events": events, "eventCount": eventCount})
}
func (s *server) claim(w http.ResponseWriter, r *http.Request) { s.transition(w, r, "picking") }
func (s *server) pack(w http.ResponseWriter, r *http.Request)  { s.transition(w, r, "packed") }
func (s *server) hold(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) || !validText(in.Reason, true, 500) || utf8.RuneCountInString(strings.TrimSpace(in.Reason)) < 3 {
		write(w, http.StatusBadRequest, map[string]string{"error": "reason must be 3 to 500 characters"})
		return
	}
	if err := s.repo.HoldWarehouseFulfillment(r.Context(), r.PathValue("id"), omsActor(r), strings.TrimSpace(in.Reason)); err != nil {
		failure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *server) release(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.ReleaseWarehouseFulfillment(r.Context(), r.PathValue("id"), omsActor(r)); err != nil {
		failure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *server) transition(w http.ResponseWriter, r *http.Request, next string) {
	var in struct {
		Actor  string `json:"actor"`
		Detail string `json:"detail"`
	}
	if !decode(w, r, &in) || !validText(in.Actor, true, 128) || !validText(in.Detail, false, 1000) {
		write(w, http.StatusBadRequest, map[string]string{"error": "actor is required"})
		return
	}
	err := s.repo.TransitionWarehouseFulfillment(r.Context(), r.PathValue("id"), omsActor(r), next, strings.TrimSpace(in.Detail))
	if err != nil {
		failure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *server) label(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Actor          string `json:"actor"`
		Carrier        string `json:"carrier"`
		TrackingNumber string `json:"trackingNumber"`
		TrackingURL    string `json:"trackingUrl"`
		LabelURL       string `json:"labelUrl"`
	}
	if !decode(w, r, &in) || !validText(in.Actor, true, 128) || !validText(in.Carrier, true, 100) || !validText(in.TrackingNumber, true, 160) || !https(in.TrackingURL) || !https(in.LabelURL) {
		write(w, http.StatusBadRequest, map[string]string{"error": "actor, carrier, trackingNumber, and valid HTTPS URLs are required"})
		return
	}
	err := s.repo.AttachWarehouseLabel(r.Context(), r.PathValue("id"), omsActor(r), strings.TrimSpace(in.Carrier), strings.TrimSpace(in.TrackingNumber), strings.TrimSpace(in.TrackingURL), strings.TrimSpace(in.LabelURL))
	if err != nil {
		failure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *server) handover(w http.ResponseWriter, r *http.Request) {
	item, _, _, err := s.repo.WarehouseFulfillment(r.Context(), r.PathValue("id"))
	if err != nil {
		failure(w, err)
		return
	}
	if item.Status != "label_ready" {
		write(w, http.StatusConflict, map[string]string{"error": "fulfillment must be label_ready"})
		return
	}
	if err = s.repo.MarkWarehouseShipmentBy(r.Context(), item.OrderID, omsActor(r), item.Carrier, item.TrackingNumber, item.TrackingURL, s.publicBaseURL, s.seller); err != nil {
		failure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false
	}
	var trailing any
	return errors.Is(decoder.Decode(&trailing), io.EOF)
}
func https(value string) bool {
	if value == "" {
		return true
	}
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Fragment == "" && len(value) <= 2048
}

func validText(value string, required bool, maxRunes int) bool {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || (required && value == "") || utf8.RuneCountInString(value) > maxRunes {
		return false
	}
	for _, character := range value {
		if character == 0 {
			return false
		}
	}
	return true
}
func failure(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		write(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if errors.Is(err, store.ErrCustomerFraudBlocked) {
		write(w, http.StatusConflict, map[string]string{"error": "customer account is blocked for fraud review"})
		return
	}
	if errors.Is(err, store.ErrInvalidFulfillmentTransition) || errors.Is(err, store.ErrInvalidFulfillmentHold) || errors.Is(err, store.ErrInvalidTransition) {
		write(w, http.StatusConflict, map[string]string{"error": "invalid state transition"})
		return
	}
	if errors.Is(err, store.ErrInvalidCheckout) {
		write(w, http.StatusBadRequest, map[string]string{"error": "invalid input"})
		return
	}
	write(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// invoiceDocument intentionally uses only standard PDF fonts so the isolated
// OMS can issue a portable demo document without an external rendering service.
// The prominent DEMO label prevents this fictional seller profile being used as
// a real German tax invoice before configured legal data is supplied.
type SellerProfile = invoiceprofile.Profile

// InvoicePDF renders the immutable paid-order snapshot into a portable PDF.
// Callers must authorize access to the order before calling it.
func InvoicePDF(detail store.OrderManagementDetail, seller SellerProfile) []byte {
	o := detail.Order
	if seller.Name == "" {
		seller.Name = "Rechnungssteller nicht konfiguriert"
		seller.Demo = true
	}
	if seller.Address == "" {
		seller.Address = strings.TrimSpace(strings.Join([]string{seller.Street, seller.PostalCode + " " + seller.City, seller.CountryCode}, ", "))
	}
	address := []string{o.CustomerName, o.CompanyName, o.BillingAddress.AddressLine1, o.BillingAddress.AddressLine2, o.BillingAddress.PostalCode + " " + o.BillingAddress.City, o.BillingAddress.CountryCode}
	content := pdfAt(seller.Name, 50, 795, 18) + pdfAt(seller.Address, 50, 770, 8) + pdfAt("USt-IdNr.: "+seller.VATID, 50, 756, 8) + pdfAt(seller.Email+" | "+seller.IBAN, 50, 743, 7)
	status := "OFFEN"
	if o.Status == "paid" || (o.Status == "fulfilled" && o.PaymentProvider != "mitte_invoice") {
		status = "BEZAHLT"
	}
	content += pdfAt("RECHNUNG", 390, 795, 22) + pdfAt("Rechnungsnummer: "+o.Invoice.Number, 390, 770, 9) + pdfAt("Rechnungsdatum: "+o.Invoice.IssuedAt.Format("02.01.2006"), 390, 756, 9) + pdfAt("Faellig am: "+o.Invoice.DueAt.Format("02.01.2006"), 390, 742, 9) + pdfAt("Zahlungsstatus: "+status, 390, 728, 9)
	content += pdfAt("RECHNUNGSEMPFAENGER", 50, 680, 8)
	y := 665
	for _, line := range address {
		if strings.TrimSpace(line) != "" {
			content += pdfAt(line, 50, y, 10)
			y -= 14
		}
	}
	content += pdfLine(50, 570, 545, 570) + pdfAt("POSITION", 50, 552, 8) + pdfAt("MENGE", 350, 552, 8) + pdfAt("EINZELPREIS", 410, 552, 8) + pdfAt("BETRAG", 500, 552, 8) + pdfLine(50, 544, 545, 544)
	y = 528
	for _, item := range o.Items {
		content += pdfAt(item.Name, 50, y, 9) + pdfAt(fmt.Sprint(item.Quantity), 355, y, 9) + pdfAt(eur(item.UnitPriceCents), 410, y, 9) + pdfAt(eur(item.Quantity*item.UnitPriceCents), 500, y, 9)
		y -= 18
	}
	if o.ShippingCents > 0 {
		content += pdfAt("Versand", 50, y, 9) + pdfAt(eur(o.ShippingCents), 500, y, 9)
		y -= 18
	}
	y -= 8
	content += pdfLine(330, y, 545, y)
	summary := []struct {
		label string
		value string
	}{
		{label: "Zwischensumme", value: eur(o.SubtotalCents)},
	}
	if o.DiscountCents > 0 {
		summary = append(summary, struct {
			label string
			value string
		}{label: "Rabatt", value: "-" + eur(o.DiscountCents)})
	}
	summary = append(summary,
		struct {
			label string
			value string
		}{label: "Versand", value: eur(o.ShippingCents)},
		struct {
			label string
			value string
		}{label: "Enthaltene Umsatzsteuer", value: eur(o.TaxCents)},
	)
	summaryY := y - 16
	for _, row := range summary {
		content += pdfAt(row.label, 370, summaryY, 9) + pdfAt(row.value, 500, summaryY, 9)
		summaryY -= 15
	}
	content += pdfLine(330, summaryY+6, 545, summaryY+6) + pdfAt("GESAMTBETRAG", 370, summaryY-10, 11) + pdfAt(eur(o.TotalCents), 500, summaryY-10, 11)
	footer := "Lieferadresse und Positionen entsprechen dem unveraenderlichen Bestellsnapshot."
	if seller.Demo {
		footer = "DEMO-Dokument: Keine steuerlich gueltige Rechnung."
	}
	content += pdfLine(50, 110, 545, 110) + pdfAt(seller.PaymentTerms, 50, 92, 9) + pdfAt(footer, 50, 76, 7)
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>", fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content), "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objects)+1)
	for i, obj := range objects {
		offs[i+1] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for i := 1; i <= len(objects); i++ {
		fmt.Fprintf(&b, "%010d 00000 n \n", offs[i])
	}
	fmt.Fprintf(&b, "trailer << /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}
func pdfAt(text string, x, y, size int) string {
	return fmt.Sprintf("BT /F1 %d Tf %d %d Td (%s) Tj ET ", size, x, y, pdfText(text))
}
func pdfLine(x1, y1, x2, y2 int) string {
	return fmt.Sprintf("0.5 w %d %d m %d %d l S ", x1, y1, x2, y2)
}
func eur(cents int) string { return fmt.Sprintf("EUR %.2f", float64(cents)/100) }
func pdfText(value string) string {
	value = strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)", "\r", " ", "\n", " ").Replace(value)
	return strings.Map(func(r rune) rune {
		if r < 32 || r > 126 {
			return '?'
		}
		return r
	}, value)
}
