package fiscalintegration

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type denialRoundTrip func(*http.Request) (*http.Response, error)

func (f denialRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestClientPreservesAuthoritativeHTTPStatus(t *testing.T) {
	for _, status := range []int{401, 403, 409, 503} {
		c := &Client{base: "https://fiscal.example", http: &http.Client{Transport: denialRoundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"error":"denied"}`)), Header: make(http.Header)}, nil
		})}}
		err := c.call(context.Background(), http.MethodPost, "/integration/v1/execution-authorizations", "source-credential", "request", nil, map[string]any{}, nil)
		var upstream *HTTPStatusError
		if !errors.As(err, &upstream) || upstream.StatusCode != status {
			t.Fatal(status, err)
		}
	}
}
