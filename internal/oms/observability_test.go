package oms

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"log/slog"

	"mitte/internal/store"
)

type sessionRepositoryStub struct {
	Repository
	sessions map[string]string
	actor    string
}

func TestOMSRootDoesNotServeASecondConsole(t *testing.T) {
	handler := New(&sessionRepositoryStub{}, Options{APIKey: "test-key", Username: "staff", Password: "secret", Logger: slog.Default()})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("root status = %d, want 404", recorder.Code)
	}
	if strings.Contains(recorder.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("root content type = %q, want no HTML console", recorder.Header().Get("Content-Type"))
	}
	if strings.Contains(recorder.Body.String(), "Order management") {
		t.Fatal("OMS API root still serves the legacy management console")
	}
}

func (s *sessionRepositoryStub) CreateOperatorSession(_ context.Context, token, principal string, _ time.Time) error {
	if s.sessions == nil {
		s.sessions = make(map[string]string)
	}
	s.sessions[token] = principal
	return nil
}

func (s *sessionRepositoryStub) OperatorSessionPrincipal(_ context.Context, token string, _ time.Time) (string, error) {
	principal, ok := s.sessions[token]
	if !ok {
		return "", store.ErrInvalidSession
	}
	return principal, nil
}

func (s *sessionRepositoryStub) RevokeOperatorSession(_ context.Context, token string, _ time.Time) error {
	delete(s.sessions, token)
	return nil
}

func (s *sessionRepositoryStub) AddInternalOrderNote(_ context.Context, _, author, _ string) (store.InternalOrderNote, error) {
	s.actor = author
	return store.InternalOrderNote{}, nil
}

func TestLoginCookieSecureFlagFollowsProductionConfig(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	repo := &sessionRepositoryStub{sessions: make(map[string]string)}
	handler := New(repo, Options{APIKey: "test-key", Username: "staff", Password: "secret", SecureCookies: true, Logger: logger})
	request := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"staff","password":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", recorder.Code)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("OMS session cookie = %#v, want Secure, HttpOnly, SameSite=Strict", cookies)
	}
	if _, ok := repo.sessions[cookies[0].Value]; !ok {
		t.Fatal("OMS login did not persist the session token")
	}
}

func TestCookieAuthenticatedMutationsRequireSameOrigin(t *testing.T) {
	repo := &sessionRepositoryStub{sessions: make(map[string]string)}
	s := &server{key: "test-api-key", username: "staff", repo: repo, logger: slog.Default()}
	next := s.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, test := range []struct {
		name       string
		origin     string
		wantStatus int
	}{
		{name: "missing origin", wantStatus: http.StatusForbidden},
		{name: "cross origin", origin: "https://attacker.example", wantStatus: http.StatusForbidden},
		{name: "same origin", origin: "https://oms.example.test", wantStatus: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "https://oms.example.test/v1/orders/42/notes", nil)
			token, err := store.NewOperatorSessionToken()
			if err != nil {
				t.Fatal(err)
			}
			if err := repo.CreateOperatorSession(request.Context(), token, "oms:staff", time.Now().Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			request.AddCookie(&http.Cookie{Name: "mitte_oms", Value: token})
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			recorder := httptest.NewRecorder()

			next.ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
		})
	}
}

func TestOMSLogoutRevokesCopiedSession(t *testing.T) {
	repo := &sessionRepositoryStub{sessions: make(map[string]string)}
	s := &server{key: "test-api-key", username: "staff", repo: repo, logger: slog.Default()}
	token, err := store.NewOperatorSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateOperatorSession(context.Background(), token, "oms:staff", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	logout := s.auth(http.HandlerFunc(s.logout))
	request := httptest.NewRequest(http.MethodPost, "https://oms.example.test/auth/logout", nil)
	request.Header.Set("Origin", "https://oms.example.test")
	request.AddCookie(&http.Cookie{Name: "mitte_oms", Value: token})
	recorder := httptest.NewRecorder()

	logout.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", recorder.Code)
	}
	replay := httptest.NewRequest(http.MethodGet, "https://oms.example.test/v1/orders", nil)
	replay.AddCookie(&http.Cookie{Name: "mitte_oms", Value: token})
	replayRecorder := httptest.NewRecorder()
	s.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(replayRecorder, replay)
	if replayRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("replayed revoked session status = %d, want 401", replayRecorder.Code)
	}
}

func TestOMSNotesUseAuthenticatedActor(t *testing.T) {
	repo := &sessionRepositoryStub{sessions: make(map[string]string)}
	s := &server{key: "test-api-key", username: "staff", repo: repo, logger: slog.Default()}
	token, err := store.NewOperatorSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateOperatorSession(context.Background(), token, "oms:staff", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	handler := s.auth(http.HandlerFunc(s.note))
	request := httptest.NewRequest(http.MethodPost, "https://oms.example.test/v1/orders/42/notes", strings.NewReader(`{"body":"note","author":"forged-admin"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://oms.example.test")
	request.AddCookie(&http.Cookie{Name: "mitte_oms", Value: token})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("note status = %d, want 201", recorder.Code)
	}
	if repo.actor != "oms:staff" {
		t.Fatalf("audit author = %q, want authenticated principal", repo.actor)
	}
}

func TestOMSDecodeRejectsUnknownFieldsAndTrailingValues(t *testing.T) {
	for _, body := range []string{
		`{"body":"note","unexpected":true}`,
		`{"body":"note"} {"body":"another"}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		var input struct {
			Body string `json:"body"`
		}
		if decode(recorder, request, &input) {
			t.Fatalf("decode() accepted invalid request body %q", body)
		}
	}
}

func TestBearerAuthenticatedMutationsDoNotRequireOrigin(t *testing.T) {
	s := &server{key: "test-api-key"}
	next := s.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "https://oms.example.test/v1/orders/42/notes", nil)
	request.Header.Set("Authorization", "Bearer test-api-key")
	recorder := httptest.NewRecorder()

	next.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestOMSLogsAndMetricsUseBoundedRouteLabels(t *testing.T) {
	var logs bytes.Buffer
	handler := New(nil, Options{MetricsBearerToken: "a-long-random-metrics-token", Logger: slog.New(slog.NewJSONHandler(&logs, nil))})
	request := httptest.NewRequest(http.MethodGet, "/v1/orders/sensitive-order-id", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated order request status = %d, want 401", recorder.Code)
	}
	if strings.Contains(logs.String(), "sensitive-order-id") {
		t.Fatalf("OMS logs contain raw order ID: %s", logs.String())
	}
	if recorder.Header().Get("X-Request-ID") == "" || recorder.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("missing request ID or private cache policy: %#v", recorder.Header())
	}

	metricsRequest := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRecorder := httptest.NewRecorder()
	handler.ServeHTTP(metricsRecorder, metricsRequest)
	if metricsRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated metrics status = %d, want 401", metricsRecorder.Code)
	}
	metricsRequest = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRequest.Header.Set("Authorization", "Bearer a-long-random-metrics-token")
	metricsRecorder = httptest.NewRecorder()
	handler.ServeHTTP(metricsRecorder, metricsRequest)
	if metricsRecorder.Code != http.StatusOK {
		t.Fatalf("authenticated metrics status = %d, want 200", metricsRecorder.Code)
	}
	if !strings.Contains(metricsRecorder.Body.String(), `mitte_oms_http_requests_total{method="GET",path="/v1/orders",status="401"} 1`) {
		t.Fatalf("OMS metrics missing bounded unauthorized request series: %s", metricsRecorder.Body.String())
	}
	if !strings.Contains(metricsRecorder.Body.String(), `mitte_oms_http_request_duration_seconds_bucket{method="GET",path="/v1/orders",status="401",le="0.05"}`) {
		t.Fatalf("OMS metrics missing duration histogram: %s", metricsRecorder.Body.String())
	}
	if strings.Contains(metricsRecorder.Body.String(), "sensitive-order-id") {
		t.Fatalf("OMS metrics contain raw order ID: %s", metricsRecorder.Body.String())
	}
}
