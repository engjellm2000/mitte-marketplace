package web

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// metrics is deliberately small and dependency-free. Labels are bounded by
// metricPath and HTTP status so a malformed URL cannot create unbounded series.
type metrics struct {
	mu       sync.Mutex
	started  time.Time
	requests map[string]uint64
	duration map[string]float64
	buckets  map[string][7]uint64
}

func newMetrics() *metrics {
	return &metrics{started: time.Now(), requests: make(map[string]uint64), duration: make(map[string]float64), buckets: make(map[string][7]uint64)}
}

var requestDurationBuckets = [...]float64{0.01, 0.05, 0.1, 0.3, 1, 3, 10}

func (m *metrics) record(method, path string, status int, duration time.Duration) {
	key := method + "\x00" + path + "\x00" + fmt.Sprint(status)
	m.mu.Lock()
	m.requests[key]++
	m.duration[key] += duration.Seconds()
	buckets := m.buckets[key]
	for i, bound := range requestDurationBuckets {
		if duration.Seconds() <= bound {
			buckets[i]++
		}
	}
	m.buckets[key] = buckets
	m.mu.Unlock()
}

func (a *App) prometheusMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if a.metricsBearerToken != "" && !validBearerToken(r, a.metricsBearerToken) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	a.metrics.mu.Lock()
	keys := make([]string, 0, len(a.metrics.requests))
	for key := range a.metrics.requests {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	var output strings.Builder
	output.WriteString("# HELP mitte_http_requests_total Total completed HTTP requests.\n# TYPE mitte_http_requests_total counter\n")
	for _, key := range keys {
		parts := strings.Split(key, "\x00")
		labels := fmt.Sprintf("method=%q,path=%q,status=%q", parts[0], parts[1], parts[2])
		fmt.Fprintf(&output, "mitte_http_requests_total{%s} %d\n", labels, a.metrics.requests[key])
	}
	output.WriteString("# HELP mitte_http_request_duration_seconds HTTP request duration in seconds.\n# TYPE mitte_http_request_duration_seconds histogram\n")
	for _, key := range keys {
		parts := strings.Split(key, "\x00")
		baseLabels := fmt.Sprintf("method=%q,path=%q,status=%q", parts[0], parts[1], parts[2])
		buckets := a.metrics.buckets[key]
		for i, bound := range requestDurationBuckets {
			fmt.Fprintf(&output, "mitte_http_request_duration_seconds_bucket{%s,le=%q} %d\n", baseLabels, fmt.Sprint(bound), buckets[i])
		}
		fmt.Fprintf(&output, "mitte_http_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\n", baseLabels, a.metrics.requests[key])
		fmt.Fprintf(&output, "mitte_http_request_duration_seconds_sum{%s} %.6f\n", baseLabels, a.metrics.duration[key])
		fmt.Fprintf(&output, "mitte_http_request_duration_seconds_count{%s} %d\n", baseLabels, a.metrics.requests[key])
	}
	fmt.Fprintf(&output, "# HELP mitte_process_uptime_seconds Seconds since process start.\n# TYPE mitte_process_uptime_seconds gauge\nmitte_process_uptime_seconds %.3f\n", time.Since(a.metrics.started).Seconds())
	a.metrics.mu.Unlock()
	if a.store != nil {
		stats := a.store.DatabaseStats()
		output.WriteString("# HELP mitte_database_open_connections Open database connections.\n# TYPE mitte_database_open_connections gauge\n")
		fmt.Fprintf(&output, "mitte_database_open_connections %d\n", stats.OpenConnections)
		fmt.Fprintf(&output, "mitte_database_in_use_connections %d\n", stats.InUse)
		fmt.Fprintf(&output, "mitte_database_idle_connections %d\n", stats.Idle)
		fmt.Fprintf(&output, "# HELP mitte_database_max_open_connections Configured maximum open database connections for this process.\n# TYPE mitte_database_max_open_connections gauge\nmitte_database_max_open_connections %d\n", stats.MaxOpenConnections)
		fmt.Fprintf(&output, "# HELP mitte_database_wait_count_total Database pool waits.\n# TYPE mitte_database_wait_count_total counter\nmitte_database_wait_count_total %d\n", stats.WaitCount)
		fmt.Fprintf(&output, "# HELP mitte_database_wait_duration_seconds_total Total time waiting for a database connection.\n# TYPE mitte_database_wait_duration_seconds_total counter\nmitte_database_wait_duration_seconds_total %.6f\n", stats.WaitDuration.Seconds())
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(output.String()))
}

func validBearerToken(r *http.Request, expected string) bool {
	const prefix = "Bearer "
	got := r.Header.Get("Authorization")
	return strings.HasPrefix(got, prefix) && subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(got, prefix)), []byte(expected)) == 1
}
