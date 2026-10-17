package mqttclient

import (
	"encoding/json"
	"fiscalisation/fiscal-backend/internal/domain"
	"testing"
)

func TestPreparePreservesValidLowercaseOperatorCode(t *testing.T) {
	b := NewBridge("0123456789abcdef0123456789abcdef")
	sale := domain.Sale{ID: "sale", TenantID: "tenant", OperatorID: "operator-resource", OperatorCode: "a001", UNP: "AB123456-a001-0005000", FiscalDevice: domain.FiscalDeviceSnapshot{DeviceID: "device", BindingVersion: 1}}
	record, err := b.Prepare(domain.Operation{ID: "op", Type: "SALE_FINALIZE"}, sale, domain.PaymentRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var command map[string]any
	if err = json.Unmarshal([]byte(record.Data["body"].(string)), &command); err != nil {
		t.Fatal(err)
	}
	payload := command["payload"].(map[string]any)
	if payload["operator_code"] != "a001" || payload["unp"] != sale.UNP {
		t.Fatal(payload)
	}
}
