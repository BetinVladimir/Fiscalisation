package webhook

import (
	"net"
	"net/http"
	"testing"
)

func TestSafeHTTPClientDisablesProxyAndRedirects(t *testing.T) {
	c := safeHTTPClient()
	tr, ok := c.Transport.(*http.Transport)
	if !ok || tr.Proxy != nil || tr.DialContext == nil || c.CheckRedirect == nil {
		t.Fatal("unsafe webhook client configuration")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.test/next", nil)
	if err := c.CheckRedirect(req, nil); err != http.ErrUseLastResponse {
		t.Fatalf("redirect must be rejected, got %v", err)
	}
}

func TestLiteralPrivateAddressClassification(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.1.1", "::1"} {
		ip := net.ParseIP(raw)
		if ip == nil || !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
			t.Fatalf("test address not classified private: %s", raw)
		}
	}
}
