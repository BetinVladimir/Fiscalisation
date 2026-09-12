package domain

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
)

type ExportRequest struct {
	Type       string    `json:"type"`
	From       time.Time `json:"from"`
	To         time.Time `json:"to"`
	RegisterID string    `json:"register_id,omitempty"`
	OperatorID string    `json:"operator_id,omitempty"`
	LocationID string    `json:"location_id,omitempty"`
	DeviceID   string    `json:"device_id,omitempty"`
	Format     string    `json:"format"`
}
type exportRow struct {
	SaleID            string               `json:"sale_id"`
	ExternalID        string               `json:"external_id"`
	UNP               string               `json:"unp"`
	RegisterID        string               `json:"register_id"`
	LocationID        string               `json:"location_id,omitempty"`
	OperatorID        string               `json:"operator_id"`
	State             string               `json:"state"`
	FiscalOperationID string               `json:"fiscal_operation_id,omitempty"`
	ReceiptArtifactID string               `json:"receipt_artifact_id,omitempty"`
	FiscalDevice      FiscalDeviceSnapshot `json:"fiscal_device"`
	OfficialCurrency  string               `json:"official_currency"`
	Total             Money                `json:"total"`
	Lines             []SaleLine           `json:"lines"`
	Payments          []PaymentRecord      `json:"payments"`
	CreatedAt         string               `json:"created_at"`
	CompletedAt       string               `json:"completed_at,omitempty"`
	CancelledAt       string               `json:"cancelled_at,omitempty"`
	ReversedAt        string               `json:"reversed_at,omitempty"`
	LocationCode      string               `json:"location_code,omitempty"`
	LocationName      string               `json:"location_name,omitempty"`
	RegisterCode      string               `json:"register_code,omitempty"`
	OperatorCode      string               `json:"operator_code,omitempty"`
	OperatorName      string               `json:"operator_name,omitempty"`
}

// 2026-01-01T00:00:00 Europe/Sofia is 2025-12-31T22:00:00Z. Pinning
// the legal instant avoids a runtime dependency on container tzdata.
var bgEuroAdoption = time.Date(2025, time.December, 31, 22, 0, 0, 0, time.UTC)

type exportPeriod struct {
	Currency string
	From     time.Time
	To       time.Time
}

func (s *Service) CreateExport(in ExportRequest, tenant string) (Operation, error) {
	if tenant == "" || !contains([]string{"SUPTO_18_1", "SUPTO_18_2", "SUPTO_18_3", "SUPTO_18_4", "SUPTO_18_5", "SUPTO_18_6", "SUPTO_18_7", "SUPTO_18_8", "SUPTO_18_9", "KLEN", "FISCAL_MEMORY"}, in.Type) || !contains([]string{"JSON", "CSV", "XLSX"}, in.Format) || in.From.IsZero() || in.To.IsZero() || !in.To.After(in.From) {
		return Operation{}, errors.New("invalid export request")
	}
	rows := make([]exportRow, 0)
	for _, sale := range s.repo.Sales(tenant) {
		if !exportSaleMatches(sale, in, in.From, in.To) {
			continue
		}
		row, rowErr := s.detailedExportRow(sale, "EUR")
		if rowErr != nil {
			return Operation{}, rowErr
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt < rows[j].CreatedAt })
	var artifact []byte
	media := "application/json"
	var err error
	switch in.Format {
	case "JSON":
		headers, records := normativeExportTable(in.Type, rows)
		artifact, err = json.Marshal(map[string]any{"schema_version": "2026-08-24", "schema_id": exportSchemaID(in.Type), "columns": headers, "policy_version": "BG-N18-APP29", "official_currency": "EUR", "interval_semantics": "[from,to)", "type": in.Type, "from": in.From, "to": in.To, "records": records, "rows": rows})
	case "CSV":
		media = "text/csv"
		artifact, err = exportNormativeCSV(in.Type, rows)
	case "XLSX":
		media = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		artifact, err = exportNormativeXLSX(in.Type, rows)
	}
	if err != nil {
		return Operation{}, err
	}
	exportID, _ := newUUID()
	artifactID, _ := newUUID()
	now := time.Now().UTC()
	sum := sha256.Sum256(artifact)
	digest := hex.EncodeToString(sum[:])
	manifest := map[string]any{"artifact_id": artifactID, "media_type": media, "sha256": digest, "size": len(artifact), "created_at": now}
	data := map[string]any{"export_id": exportID, "state": "COMPLETED", "type": in.Type, "requested_at": now, "completed_at": now, "artifact": manifest, "official_currency": "EUR", "interval_semantics": "[from,to)"}
	resource := ResourceRecord{Kind: "export", TenantID: tenant, ID: exportID, Version: 1, Data: data, CreatedAt: now, UpdatedAt: now}
	op := Operation{ID: newID("op"), TenantID: tenant, Type: "COMPLIANCE_EXPORT", State: "FISCALIZED", Version: 2, FiscalReference: exportID, Simulated: true, AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	return op, s.repo.CommitResourceArtifactsOperation(resource, op, map[string][]byte{artifactID: artifact})
}

func exportSchemaID(exportType string) string { return "BG_" + exportType + "_V1" }

// CreatePeriodizedExport is the additive BG-020 export path. Its interval is
// deliberately half-open [from,to), so the legal BGN/EUR boundary can never
// duplicate or omit a sale. The locked canonical ComplianceExport remains
// EUR-only and is not widened with undocumented fields.
func (s *Service) CreatePeriodizedExport(in ExportRequest, tenant string) (Operation, error) {
	if tenant == "" || !contains([]string{"SUPTO_18_1", "SUPTO_18_2", "SUPTO_18_3", "SUPTO_18_4", "SUPTO_18_5", "SUPTO_18_6", "SUPTO_18_7", "SUPTO_18_8", "SUPTO_18_9", "KLEN", "FISCAL_MEMORY"}, in.Type) || !contains([]string{"JSON", "CSV", "XLSX"}, in.Format) || in.From.IsZero() || in.To.IsZero() || !in.To.After(in.From) {
		return Operation{}, errors.New("invalid periodized export request")
	}
	periods := splitOfficialCurrencyPeriods(in.From, in.To)
	exportID, _ := newUUID()
	now := time.Now().UTC()
	periodManifests := make([]map[string]any, 0, len(periods))
	artifacts := make(map[string][]byte, len(periods))
	for _, period := range periods {
		rows, err := s.exportRows(in, tenant, period.From, period.To, period.Currency)
		if err != nil {
			return Operation{}, err
		}
		artifact, media, err := renderExportArtifact(in, rows, period.Currency, period.From, period.To)
		if err != nil {
			return Operation{}, err
		}
		artifactID, _ := newUUID()
		artifacts[artifactID] = artifact
		sum := sha256.Sum256(artifact)
		periodManifests = append(periodManifests, map[string]any{
			"official_currency": period.Currency,
			"from_inclusive":    period.From.UTC(),
			"to_exclusive":      period.To.UTC(),
			"artifact": map[string]any{
				"artifact_id": artifactID, "media_type": media, "sha256": hex.EncodeToString(sum[:]),
				"size": len(artifact), "created_at": now,
			},
		})
	}
	data := map[string]any{
		"export_id": exportID, "state": "COMPLETED", "type": in.Type, "format": in.Format,
		"requested_at": now, "completed_at": now, "periods": periodManifests,
	}
	resource := ResourceRecord{Kind: "export_periods", TenantID: tenant, ID: exportID, Version: 1, Data: data, CreatedAt: now, UpdatedAt: now}
	op := Operation{ID: newID("op"), TenantID: tenant, Type: "COMPLIANCE_EXPORT", State: "FISCALIZED", Version: 2, FiscalReference: exportID, Simulated: true, AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	return op, s.repo.CommitResourceArtifactsOperation(resource, op, artifacts)
}

func splitOfficialCurrencyPeriods(from, to time.Time) []exportPeriod {
	boundary := bgEuroAdoption
	if !from.Before(boundary) {
		return []exportPeriod{{Currency: "EUR", From: from, To: to}}
	}
	if !to.After(boundary) {
		return []exportPeriod{{Currency: "BGN", From: from, To: to}}
	}
	return []exportPeriod{
		{Currency: "BGN", From: from, To: boundary},
		{Currency: "EUR", From: boundary, To: to},
	}
}

func (s *Service) exportRows(in ExportRequest, tenant string, from, to time.Time, currency string) ([]exportRow, error) {
	rows := make([]exportRow, 0)
	for _, sale := range s.repo.Sales(tenant) {
		if !exportSaleMatches(sale, in, from, to) {
			continue
		}
		row, err := s.detailedExportRow(sale, currency)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt == rows[j].CreatedAt {
			return rows[i].SaleID < rows[j].SaleID
		}
		return rows[i].CreatedAt < rows[j].CreatedAt
	})
	return rows, nil
}

func exportSaleMatches(sale Sale, in ExportRequest, from, to time.Time) bool {
	return !sale.CreatedAt.Before(from) && sale.CreatedAt.Before(to) &&
		(in.LocationID == "" || sale.LocationID == in.LocationID) &&
		(in.RegisterID == "" || sale.RegisterID == in.RegisterID) &&
		(in.OperatorID == "" || sale.OperatorID == in.OperatorID) &&
		(in.DeviceID == "" || sale.FiscalDevice.DeviceID == in.DeviceID)
}

func (s *Service) detailedExportRow(sale Sale, currency string) (exportRow, error) {
	if currency != "EUR" && currency != "BGN" {
		return exportRow{}, errors.New("invalid official currency")
	}
	for _, line := range sale.Lines {
		if line.UnitPrice.Currency != currency || (line.Discount != nil && line.Discount.Currency != currency) {
			return exportRow{}, errors.New("sale line currency does not match export period")
		}
	}
	for _, payment := range sale.Payments {
		if payment.Amount.Currency != currency {
			return exportRow{}, errors.New("payment currency does not match export period")
		}
	}
	total, err := saleTotal(sale)
	if err != nil {
		return exportRow{}, errors.New("invalid sale amount evidence")
	}
	lines := append([]SaleLine(nil), sale.Lines...)
	payments := append([]PaymentRecord(nil), sale.Payments...)
	row := exportRow{
		SaleID: sale.ID, ExternalID: sale.ExternalID, UNP: sale.UNP,
		LocationID: sale.LocationID, RegisterID: sale.RegisterID, OperatorID: sale.OperatorID, State: sale.State,
		FiscalOperationID: sale.FiscalOperationID, ReceiptArtifactID: sale.ReceiptArtifactID,
		FiscalDevice:     sale.FiscalDevice,
		OfficialCurrency: currency, Total: Money{Amount: formatFixed(total), Currency: currency},
		Lines: lines, Payments: payments, CreatedAt: sale.CreatedAt.Format(time.RFC3339Nano),
		LocationCode: sale.LocationCode, LocationName: sale.LocationName, RegisterCode: sale.RegisterCode,
		OperatorCode: sale.OperatorCode, OperatorName: sale.OperatorName,
	}
	if sale.CompletedAt != nil {
		row.CompletedAt = sale.CompletedAt.Format(time.RFC3339Nano)
	}
	if sale.CancelledAt != nil {
		row.CancelledAt = sale.CancelledAt.Format(time.RFC3339Nano)
	}
	if sale.ReversedAt != nil {
		row.ReversedAt = sale.ReversedAt.Format(time.RFC3339Nano)
	}
	// Backward-compatible projection for aggregates created before lifecycle timestamps.
	if row.CompletedAt == "" && sale.State == "COMPLETED" {
		row.CompletedAt = sale.UpdatedAt.Format(time.RFC3339Nano)
	}
	if row.CancelledAt == "" && sale.State == "CANCELLED" && sale.FiscalOperationID == "" {
		row.CancelledAt = sale.UpdatedAt.Format(time.RFC3339Nano)
	}
	if row.ReversedAt == "" && sale.State == "CANCELLED" && sale.FiscalOperationID != "" {
		row.ReversedAt = sale.UpdatedAt.Format(time.RFC3339Nano)
	}
	if location, err := s.repo.Resource("location", sale.LocationID); err == nil {
		row.LocationCode = stringField(location.Data, "code")
		row.LocationName = stringField(location.Data, "name")
	}
	if register, err := s.repo.Resource("register", sale.RegisterID); err == nil {
		row.RegisterCode = stringField(register.Data, "code")
	}
	if operator, err := s.repo.Resource("operator", sale.OperatorID); err == nil {
		row.OperatorCode = stringField(operator.Data, "code")
		row.OperatorName = strings.TrimSpace(stringField(operator.Data, "first_name") + " " + stringField(operator.Data, "last_name"))
	}
	if row.LocationCode == "" {
		row.LocationCode = sale.LocationID
	}
	if row.LocationName == "" {
		row.LocationName = sale.LocationID
	}
	if row.RegisterCode == "" {
		row.RegisterCode = sale.RegisterID
	}
	if row.OperatorCode == "" {
		row.OperatorCode = sale.OperatorID
	}
	return row, nil
}

func renderExportArtifact(in ExportRequest, rows []exportRow, currency string, from, to time.Time) ([]byte, string, error) {
	switch in.Format {
	case "JSON":
		headers, records := normativeExportTable(in.Type, rows)
		artifact, err := json.Marshal(map[string]any{
			"schema_version": "2026-08-24", "schema_id": exportSchemaID(in.Type), "columns": headers, "policy_version": "BG-N18-APP29",
			"official_currency": currency, "type": in.Type, "from_inclusive": from.UTC(), "to_exclusive": to.UTC(), "records": records, "rows": rows,
		})
		return artifact, "application/json", err
	case "CSV":
		artifact, err := exportPeriodCSV(in.Type, rows, currency, from, to)
		return artifact, "text/csv", err
	case "XLSX":
		artifact, err := exportPeriodXLSX(in.Type, rows, currency, from, to)
		return artifact, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", err
	default:
		return nil, "", errors.New("invalid export format")
	}
}

func exportPeriodCSV(exportType string, rows []exportRow, currency string, from, to time.Time) ([]byte, error) {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	headers, records := normativeExportTable(exportType, rows)
	_ = w.Write(append([]string{"period_official_currency", "period_from_inclusive", "period_to_exclusive"}, headers...))
	if len(rows) == 0 {
		_ = w.Write(append([]string{currency, from.UTC().Format(time.RFC3339Nano), to.UTC().Format(time.RFC3339Nano)}, make([]string, len(headers))...))
	}
	for _, record := range records {
		_ = w.Write(append([]string{currency, from.UTC().Format(time.RFC3339Nano), to.UTC().Format(time.RFC3339Nano)}, record...))
	}
	w.Flush()
	return b.Bytes(), w.Error()
}

func normativeExportTable(exportType string, rows []exportRow) ([]string, [][]string) {
	switch exportType {
	case "SUPTO_18_1":
		headers := []string{"unp", "system_sale_id", "location_code", "location_name", "opened_date", "opened_time", "workstation_code", "operator_code", "net_amount", "discount_amount", "vat_amount", "amount_due", "invoice_number", "invoice_date", "completed_date", "completed_time", "customer_code", "customer_name"}
		records := make([][]string, 0, len(rows))
		for _, row := range rows {
			net, discount, vat, due := exportSaleAmounts(row)
			openedDate, openedTime := exportDateTime(row.CreatedAt)
			completedDate, completedTime := exportDateTime(row.CompletedAt)
			records = append(records, []string{row.UNP, row.SaleID, row.LocationCode, row.LocationName, openedDate, openedTime, row.RegisterCode, row.OperatorCode, net, discount, vat, due, "", "", completedDate, completedTime, "", ""})
		}
		return headers, records
	case "SUPTO_18_2":
		headers := []string{"unp", "system_sale_id", "opened_date", "completed_date", "sale_total", "payment_date", "operator_code", "paid_amount", "payment_type", "fiscal_device_number"}
		var records [][]string
		for _, row := range rows {
			openedDate, _ := exportDateTime(row.CreatedAt)
			completedDate, _ := exportDateTime(row.CompletedAt)
			for _, payment := range row.Payments {
				records = append(records, []string{row.UNP, row.SaleID, openedDate, completedDate, row.Total.Amount, payment.CreatedAt.Format("2006-01-02"), row.OperatorCode, payment.Amount.Amount, payment.Type, row.FiscalDevice.FiscalDeviceNumber})
			}
		}
		return headers, records
	case "SUPTO_18_3":
		headers := []string{"unp", "system_sale_id", "product_code", "product_name", "quantity", "unit_net_price", "discount_amount", "vat_rate", "vat_amount", "gross_amount", "period_from", "period_to"}
		return headers, exportLineRecords(rows, false)
	case "SUPTO_18_4":
		headers := []string{"unp", "system_sale_id", "product_code", "product_name", "quantity", "unit_net_price", "discount_amount", "vat_rate", "vat_amount", "gross_amount", "completed_date", "completed_time", "reversed_date", "reversed_time", "fiscal_device_number", "operator_code"}
		return headers, exportLineRecords(rows, true)
	case "SUPTO_18_5":
		headers := []string{"unp", "system_sale_id", "product_code", "product_name", "quantity", "unit_net_price", "discount_amount", "vat_rate", "vat_amount", "gross_amount", "opened_date", "opened_time", "cancelled_date", "cancelled_time", "operator_code"}
		var records [][]string
		for _, row := range rows {
			if row.State != "CANCELLED" || row.FiscalOperationID != "" {
				continue
			}
			openedDate, openedTime := exportDateTime(row.CreatedAt)
			cancelledDate, cancelledTime := exportDateTime(row.CancelledAt)
			for _, line := range row.Lines {
				net, discount, rate, vat, gross := exportLineAmounts(line)
				records = append(records, []string{row.UNP, row.SaleID, exportProductCode(line), line.Name, line.Quantity, net, discount, rate, vat, gross, openedDate, openedTime, cancelledDate, cancelledTime, row.OperatorCode})
			}
		}
		return headers, records
	case "SUPTO_18_6":
		return []string{"record_id", "delivery_date", "delivery_time", "operator_code", "supplier_code", "supplier_name", "invoice_number", "invoice_date", "net_amount", "discount_amount", "vat_amount", "gross_amount", "payment_type"}, nil
	case "SUPTO_18_7":
		return []string{"record_id", "product_code", "product_name", "quantity", "unit_price", "discount_amount", "vat_amount", "gross_amount"}, nil
	case "SUPTO_18_8":
		return []string{"product_code", "product_name", "opening_quantity", "opening_value", "debit_quantity", "debit_value", "credit_quantity", "credit_value", "closing_quantity", "closing_value"}, nil
	case "SUPTO_18_9":
		return exportNomenclatureTable(rows)
	default:
		headers := exportCSVHeader()
		var records [][]string
		for _, row := range rows {
			records = append(records, exportCSVRecord(row))
		}
		return headers, records
	}
}

func exportDateTime(value string) (string, string) {
	if value == "" {
		return "", ""
	}
	at, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "", ""
	}
	return at.Format("2006-01-02"), at.Format("15:04:05")
}

func taxGroupRate(code string) int64 {
	switch code {
	case "B", "C":
		return 20
	case "D":
		return 9
	default:
		return 0
	}
}

func exportProductCode(line SaleLine) string {
	if line.ProductCode != "" {
		return line.ProductCode
	}
	return line.LineID
}

func exportLineAmounts(line SaleLine) (net, discount, rate, vat, gross string) {
	grossCents, _ := discountedLineTotalCents(line)
	discountCents := int64(0)
	if line.Discount != nil {
		discountCents, _ = parseFixed(line.Discount.Amount, 2)
	}
	rateValue := taxGroupRate(line.TaxGroup)
	vatCents := (grossCents*rateValue + (100+rateValue)/2) / (100 + rateValue)
	return formatFixed(grossCents - vatCents), formatFixed(discountCents), fmt.Sprintf("%d.00", rateValue), formatFixed(vatCents), formatFixed(grossCents)
}

func exportSaleAmounts(row exportRow) (net, discount, vat, due string) {
	var netCents, discountCents, vatCents int64
	for _, line := range row.Lines {
		lineNet, lineDiscount, _, lineVAT, _ := exportLineAmounts(line)
		n, _ := parseFixed(lineNet, 2)
		d, _ := parseFixed(lineDiscount, 2)
		v, _ := parseFixed(lineVAT, 2)
		netCents += n
		discountCents += d
		vatCents += v
	}
	return formatFixed(netCents), formatFixed(discountCents), formatFixed(vatCents), row.Total.Amount
}

func exportLineRecords(rows []exportRow, reversal bool) [][]string {
	var records [][]string
	for _, row := range rows {
		if reversal && (row.State != "CANCELLED" || row.FiscalOperationID == "") {
			continue
		}
		completedDate, completedTime := exportDateTime(row.CompletedAt)
		reversedDate, reversedTime := exportDateTime(row.ReversedAt)
		for _, line := range row.Lines {
			net, discount, rate, vat, gross := exportLineAmounts(line)
			base := []string{row.UNP, row.SaleID, exportProductCode(line), line.Name, line.Quantity, net, discount, rate, vat, gross}
			if reversal {
				base = append(base, completedDate, completedTime, reversedDate, reversedTime, row.FiscalDevice.FiscalDeviceNumber, row.OperatorCode)
			} else {
				base = append(base, "", "")
			}
			records = append(records, base)
		}
	}
	return records
}

func exportNomenclatureTable(rows []exportRow) ([]string, [][]string) {
	headers := []string{"catalog", "code", "name", "configured_at", "changed_at", "deactivated_at", "details"}
	seen := map[string]bool{}
	var records [][]string
	add := func(catalog, code, name, at, details string) {
		key := catalog + "\x00" + code
		if !seen[key] {
			seen[key] = true
			records = append(records, []string{catalog, code, name, at, at, "", details})
		}
	}
	for _, payment := range []string{"CASH", "CARD", "CHEQUE", "VOUCHER", "DEFERRED", "NHIF", "INTERNAL_CONSUMPTION", "COUPON"} {
		add("PAYMENT_TYPE", payment, payment, "2026-01-01T00:00:00Z", "")
	}
	for _, operation := range []string{"SALE", "PAYMENT", "CANCEL_SALE", "CANCEL_LINE", "REVERSAL", "LOGIN", "LOGOUT"} {
		add("OPERATION_TYPE", operation, operation, "2026-01-01T00:00:00Z", "")
	}
	for _, row := range rows {
		add("LOCATION", row.LocationCode, row.LocationName, row.CreatedAt, "location_id="+row.LocationID)
		add("WORKSTATION", row.RegisterCode, row.RegisterCode, row.CreatedAt, "location_code="+row.LocationCode+";fiscal_device_number="+row.FiscalDevice.FiscalDeviceNumber)
		add("OPERATOR", row.OperatorCode, row.OperatorName, row.CreatedAt, "")
		for _, line := range row.Lines {
			add("PRODUCT", exportProductCode(line), line.Name, row.CreatedAt, "")
		}
	}
	return headers, records
}

func exportNormativeCSV(exportType string, rows []exportRow) ([]byte, error) {
	headers, records := normativeExportTable(exportType, rows)
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.Write(headers)
	for _, record := range records {
		_ = w.Write(record)
	}
	w.Flush()
	return b.Bytes(), w.Error()
}

func exportNormativeXLSX(exportType string, rows []exportRow) ([]byte, error) {
	headers, records := normativeExportTable(exportType, rows)
	return exportTableXLSX(headers, records)
}

func exportPeriodXLSX(exportType string, rows []exportRow, currency string, from, to time.Time) ([]byte, error) {
	headers, records := normativeExportTable(exportType, rows)
	prefix := []string{"period_official_currency", "period_from_inclusive", "period_to_exclusive"}
	values := make([][]string, 0, len(records))
	for _, record := range records {
		values = append(values, append([]string{currency, from.UTC().Format(time.RFC3339Nano), to.UTC().Format(time.RFC3339Nano)}, record...))
	}
	if len(values) == 0 {
		values = append(values, append([]string{currency, from.UTC().Format(time.RFC3339Nano), to.UTC().Format(time.RFC3339Nano)}, make([]string, len(headers))...))
	}
	return exportTableXLSX(append(prefix, headers...), values)
}
func exportCSV(rows []exportRow) ([]byte, error) {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.Write(exportCSVHeader())
	for _, r := range rows {
		_ = w.Write(exportCSVRecord(r))
	}
	w.Flush()
	return b.Bytes(), w.Error()
}

func exportCSVHeader() []string {
	return []string{"sale_id", "external_id", "unp", "location_id", "register_id", "operator_id", "state", "fiscal_operation_id", "receipt_artifact_id", "fiscal_device_json", "official_currency", "total_amount", "lines_json", "payments_json", "created_at"}
}

func exportCSVRecord(r exportRow) []string {
	lines, _ := json.Marshal(r.Lines)
	payments, _ := json.Marshal(r.Payments)
	device, _ := json.Marshal(r.FiscalDevice)
	return []string{r.SaleID, r.ExternalID, r.UNP, r.LocationID, r.RegisterID, r.OperatorID, r.State, r.FiscalOperationID, r.ReceiptArtifactID, string(device), r.OfficialCurrency, r.Total.Amount, string(lines), string(payments), r.CreatedAt}
}

func exportXLSX(rows []exportRow) ([]byte, error) {
	values := make([][]string, 0, len(rows))
	for _, r := range rows {
		values = append(values, exportCSVRecord(r))
	}
	return exportTableXLSX(exportCSVHeader(), values)
}

func exportTableXLSX(headers []string, values [][]string) ([]byte, error) {
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	files := map[string]string{"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`, "_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`, "xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sales" sheetId="1" r:id="rId1"/></sheets></workbook>`, "xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`}
	values = append([][]string{headers}, values...)
	var sheet strings.Builder
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for i, row := range values {
		sheet.WriteString(fmt.Sprintf(`<row r="%d">`, i+1))
		for j, v := range row {
			sheet.WriteString(fmt.Sprintf(`<c r="%s%d" t="inlineStr"><is><t>%s</t></is></c>`, xlsxColumnName(j), i+1, html.EscapeString(v)))
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData></worksheet>`)
	files["xl/worksheets/sheet1.xml"] = sheet.String()
	for name, data := range files {
		w, err := z.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = w.Write([]byte(data)); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func xlsxColumnName(index int) string {
	index++
	var name string
	for index > 0 {
		index--
		name = string(rune('A'+index%26)) + name
		index /= 26
	}
	return name
}
func (s *Service) Export(id, tenant string) (map[string]any, error) {
	return s.GetResource("export", id, tenant)
}
func (s *Service) ExportArtifact(exportID, tenant string) ([]byte, string, error) {
	v, err := s.repo.Resource("export", exportID)
	if err != nil || v.TenantID != tenant {
		return nil, "", ErrNotFound
	}
	m, ok := v.Data["artifact"].(map[string]any)
	if !ok {
		return nil, "", ErrNotFound
	}
	id, _ := m["artifact_id"].(string)
	media, _ := m["media_type"].(string)
	b, err := s.repo.Artifact(id, tenant)
	return b, media, err
}

func (s *Service) ExportArtifactByID(exportID, artifactID, tenant string) ([]byte, string, error) {
	v, err := s.repo.Resource("export", exportID)
	if err != nil || v.TenantID != tenant {
		return nil, "", ErrNotFound
	}
	manifest, ok := v.Data["artifact"].(map[string]any)
	if !ok || manifest["artifact_id"] != artifactID {
		return nil, "", ErrNotFound
	}
	media, _ := manifest["media_type"].(string)
	artifact, err := s.repo.Artifact(artifactID, tenant)
	return artifact, media, err
}

func (s *Service) ExportPeriods(id, tenant string) (map[string]any, error) {
	v, err := s.repo.Resource("export_periods", id)
	if err != nil || v.TenantID != tenant {
		return nil, ErrNotFound
	}
	return cloneMap(v.Data), nil
}

func (s *Service) ExportPeriodArtifact(exportID, artifactID, tenant string) ([]byte, string, error) {
	v, err := s.repo.Resource("export_periods", exportID)
	if err != nil || v.TenantID != tenant {
		return nil, "", ErrNotFound
	}
	encoded, err := json.Marshal(v.Data["periods"])
	if err != nil {
		return nil, "", ErrNotFound
	}
	var periods []struct {
		Artifact struct {
			ArtifactID string `json:"artifact_id"`
			MediaType  string `json:"media_type"`
		} `json:"artifact"`
	}
	if json.Unmarshal(encoded, &periods) != nil {
		return nil, "", ErrNotFound
	}
	for _, period := range periods {
		if period.Artifact.ArtifactID == artifactID {
			artifact, artifactErr := s.repo.Artifact(artifactID, tenant)
			return artifact, period.Artifact.MediaType, artifactErr
		}
	}
	return nil, "", ErrNotFound
}
