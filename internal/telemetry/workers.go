package telemetry

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type WorkerMetrics struct {
	mu       sync.Mutex
	worker   string
	started  time.Time
	requests map[string]uint64
	lastOK   map[string]float64
}

func StartWorkerMetrics(ctx context.Context, worker, token string, logger *slog.Logger) (*WorkerMetrics, error) {
	switch worker {
	case "order-relay", "email-relay", "payment-expiry":
	default:
		return nil, fmt.Errorf("unsupported worker metrics name %q", worker)
	}
	metrics := &WorkerMetrics{
		worker: worker, started: time.Now(),
		requests: make(map[string]uint64), lastOK: make(map[string]float64),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", metrics.serveHTTP)
	server := &http.Server{
		Addr:              ":9464",
		Handler:           workerMetricsAuth(token, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return nil, fmt.Errorf("listen for worker metrics: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	go func() {
		serveErr := make(chan error, 1)
		go func() { serveErr <- server.Serve(listener) }()
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				logger.Error("worker metrics shutdown failed", "worker", worker, "error", err)
			}
			if err := <-serveErr; err != nil && err != http.ErrServerClosed {
				logger.Error("worker metrics server stopped", "worker", worker, "error", err)
			}
		case err := <-serveErr:
			if err != nil && err != http.ErrServerClosed {
				logger.Error("worker metrics server stopped", "worker", worker, "error", err)
			}
		}
	}()
	return metrics, nil
}

func (m *WorkerMetrics) Record(operation, result string) {
	if !validWorkerOperation(operation) || (result != "success" && result != "error") {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := operation + "\x00" + result
	m.requests[key]++
	if result == "success" {
		m.lastOK[operation] = float64(time.Now().Unix())
	}
}

func (m *WorkerMetrics) serveHTTP(w http.ResponseWriter, _ *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	var output strings.Builder
	output.WriteString("# HELP mitte_worker_operations_total Completed background worker operations.\n")
	output.WriteString("# TYPE mitte_worker_operations_total counter\n")
	keys := make([]string, 0, len(m.requests))
	for key := range m.requests {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts := strings.Split(key, "\x00")
		fmt.Fprintf(&output, "mitte_worker_operations_total{worker=%q,operation=%q,result=%q} %d\n", m.worker, parts[0], parts[1], m.requests[key])
	}
	output.WriteString("# HELP mitte_worker_last_success_timestamp_seconds Unix timestamp of the last successful worker operation.\n")
	output.WriteString("# TYPE mitte_worker_last_success_timestamp_seconds gauge\n")
	operations := make([]string, 0, len(m.lastOK))
	for operation := range m.lastOK {
		operations = append(operations, operation)
	}
	sort.Strings(operations)
	for _, operation := range operations {
		fmt.Fprintf(&output, "mitte_worker_last_success_timestamp_seconds{worker=%q,operation=%q} %.0f\n", m.worker, operation, m.lastOK[operation])
	}
	fmt.Fprintf(&output, "# HELP mitte_worker_process_uptime_seconds Worker process uptime in seconds.\n# TYPE mitte_worker_process_uptime_seconds gauge\nmitte_worker_process_uptime_seconds{worker=%q} %.3f\n", m.worker, time.Since(m.started).Seconds())
	_, _ = w.Write([]byte(output.String()))
}

func workerMetricsAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" {
			const prefix = "Bearer "
			got := r.Header.Get("Authorization")
			if !strings.HasPrefix(got, prefix) || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(got, prefix)), []byte(token)) != 1 {
				w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func validWorkerOperation(operation string) bool {
	switch operation {
	case "publish", "email_delivery", "payment_expiry":
		return true
	default:
		return false
	}
}
