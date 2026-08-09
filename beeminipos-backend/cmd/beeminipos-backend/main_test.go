package main

import (
	"net/http"
	"testing"
	"time"
)

func TestHTTPServerHasBoundedResourceTimeouts(t *testing.T) {
	server := newHTTPServer(":0", http.NotFoundHandler())
	if server.ReadHeaderTimeout != 5*time.Second || server.ReadTimeout != 15*time.Second || server.WriteTimeout != 30*time.Second || server.IdleTimeout != 60*time.Second {
		t.Fatalf("unsafe HTTP timeout policy: header=%s read=%s write=%s idle=%s", server.ReadHeaderTimeout, server.ReadTimeout, server.WriteTimeout, server.IdleTimeout)
	}
}
