package web

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

func (a *App) timeout(next http.Handler) http.Handler {
	return http.TimeoutHandler(next, a.requestTimeout, "Die Anfrage hat zu lange gedauert. Bitte versuche es erneut.")
}

func (a *App) hostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(a.trustedHosts) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		host := r.Host
		if value, _, err := net.SplitHostPort(r.Host); err == nil {
			host = value
		}
		if isPrivateScrapeHost(host) && r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		if _, allowed := a.trustedHosts[host]; !allowed {
			http.Error(w, "Ungültiger Host.", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isPrivateScrapeHost(host string) bool {
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast()
	}
	return false
}

func (a *App) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; object-src 'none'; img-src 'self' https://images.unsplash.com https://images.openbeautyfacts.org https://picsum.photos https://fastly.picsum.photos; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; script-src 'self'; connect-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		if a.production {
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) requestLimits(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && !a.apiLimiter.allow(a.clientIP(r)) {
			w.Header().Set("Retry-After", "60")
			respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Zu viele Anfragen. Bitte warte kurz."})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started, id := time.Now(), randomToken(8)
		route := metricPath(r.URL.Path)
		parent := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := otel.Tracer("mitte/web").Start(parent, r.Method+" "+route)
		defer span.End()
		span.SetAttributes(
			attribute.String("http.request.method", r.Method),
			attribute.String("http.route", route),
		)
		w.Header().Set("X-Request-ID", id)
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r.WithContext(context.WithValue(ctx, requestIDKey{}, id)))
		duration := time.Since(started)
		span.SetAttributes(attribute.Int("http.response.status_code", recorder.status))
		if recorder.status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(recorder.status))
		}
		a.metrics.record(r.Method, metricPath(r.URL.Path), recorder.status, duration)
		spanContext := span.SpanContext()
		if spanContext.IsValid() {
			a.logger.Info("request completed", "request_id", id, "trace_id", spanContext.TraceID().String(), "span_id", spanContext.SpanID().String(), "method", r.Method, "path", route, "status", recorder.status, "duration_ms", duration.Milliseconds())
		} else {
			a.logger.Info("request completed", "request_id", id, "method", r.Method, "path", route, "status", recorder.status, "duration_ms", duration.Milliseconds())
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(body []byte) (int, error) {
	return w.ResponseWriter.Write(body)
}

func metricPath(path string) string {
	switch {
	case path == "/api/checkout", path == "/webhooks/stripe":
		return path
	case strings.HasPrefix(path, "/api/"):
		return "/api"
	case strings.HasPrefix(path, "/webhooks/"):
		return "/webhooks"
	case strings.HasPrefix(path, "/static/"):
		return "/static"
	case path == "/healthz", path == "/readyz", path == "/metrics":
		return path
	default:
		return "/page"
	}
}

func (a *App) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.logger.Error("panic recovered", "request_id", requestID(r.Context()), "panic", recovered)
				http.Error(w, "Es ist etwas schiefgelaufen. Bitte versuche es erneut.", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// clientIP honours forwarding headers only when the directly connected peer is
// a configured trusted proxy. This keeps client-controlled X-Forwarded-For
// headers from bypassing request and login rate limits.
func (a *App) clientIP(r *http.Request) string {
	peer := clientIP(r)
	parsedPeer := net.ParseIP(peer)
	trustedPeer := false
	for _, network := range a.trustedProxyCIDRs {
		if parsedPeer != nil && network.Contains(parsedPeer) {
			trustedPeer = true
			break
		}
	}
	if !trustedPeer {
		return peer
	}

	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	if len(forwarded) == 0 || strings.TrimSpace(forwarded[0]) == "" {
		return peer
	}
	for _, value := range forwarded {
		if net.ParseIP(strings.TrimSpace(value)) == nil {
			return peer
		}
	}
	for index := len(forwarded) - 1; index >= 0; index-- {
		candidate := net.ParseIP(strings.TrimSpace(forwarded[index]))
		trusted := false
		for _, network := range a.trustedProxyCIDRs {
			if network.Contains(candidate) {
				trusted = true
				break
			}
		}
		if !trusted {
			return candidate.String()
		}
	}
	return strings.TrimSpace(forwarded[0])
}
func withStaticCaching(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=300, must-revalidate")
		next.ServeHTTP(w, r)
	})
}

type requestIDKey struct{}

func requestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}

type clientLimiter struct {
	mu       sync.Mutex
	visitors map[string]rateWindow
	limit    int
	window   time.Duration
}
type rateWindow struct {
	started time.Time
	count   int
}

func newClientLimiter(limit int, window time.Duration) *clientLimiter {
	return &clientLimiter{visitors: make(map[string]rateWindow), limit: limit, window: window}
}
func (l *clientLimiter) allow(client string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now, window := time.Now(), l.visitors[client]
	if window.started.IsZero() && len(l.visitors) >= 10000 {
		for key, existing := range l.visitors {
			if now.Sub(existing.started) >= l.window {
				delete(l.visitors, key)
			}
		}
		if len(l.visitors) >= 10000 {
			return false
		}
	}
	if window.started.IsZero() || now.Sub(window.started) >= l.window {
		l.visitors[client] = rateWindow{started: now, count: 1}
		return true
	}
	if window.count >= l.limit {
		return false
	}
	window.count++
	l.visitors[client] = window
	return true
}
