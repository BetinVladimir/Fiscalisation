package webhook

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"time"
)

func safeHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil || len(resolved) == 0 {
				return nil, errors.New("webhook DNS resolution failed")
			}
			for _, candidate := range resolved {
				ip := candidate.IP
				if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
					return nil, errors.New("webhook DNS resolved to a non-public address")
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(resolved[0].IP.String(), port))
		},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
