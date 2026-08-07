package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fiscalisation/fiscal-backend/internal/domain"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Dispatcher struct {
	svc    *domain.Service
	target string
	key    []byte
	client *http.Client
	now    func() time.Time
}

var errEndpointGone = errors.New("webhook endpoint gone")

func New(s *domain.Service, target, key string) *Dispatcher {
	return &Dispatcher{svc: s, target: target, key: []byte(key), client: safeHTTPClient(), now: time.Now}
}
func (d *Dispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = d.DeliverOnce(ctx)
		}
	}
}
func (d *Dispatcher) DeliverOnce(ctx context.Context) error {
	var first error
	for _, item := range d.svc.PendingOutbox(d.now().UTC()) {
		endpoints := d.svc.WebhookDeliveryEndpoints(item.Event.TenantID, item.Event.EventType)
		if d.target != "" { // compatibility endpoint configured during migration
			endpoints = append(endpoints, domain.WebhookDeliveryEndpoint{ID: "legacy", URL: d.target, Secret: string(d.key)})
		}
		if len(endpoints) == 0 {
			continue
		}
		if item.Deliveries == nil {
			item.Deliveries = map[string]domain.OutboxDelivery{}
		}
		b, marshalErr := json.Marshal(item.Event)
		allDelivered := true
		for _, endpoint := range endpoints {
			state := item.Deliveries[endpoint.ID]
			if state.DeliveredAt != nil {
				continue
			}
			if !state.NextAttempt.IsZero() && d.now().UTC().Before(state.NextAttempt) {
				allDelivered = false
				continue
			}
			deliveryErr := marshalErr
			if deliveryErr == nil {
				deliveryErr = d.deliver(ctx, endpoint, b)
			}
			state.Attempts++
			if errors.Is(deliveryErr, errEndpointGone) {
				if endpoint.ID == "legacy" {
					deliveryErr = nil
				} else {
					deliveryErr = d.svc.DisableWebhookEndpoint(endpoint.ID, item.Event.TenantID)
				}
			}
			if deliveryErr == nil {
				now := d.now().UTC()
				state.DeliveredAt = &now
			} else {
				state.NextAttempt = d.now().UTC().Add(retryDelay(state.Attempts))
				allDelivered = false
				if first == nil {
					first = deliveryErr
				}
			}
			item.Deliveries[endpoint.ID] = state
		}
		item.Attempts++
		if allDelivered && marshalErr == nil {
			now := d.now().UTC()
			item.DeliveredAt = &now
		} else {
			item.NextAttempt = d.now().UTC().Add(retryDelay(item.Attempts))
			if first == nil {
				first = marshalErr
			}
		}
		if saveErr := d.svc.UpdateOutbox(item); saveErr != nil && first == nil {
			first = saveErr
		}
	}
	return first
}
func (d *Dispatcher) deliver(ctx context.Context, endpoint domain.WebhookDeliveryEndpoint, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	timestamp := strconv.FormatInt(d.now().UTC().Unix(), 10)
	m := hmac.New(sha256.New, []byte(endpoint.Secret))
	m.Write([]byte(timestamp + "."))
	m.Write(body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("BeeFiscal-Event-Id", stringValue(body, "event_id"))
	req.Header.Set("BeeFiscal-Signature", fmt.Sprintf("t=%s,kid=%s,v1=%s", timestamp, endpoint.ID, hex.EncodeToString(m.Sum(nil))))
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusGone {
			return errEndpointGone
		}
		return errors.New("webhook rejected")
	}
	return nil
}

func stringValue(body []byte, key string) string {
	var v map[string]any
	_ = json.Unmarshal(body, &v)
	x, _ := v[key].(string)
	return x
}
func retryDelay(attempt int) time.Duration {
	delays := [...]time.Duration{10 * time.Second, 30 * time.Second, 2 * time.Minute, 10 * time.Minute, time.Hour, 6 * time.Hour, 24 * time.Hour}
	if attempt < 1 {
		attempt = 1
	}
	if attempt > len(delays) {
		attempt = len(delays)
	}
	return delays[attempt-1]
}
