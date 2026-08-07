package domain

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"
)

type WebhookDeliveryEndpoint struct {
	ID     string
	URL    string
	Secret string
}

func validateWebhookInput(data map[string]any) error {
	raw := stringField(data, "url")
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return errors.New("webhook URL must be an absolute HTTPS URL without credentials or fragment")
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errors.New("loopback webhook target forbidden")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
		return errors.New("non-public webhook target forbidden")
	}
	events, ok := data["events"].([]any)
	if !ok || len(events) == 0 {
		return errors.New("webhook events required")
	}
	seen := map[string]bool{}
	allowed := map[string]bool{"fiscal.operation.updated": true, "fiscal.operation.succeeded": true, "fiscal.operation.failed": true, "fiscal.operation.reconciliation_required": true, "payment.updated": true, "device.readiness.changed": true, "device.connectivity.changed": true, "edge.connectivity.changed": true, "register.report.completed": true, "ble.session.revoked": true}
	for _, item := range events {
		event, ok := item.(string)
		if !ok || !allowed[event] || seen[event] {
			return errors.New("unsupported or duplicate webhook event")
		}
		seen[event] = true
	}
	return nil
}

func generateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func publicWebhook(v ResourceRecord) map[string]any {
	out := map[string]any{
		"id": v.ID, "version": v.Version, "url": v.Data["url"], "events": v.Data["events"],
		"status": v.Data["status"], "created_at": v.CreatedAt, "updated_at": v.UpdatedAt,
	}
	return out
}

func (s *Service) CreateWebhookEndpoint(tenant string, data map[string]any) (map[string]any, error) {
	if tenant == "" || validateWebhookInput(data) != nil {
		return nil, errors.New("invalid webhook endpoint")
	}
	secret, err := generateWebhookSecret()
	if err != nil {
		return nil, err
	}
	id, err := newUUID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	record := ResourceRecord{Kind: "webhook_endpoint", TenantID: tenant, ID: id, Version: 1, CreatedAt: now, UpdatedAt: now, Data: map[string]any{
		"url": data["url"], "events": data["events"], "status": "ACTIVE", "secret": secret,
	}}
	if err = s.repo.PutResource(record); err != nil {
		return nil, err
	}
	out := publicWebhook(record)
	out["secret"] = secret // returned once; never returned by GET/PATCH
	return out, nil
}

func (s *Service) GetWebhookEndpoint(id, tenant string) (map[string]any, error) {
	v, err := s.repo.Resource("webhook_endpoint", id)
	if err != nil || v.TenantID != tenant {
		return nil, ErrNotFound
	}
	return publicWebhook(v), nil
}

func (s *Service) UpdateWebhookEndpoint(id, tenant string, expected int64, data map[string]any) (map[string]any, error) {
	v, err := s.repo.Resource("webhook_endpoint", id)
	if err != nil || v.TenantID != tenant {
		return nil, ErrNotFound
	}
	if v.Version != expected || validateWebhookInput(data) != nil || stringField(v.Data, "status") != "ACTIVE" {
		return nil, errors.New("webhook update rejected")
	}
	v.Data["url"], v.Data["events"] = data["url"], data["events"]
	v.Version++
	v.UpdatedAt = time.Now().UTC()
	if err = s.repo.PutResource(v); err != nil {
		return nil, err
	}
	return publicWebhook(v), nil
}

func (s *Service) DisableWebhookEndpoint(id, tenant string) error {
	v, err := s.repo.Resource("webhook_endpoint", id)
	if err != nil || v.TenantID != tenant {
		return ErrNotFound
	}
	if stringField(v.Data, "status") != "DISABLED" {
		v.Data["status"] = "DISABLED"
		v.Version++
		v.UpdatedAt = time.Now().UTC()
		return s.repo.PutResource(v)
	}
	return nil
}

func (s *Service) RotateWebhookSecret(id, tenant string) (map[string]any, error) {
	v, err := s.repo.Resource("webhook_endpoint", id)
	if err != nil || v.TenantID != tenant || stringField(v.Data, "status") != "ACTIVE" {
		return nil, ErrNotFound
	}
	secret, err := generateWebhookSecret()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	previousUntil := now.Add(24 * time.Hour)
	v.Data["previous_secret"], v.Data["previous_valid_until"] = v.Data["secret"], previousUntil.Format(time.RFC3339Nano)
	v.Data["secret"] = secret
	v.Version++
	v.UpdatedAt = now
	if err = s.repo.PutResource(v); err != nil {
		return nil, err
	}
	return map[string]any{"secret": secret, "valid_from": now, "previous_valid_until": previousUntil}, nil
}

func (s *Service) WebhookDeliveryEndpoints(tenant, event string) []WebhookDeliveryEndpoint {
	var out []WebhookDeliveryEndpoint
	for _, v := range s.repo.Resources("webhook_endpoint", tenant) {
		if stringField(v.Data, "status") != "ACTIVE" {
			continue
		}
		matched := false
		if events, ok := v.Data["events"].([]any); ok {
			for _, item := range events {
				matched = matched || item == event
			}
		}
		if matched {
			out = append(out, WebhookDeliveryEndpoint{ID: v.ID, URL: stringField(v.Data, "url"), Secret: stringField(v.Data, "secret")})
		}
	}
	return out
}
