package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersAllowConfiguredCatalogMediaAndFonts(t *testing.T) {
	app := &App{}
	handler := app.securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	policy := recorder.Header().Get("Content-Security-Policy")
	for _, directive := range []string{
		"img-src 'self' https://images.unsplash.com https://images.openbeautyfacts.org",
		"style-src 'self' https://fonts.googleapis.com",
		"font-src 'self' https://fonts.gstatic.com",
		"script-src 'self'",
		"frame-ancestors 'none'",
	} {
		if !strings.Contains(policy, directive) {
			t.Errorf("Content-Security-Policy %q does not contain %q", policy, directive)
		}
	}
}
