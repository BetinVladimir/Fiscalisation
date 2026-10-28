package domain

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

type delegatedQueue struct {
	durableQueueDriver
	bad   bool
	count int
}

func (d *delegatedQueue) Prepare(o Operation, s Sale, p PaymentRequest) (ResourceRecord, error) {
	r, e := d.durableQueueDriver.Prepare(o, s, p)
	payload, e := BuildFinalizePayload(s, "operator-1", "A001")
	if e != nil {
		return r, e
	}
	if d.bad {
		payload["currency"] = "BGN"
	}
	b, _ := json.Marshal(map[string]any{"payload": payload})
	r.Data["body"] = string(b)
	return r, e
}
func (d *delegatedQueue) Publish(r ResourceRecord) error {
	d.count++
	return d.durableQueueDriver.Publish(r)
}
func TestDelegatedDispatchExactPlanAndSource(t *testing.T) {
	for _, scenario := range []string{"valid", "foreign-source", "foreign-tenant", "bad-command", "existing-bad-command", "terminal"} {
		t.Run(scenario, func(t *testing.T) {
			f, p, in := delegatedFixture(t)
			f.svc.SyncWorkstationClock("register-1", p.TenantID)
			f.svc.RefreshReadiness("register-1", p.TenantID)
			in.SalePreparation = &DelegatedSalePreparation{SourceOrderID: "source-order", SourceShiftID: "source-shift", Items: []SaleLine{{LineID: "30000000-0000-4000-8000-000000000001", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}}, Payments: []PaymentRequest{{PaymentID: "20000000-0000-4000-8000-000000000001", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}}}, ExpectedTotal: Money{Amount: "2.50", Currency: "EUR"}}
			issued, e := f.svc.IssueDelegatedExecutionAuthorization(p, in, "prepare")
			if e != nil {
				t.Fatal(e)
			}
			record, _ := f.repo.Resource("execution_authorization", func() string {
				var a ExecutionAuthorization
				raw, _ := base64.RawURLEncoding.DecodeString(issued.Authorization.Payload)
				json.Unmarshal(raw, &a)
				return a.AuthorizationID
			}())
			a, _, _ := executionAuthorizationFromResource(record)
			driver := &delegatedQueue{durableQueueDriver: durableQueueDriver{repo: f.repo}}
			f.svc.driver = driver
			switch scenario {
			case "foreign-source":
				p.SystemID = "foreign"
			case "foreign-tenant":
				p.TenantID = "foreign"
			case "bad-command":
				driver.bad = true
			case "existing-bad-command":
				op, _ := f.repo.Operation(a.UNPAuthority.OperationID)
				sale, _ := f.repo.Sale(op.SaleID)
				driver.bad = true
				r, _ := driver.Prepare(op, sale, PaymentRequest{})
				f.repo.PutResource(r)
			case "terminal":
				op, _ := f.repo.Operation(a.UNPAuthority.OperationID)
				op.State = "FISCALIZED"
				f.repo.operations[op.ID] = op
			}
			_, e = f.svc.DispatchDelegatedExecution(p, a.AuthorizationID)
			valid := scenario == "valid" || scenario == "terminal"
			if valid != (e == nil) {
				t.Fatal(scenario, e)
			}
			if scenario == "valid" && driver.count != 1 {
				t.Fatal("no durable publish")
			}
			if scenario != "valid" && driver.count != 0 {
				t.Fatal("invalid publish")
			}
			if scenario == "bad-command" && len(f.repo.Resources("device_command_outbox", p.TenantID)) != 0 {
				t.Fatal("invalid command persisted")
			}
		})
	}
}
