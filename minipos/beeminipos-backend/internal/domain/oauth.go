package domain

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AccessTokenProvider interface {
	Token(context.Context) (string, error)
	Invalidate(string)
}

type ClientCredentialsProvider struct {
	mu                                                sync.Mutex
	client                                            *http.Client
	tokenURL, clientID, clientSecret, scope, audience string
	token                                             string
	expires                                           time.Time
}

func NewClientCredentialsProvider(tokenURL, clientID, clientSecret, scope, audience string, client *http.Client) *ClientCredentialsProvider {
	if tokenURL == "" || clientID == "" || clientSecret == "" || scope == "" {
		return nil
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &ClientCredentialsProvider{client: client, tokenURL: tokenURL, clientID: clientID, clientSecret: clientSecret, scope: scope, audience: audience}
}
func (p *ClientCredentialsProvider) Token(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Now().Add(30*time.Second).Before(p.expires) {
		return p.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "scope": {p.scope}}
	if p.audience != "" {
		form.Set("audience", p.audience)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(p.clientID, p.clientSecret)
	resp, err := p.client.Do(req)
	if err != nil {
		return "", errors.New("oauth token endpoint unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("oauth token status " + strconv.Itoa(resp.StatusCode))
	}
	var wire struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if json.NewDecoder(resp.Body).Decode(&wire) != nil || wire.AccessToken == "" || !strings.EqualFold(wire.TokenType, "Bearer") || wire.ExpiresIn < 60 || wire.ExpiresIn > 86400 {
		return "", errors.New("invalid oauth token response")
	}
	p.token, p.expires = wire.AccessToken, time.Now().Add(time.Duration(wire.ExpiresIn)*time.Second)
	return p.token, nil
}
func (p *ClientCredentialsProvider) Invalidate(token string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token == token {
		p.token = ""
		p.expires = time.Time{}
	}
}

type staticTokenProvider string

func (p staticTokenProvider) Token(context.Context) (string, error) { return string(p), nil }
func (p staticTokenProvider) Invalidate(string)                     {}
