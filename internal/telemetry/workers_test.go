package telemetry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWorkerMetricsExposeBoundedOperationSeries(t *testing.T) {
	metrics := &WorkerMetrics{
		worker: "email-relay", started: time.Now(), requests: make(map[string]uint64), lastOK: make(map[string]float64),
	}
	metrics.Record("email_delivery", "success")
	metrics.Record("email_delivery", "error")
	metrics.Record("customer@example.test", "error")

	recorder := httptest.NewRecorder()
	metrics.serveHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	if !strings.Contains(body, `worker="email-relay",operation="email_delivery",result="success"} 1`) {
		t.Fatalf("worker success metric missing: %s", body)
	}
	if !strings.Contains(body, `worker="email-relay",operation="email_delivery",result="error"} 1`) {
		t.Fatalf("worker error metric missing: %s", body)
	}
	if strings.Contains(body, "customer@example.test") {
		t.Fatalf("worker metrics contain an invalid high-cardinality operation: %s", body)
	}
}

func TestWorkerMetricsRequireBearerToken(t *testing.T) {
	handler := workerMetricsAuth("a-long-random-metrics-token", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, test := range []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "missing", wantStatus: http.StatusUnauthorized},
		{name: "valid", token: "Bearer a-long-random-metrics-token", wantStatus: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			request.Header.Set("Authorization", test.token)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
		})
	}
}
