package domain

import "testing"

type bindingCapture struct {
	tenant, adapter string
	envelope        []byte
}

func (c *bindingCapture) PublishCompositeBinding(tenant, adapter string, envelope []byte) error {
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
	after, _ := s.GetResource("register", r["id"].(string), "tenant-1")
	if after["fiscal_device_id"] != fiscal["id"] || after["payment_terminal_id"] != pay["id"] || after["version"] != b["generation"] {
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
	patch(adapter, map[string]any{"mqtt_uri": "ssl://mqtt.test:8883", "mqtt_client_id": adapter["id"], "ble_advertising_identity": adapter["id"], "transaction_signing_kid": "kid-1", "unp_prefix": "FM1", "unp_range_start": int64(1), "unp_range_end": int64(1000), "uart_tx_pin": int64(17), "uart_rx_pin": int64(18)})
	patch(fiscal, map[string]any{"uart_baud": int64(115200), "uart_data_bits": int64(8), "uart_parity": "N", "uart_stop_bits": int64(1)})
	patch(pay, map[string]any{"ble_identity": "bluepad-1", "service_uuid": "service", "tx_characteristic_uuid": "tx", "rx_characteristic_uuid": "rx"})
	_, err := s.CreateCompositeBinding(r["id"].(string), "tenant-1", "DATECS_DP150_BLUEPAD50", adapter["id"].(string), fiscal["id"].(string), pay["id"].(string), r["version"].(int64))
	if err != nil {
		t.Fatal(err)
	}
	if capture.tenant != "tenant-1" || capture.adapter != adapter["id"] || len(capture.envelope) < 12 || string(capture.envelope[:4]) != "BFPE" || capture.envelope[4] != 1 {
		t.Fatal("signed BFPE was not delivered", capture)
	}
}
