package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type bindingCapture struct {
	tenant, adapter string
	beforePublish   func([]byte)
	envelope        []byte
}

func (c *bindingCapture) PublishCompositeBinding(tenant, adapter string, envelope []byte) error {
	if c.beforePublish != nil {
		c.beforePublish(envelope)
	}
	c.tenant, c.adapter, c.envelope = tenant, adapter, append([]byte(nil), envelope...)
	return nil
}

type bindingSigner struct{}

func (bindingSigner) Issue(DeviceActivationRequest) (DeviceCredential, error) {
	return DeviceCredential{}, nil
}
func (bindingSigner) SignCompositeBinding([]byte) (string, string, error) {
	return "signature", "binding-test", nil
}

func activateForComposite(t *testing.T, s *Service, kind, model, serial string) map[string]any {
	t.Helper()
	d, e := s.CreateResource("device", "tenant-1", map[string]any{"kind": kind, "vendor": "Datecs", "model": model, "serial": serial, "status": "DRAFT", "environment": "DEV", "simulated": true})
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.UpdateResource("device", d["id"].(string), "tenant-1", 1, map[string]any{"kind": kind, "vendor": "Datecs", "model": model, "serial": serial, "status": "PENDING_SERVICE_ACTIVATION", "environment": "DEV", "simulated": true})
	if e != nil {
		t.Fatal(e)
	}
	a, e := s.UpdateResource("device", d["id"].(string), "tenant-1", p["version"].(int64), map[string]any{"kind": kind, "vendor": "Datecs", "model": model, "serial": serial, "status": "ACTIVE", "environment": "DEV", "simulated": true})
	if e != nil {
		t.Fatal(e)
	}
	return a
}

func TestCompositeBindingDisableAllowsRebind(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	l, _ := s.CreateResource("location", "tenant-1", map[string]any{"code": "REB", "name": "Rebind", "address": "A", "status": "ACTIVE"})
	r, _ := s.CreateResource("register", "tenant-1", map[string]any{"location_id": l["id"], "code": "R2", "status": "ACTIVE"})
	a := activateForComposite(t, s, "SMART_DEVICE", "EDGE_AGENT_S3", "EDGE-R")
	f := activateForComposite(t, s, "FISCAL_DEVICE", "DP-150 MX", "F-R")
	p := activateForComposite(t, s, "PAYMENT_TERMINAL", "BLUEPAD-50 PLUS", "P-R")
	b, err := s.CreateCompositeBinding(r["id"].(string), "tenant-1", "DATECS_DP150_BLUEPAD50", a["id"].(string), f["id"].(string), p["id"].(string), r["version"].(int64))
	if err != nil {
		t.Fatal(err)
	}
	active, err := s.ApplyCompositeBinding(r["id"].(string), b["binding_id"].(string), "tenant-1", a["id"].(string), b["generation"].(int64))
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.CompositeBindings(r["id"].(string), "tenant-1")
	if err != nil || len(list) != 1 || list[0]["applied_generation"] != b["generation"] {
		t.Fatal(list, err)
	}
	disabled, err := s.DisableCompositeBinding(r["id"].(string), b["binding_id"].(string), "tenant-1", 2)
	if err != nil || disabled["status"] != "REVOKED" {
		t.Fatal(disabled, err)
	}
	current, _ := s.GetResource("register", r["id"].(string), "tenant-1")
	if current["fiscal_device_id"] != nil || current["payment_terminal_id"] != nil {
		t.Fatal(current)
	}
	if _, err = s.CreateCompositeBinding(r["id"].(string), "tenant-1", "DATECS_DP150_BLUEPAD50", a["id"].(string), f["id"].(string), p["id"].(string), current["version"].(int64)); err != nil {
		t.Fatal("rebind rejected", err)
	}
	_ = active
}

func TestCompositeBindingIsNotRoutableUntilExactAdapterAck(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	l, _ := s.CreateResource("location", "tenant-1", map[string]any{"code": "SOF", "name": "Sofia", "address": "A", "status": "ACTIVE"})
	r, _ := s.CreateResource("register", "tenant-1", map[string]any{"location_id": l["id"], "code": "R1", "status": "ACTIVE"})
	adapter := activateForComposite(t, s, "SMART_DEVICE", "EDGE_AGENT_S3", "EDGE-1")
	fiscal := activateForComposite(t, s, "FISCAL_DEVICE", "DP-150 MX", "F-1")
	pay := activateForComposite(t, s, "PAYMENT_TERMINAL", "BLUEPAD-50 PLUS", "P-1")
	b, e := s.CreateCompositeBinding(r["id"].(string), "tenant-1", "DATECS_DP150_BLUEPAD50", adapter["id"].(string), fiscal["id"].(string), pay["id"].(string), r["version"].(int64))
	if e != nil {
		t.Fatal(e)
	}
	before, _ := s.GetResource("register", r["id"].(string), "tenant-1")
	if before["fiscal_device_id"] != nil || before["payment_terminal_id"] != nil {
		t.Fatal("pending binding changed route")
	}
	if _, e = s.ApplyCompositeBinding(r["id"].(string), b["binding_id"].(string), "tenant-1", adapter["id"].(string), b["generation"].(int64)+1); e == nil {
		t.Fatal("wrong generation applied")
	}
	a, e := s.ApplyCompositeBinding(r["id"].(string), b["binding_id"].(string), "tenant-1", adapter["id"].(string), b["generation"].(int64))
	if e != nil || a["status"] != "ACTIVE" || a["applied_generation"] != b["generation"] {
		t.Fatal(a, e)
	}
	snapshot, snapshotErr := s.activeFiscalDeviceSnapshot(r["id"].(string), "tenant-1", time.Now().UTC())
	if snapshotErr != nil || snapshot.DeviceID != fiscal["id"] || snapshot.AdapterDeviceID != adapter["id"] || snapshot.BindingGeneration != b["generation"] {
		t.Fatalf("incorrect composite snapshot: %#v %v", snapshot, snapshotErr)
	}
	// An unrelated register edit must not advance the adapter's applied generation.
	storedRegister, _ := s.repo.Resource("register", r["id"].(string))
	storedRegister.Version++
	if err := s.repo.PutResource(storedRegister); err != nil {
		t.Fatal(err)
	}
	snapshot, snapshotErr = s.activeFiscalDeviceSnapshot(r["id"].(string), "tenant-1", time.Now().UTC())
	if snapshotErr != nil || snapshot.BindingGeneration != b["generation"] || snapshot.BindingVersion != storedRegister.Version {
		t.Fatal(snapshot, snapshotErr)
	}
	after, _ := s.GetResource("register", r["id"].(string), "tenant-1")
	if after["fiscal_device_id"] != fiscal["id"] || after["payment_terminal_id"] != pay["id"] || after["version"] != storedRegister.Version {
		t.Fatal(after)
	}
}

func TestCompositeBindingPublishesBFPEBeforeAdapterApplyAck(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	capture := &bindingCapture{}
	s.SetCompositeBindingPublisher(capture)
	s.SetDeviceCredentialIssuer(bindingSigner{})
	l, _ := s.CreateResource("location", "tenant-1", map[string]any{"code": "M", "name": "M", "address": "A", "status": "ACTIVE"})
	r, _ := s.CreateResource("register", "tenant-1", map[string]any{"location_id": l["id"], "code": "M1", "status": "ACTIVE"})
	adapter := activateForComposite(t, s, "SMART_DEVICE", "EDGE_AGENT_S3", "EDGE-M")
	fiscal := activateForComposite(t, s, "FISCAL_DEVICE", "DP-150 MX", "F-M")
	pay := activateForComposite(t, s, "PAYMENT_TERMINAL", "BLUEPAD-50 PLUS", "P-M")
	patch := func(device map[string]any, fields map[string]any) {
		stored, _ := s.repo.Resource("device", device["id"].(string))
		for key, value := range fields {
			stored.Data[key] = value
		}
		stored.Version++
		if err := s.repo.PutResource(stored); err != nil {
			t.Fatal(err)
		}
	}
	patch(adapter, map[string]any{"mqtt_uri": "ssl://mqtt.test:8883", "mqtt_client_id": adapter["id"], "ble_advertising_identity": "BeeFiscal-S3", "transaction_signing_kid": "kid-1", "unp_prefix": "BOARD001", "unp_range_start": int64(987), "unp_range_end": int64(999), "uart_tx_pin": int64(17), "uart_rx_pin": int64(18)})
	patch(fiscal, map[string]any{"fiscal_memory_number": "12345678", "fiscal_device_number": "DT000001", "execution": map[string]any{"operator_number": 1, "operator_count": 30, "operator_password_ref": "fiscal-pass", "device_configuration_id": "installation-1", "till_number": 1, "payment_mapping": map[string]any{"CASH": 0, "CARD": 1}}, "uart_baud": int64(115200), "uart_data_bits": int64(8), "uart_parity": "N", "uart_stop_bits": int64(1)})
	patch(pay, map[string]any{"ble_identity": "bluepad-1", "service_uuid": "service", "tx_characteristic_uuid": "tx", "rx_characteristic_uuid": "rx"})
	capture.beforePublish = func(envelope []byte) {
		keyLen := int(envelope[5])
		sigLen := int(envelope[6])<<8 | int(envelope[7])
		canonical := envelope[12+keyLen+sigLen:]
		h := sha256.Sum256(canonical)
		hash := hex.EncodeToString(h[:])
		snapshot, e := s.repo.Resource("issued_binding", hash)
		if e != nil || snapshot.TenantID != "tenant-1" || snapshot.Data["canonical_json"] != string(canonical) || snapshot.Data["payload_sha256"] != hash || snapshot.Data["signature"] != "signature" || snapshot.Data["key_id"] != "binding-test" {
			t.Fatalf("missing exact snapshot before publish: %#v %v", snapshot, e)
		}
		var payload map[string]any
		if json.Unmarshal(canonical, &payload) != nil || payload["schema_version"] != float64(3) {
			t.Fatal("not schema3")
		}
		if payload["mqtt"].(map[string]any)["uri"] != "mqtts://mqtt.test:8883" {
			t.Fatal("runtime MQTT URI mismatch")
		}
		authority := payload["operational_authority"].(map[string]any)
		if authority["unp_prefix"] != "DT000001" || authority["unp_range_start"] != float64(1) || authority["unp_range_end"] != float64(1000) { t.Fatalf("board/caller authority leaked: %#v", authority) }
		fiscal := payload["fiscal_endpoint"].(map[string]any)
		if fiscal["fiscal_device_number"] != "DT000001" || fiscal["fiscal_memory_number"] != "12345678" || fiscal["execution"] == nil {
			t.Fatal("missing signed execution identity")
		}
		roundtrip, _ := json.Marshal(snapshot.Data["canonical_payload"])
		if !bytes.Equal(roundtrip, canonical) {
			t.Fatal("snapshot map differs")
		}
		bindings := s.repo.Resources("composite_binding", "tenant-1")
		if len(bindings) != 1 || bindings[0].Data["payload_sha256"] != hash {
			t.Fatal("binding must reference durable snapshot before publish")
		}
	}
	_, err := s.CreateCompositeBinding(r["id"].(string), "tenant-1", "DATECS_DP150_BLUEPAD50", adapter["id"].(string), fiscal["id"].(string), pay["id"].(string), r["version"].(int64))
	if err != nil {
		t.Fatal(err)
	}
	if capture.tenant != "tenant-1" || capture.adapter != adapter["id"] || len(capture.envelope) < 12 || string(capture.envelope[:4]) != "BFPE" || capture.envelope[4] != 1 {
		t.Fatal("signed BFPE was not delivered", capture)
	}
}

type failBindingWrite struct {
	Repository
	kind string
}

func (r *failBindingWrite) PutResource(v ResourceRecord) error {
	if v.Kind == r.kind {
		return errors.New("injected durable write failure")
	}
	return r.Repository.PutResource(v)
}
func TestCompositeBindingNeverPublishesBeforeDurableEvidence(t *testing.T) {
	for _, kind := range []string{"issued_binding", "composite_binding"} {
		t.Run(kind, func(t *testing.T) {
			s := NewService(NewMemoryRepository(), NewSimulator(true))
			capture := &bindingCapture{}
			s.SetCompositeBindingPublisher(capture)
			s.SetDeviceCredentialIssuer(bindingSigner{})
			l, _ := s.CreateResource("location", "tenant-1", map[string]any{"code": "M", "name": "M", "address": "A", "status": "ACTIVE"})
			r, _ := s.CreateResource("register", "tenant-1", map[string]any{"location_id": l["id"], "code": "M1", "status": "ACTIVE"})
			adapter := activateForComposite(t, s, "SMART_DEVICE", "EDGE_AGENT_S3", "EDGE-M")
			fiscal := activateForComposite(t, s, "FISCAL_DEVICE", "DP-150 MX", "F-M")
			pay := activateForComposite(t, s, "PAYMENT_TERMINAL", "BLUEPAD-50 PLUS", "P-M")
			patch := func(device map[string]any, fields map[string]any) {
				stored, _ := s.repo.Resource("device", device["id"].(string))
				for key, value := range fields {
					stored.Data[key] = value
				}
				stored.Version++
				if err := s.repo.PutResource(stored); err != nil {
					t.Fatal(err)
				}
			}
			patch(adapter, map[string]any{"mqtt_uri": "ssl://mqtt.test:8883", "mqtt_client_id": adapter["id"], "ble_advertising_identity": "BeeFiscal-S3", "transaction_signing_kid": "kid-1", "unp_prefix": "BOARD001", "unp_range_start": int64(987), "unp_range_end": int64(999), "uart_tx_pin": int64(17), "uart_rx_pin": int64(18)})
			patch(fiscal, map[string]any{"fiscal_memory_number": "12345678", "fiscal_device_number": "DT000001", "execution": map[string]any{"operator_number": 1, "operator_count": 30, "operator_password_ref": "fiscal-pass", "device_configuration_id": "installation-1", "till_number": 1, "payment_mapping": map[string]any{"CASH": 0, "CARD": 1}}, "uart_baud": int64(115200), "uart_data_bits": int64(8), "uart_parity": "N", "uart_stop_bits": int64(1)})
			patch(pay, map[string]any{"ble_identity": "bluepad-1", "service_uuid": "service", "tx_characteristic_uuid": "tx", "rx_characteristic_uuid": "rx"})

			s.repo = &failBindingWrite{Repository: s.repo, kind: kind}
			if _, e := s.CreateCompositeBinding(r["id"].(string), "tenant-1", "DATECS_DP150_BLUEPAD50", adapter["id"].(string), fiscal["id"].(string), pay["id"].(string), r["version"].(int64)); e == nil {
				t.Fatal("storage failure accepted")
			}
			if len(capture.envelope) != 0 {
				t.Fatal("publish occurred despite durable write failure")
			}
		})
	}
}
