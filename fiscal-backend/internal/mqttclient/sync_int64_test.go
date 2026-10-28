package mqttclient

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fiscalisation/fiscal-backend/internal/domain"
	"testing"
	"time"
)

type int64Store struct{ data []byte }

func (s *int64Store) Load() ([]byte, error) { return append([]byte(nil), s.data...), nil }
func (s *int64Store) Save(b []byte) error   { s.data = append([]byte(nil), b...); return nil }
func signedIntegerBatch(t *testing.T) (*domain.Service, *domain.MemoryRepository, *int64Store, []byte) {
	t.Helper()
	store := &int64Store{}
	repo, err := domain.NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err = repo.PutResource(domain.ResourceRecord{ID: "device", Kind: "device", TenantID: "tenant", Version: 1, Data: map[string]any{"transaction_signing_public_key": base64.RawURLEncoding.EncodeToString(der), "transaction_signing_kid": "kid"}}); err != nil {
		t.Fatal(err)
	}
	sign := func(hash string) string {
		raw, _ := hex.DecodeString(hash)
		digest := sha256.Sum256(raw)
		r, s, e := ecdsa.Sign(rand.Reader, key, digest[:])
		if e != nil {
			t.Fatal(e)
		}
		signature := make([]byte, 64)
		r.FillBytes(signature[:32])
		s.FillBytes(signature[32:])
		return "kid:" + base64.RawURLEncoding.EncodeToString(signature)
	}
	event := domain.DeviceEventEnvelope{EventID: "event", OperationID: "operation", DeviceID: "device", EventType: "ACCEPTED", JournalSeq: 1, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Payload: map[string]any{"operation_sequence": int64(9007199254740993), "unp_sequence": int64(0), "command": map[string]any{"operation_id": "operation", "tenant_id": "tenant", "register_id": "register", "device_id": "device", "command_type": "REPORT_X", "payload": map[string]any{"metadata": map[string]any{"max": int64(9223372036854775807), "min": int64(-9223372036854775808), "over_binary64": int64(9007199254740993), "array": []any{int64(9007199254740995), map[string]any{"nested": int64(-9007199254740993)}}}}}}}
	event.EventHash = domain.DeviceEventHash(event)
	sig := sign(event.EventHash)
	event.Signature = &sig
	batch := domain.EdgeSyncBatch{EdgeID: "device", SchemaVersion: "2026-08-07", FirstSeq: 1, LastSeq: 1, Events: []domain.DeviceEventEnvelope{event}}
	batch.BatchSHA256 = domain.EdgeBatchHash(batch)
	batch.Signature = sign(batch.BatchSHA256)
	body, _ := json.Marshal(batch)
	svc := domain.NewService(repo, nil)
	svc.SetBLESigningKey("01234567890123456789012345678901")
	svc.SetRequireHardwareSyncSignatures(true)
	return svc, repo, store, body
}
func assertStoredIntegers(t *testing.T, repo *domain.MemoryRepository) {
	t.Helper()
	pending, err := repo.EdgePendingCommand("operation", "tenant")
	if err != nil {
		t.Fatal(err)
	}
	if pending.OperationSequence != 9007199254740993 {
		t.Fatal("operation sequence rounded", pending.OperationSequence)
	}
	b, _ := json.Marshal(pending.Payload)
	for _, exact := range []string{"9223372036854775807", "-9223372036854775808", "9007199254740993", "9007199254740995", "-9007199254740993"} {
		if !bytes.Contains(b, []byte(exact)) {
			t.Fatalf("nested integer %s lost: %s", exact, b)
		}
	}
}
func TestMQTTSignedNestedInt64ActualBackendAndRestart(t *testing.T) {
	svc, repo, store, body := signedIntegerBatch(t)
	p := NewProcessor(svc)
	topic, ack, err := p.Process("tenants/tenant/devices/device/sync/batches/batch", append(append([]byte(nil), body...), []byte(" \n\t")...))
	if err != nil {
		t.Fatal(err)
	}
	if topic != "tenants/tenant/devices/device/sync/acks/batch" || len(ack) == 0 {
		t.Fatal(topic, string(ack))
	}
	assertStoredIntegers(t, repo)
	restarted, err := domain.NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	assertStoredIntegers(t, restarted)
	if _, _, err = p.Process("tenants/tenant/devices/device/sync/batches/batch", body); err != nil {
		t.Fatal("exact replay", err)
	}
}
func TestMQTTSyncRejectsTrailingJSONBeforeBackendMutation(t *testing.T) {
	for _, suffix := range []string{` {}`, ` []`, ` null`, ` garbage`} {
		t.Run(suffix, func(t *testing.T) {
			svc, repo, _, body := signedIntegerBatch(t)
			body = append(body, []byte(suffix)...)
			if _, _, err := NewProcessor(svc).Process("tenants/tenant/devices/device/sync/batches/batch", body); err == nil {
				t.Fatal("trailing content accepted")
			}
			if _, ok := repo.LastSyncAck("tenant", "device"); ok {
				t.Fatal("invalid payload committed")
			}
		})
	}
}
