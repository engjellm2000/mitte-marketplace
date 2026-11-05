package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mitte/internal/store"
)

type operatorSessionStub struct {
	principals map[string]string
}

func (s *operatorSessionStub) CreateOperatorSession(_ context.Context, token, principal string, _ time.Time) error {
	if s.principals == nil {
		s.principals = make(map[string]string)
	}
	s.principals[token] = principal
	return nil
}

func (s *operatorSessionStub) OperatorSessionPrincipal(_ context.Context, token string, _ time.Time) (string, error) {
	principal, ok := s.principals[token]
	if !ok {
		return "", store.ErrInvalidSession
	}
	return principal, nil
}

func (s *operatorSessionStub) RevokeOperatorSession(_ context.Context, token string, _ time.Time) error {
	delete(s.principals, token)
	return nil
}

func TestStaffLogoutRevokesCopiedSession(t *testing.T) {
	sessions := &operatorSessionStub{principals: make(map[string]string)}
	app := &App{
		sessions:         sessions,
		limiter:          newClientLimiter(10, time.Minute),
		staffCredentials: map[string]string{"warehouse": "a-long-secret-password"},
		staffRoles:       map[string]string{"warehouse": "warehouse"},
	}
	token, err := store.NewOperatorSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := sessions.CreateOperatorSession(context.Background(), token, "staff:warehouse", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/staff/logout", nil)
	csrf := strings.Repeat("a", 64)
	request.AddCookie(&http.Cookie{Name: staffCookieName, Value: token})
	request.AddCookie(&http.Cookie{Name: "mitte_csrf", Value: csrf})
	request.Header.Set("X-CSRF-Token", csrf)
	recorder := httptest.NewRecorder()

	app.staffLogout(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", recorder.Code)
	}
	if _, err := app.currentStaffUsername(httptest.NewRequest(http.MethodGet, "/", nil)); !errors.Is(err, store.ErrInvalidSession) {
		t.Fatalf("revoked session validation error = %v, want invalid session", err)
	}
	replay := httptest.NewRequest(http.MethodGet, "/", nil)
	replay.AddCookie(&http.Cookie{Name: staffCookieName, Value: token})
	if _, err := app.currentStaffUsername(replay); !errors.Is(err, store.ErrInvalidSession) {
		t.Fatalf("copied token validation error = %v, want invalid session", err)
	}
}

func TestCurrentStaffEnforcesRoleScope(t *testing.T) {
	sessions := &operatorSessionStub{principals: make(map[string]string)}
	app := &App{
		sessions:         sessions,
		staffCredentials: map[string]string{"warehouse": "a-long-secret-password"},
		staffRoles:       map[string]string{"warehouse": "warehouse"},
	}
	token, err := store.NewOperatorSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := sessions.CreateOperatorSession(context.Background(), token, "staff:warehouse", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/payments", nil)
	request.AddCookie(&http.Cookie{Name: staffCookieName, Value: token})
	recorder := httptest.NewRecorder()

	if _, ok := app.currentStaff(recorder, request); ok {
		t.Fatal("warehouse role accessed the payments area")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unauthorized role status = %d, want 403", recorder.Code)
	}
}
