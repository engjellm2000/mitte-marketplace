package web

import (
	"net"
	"net/http/httptest"
	"testing"
)

func mustCIDR(t *testing.T, value string) *net.IPNet {
	t.Helper()
	_, network, err := net.ParseCIDR(value)
	if err != nil {
		t.Fatal(err)
	}
	return network
}

func TestClientIPOnlyTrustsForwardedHeadersFromConfiguredProxy(t *testing.T) {
	app := &App{trustedProxyCIDRs: []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}}

	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "203.0.113.10:4000"
	request.Header.Set("X-Forwarded-For", "198.51.100.7")
	if got := app.clientIP(request); got != "203.0.113.10" {
		t.Fatalf("clientIP() = %q, want direct peer when proxy is untrusted", got)
	}

	request.RemoteAddr = "10.1.2.3:4000"
	request.Header.Set("X-Forwarded-For", "192.0.2.44, 10.2.3.4")
	if got := app.clientIP(request); got != "192.0.2.44" {
		t.Fatalf("clientIP() = %q, want first untrusted address in proxy chain", got)
	}
}

func TestClientIPIgnoresMalformedForwardedChain(t *testing.T) {
	app := &App{trustedProxyCIDRs: []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}}
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "10.1.2.3:4000"
	request.Header.Set("X-Forwarded-For", "198.51.100.7, invalid")

	if got := app.clientIP(request); got != "10.1.2.3" {
		t.Fatalf("clientIP() = %q, want trusted proxy peer for malformed chain", got)
	}
}
