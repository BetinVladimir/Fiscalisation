package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// OpenSaleWithFirstLineRequest bundles the validated session and first line for
// an atomic open-with-line write. OperatorCode is resolved from the session and
// must never be set directly by the caller.
type OpenSaleWithFirstLineRequest struct {
	TenantID             string
	ClientSaleSurrogateID string
	WorkstationID        string
	OperatorSessionID    string
	OperatorCode         string `json:"-"`
	Line                 SaleLine `json:"line"`
}

func (s *Service) OpenSaleWithFirstLine(in OpenSaleWithFirstLineRequest) (Sale, error) {
	if in.TenantID == "" || in.ClientSaleSurrogateID == "" || in.WorkstationID == "" || in.OperatorSessionID == "" {
		return Sale{}, errors.New("invalid open-with-line request")
	}
	session, err := s.WorkstationSession(in.OperatorSessionID, in.WorkstationID, in.TenantID)
	if err != nil {
		return Sale{}, err
	}
	in.OperatorCode = session.OperatorCode
	if in.Line.LineID == "" || in.Line.Name == "" || !validMoney(in.Line.UnitPrice) || !validDiscount(in.Line.Discount) || !validQuantity(in.Line.Quantity) || !validTax(in.Line.TaxGroup) || !s.policy.AllowsTaxGroup(in.Line.TaxGroup, time.Now().UTC()) {
		return Sale{}, errors.New("invalid first line")
	}
	now := time.Now().UTC()
	if !s.HasDailyClockSync(in.WorkstationID, in.TenantID, now) {
		return Sale{}, errors.New("daily clock synchronization required")
	}
	if _, err := s.CurrentReadiness(in.WorkstationID, in.TenantID); err != nil {
		return Sale{}, err
	}
	device, err := s.activeFiscalDeviceSnapshot(in.WorkstationID, in.TenantID, now)
	if err != nil || !bgFMINPattern.MatchString(device.FiscalDeviceNumber) {
		return Sale{}, errors.New("verified fiscal device number unavailable")
	}
	if !s.activeOperatorForTenant(in.OperatorCode, in.TenantID, time.Now().UTC()) {
		return Sale{}, errors.New("operator unavailable")
	}
	hasShift := false
	for _, shift := range s.repo.Shifts(in.TenantID) {
		if shift.RegisterID == in.WorkstationID && (shift.OperatorID == in.OperatorCode || shift.OperatorID == session.OperatorID) && shift.State == "OPEN" {
			hasShift = true
			break
		}
	}
	if !hasShift {
		return Sale{}, errors.New("open shift required")
	}
	register, err := s.repo.Resource("register", in.WorkstationID)
	if err != nil || stringField(register.Data, "location_id") == "" {
		return Sale{}, errors.New("workstation location unavailable")
	}
	sale := Sale{ID: newID("sale"), TenantID: in.TenantID, ExternalID: in.ClientSaleSurrogateID, LocationID: stringField(register.Data, "location_id"), RegisterID: in.WorkstationID, OperatorID: in.OperatorCode, RegisterCode: stringField(register.Data, "code"), OperatorCode: in.OperatorCode, State: "OPEN", Version: 1, Lines: []SaleLine{}, Payments: []PaymentRecord{}, FiscalDevice: device, CreatedAt: now, UpdatedAt: now}
	s.snapshotSaleNomenclature(&sale)
	return s.repo.OpenSaleWithFirstLine(sale, in.Line, device.FiscalDeviceNumber)
}

func (s *Service) CreateSale(in CreateSale) (Sale, error) {
	if in.ExternalID == "" || in.RegisterID == "" || len(in.OperatorID) != 4 {
		return Sale{}, errors.New("invalid sale")
	}
	if in.TenantID != "" && !s.registerHasActiveFiscalDevice(in.RegisterID, in.TenantID) {
		return Sale{}, errors.New("fiscal device unavailable")
	}
	if in.TenantID != "" && !s.activeOperatorForTenant(in.OperatorID, in.TenantID, time.Now().UTC()) {
		return Sale{}, errors.New("operator unavailable")
	}
	locationID := ""
	if in.TenantID != "" {
		register, err := s.repo.Resource("register", in.RegisterID)
		if err != nil || register.TenantID != in.TenantID || stringField(register.Data, "location_id") == "" {
			return Sale{}, errors.New("register location unavailable")
		}
		locationID = stringField(register.Data, "location_id")
	}
	now := time.Now().UTC()
	device := FiscalDeviceSnapshot{}
	if in.TenantID == "" {
		device = FiscalDeviceSnapshot{DeviceID: in.RegisterID, FiscalDeviceNumber: "00000001", FiscalMemoryNumber: "00000001", Vendor: "SIMULATOR", Model: "DEV"}
	}
	v := Sale{ID: newID("sale"), TenantID: in.TenantID, ExternalID: in.ExternalID, LocationID: locationID, RegisterID: in.RegisterID, OperatorID: in.OperatorID, OperatorCode: in.OperatorID, State: "DRAFT", Version: 1, Lines: []SaleLine{}, Payments: []PaymentRecord{}, FiscalDevice: device, CreatedAt: now, UpdatedAt: now}
	s.snapshotSaleNomenclature(&v)
	return v, s.repo.PutSale(v)
}

func (s *Service) snapshotSaleNomenclature(sale *Sale) {
	if location, err := s.repo.Resource("location", sale.LocationID); err == nil {
		sale.LocationCode = stringField(location.Data, "code")
		sale.LocationName = stringField(location.Data, "name")
	}
	if register, err := s.repo.Resource("register", sale.RegisterID); err == nil {
		sale.RegisterCode = stringField(register.Data, "code")
	}
	for _, operator := range s.repo.Resources("operator", sale.TenantID) {
		if operator.ID == sale.OperatorID || stringField(operator.Data, "code") == sale.OperatorID {
			sale.OperatorCode = stringField(operator.Data, "code")
			sale.OperatorName = strings.TrimSpace(stringField(operator.Data, "first_name") + " " + stringField(operator.Data, "last_name"))
			break
		}
	}
}
func (s *Service) AddLine(id string, line SaleLine) (Sale, error) {
	v, err := s.repo.Sale(id)
	if err != nil {
		return v, err
	}
	return s.addLineForTenant(id, v.Version, line, v.TenantID)
}
func (s *Service) AddLineForTenant(id string, line SaleLine, tenant string) (Sale, error) {
	v, err := s.saleForTenantMutation(id, tenant)
	if err != nil {
		return v, err
	}
	return s.addLineForTenant(id, v.Version, line, tenant)
}
func (s *Service) AddLineExpectedForTenant(id string, expected int64, line SaleLine, tenant string) (Sale, error) {
	return s.addLineForTenant(id, expected, line, tenant)
}

func (s *Service) ChangeLineExpectedForTenant(id, lineID string, expected int64, replacement SaleLine, tenant string) (Sale, error) {
	if lineID == "" || replacement.LineID != lineID || replacement.Name == "" || !validMoney(replacement.UnitPrice) || !validDiscount(replacement.Discount) || !validQuantity(replacement.Quantity) || !validTax(replacement.TaxGroup) || !s.policy.AllowsTaxGroup(replacement.TaxGroup, time.Now().UTC()) {
		return Sale{}, errors.New("invalid line")
	}
	v, err := s.saleForTenantMutation(id, tenant)
	if err != nil || v.Version != expected {
		return Sale{}, errors.New("version conflict")
	}
	lines := append([]SaleLine(nil), v.Lines...)
	found := false
	for i := range lines {
		if lines[i].LineID == lineID {
			lines[i] = replacement
			found = true
			break
		}
	}
	if !found {
		return Sale{}, ErrNotFound
	}
	probe := v
	probe.Lines = lines
	if _, err = saleTotal(probe); err != nil {
		return Sale{}, errors.New("line total overflow")
	}
	return s.repo.ReplaceSaleLinesExpected(id, tenant, expected, lines, "SALE_LINE_CHANGED")
}

func (s *Service) CancelLineExpectedForTenant(id, lineID string, expected int64, tenant string) (Sale, error) {
	if lineID == "" {
		return Sale{}, errors.New("line required")
	}
	v, err := s.saleForTenantMutation(id, tenant)
	if err != nil || v.Version != expected {
		return Sale{}, errors.New("version conflict")
	}
	lines := make([]SaleLine, 0, len(v.Lines))
	found := false
	for _, line := range v.Lines {
		if line.LineID == lineID {
			found = true
			continue
		}
		lines = append(lines, line)
	}
	if !found {
		return Sale{}, ErrNotFound
	}
	return s.repo.ReplaceSaleLinesExpected(id, tenant, expected, lines, "SALE_LINE_CANCELLED")
}

func (s *Service) SalesForTenant(tenant, operator, register, state string) []Sale {
	items := s.repo.Sales(tenant)
	out := make([]Sale, 0, len(items))
	for _, v := range items {
		if operator != "" && v.OperatorID != operator {
			continue
		}
		if register != "" && v.RegisterID != register {
			continue
		}
		if state != "" && v.State != state {
			continue
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}
func (s *Service) addLineForTenant(id string, expected int64, line SaleLine, tenant string) (Sale, error) {
	if line.LineID == "" || line.Name == "" || !validMoney(line.UnitPrice) || !validDiscount(line.Discount) || !validQuantity(line.Quantity) || !validTax(line.TaxGroup) || !s.policy.AllowsTaxGroup(line.TaxGroup, time.Now().UTC()) {
		return Sale{}, errors.New("invalid line")
	}
	current, err := s.saleForTenantMutation(id, tenant)
	if err != nil || current.Version != expected {
		return Sale{}, errors.New("version conflict")
	}
	current.Lines = append(current.Lines, line)
	if _, err = saleTotal(current); err != nil {
		return Sale{}, errors.New("line total overflow")
	}
	return s.repo.AddSaleLineExpected(id, tenant, expected, line)
}
func (s *Service) Pay(id string, p PaymentRequest) (Operation, error) {
	return s.payForTenant(id, p, "")
}
func (s *Service) PayForTenant(id string, p PaymentRequest, tenant string) (Operation, error) {
	return s.payForTenant(id, p, tenant)
}

// FinalizeSaleForTenant reserves one immutable operation for an entire receipt.
// Adapters expand it into ordered payment/fiscal steps and journal each boundary
// before I/O, so split tender cannot create multiple physical receipts.
func (s *Service) FinalizeSaleForTenant(id string, in SaleFinalizeRequest, tenant string) (Operation, error) {
	return s.finalizeSaleForTenant(id, in, tenant, true)
}

// PrepareFinalizeSaleForTenant freezes the exact receipt plan without publishing
// or executing a money command. Local dispatch requires a root-issued ticket.
func (s *Service) PrepareFinalizeSaleForTenant(id string, in SaleFinalizeRequest, tenant string) (Operation, error) {
	return s.finalizeSaleForTenant(id, in, tenant, false)
}
func (s *Service) finalizeSaleForTenant(id string, in SaleFinalizeRequest, tenant string, dispatch bool) (Operation, error) {
	sale, err := s.saleForTenantMutation(id, tenant)
	if err != nil {
		return Operation{}, err
	}
	plan := ReceiptFinalizePlan{ClientOperationID: in.ClientOperationID, ReceiptSessionID: in.ReceiptSessionID, SaleID: id, UNP: sale.UNP, Items: sale.Lines, Payments: in.Payments, ExpectedTotal: in.ExpectedTotal}
	if err = ValidateReceiptPlan(plan); err != nil {
		return Operation{}, err
	}
	total, err := saleTotal(sale)
	if err != nil {
		return Operation{}, err
	}
	expected, err := receiptMoneyCents(in.ExpectedTotal.Amount)
	if err != nil || expected != total {
		return Operation{}, errors.New("finalize total mismatch")
	}
	if sale.State != "OPEN" || len(sale.Lines) == 0 {
		return Operation{}, errors.New("sale not finalizable")
	}
	if sale.TenantID != "" {
		sale.FiscalDevice, err = s.activeFiscalDeviceSnapshot(sale.RegisterID, sale.TenantID, time.Now().UTC())
		if err != nil {
			return Operation{}, fmt.Errorf("fiscal device unavailable: %w", err)
		}
	}
	for _, p := range in.Payments {
		if p.Type == "CARD" && (p.TerminalPolicy == "NONE" || (sale.TenantID != "" && !s.registerHasActivePaymentTerminal(sale.RegisterID, sale.TenantID))) {
			return Operation{}, errors.New("payment terminal unavailable")
		}
	}
	driver, routeErr := s.boundDriver(sale.TenantID, sale.RegisterID, sale.FiscalDevice)
	if routeErr != nil {
		return Operation{}, fmt.Errorf("fiscal device route unavailable: %w", routeErr)
	}
	if probeErr := func() error {
		if dispatch {
			return driver.Probe()
		}
		return nil
	}(); probeErr != nil {
		return Operation{}, fmt.Errorf("fiscal device probe failed: %w", probeErr)
	}
	now := time.Now().UTC()
	op := Operation{ID: in.ClientOperationID, ClientOperationID: in.ClientOperationID, ReceiptSessionID: in.ReceiptSessionID, TenantID: sale.TenantID, SaleID: id, RegisterID: sale.RegisterID, Type: "SALE_FINALIZE", State: "EXECUTING", Version: 1, Simulated: driverIsSimulated(driver), AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	for _, p := range in.Payments {
		sale.Payments = append(sale.Payments, PaymentRecord{PaymentID: p.PaymentID, Type: p.Type, Amount: p.Amount, CreatedAt: now})
	}
	writer, ok := s.repo.(interface {
		ReserveSaleFinalize(Sale, int64, Operation, *ResourceRecord) (Sale, error)
	})
	if !ok {
		return Operation{}, errors.New("atomic receipt finalization unavailable")
	}
	if !dispatch {
		_, err = writer.ReserveSaleFinalize(sale, sale.Version, op, nil)
		return op, err
	}
	if queued, ok := driver.(durableQueuedDriver); ok {
		command, prepErr := queued.Prepare(op, sale, PaymentRequest{})
		if prepErr != nil {
			return Operation{}, prepErr
		}
		sale, err = writer.ReserveSaleFinalize(sale, sale.Version, op, &command)
		if err != nil {
			return Operation{}, err
		}
		if publishErr := queued.Publish(command); publishErr != nil {
			return op, nil
		}
		return op, nil
	}
	// Atomically transition the sale to PAYMENT_PENDING and store the
	// operation as EXECUTING before any I/O. This prevents concurrent
	// goroutines with the same ClientOperationID from each calling Execute.
	sale, err = writer.ReserveSaleFinalize(sale, sale.Version, op, nil)
	if err != nil {
		return Operation{}, err
	}
	ref, code := driver.Execute(op, sale, PaymentRequest{})
	op.Version++
	op.UpdatedAt = time.Now().UTC()
	if code != "" {
		op.State = "UNKNOWN"
		op.ErrorCode = code
		op.AllowedActions = []string{"RECONCILE"}
		sale.State = "UNKNOWN"
	} else {
		op.State = "FISCALIZED"
		op.FiscalReference = ref
		op.FiscalDocumentAt = op.UpdatedAt
		sale.State = "COMPLETED"
		sale.CompletedAt = &op.UpdatedAt
		sale.FiscalOperationID = op.ID
	}
	sale.Version++
	sale.UpdatedAt = op.UpdatedAt
	return op, s.repo.CommitSaleOperationEvent(sale, op, fiscalOperationEvent(sale, op))
}
func (s *Service) payForTenant(id string, p PaymentRequest, tenant string) (Operation, error) {
	sale, e := s.saleForTenantMutation(id, tenant)
	if e != nil {
		return Operation{}, e
	}
	if sale.State != "OPEN" || len(sale.Lines) == 0 || p.PaymentID == "" || !validMoney(p.Amount) || !supportedPaymentType(p.Type) {
		return Operation{}, errors.New("payment not allowed")
	}
	// New SUPTO aggregates are identifiable by their immutable regulatory
	// projection. Payment always performs a fresh physical check; a lease used
	// to open the sale is intentionally insufficient at this irreversible edge.
	if len(sale.RegulatoryIdentifiers) > 0 {
		if !s.HasDailyClockSync(sale.RegisterID, sale.TenantID, time.Now().UTC()) {
			return Operation{}, errors.New("daily clock synchronization required")
		}
		if _, e = s.RefreshReadiness(sale.RegisterID, sale.TenantID); e != nil {
			return Operation{}, errors.New("fresh payment readiness failed")
		}
	}
	if sale.TenantID != "" && !s.registerHasActiveFiscalDevice(sale.RegisterID, sale.TenantID) {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	if sale.TenantID != "" {
		snapshot, snapshotErr := s.activeFiscalDeviceSnapshot(sale.RegisterID, sale.TenantID, time.Now().UTC())
		if snapshotErr != nil {
			return Operation{}, errors.New("fiscal device unavailable")
		}
		sale.FiscalDevice = snapshot
	}
	if p.Type == "CARD" {
		if p.TerminalPolicy == "NONE" {
			return Operation{}, errors.New("card payment requires terminal")
		}
		if sale.TenantID != "" && !s.registerHasActivePaymentTerminal(sale.RegisterID, sale.TenantID) {
			return Operation{}, errors.New("payment terminal unavailable")
		}
	}
	driver, routeErr := s.boundDriver(sale.TenantID, sale.RegisterID, sale.FiscalDevice)
	if routeErr != nil || driver.Probe() != nil {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	amount, _ := parseFixed(p.Amount.Amount, 2)
	total, totalErr := saleTotal(sale)
	if totalErr != nil {
		return Operation{}, errors.New("invalid sale total")
	}
	paid := int64(0)
	for _, x := range sale.Payments {
		if x.PaymentID == p.PaymentID {
			return Operation{}, errors.New("duplicate payment id")
		}
		v, _ := parseFixed(x.Amount.Amount, 2)
		if v > math.MaxInt64-paid {
			return Operation{}, errors.New("invalid paid total")
		}
		paid += v
	}
	if amount <= 0 || paid > total || amount > total-paid {
		return Operation{}, errors.New("payment amount exceeds balance")
	}
	now := time.Now().UTC()
	op := Operation{ID: newID("op"), TenantID: sale.TenantID, SaleID: id, RegisterID: sale.RegisterID, Type: "FISCAL_SALE", State: "EXECUTING", Version: 1, Simulated: driverIsSimulated(driver), AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	if queued, ok := driver.(durableQueuedDriver); ok {
		command, prepareErr := queued.Prepare(op, sale, p)
		if prepareErr != nil {
			return Operation{}, prepareErr
		}
		sale, e = s.repo.ReserveSalePaymentCommand(id, tenant, sale.Version, op, sale.FiscalDevice, command)
		if e != nil {
			return Operation{}, e
		}
		if e = queued.Publish(command); e != nil {
			// The command remains durable and will be republished by the bridge
			// after reconnect/restart. Publication failure is not a fiscal result.
			return op, nil
		}
		return op, nil
	}
	sale, e = s.repo.ReserveSalePayment(id, tenant, sale.Version, op, sale.FiscalDevice)
	if e != nil {
		return Operation{}, e
	}
	if queued, ok := driver.(queuedDriver); ok {
		if e = queued.Queue(op, sale, p); e != nil {
			op.State = "UNKNOWN"
			op.ErrorCode = "MQTT_COMMAND_PUBLICATION_UNKNOWN"
			op.AllowedActions = []string{"RECONCILE"}
			op.Version++
			op.UpdatedAt = time.Now().UTC()
			sale.State = "UNKNOWN"
			_ = s.repo.CommitSaleOperation(sale, op) // best-effort: persist UNKNOWN state after MQTT failure; state already set in-memory
			return op, nil
		}
		return op, nil
	}
	ref, code := driver.Execute(op, sale, p)
	op.Version++
	op.UpdatedAt = time.Now().UTC()
	if code == "FISCAL_RESULT_UNKNOWN" {
		op.State = "UNKNOWN"
		op.ErrorCode = code
		op.AllowedActions = []string{"RECONCILE"}
		sale.State = "UNKNOWN"
	} else if code != "" {
		op.State = "FAILED"
		op.ErrorCode = code
		sale.State = "OPEN"
	} else {
		op.FiscalReference = ref
		sale.Payments = append(sale.Payments, PaymentRecord{PaymentID: p.PaymentID, Type: p.Type, Amount: p.Amount, FiscalReference: ref, CreatedAt: op.UpdatedAt})
		if paid+amount == total {
			op.State = "FISCALIZED"
			op.FiscalDocumentAt = op.UpdatedAt
			sale.State = "COMPLETED"
			sale.CompletedAt = &op.UpdatedAt
			sale.FiscalOperationID = op.ID
			sale.ReceiptArtifactID, e = newUUID()
			if e != nil {
				return Operation{}, e
			}
			receiptBytes, _ := json.Marshal(map[string]any{"sale_id": sale.ID, "location_id": sale.LocationID, "register_id": sale.RegisterID, "operator_id": sale.OperatorID, "operation_id": op.ID, "unp": sale.UNP, "fiscal_reference": op.FiscalReference, "issued_at": op.UpdatedAt, "total": Money{Amount: formatFixed(total), Currency: "EUR"}, "fiscal_device": sale.FiscalDevice, "fiscal_device_number": sale.FiscalDevice.FiscalDeviceNumber, "fiscal_memory_number": sale.FiscalDevice.FiscalMemoryNumber, "lines": sale.Lines, "payments": sale.Payments})
			sale.Version++
			sale.UpdatedAt = op.UpdatedAt
			return op, s.repo.CommitSaleOperationArtifactEvent(sale, op, sale.ReceiptArtifactID, sale.TenantID, receiptBytes, fiscalOperationEvent(sale, op))
		} else {
			op.State = "PAYMENT_ACCEPTED"
			sale.State = "OPEN"
		}
	}
	sale.Version++
	sale.UpdatedAt = op.UpdatedAt
	return op, s.repo.CommitSaleOperationEvent(sale, op, fiscalOperationEvent(sale, op))
}
func (s *Service) GetSale(id string) (Sale, error)           { return s.repo.Sale(id) }
func (s *Service) GetOperation(id string) (Operation, error) { return s.repo.Operation(id) }

func (s *Service) GetSaleForTenant(id, tenant string) (Sale, error) {
	if r, ok := s.repo.(interface {
		SaleForTenant(string, string) (Sale, error)
	}); ok {
		return r.SaleForTenant(id, tenant)
	}
	v, e := s.repo.Sale(id)
	if e != nil || v.TenantID != tenant {
		return Sale{}, ErrNotFound
	}
	return v, nil
}
func (s *Service) GetOperationForTenant(id, tenant string) (Operation, error) {
	if r, ok := s.repo.(interface {
		OperationForTenant(string, string) (Operation, error)
	}); ok {
		return r.OperationForTenant(id, tenant)
	}
	v, e := s.repo.Operation(id)
	if e != nil || v.TenantID != tenant {
		return Operation{}, ErrNotFound
	}
	return v, nil
}
func (s *Service) Operations(tenant string) []Operation {
	var items []Operation
	if r, ok := s.repo.(interface{ OperationsForTenant(string) []Operation }); ok {
		items = r.OperationsForTenant(tenant)
	} else {
		all := s.repo.Operations()
		if tenant == "" {
			items = all
		} else {
			items = make([]Operation, 0)
			for _, x := range all {
				if x.TenantID == tenant {
					items = append(items, x)
				}
			}
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items
}

func (s *Service) Receipt(id string) (map[string]any, error) {
	return s.receiptForTenant(id, "")
}
func (s *Service) ReceiptForTenant(id, tenant string) (map[string]any, error) {
	return s.receiptForTenant(id, tenant)
}
func (s *Service) receiptForTenant(id, tenant string) (map[string]any, error) {
	v, e := s.saleForTenantMutation(id, tenant)
	if e != nil {
		return nil, e
	}
	if v.State != "COMPLETED" {
		return nil, errors.New("receipt unavailable")
	}
	ref := ""
	if len(v.Payments) > 0 {
		ref = v.Payments[len(v.Payments)-1].FiscalReference
	}
	total, err := saleTotal(v)
	if err != nil {
		return nil, errors.New("invalid sale total")
	}
	device := v.FiscalDevice
	device.BindingVersion = 0 // Internal route-fencing metadata is not part of the public receipt contract.
	merchantName, merchantAddress, merchantEIK, merchantVAT := "", "", "", ""
	if organizations := s.repo.Resources("organization", v.TenantID); len(organizations) > 0 {
		merchantName = stringField(organizations[0].Data, "legal_name")
		merchantAddress = stringField(organizations[0].Data, "correspondence_address")
		if merchantAddress == "" {
			merchantAddress = stringField(organizations[0].Data, "address")
		}
		merchantEIK = stringField(organizations[0].Data, "eik")
		merchantVAT = stringField(organizations[0].Data, "vat_number")
	}
	locationName, locationAddress, operatorCode, operatorName, registerCode := "", "", v.OperatorID, "", v.RegisterID
	if location, err := s.repo.Resource("location", v.LocationID); err == nil {
		locationName, locationAddress = stringField(location.Data, "name"), stringField(location.Data, "address")
		if merchantAddress == "" {
			merchantAddress = locationAddress
		}
	}
	if register, err := s.repo.Resource("register", v.RegisterID); err == nil {
		registerCode = stringField(register.Data, "code")
	}
	if operator, err := s.repo.Resource("operator", v.OperatorID); err == nil {
		operatorCode = stringField(operator.Data, "code")
		operatorName = strings.TrimSpace(stringField(operator.Data, "first_name") + " " + stringField(operator.Data, "last_name"))
	}
	issuedAt := v.UpdatedAt.UTC()
	qrData := fmt.Sprintf("%s*%s*%s*%s*%s", v.FiscalDevice.FiscalMemoryNumber, ref, issuedAt.Format("2006-01-02"), issuedAt.Format("15:04:05"), formatFixed(total))
	return map[string]any{"sale_id": v.ID, "operation_id": v.FiscalOperationID, "unp": v.UNP, "state": v.State, "fiscal_reference": ref, "receipt_number": ref, "issued_at": issuedAt, "total": Money{Amount: formatFixed(total), Currency: "EUR"}, "artifact_id": v.ReceiptArtifactID, "fiscal_device": device, "fiscal_device_number": v.FiscalDevice.FiscalDeviceNumber, "fiscal_memory_number": v.FiscalDevice.FiscalMemoryNumber, "merchant_name": merchantName, "merchant_correspondence_address": merchantAddress, "merchant_eik": merchantEIK, "merchant_vat_number": merchantVAT, "location_name": locationName, "location_address": locationAddress, "register_code": registerCode, "operator_code": operatorCode, "operator_name": operatorName, "fiscal_logo": "BG", "document_label": "ФИСКАЛЕН БОН", "qr_code_data": qrData, "lines": v.Lines, "payments": v.Payments}, nil
}
func (s *Service) saleForTenantMutation(id, tenant string) (Sale, error) {
	if tenant == "" {
		return s.repo.Sale(id)
	}
	return s.GetSaleForTenant(id, tenant)
}
func (s *Service) operationForTenantMutation(id, tenant string) (Operation, error) {
	if tenant == "" {
		return s.repo.Operation(id)
	}
	return s.GetOperationForTenant(id, tenant)
}
