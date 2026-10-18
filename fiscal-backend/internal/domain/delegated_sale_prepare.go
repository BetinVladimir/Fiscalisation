package domain

import (
	"errors"
	"fmt"
	"reflect"
	"time"
)

type DelegatedSalePreparation struct {
	SourceOrderID string           `json:"source_order_id"`
	SourceShiftID string           `json:"source_shift_id"`
	Items         []SaleLine       `json:"items"`
	Payments      []PaymentRequest `json:"payments"`
	ExpectedTotal Money            `json:"expected_total"`
}

func delegatedUUID(seed string) string {
	raw := edgeDigest(seed)[:32]
	return fmt.Sprintf("%s-%s-4%s-a%s-%s", raw[:8], raw[8:12], raw[13:16], raw[17:20], raw[20:32])
}
func (s *Service) prepareDelegatedSale(p DelegatedExecutionPrincipal, in DelegatedSalePreparation, session WorkstationSession) (Sale, error) {
	if in.SourceOrderID == "" || in.SourceShiftID == "" || len(in.Items) == 0 || len(in.Items) > 1000 {
		return Sale{}, errors.New("persisted source order and shift required")
	}
	operation := delegatedUUID("operation:" + edgeDigest([]string{p.TenantID, p.SystemID, in.SourceOrderID}))
	receipt := delegatedUUID("receipt:" + operation)
	if err := ValidateReceiptPlan(ReceiptFinalizePlan{ClientOperationID: operation, ReceiptSessionID: receipt, SaleID: "prepared", UNP: "pending", Items: in.Items, Payments: in.Payments, ExpectedTotal: in.ExpectedTotal}); err != nil {
		return Sale{}, err
	}
	for _, line := range in.Items {
		if !uuidPattern.MatchString(line.LineID) || line.Name == "" || !validMoney(line.UnitPrice) || !validDiscount(line.Discount) || !validQuantity(line.Quantity) || !validTax(line.TaxGroup) || !s.policy.AllowsTaxGroup(line.TaxGroup, time.Now().UTC()) {
			return Sale{}, errors.New("invalid prepared receipt line")
		}
	}
	calculated, err := saleTotal(Sale{Lines: in.Items})
	expected, e := receiptMoneyCents(in.ExpectedTotal.Amount)
	if err != nil || e != nil || calculated != expected {
		return Sale{}, errors.New("source receipt total mismatch")
	}
	key := edgeDigest([]string{p.TenantID, p.SystemID, in.SourceOrderID})
	external := "integration-order:" + key
	now := time.Now().UTC()
	record := ResourceRecord{Kind: "delegated_sale_request", ID: key, TenantID: p.TenantID, Version: 1, Data: map[string]any{"system_id": p.SystemID, "operator_id": session.OperatorID, "register_id": session.WorkstationID, "source_shift_id": in.SourceShiftID, "plan_sha256": edgeDigest(in)}, CreatedAt: now, UpdatedAt: now}
	if err = s.repo.PutResource(record); err != nil {
		return Sale{}, err
	}
	// The delegated source already authenticated its persisted open shift. Link it
	// explicitly to a fiscal shift; this is a database transition, never a report.
	var shift Shift
	for _, v := range s.repo.Shifts(p.TenantID) {
		if v.RegisterID == session.WorkstationID && (v.State == "OPEN" || v.State == "CLOSING" || v.State == "RECONCILIATION_REQUIRED") {
			if v.State != "OPEN" || (v.OperatorID != session.OperatorID && v.OperatorID != session.OperatorCode) {
				return Sale{}, errors.New("fiscal shift belongs to another operator or is blocked")
			}
			shift = v
			break
		}
	}
	if shift.ID == "" {
		shift, err = s.OpenShift(session.WorkstationID, session.OperatorID, p.TenantID)
		if err != nil {
			return Sale{}, err
		}
	}
	shiftKey := edgeDigest([]string{p.TenantID, p.SystemID, in.SourceShiftID})
	if err = s.repo.PutResource(ResourceRecord{Kind: "delegated_shift", ID: shiftKey, TenantID: p.TenantID, Version: 1, Data: map[string]any{"system_id": p.SystemID, "source_shift_id": in.SourceShiftID, "fiscal_shift_id": shift.ID, "source_assertion": "MINIPOS_LOGICAL_OPEN", "physical_verified": false, "operator_id": session.OperatorID, "register_id": session.WorkstationID}, CreatedAt: now, UpdatedAt: now}); err != nil {
		return Sale{}, err
	}
	var sale Sale
	for _, v := range s.repo.Sales(p.TenantID) {
		if v.ExternalID == external {
			sale = v
			break
		}
	}
	if sale.ID == "" {
		sale, err = s.OpenSaleWithFirstLine(OpenSaleWithFirstLineRequest{TenantID: p.TenantID, ClientSaleSurrogateID: external, WorkstationID: session.WorkstationID, OperatorSessionID: session.SessionID, Line: in.Items[0]})
		if err != nil {
			return Sale{}, err
		}
	}
	if sale.RegisterID != session.WorkstationID || sale.OperatorCode != session.OperatorCode || len(sale.Lines) > len(in.Items) {
		return Sale{}, errors.New("prepared source sale context changed")
	}
	for i, line := range sale.Lines {
		if !reflect.DeepEqual(line, in.Items[i]) {
			return Sale{}, errors.New("prepared source receipt lines changed")
		}
	}
	for len(sale.Lines) < len(in.Items) {
		sale, err = s.addLineForTenant(sale.ID, sale.Version, in.Items[len(sale.Lines)], p.TenantID)
		if err != nil {
			return Sale{}, err
		}
	}
	if sale.State == "PAYMENT_PENDING" && sale.FiscalOperationID == operation {
		op, err := s.repo.Operation(operation)
		if err != nil || op.ReceiptSessionID != receipt {
			return Sale{}, errors.New("prepared operation mismatch")
		}
		return sale, nil
	}
	if _, err = s.PrepareFinalizeSaleForTenant(sale.ID, SaleFinalizeRequest{ClientOperationID: operation, ReceiptSessionID: receipt, Payments: in.Payments, ExpectedTotal: in.ExpectedTotal}, p.TenantID); err != nil {
		return Sale{}, err
	}
	return s.repo.Sale(sale.ID)
}
