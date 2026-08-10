package gateway

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fiscalisation/edge-agent/authority"
	"fiscalisation/edge-agent/journal"
	edgeruntime "fiscalisation/edge-agent/runtime"
)

type captureDevice struct {
	command edgeruntime.Command
	calls   int
}

func (d *captureDevice) Probe() error { return nil }
func (d *captureDevice) Execute(c edgeruntime.Command) (string, error) {
	d.command = c
	d.calls++
	return "FD-1", nil
}

func TestComplianceGatewayBuildsUNPAndJournalsIntentBeforeDevice(t *testing.T) {
	j, err := journal.Open(filepath.Join(t.TempDir(), "edge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	now := time.Now().UTC()
	manager := authority.New(authority.Lease{RegisterID: "register", EdgeID: "edge", FencingToken: 7, OperationFrom: 1, OperationTo: 10, UNPFrom: 41, UNPTo: 50, ExpiresAt: now.Add(time.Hour)})
	device := &captureDevice{}
	runtime := edgeruntime.New(j, manager, device)
	binding := SessionBinding{TenantID: "tenant", RegisterID: "register", DeviceID: "device", SessionID: "session", OperatorCode: "A001", AppInstanceID: "app", FencingToken: 7, ExpiresAt: now.Add(time.Hour), IsRevoked: func(string, time.Time) bool { return false }}
	gateway, err := NewComplianceGateway(runtime, binding, CountryPolicyBundle{CountryCode: "BG", ProfileVersion: "2026-08-10.1", IdentifierScheme: "BG_UNP_V1", FiscalDeviceNumber: "AB123456", Signature: "signed-policy", ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := gateway.Execute(ComplianceIntent{IntentID: "intent-1", Action: "OPEN_WITH_LINE", ClientSaleSurrogateID: "surrogate", OperatorCode: "A001", AppInstanceID: "app", Line: &IntentLine{LineID: "line", Name: "Кафе", Quantity: "1.000", UnitPrice: "2.50", TaxGroup: "B"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "FISCALIZED" || result.RegulatoryIdentifiers[0]["value"] != "AB123456-A001-0000041" {
		t.Fatalf("bad result: %#v", result)
	}
	if device.calls != 1 || !strings.Contains(string(device.command.Payload), `"unp":"AB123456-A001-0000041"`) {
		t.Fatalf("device did not receive middleware-owned UNP: %s", device.command.Payload)
	}
	events := j.Events()
	if len(events) != 2 || events[0].Type != "COMMAND_DURABLE" || events[1].Type != "COMMAND_RESULT" {
		t.Fatalf("intent/result not durable: %#v", events)
	}
	second, err := gateway.Execute(ComplianceIntent{IntentID: "intent-1", Action: "OPEN_WITH_LINE", ClientSaleSurrogateID: "surrogate", OperatorCode: "A001", AppInstanceID: "app", Line: &IntentLine{LineID: "line", Name: "Кафе", Quantity: "1.000", UnitPrice: "2.50", TaxGroup: "B"}})
	if err != nil || second.RegulatoryIdentifiers[0]["value"] != "AB123456-A001-0000041" || device.calls != 1 {
		t.Fatalf("idempotent replay failed: %#v %v calls=%d", second, err, device.calls)
	}
	_, err = gateway.Execute(ComplianceIntent{IntentID: "intent-2", Action: "PAYMENT", ClientSaleSurrogateID: "surrogate", ServerSaleID: "sale-1", OperatorCode: "A001", AppInstanceID: "app", ExpectedVersion: 1, Payment: map[string]any{"type": "CASH"}})
	if err != nil || device.calls != 2 || !strings.Contains(string(device.command.Payload), `"unp":"AB123456-A001-0000041"`) {
		t.Fatalf("later intent did not reuse durable sale UNP: %v %s", err, device.command.Payload)
	}
}

func TestComplianceGatewayFailsClosedForExpiredPolicyAndMissingDevice(t *testing.T) {
	j, _ := journal.Open(filepath.Join(t.TempDir(), "edge.db"))
	defer j.Close()
	now := time.Now().UTC()
	runtime := edgeruntime.New(j, authority.New(authority.Lease{FencingToken: 1, OperationFrom: 1, OperationTo: 2, UNPFrom: 1, UNPTo: 2, ExpiresAt: now.Add(time.Hour)}), &captureDevice{})
	binding := SessionBinding{TenantID: "t", RegisterID: "r", DeviceID: "d", SessionID: "s", OperatorCode: "A001", AppInstanceID: "a", FencingToken: 1, ExpiresAt: now.Add(time.Hour), IsRevoked: func(string, time.Time) bool { return false }}
	gateway, err := NewComplianceGateway(runtime, binding, CountryPolicyBundle{CountryCode: "BG", ProfileVersion: "v", IdentifierScheme: "BG_UNP_V1", FiscalDeviceNumber: "AB123456", Signature: "sig", ExpiresAt: now.Add(-time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = gateway.Execute(ComplianceIntent{IntentID: "i", Action: "PAYMENT", ClientSaleSurrogateID: "s", OperatorCode: "A001", AppInstanceID: "a"}); err == nil {
		t.Fatal("expired policy accepted")
	}
	gateway.now = func() time.Time { return now.Add(-time.Minute) }
	if _, err = gateway.Execute(ComplianceIntent{IntentID: "unbound", Action: "PAYMENT", ClientSaleSurrogateID: "missing", OperatorCode: "A001", AppInstanceID: "a"}); err == nil {
		t.Fatal("payment without durable sale binding accepted")
	}
	if _, err = gateway.Execute(ComplianceIntent{IntentID: "stolen", Action: "OPEN_WITH_LINE", ClientSaleSurrogateID: "s", OperatorCode: "B002", AppInstanceID: "other", Line: &IntentLine{LineID: "l", Name: "x", Quantity: "1.000", UnitPrice: "1.00", TaxGroup: "B"}}); err == nil {
		t.Fatal("intent outside operator/app session binding accepted")
	}
}
