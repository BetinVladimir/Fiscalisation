package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	webhooksafe "fiscalisation/fiscal-backend/internal/webhook"
	"github.com/rabbitmq/amqp091-go"
)

type ApplyResource func(ctx context.Context, tenantID, systemID, method, resourceType, sourceID string, sourceVersion int64, payload map[string]any) (map[string]any, error)

const integrationExchange = "beefiscal.integration"
const commandQueue = "beefiscal.integration.commands"
const webhookQueue = "beefiscal.integration.webhooks"

func (s *Service) RunRabbit(ctx context.Context, url string, apply ApplyResource) {
	if url == "" {
		return
	}
	for {
		if ctx.Err() != nil {
			return
		}
		if e := s.rabbitSession(ctx, url, apply); e != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
}
func (s *Service) rabbitSession(ctx context.Context, url string, apply ApplyResource) error {
	conn, e := amqp091.Dial(url)
	if e != nil {
		return e
	}
	defer conn.Close()
	ch, e := conn.Channel()
	if e != nil {
		return e
	}
	defer ch.Close()
	if e = ch.ExchangeDeclare(integrationExchange, "topic", true, false, false, false, nil); e != nil {
		return e
	}
	if _, e = ch.QueueDeclare(commandQueue, true, false, false, false, amqp091.Table{"x-dead-letter-exchange": integrationExchange}); e != nil {
		return e
	}
	if e = ch.QueueBind(commandQueue, "command.#", integrationExchange, false, nil); e != nil {
		return e
	}
	if _, e = ch.QueueDeclare(webhookQueue, true, false, false, false, nil); e != nil {
		return e
	}
	if e = ch.QueueBind(webhookQueue, "webhook.deliver", integrationExchange, false, nil); e != nil {
		return e
	}
	if e = ch.Confirm(false); e != nil {
		return e
	}
	commands, e := ch.Consume(commandQueue, "", false, false, false, false, nil)
	if e != nil {
		return e
	}
	webhooks, e := ch.Consume(webhookQueue, "", false, false, false, false, nil)
	if e != nil {
		return e
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-commands:
			if !ok {
				return errors.New("command consumer closed")
			}
			var v struct {
				CommandID string `json:"command_id"`
			}
			if json.Unmarshal(d.Body, &v) != nil || v.CommandID == "" {
				_ = d.Nack(false, false)
				continue
			}
			if e = s.processCommand(ctx, v.CommandID, apply); e != nil {
				var dead bool
				_ = s.db.QueryRowContext(ctx, `update integration_commands set status=case when attempts>=5 then 'DEAD' else 'QUEUED' end,processing_started_at=null,last_error_code='WORKER_ERROR',last_error_detail=$2,updated_at=now() where id=$1 returning status='DEAD'`, v.CommandID, e.Error()).Scan(&dead)
				if dead {
					_ = s.finishDeadCommand(ctx, v.CommandID, e)
					_ = d.Ack(false)
				} else {
					_ = d.Nack(false, true)
				}
			} else {
				_ = d.Ack(false)
			}
		case d, ok := <-webhooks:
			if !ok {
				return errors.New("webhook consumer closed")
			}
			var v struct {
				DeliveryID string `json:"delivery_id"`
			}
			if json.Unmarshal(d.Body, &v) != nil || v.DeliveryID == "" {
				_ = d.Nack(false, false)
				continue
			}
			_ = s.sendWebhook(ctx, v.DeliveryID)
			_ = d.Ack(false)
		case <-ticker.C:
			if e = s.publishCommandBatch(ctx, ch); e != nil {
				return e
			}
			if e = s.publishWebhookBatch(ctx, ch); e != nil {
				return e
			}
		}
	}
}

func (s *Service) finishDeadCommand(ctx context.Context, id string, workerErr error) error {
	var tenant, system, method, resource, source string
	var version int64
	e := s.db.QueryRowContext(ctx, `select tenant_id::text,external_system_id::text,http_method,resource_type,aggregate_source_id,source_version from integration_commands where id=$1 and status='DEAD'`, id).Scan(&tenant, &system, &method, &resource, &source, &version)
	if e != nil {
		return e
	}
	return s.finishCommand(ctx, id, tenant, system, method, resource, source, version, "DEAD", nil, workerErr)
}
func publishConfirmed(ctx context.Context, ch *amqp091.Channel, routing string, body []byte) error {
	dc, dcancel := context.WithTimeout(ctx, 5*time.Second)
	defer dcancel()
	confirm, e := ch.PublishWithDeferredConfirmWithContext(dc, integrationExchange, routing, false, false, amqp091.Publishing{ContentType: "application/json", DeliveryMode: amqp091.Persistent, MessageId: routing + ":" + hex.EncodeToString(sha256Bytes(body))[:16], Timestamp: time.Now().UTC(), Body: body})
	if e != nil {
		return e
	}
	ok, e := confirm.WaitContext(dc)
	if e != nil {
		return e
	}
	if !ok {
		return errors.New("Rabbit publisher nack")
	}
	return nil
}
func sha256Bytes(v []byte) []byte { h := sha256.Sum256(v); return h[:] }
func (s *Service) publishCommandBatch(ctx context.Context, ch *amqp091.Channel) error {
	rows, e := s.db.QueryContext(ctx, `with picked as (select id from integration_command_outbox where ((status in ('PENDING','FAILED') and available_at<=now()) or (status='LEASED' and lease_until<now())) order by available_at,id for update skip locked limit 50) update integration_command_outbox o set status='LEASED',lease_id=gen_random_uuid(),lease_until=now()+interval '30 seconds',updated_at=now() from picked where o.id=picked.id returning o.id::text,o.command_id::text,o.topic,o.payload`)
	if e != nil {
		return e
	}
	defer rows.Close()
	type item struct {
		id, command, topic string
		payload            []byte
	}
	var items []item
	for rows.Next() {
		var v item
		if e = rows.Scan(&v.id, &v.command, &v.topic, &v.payload); e != nil {
			return e
		}
		items = append(items, v)
	}
	for _, v := range items {
		body := mapJSON(map[string]string{"command_id": v.command})
		if e = publishConfirmed(ctx, ch, "command."+v.topic, body); e != nil {
			_, _ = s.db.ExecContext(ctx, `update integration_command_outbox set status='FAILED',attempts=attempts+1,available_at=now()+interval '5 seconds',lease_id=null,lease_until=null,last_error=$2,updated_at=now() where id=$1`, v.id, e.Error())
			continue
		}
		_, e = s.db.ExecContext(ctx, `update integration_command_outbox set status='PUBLISHED',published_at=now(),lease_id=null,lease_until=null,last_error=null,updated_at=now() where id=$1`, v.id)
		if e != nil {
			return e
		}
		_, _ = s.db.ExecContext(ctx, `update integration_commands set status='QUEUED',updated_at=now() where id=$1 and status='ACCEPTED'`, v.command)
	}
	return rows.Err()
}
func (s *Service) processCommand(ctx context.Context, id string, apply ApplyResource) error {
	var tenant, system, method, resource, source, status string
	var version int64
	var payload []byte
	e := s.db.QueryRowContext(ctx, `update integration_commands set status='PROCESSING',attempts=attempts+1,processing_started_at=now(),updated_at=now() where id=$1 and (status in ('ACCEPTED','QUEUED','FAILED') or (status='PROCESSING' and processing_started_at<now()-interval '2 minutes')) returning tenant_id::text,external_system_id::text,http_method,resource_type,aggregate_source_id,source_version,payload,status`, id).Scan(&tenant, &system, &method, &resource, &source, &version, &payload, &status)
	if errors.Is(e, sql.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	var newer bool
	e = s.db.QueryRowContext(ctx, `select exists(select 1 from integration_commands where tenant_id=$1 and external_system_id=$2 and resource_type=$3 and aggregate_source_id=$4 and source_version>$5 and status in ('PROCESSING','SUCCEEDED'))`, tenant, system, resource, source, version).Scan(&newer)
	if e != nil {
		return e
	}
	outcome := "SUCCEEDED"
	var result map[string]any
	var applyErr error
	if newer {
		outcome = "SUPERSEDED"
	} else {
		var doc map[string]any
		if json.Unmarshal(payload, &doc) != nil {
			applyErr = errors.New("invalid command payload")
		} else if apply != nil {
			result, applyErr = apply(ctx, tenant, system, method, resource, source, version, doc)
		} else {
			result = doc
		}
	}
	if applyErr != nil {
		outcome = "FAILED"
	}
	return s.finishCommand(ctx, id, tenant, system, method, resource, source, version, outcome, result, applyErr)
}
func (s *Service) finishCommand(ctx context.Context, id, tenant, system, method, resource, source string, version int64, status string, result map[string]any, applyErr error) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var code, detail any
	if applyErr != nil {
		code = "RESOURCE_APPLY_FAILED"
		detail = applyErr.Error()
	}
	resultJSON := mapJSON(result)
	if status == "SUCCEEDED" {
		if projection, ok := result["_projection_record"]; ok {
			projectionJSON := string(mapJSON(projection))
			_, e = tx.ExecContext(ctx, `insert into fiscal_state_rows(collection,entity_key,payload,updated_at) values('resources',($1::jsonb->>'kind')||':'||($1::jsonb->>'id'),$1::jsonb,now()) on conflict(collection,entity_key) do update set payload=excluded.payload,updated_at=now() where (fiscal_state_rows.payload->>'version')::bigint<(excluded.payload->>'version')::bigint`, projectionJSON)
			if e != nil {
				return e
			}
			_, e = tx.ExecContext(ctx, `insert into fiscal_runtime_resources(kind,id,tenant_id,version,data,created_at,updated_at,payload) values($1::jsonb->>'kind',$1::jsonb->>'id',$1::jsonb->>'tenant_id',($1::jsonb->>'version')::bigint,$1::jsonb->'data',($1::jsonb->>'created_at')::timestamptz,($1::jsonb->>'updated_at')::timestamptz,$1::jsonb) on conflict(kind,id) do update set tenant_id=excluded.tenant_id,version=excluded.version,data=excluded.data,updated_at=excluded.updated_at,payload=excluded.payload where fiscal_runtime_resources.version<excluded.version`, projectionJSON)
			if e != nil {
				return e
			}
			if resource == "organization" {
				_, e = tx.ExecContext(ctx, `update tenant_source_bindings set tax_country=$1::jsonb->'data'->>'country',tax_type=$1::jsonb->'data'->>'tax_identifier_type',tax_normalized_value=$1::jsonb->'data'->>'tax_identifier_normalized',version=version+1,updated_at=now() where tenant_id=$1::jsonb->>'tenant_id' and external_system_id=$2 and (tax_country,tax_type,tax_normalized_value) is distinct from (($1::jsonb->'data'->>'country')::char(2),$1::jsonb->'data'->>'tax_identifier_type',$1::jsonb->'data'->>'tax_identifier_normalized')`, projectionJSON, system)
				if e != nil {
					return e
				}
			}
			delete(result, "_projection_record")
			resultJSON = mapJSON(result)
		}
		resourceStatus := "ACTIVE"
		if method == http.MethodDelete {
			resourceStatus = "INACTIVE"
		}
		var resourceID string
		var resourceVersion int64
		e = tx.QueryRowContext(ctx, `insert into integration_resources(tenant_id,external_system_id,resource_type,source_entity_id,source_version,payload,status) values($1,$2,$3,$4,$5,$6,$7) on conflict(tenant_id,external_system_id,resource_type,source_entity_id) do update set source_version=excluded.source_version,payload=excluded.payload,status=excluded.status,updated_at=now() where integration_resources.source_version<excluded.source_version returning id::text,source_version`, tenant, system, resource, source, version, resultJSON, resourceStatus).Scan(&resourceID, &resourceVersion)
		if errors.Is(e, sql.ErrNoRows) {
			status = "SUPERSEDED"
			e = nil
		} else if e == nil {
			result = map[string]any{"id": resourceID, "version": resourceVersion, "data": result, "status": resourceStatus}
			resultJSON = mapJSON(result)
		}
		if e != nil {
			return e
		}
	}
	_, e = tx.ExecContext(ctx, `update integration_commands set status=$2,result=$3,last_error_code=$4,last_error_detail=$5,processing_started_at=null,updated_at=now() where id=$1`, id, status, resultJSON, code, detail)
	if e != nil {
		return e
	}
	eventID, _ := uuid()
	payload := mapJSON(map[string]any{"event_id": eventID, "event_type": "integration.command.updated", "source_system_id": system, "tenant_id": tenant, "operation_id": id, "resource_type": resource, "source_entity_id": source, "source_version": version, "status": status, "result": result, "error": func() any {
		if applyErr == nil {
			return nil
		}
		return map[string]string{"code": "RESOURCE_APPLY_FAILED", "message": applyErr.Error()}
	}(), "occurred_at": s.now()})
	hash := sha256.Sum256(payload)
	_, e = tx.ExecContext(ctx, `insert into webhook_deliveries(event_id,external_system_id,tenant_id,event_type,payload,payload_hash,status,destination_url,signing_secret_ciphertext,signing_key_id) select $1,$2,$3,'integration.command.updated',$4,$5,'PENDING',webhook_url,webhook_signing_secret_ciphertext,id::text from external_systems where id=$2`, eventID, system, tenant, payload, hash[:])
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `insert into integration_change_journal(tenant_id,external_system_id,authenticated_system_id,operation_id,resource_type,source_entity_id,action,outcome,after_redacted,reason_code) select tenant_id,external_system_id,authenticated_system_id,id,resource_type,aggregate_source_id,http_method,$2,$3,$4 from integration_commands where id=$1`, id, status, resultJSON, code)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) publishWebhookBatch(ctx context.Context, ch *amqp091.Channel) error {
	rows, e := s.db.QueryContext(ctx, `with picked as (select id from webhook_deliveries where ((status in ('PENDING','RETRY') and next_attempt_at<=now()) or (status in ('LEASED','QUEUED','DELIVERING') and lease_until<now())) and attempts<5 order by next_attempt_at,id for update skip locked limit 50) update webhook_deliveries d set status='LEASED',lease_id=gen_random_uuid(),lease_until=now()+interval '30 seconds',updated_at=now() from picked where d.id=picked.id returning d.id::text`)
	if e != nil {
		return e
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return e
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		body := mapJSON(map[string]string{"delivery_id": id})
		if e = publishConfirmed(ctx, ch, "webhook.deliver", body); e != nil {
			_, _ = s.db.ExecContext(ctx, `update webhook_deliveries set status='RETRY',lease_id=null,lease_until=null,next_attempt_at=now()+interval '5 seconds',last_error_detail=$2 where id=$1`, id, e.Error())
			continue
		}
		_, e = s.db.ExecContext(ctx, `update webhook_deliveries set status='QUEUED',lease_id=null,lease_until=null,updated_at=now() where id=$1`, id)
		if e != nil {
			return e
		}
	}
	return rows.Err()
}
func (s *Service) sendWebhook(ctx context.Context, id string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var eventID, system, url string
	var payload, encrypted []byte
	var attempts int
	e = tx.QueryRowContext(ctx, `select d.event_id::text,d.external_system_id::text,coalesce(d.destination_url,s.webhook_url),d.payload,coalesce(d.signing_secret_ciphertext,s.webhook_signing_secret_ciphertext),d.attempts from webhook_deliveries d join external_systems s on s.id=d.external_system_id where d.id=$1 and d.status='QUEUED' for update`, id).Scan(&eventID, &system, &url, &payload, &encrypted, &attempts)
	if errors.Is(e, sql.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `update webhook_deliveries set status='DELIVERING',lease_until=now()+interval '30 seconds',updated_at=now() where id=$1`, id)
	if e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	secret, e := s.decrypt(encrypted)
	if e != nil {
		return e
	}
	timestamp := strconv.FormatInt(s.now().Unix(), 10)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(timestamp + "."))
	mac.Write(payload)
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if e == nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("BeeFiscal-Event-Id", eventID)
		req.Header.Set("BeeFiscal-Source-System-Id", system)
		req.Header.Set("BeeFiscal-Delivery-Id", id)
		req.Header.Set("BeeFiscal-Signature", fmt.Sprintf("t=%s,kid=%s,v1=%s", timestamp, system, hex.EncodeToString(mac.Sum(nil))))
	}
	var status int
	if e == nil {
		client := webhooksafe.SafeHTTPClient()
		var resp *http.Response
		resp, e = client.Do(req)
		if resp != nil {
			status = resp.StatusCode
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			if status >= 200 && status < 300 {
				e = nil
			} else {
				e = fmt.Errorf("webhook HTTP %d", status)
			}
		}
	}
	if e == nil {
		tx, e := s.db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		defer tx.Rollback()
		_, e = tx.ExecContext(ctx, `update webhook_deliveries set status='DELIVERED',attempts=attempts+1,delivered_at=now(),lease_until=null,last_http_status=$2,last_error_code=null,last_error_detail=null,updated_at=now() where id=$1`, id, status)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `insert into webhook_delivery_attempts(delivery_id,attempt_number,outcome,destination_url,http_status) values($1,$2,'DELIVERED',$3,$4) on conflict(delivery_id,attempt_number) do nothing`, id, attempts+1, url, status)
		if e != nil {
			return e
		}
		e = tx.Commit()
		return e
	}
	attempts++
	next := []time.Duration{30 * time.Second, 5 * time.Minute, 30 * time.Minute, 6 * time.Hour, 6 * time.Hour}[min(attempts-1, 4)]
	state := "RETRY"
	if attempts >= 5 {
		state = "DEAD"
	}
	dbtx, dbErr := s.db.BeginTx(ctx, nil)
	if dbErr != nil {
		return dbErr
	}
	defer dbtx.Rollback()
	_, dbErr = dbtx.ExecContext(ctx, `update webhook_deliveries set status=$2,attempts=$3,next_attempt_at=$4,lease_until=null,last_http_status=$5,last_error_code='DELIVERY_FAILED',last_error_detail=$6,updated_at=now() where id=$1`, id, state, attempts, s.now().Add(next), status, e.Error())
	if dbErr != nil {
		return dbErr
	}
	_, dbErr = dbtx.ExecContext(ctx, `insert into webhook_delivery_attempts(delivery_id,attempt_number,outcome,destination_url,http_status,error_code,error_detail) values($1,$2,'FAILED',$3,$4,'DELIVERY_FAILED',$5) on conflict(delivery_id,attempt_number) do nothing`, id, attempts, url, status, e.Error())
	if dbErr != nil {
		return dbErr
	}
	return dbtx.Commit()
}
