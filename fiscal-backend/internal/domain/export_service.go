package domain

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// bgEuroAdoption is the boundary between BGN and EUR periods for Bulgaria.
// Periods starting before this date use BGN; at or after, EUR.
var bgEuroAdoption = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// ExportRequest specifies parameters for a compliance data-export operation.
type ExportRequest struct {
	Type       string    // SUPTO_18_1 … SUPTO_18_9
	From       time.Time // inclusive lower bound
	To         time.Time // exclusive upper bound
	Format     string    // "JSON", "CSV", "XLSX"
	LocationID string    // optional filter
	DeviceID   string    // optional filter
}

// exportRow is the internal representation of one sale for normative table generation.
type exportRow struct {
	UNP               string               `json:"unp"`
	SaleID            string               `json:"sale_id"`
	ExternalID        string               `json:"external_id"`
	State             string               `json:"state"`
	FiscalOperationID string               `json:"fiscal_operation_id,omitempty"`
	CompletedAt       string               `json:"completed_at,omitempty"`
	ReversedAt        string               `json:"reversed_at,omitempty"`
	CancelledAt       string               `json:"cancelled_at,omitempty"`
	OperatorCode      string               `json:"operator_code,omitempty"`
	LocationCode      string               `json:"location_code,omitempty"`
	LocationName      string               `json:"location_name,omitempty"`
	RegisterCode      string               `json:"register_code,omitempty"`
	FiscalDevice      FiscalDeviceSnapshot `json:"fiscal_device,omitempty"`
	Lines             []SaleLine           `json:"lines,omitempty"`
	Payments          []PaymentRecord      `json:"payments,omitempty"`
}

// officialCurrencyAt returns the legal tender for a given point in time.
func officialCurrencyAt(at time.Time) string {
	if at.Before(bgEuroAdoption) {
		return "BGN"
	}
	return "EUR"
}

// xlsxColumnName returns the Excel-style column name for a 0-based column index.
// 0→"A", 25→"Z", 26→"AA", 27→"AB", 52→"BA", etc.
func xlsxColumnName(index int) string {
	result := ""
	for index >= 0 {
		result = string(rune('A'+index%26)) + result
		index = index/26 - 1
	}
	return result
}

// normativeExportTable returns the headers and row data for the given SUPTO export type.
func normativeExportTable(exportType string, rows []exportRow) ([]string, [][]string) {
	headers := normativeHeaders(exportType)
	var records [][]string
	for _, row := range rows {
		records = append(records, normativeRows(exportType, row)...)
	}
	return headers, records
}

func normativeHeaders(exportType string) []string {
	switch exportType {
	case "SUPTO_18_1":
		return []string{"unp", "system_sale_id", "location_code", "location_name", "opened_date", "opened_time", "workstation_code", "operator_code", "net_amount", "discount_amount", "vat_amount", "amount_due", "invoice_number", "invoice_date", "completed_date", "completed_time", "customer_code", "customer_name"}
	case "SUPTO_18_2":
		return []string{"unp", "system_sale_id", "opened_date", "completed_date", "sale_total", "payment_date", "operator_code", "paid_amount", "payment_type", "fiscal_device_number"}
	case "SUPTO_18_3":
		return []string{"unp", "system_sale_id", "product_code", "product_name", "quantity", "unit_net_price", "discount_amount", "vat_rate", "vat_amount", "gross_amount", "period_from", "period_to"}
	case "SUPTO_18_4":
		return []string{"unp", "system_sale_id", "product_code", "product_name", "quantity", "unit_net_price", "discount_amount", "vat_rate", "vat_amount", "gross_amount", "completed_date", "completed_time", "reversed_date", "reversed_time", "fiscal_device_number", "operator_code"}
	case "SUPTO_18_5":
		return []string{"unp", "system_sale_id", "product_code", "product_name", "quantity", "unit_net_price", "discount_amount", "vat_rate", "vat_amount", "gross_amount", "opened_date", "opened_time", "cancelled_date", "cancelled_time", "operator_code"}
	case "SUPTO_18_6":
		return []string{"record_id", "delivery_date", "delivery_time", "operator_code", "supplier_code", "supplier_name", "invoice_number", "invoice_date", "net_amount", "discount_amount", "vat_amount", "gross_amount", "payment_type"}
	case "SUPTO_18_7":
		return []string{"record_id", "product_code", "product_name", "quantity", "unit_price", "discount_amount", "vat_amount", "gross_amount"}
	case "SUPTO_18_8":
		return []string{"product_code", "product_name", "opening_quantity", "opening_value", "debit_quantity", "debit_value", "credit_quantity", "credit_value", "closing_quantity", "closing_value"}
	case "SUPTO_18_9":
		return []string{"catalog", "code", "name", "configured_at", "changed_at", "deactivated_at", "details"}
	default:
		return []string{"unp", "system_sale_id"}
	}
}

func normativeRows(exportType string, row exportRow) [][]string {
	ts := func(s string) (date, timeStr string) {
		if s == "" {
			return "", ""
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return s, ""
		}
		return t.UTC().Format("2006-01-02"), t.UTC().Format("15:04:05")
	}
	completedDate, completedTime := ts(row.CompletedAt)
	reversedDate, reversedTime := ts(row.ReversedAt)
	cancelledDate, cancelledTime := ts(row.CancelledAt)

	switch exportType {
	case "SUPTO_18_1":
		netTotal, discTotal, vatTotal, grossTotal := saleTotals(row.Lines)
		return [][]string{{row.UNP, row.ExternalID, row.LocationCode, row.LocationName, completedDate, completedTime, row.RegisterCode, row.OperatorCode, netTotal, discTotal, vatTotal, grossTotal, "", "", completedDate, completedTime, "", ""}}
	case "SUPTO_18_2":
		var result [][]string
		grossFmt := formatLineTotals(row.Lines)
		for _, p := range row.Payments {
			payDate, payTime := ts(p.CreatedAt.Format(time.RFC3339))
			_ = payTime
			result = append(result, []string{row.UNP, row.ExternalID, completedDate, completedDate, grossFmt, payDate, row.OperatorCode, p.Amount.Amount, p.Type, row.FiscalDevice.FiscalDeviceNumber})
		}
		if len(result) == 0 {
			result = [][]string{{row.UNP, row.ExternalID, completedDate, completedDate, grossFmt, "", row.OperatorCode, "", "", row.FiscalDevice.FiscalDeviceNumber}}
		}
		return result
	case "SUPTO_18_3":
		var result [][]string
		for _, line := range row.Lines {
			net, unitNet, disc, vat, gross := exportLineAmounts(line)
			vatRate := lineVATRate(line.TaxGroup)
			result = append(result, []string{row.UNP, row.ExternalID, line.ProductCode, line.Name, line.Quantity, unitNet, disc, vatRate, vat, gross, completedDate, completedDate})
			_ = net
		}
		return result
	case "SUPTO_18_4":
		var result [][]string
		for _, line := range row.Lines {
			net, unitNet, disc, vat, gross := exportLineAmounts(line)
			vatRate := lineVATRate(line.TaxGroup)
			result = append(result, []string{row.UNP, row.ExternalID, line.ProductCode, line.Name, line.Quantity, unitNet, disc, vatRate, vat, gross, completedDate, completedTime, reversedDate, reversedTime, row.FiscalDevice.FiscalDeviceNumber, row.OperatorCode})
			_ = net
		}
		return result
	case "SUPTO_18_5":
		var result [][]string
		for _, line := range row.Lines {
			net, unitNet, disc, vat, gross := exportLineAmounts(line)
			vatRate := lineVATRate(line.TaxGroup)
			result = append(result, []string{row.UNP, row.ExternalID, line.ProductCode, line.Name, line.Quantity, unitNet, disc, vatRate, vat, gross, completedDate, completedTime, cancelledDate, cancelledTime, row.OperatorCode})
			_ = net
		}
		return result
	case "SUPTO_18_9":
		var result [][]string
		seen := map[string]bool{}
		for _, line := range row.Lines {
			if !seen[line.ProductCode] {
				seen[line.ProductCode] = true
				result = append(result, []string{"PRODUCT", line.ProductCode, line.Name, completedDate, completedDate, "", ""})
			}
		}
		for _, p := range row.Payments {
			if !seen[p.Type] {
				seen[p.Type] = true
				result = append(result, []string{"PAYMENT_TYPE", p.Type, p.Type, completedDate, completedDate, "", ""})
			}
		}
		return result
	default:
		return [][]string{{row.UNP, row.ExternalID}}
	}
}

// exportLineAmounts returns (net_line_total, unit_net_price, discount, vat_amount, gross_line_total)
// for a sale line. All amounts are formatted with 2 decimal places.
// Uses Bulgarian tax rates: A=0%, B=20%, C=20%, D=9%.
// Invariant: money is integer cents; half-up rounding.
func exportLineAmounts(line SaleLine) (net, unitNetPrice, discount, vatAmount, gross string) {
	grossCents, err := lineTotalCents(line.UnitPrice, line.Quantity)
	if err != nil {
		return "0.00", "0.00", "0.00", "0.00", "0.00"
	}
	discountCents := int64(0)
	if line.Discount != nil {
		d, _ := parseFixed(line.Discount.Amount, 2)
		discountCents = d
	}
	netGrossCents := grossCents - discountCents
	vatPercent := lineVATPercent(line.TaxGroup)
	var netCents, vatCents int64
	if vatPercent == 0 {
		netCents = netGrossCents
		vatCents = 0
	} else {
		divisor := int64(100 + vatPercent)
		// half-up rounding: (netGrossCents * 100 + divisor/2) / divisor
		netCents = (netGrossCents*100 + divisor/2) / divisor
		vatCents = netGrossCents - netCents
	}
	// unit net price: net per 1 unit (half-up: (price_cents * 100 + divisor/2) / divisor)
	priceCents, _ := parseFixed(line.UnitPrice.Amount, 2)
	var unitNetCents int64
	if vatPercent == 0 {
		unitNetCents = priceCents
	} else {
		divisor := int64(100 + vatPercent)
		unitNetCents = (priceCents*100 + divisor/2) / divisor
	}
	return formatFixed(netCents), formatFixed(unitNetCents), formatFixed(discountCents), formatFixed(vatCents), formatFixed(grossCents - discountCents)
}

func lineVATPercent(taxGroup string) int64 {
	switch taxGroup {
	case "B", "C":
		return 20
	case "D":
		return 9
	default: // "A" and others
		return 0
	}
}

func lineVATRate(taxGroup string) string {
	switch taxGroup {
	case "B", "C":
		return "20.00"
	case "D":
		return "9.00"
	default:
		return "0.00"
	}
}

func saleTotals(lines []SaleLine) (net, disc, vat, gross string) {
	var netC, discC, vatC, grossC int64
	for _, l := range lines {
		_, _, d, v, g := exportLineAmounts(l)
		dc, _ := parseFixed(d, 2)
		vc, _ := parseFixed(v, 2)
		gc, _ := parseFixed(g, 2)
		nc := gc - vc
		netC += nc
		discC += dc
		vatC += vc
		grossC += gc
	}
	return formatFixed(netC), formatFixed(discC), formatFixed(vatC), formatFixed(grossC)
}

func formatLineTotals(lines []SaleLine) string {
	_, _, _, g := saleTotals(lines)
	return g
}

// CreateExport creates a single-currency export artifact and returns a FISCALIZED Operation.
func (s *Service) CreateExport(req ExportRequest, tenant string) (Operation, error) {
	if req.From.IsZero() || req.To.IsZero() || !req.To.After(req.From) {
		return Operation{}, errors.New("invalid export interval")
	}
	if req.Format != "JSON" && req.Format != "CSV" && req.Format != "XLSX" {
		return Operation{}, errors.New("unsupported export format")
	}
	if normativeHeaders(req.Type) == nil {
		return Operation{}, errors.New("unsupported export type")
	}
	currency := officialCurrencyAt(req.From)
	rows := s.exportRowsForTenant(req, tenant)
	artifact, mediaType, err := renderExport(req, rows, currency)
	if err != nil {
		return Operation{}, err
	}
	exportID, _ := newUUID()
	artifactID, _ := newUUID()
	now := time.Now().UTC()
	digest := sha256.Sum256(artifact)
	op := Operation{
		ID: newID("op"), TenantID: tenant, Type: "COMPLIANCE_EXPORT", State: "FISCALIZED",
		FiscalReference: exportID, Version: 1, CreatedAt: now, UpdatedAt: now,
		AllowedActions: []string{},
	}
	resource := ResourceRecord{
		Kind: "export", ID: exportID, TenantID: tenant, Version: 1,
		Data: map[string]any{
			"export_id":         exportID,
			"state":             "COMPLETED",
			"type":              req.Type,
			"format":            req.Format,
			"official_currency": currency,
			"requested_at":      now.Format(time.RFC3339),
			"completed_at":      now.Format(time.RFC3339),
			"artifact": map[string]any{
				"artifact_id": artifactID,
				"media_type":  mediaType,
				"sha256":      fmt.Sprintf("%x", digest[:]),
				"size":        len(artifact),
				"created_at":  now.Format(time.RFC3339),
			},
		},
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CommitResourceArtifactsOperation(resource, op, map[string][]byte{artifactID: artifact}); err != nil {
		return Operation{}, err
	}
	return op, nil
}

// Export returns the metadata map for an export previously created by CreateExport.
func (s *Service) Export(ref, tenant string) (map[string]any, error) {
	resource, err := s.repo.Resource("export", ref)
	if err != nil || resource.TenantID != tenant {
		return nil, errors.New("export not found")
	}
	return resource.Data, nil
}

// ExportArtifactByID retrieves an artifact for a simple export, verifying the artifact ID matches.
func (s *Service) ExportArtifactByID(ref, artifactID, tenant string) ([]byte, string, error) {
	resource, err := s.repo.Resource("export", ref)
	if err != nil || resource.TenantID != tenant {
		return nil, "", errors.New("export not found")
	}
	artMap, _ := resource.Data["artifact"].(map[string]any)
	storedID, _ := artMap["artifact_id"].(string)
	if storedID == "" || storedID != artifactID {
		return nil, "", errors.New("artifact not found")
	}
	mediaType, _ := artMap["media_type"].(string)
	b, err := s.repo.Artifact(storedID, tenant)
	if err != nil {
		return nil, "", err
	}
	return b, mediaType, nil
}

// ExportArtifact returns the artifact bytes and media type for a given export reference.
func (s *Service) ExportArtifact(ref, tenant string) ([]byte, string, error) {
	resource, err := s.repo.Resource("export", ref)
	if err != nil || resource.TenantID != tenant {
		return nil, "", errors.New("export not found")
	}
	artMap, _ := resource.Data["artifact"].(map[string]any)
	artifactID, _ := artMap["artifact_id"].(string)
	mediaType, _ := artMap["media_type"].(string)
	if artifactID == "" {
		return nil, "", errors.New("export artifact not found")
	}
	b, err := s.repo.Artifact(artifactID, tenant)
	if err != nil {
		return nil, "", err
	}
	return b, mediaType, nil
}

// CreatePeriodizedExport splits the export at bgEuroAdoption and returns a manifest Operation.
func (s *Service) CreatePeriodizedExport(req ExportRequest, tenant string) (Operation, error) {
	if req.From.IsZero() || req.To.IsZero() || !req.To.After(req.From) {
		return Operation{}, errors.New("invalid export interval")
	}
	if req.Format != "JSON" && req.Format != "CSV" && req.Format != "XLSX" {
		return Operation{}, errors.New("unsupported export format")
	}
	type period struct {
		from, to time.Time
		currency string
	}
	var periods []period
	if req.From.Before(bgEuroAdoption) && req.To.After(bgEuroAdoption) {
		periods = []period{
			{from: req.From, to: bgEuroAdoption, currency: "BGN"},
			{from: bgEuroAdoption, to: req.To, currency: "EUR"},
		}
	} else {
		currency := officialCurrencyAt(req.From)
		periods = []period{{from: req.From, to: req.To, currency: currency}}
	}
	exportID, _ := newUUID()
	now := time.Now().UTC()
	allArtifacts := map[string][]byte{}
	var periodsManifest []map[string]any
	for _, p := range periods {
		subReq := ExportRequest{Type: req.Type, From: p.from, To: p.to, Format: req.Format, LocationID: req.LocationID, DeviceID: req.DeviceID}
		rows := s.exportRowsForTenant(subReq, tenant)
		// Validate: sales in a BGN period must not have EUR-priced lines
		if p.currency == "BGN" {
			for _, row := range rows {
				for _, line := range row.Lines {
					if line.UnitPrice.Currency == "EUR" {
						return Operation{}, errors.New("BGN_PERIOD_EUR_AMOUNT_INCONSISTENCY")
					}
				}
			}
		}
		artifact, mediaType, err := renderExport(subReq, rows, p.currency)
		if err != nil {
			return Operation{}, err
		}
		periodID, _ := newUUID()
		digest := sha256.Sum256(artifact)
		allArtifacts[periodID] = artifact
		periodsManifest = append(periodsManifest, map[string]any{
			"official_currency": p.currency,
			"from_inclusive":    p.from.Format(time.RFC3339),
			"to_exclusive":      p.to.Format(time.RFC3339),
			"artifact": map[string]any{
				"artifact_id": periodID,
				"media_type":  mediaType,
				"sha256":      fmt.Sprintf("%x", digest[:]),
				"size":        len(artifact),
				"created_at":  now.Format(time.RFC3339),
			},
		})
	}
	op := Operation{
		ID: newID("op"), TenantID: tenant, Type: "COMPLIANCE_EXPORT", State: "FISCALIZED",
		FiscalReference: exportID, Version: 1, CreatedAt: now, UpdatedAt: now,
		AllowedActions: []string{},
	}
	resource := ResourceRecord{
		Kind: "export_periods", ID: exportID, TenantID: tenant, Version: 1,
		Data: map[string]any{
			"export_id":    exportID,
			"state":        "COMPLETED",
			"type":         req.Type,
			"format":       req.Format,
			"requested_at": now.Format(time.RFC3339),
			"completed_at": now.Format(time.RFC3339),
			"periods":      periodsManifest,
		},
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CommitResourceArtifactsOperation(resource, op, allArtifacts); err != nil {
		return Operation{}, err
	}
	return op, nil
}

// ExportPeriods returns the periods manifest for a periodized export.
func (s *Service) ExportPeriods(ref, tenant string) (map[string]any, error) {
	resource, err := s.repo.Resource("export_periods", ref)
	if err != nil || resource.TenantID != tenant {
		return nil, errors.New("periodized export not found")
	}
	return resource.Data, nil
}

// ExportPeriodArtifact retrieves one period's artifact bytes from a periodized export.
func (s *Service) ExportPeriodArtifact(ref, periodID, tenant string) ([]byte, string, error) {
	resource, err := s.repo.Resource("export_periods", ref)
	if err != nil || resource.TenantID != tenant {
		return nil, "", errors.New("periodized export not found")
	}
	// Find the media type for this period from the artifact sub-object.
	mediaType := "application/json"
	found := false
	findInPeriods := func(periodsRaw []any) {
		for _, pr := range periodsRaw {
			p, ok := pr.(map[string]any)
			if !ok {
				continue
			}
			art, ok := p["artifact"].(map[string]any)
			if !ok {
				continue
			}
			if art["artifact_id"] == periodID {
				found = true
				if mt, ok := art["media_type"].(string); ok {
					mediaType = mt
				}
			}
		}
	}
	if periods, ok := resource.Data["periods"].([]map[string]any); ok {
		raw := make([]any, len(periods))
		for i, p := range periods {
			raw[i] = p
		}
		findInPeriods(raw)
	} else if periodsRaw, ok := resource.Data["periods"].([]any); ok {
		findInPeriods(periodsRaw)
	}
	if !found {
		return nil, "", errors.New("period artifact not found")
	}
	b, err := s.repo.Artifact(periodID, tenant)
	if err != nil {
		return nil, "", err
	}
	return b, mediaType, nil
}

// exportRowsForTenant loads and filters sales for the export request.
func (s *Service) exportRowsForTenant(req ExportRequest, tenant string) []exportRow {
	allSales := s.repo.Sales(tenant)
	var rows []exportRow
	for _, sale := range allSales {
		if sale.State != "COMPLETED" && sale.State != "CANCELLED" {
			continue
		}
		t := sale.CreatedAt
		if !t.Before(req.To) || t.Before(req.From) {
			continue
		}
		if req.LocationID != "" && sale.LocationID != req.LocationID {
			continue
		}
		if req.DeviceID != "" && sale.FiscalDevice.DeviceID != req.DeviceID {
			continue
		}
		completedAt := ""
		if sale.CompletedAt != nil {
			completedAt = sale.CompletedAt.Format(time.RFC3339)
		} else if sale.State == "COMPLETED" || sale.State == "CANCELLED" {
			completedAt = sale.UpdatedAt.Format(time.RFC3339)
		}
		reversedAt := ""
		if sale.ReversedAt != nil {
			reversedAt = sale.ReversedAt.Format(time.RFC3339)
		}
		cancelledAt := ""
		if sale.CancelledAt != nil {
			cancelledAt = sale.CancelledAt.Format(time.RFC3339)
		}
		rows = append(rows, exportRow{
			UNP:               sale.UNP,
			SaleID:            sale.ID,
			ExternalID:        sale.ExternalID,
			State:             sale.State,
			FiscalOperationID: sale.FiscalOperationID,
			CompletedAt:       completedAt,
			ReversedAt:        reversedAt,
			CancelledAt:       cancelledAt,
			OperatorCode:      sale.OperatorCode,
			LocationCode:      sale.LocationCode,
			LocationName:      sale.LocationName,
			RegisterCode:      sale.RegisterCode,
			FiscalDevice:      sale.FiscalDevice,
			Lines:             sale.Lines,
			Payments:          sale.Payments,
		})
	}
	return rows
}

func renderExport(req ExportRequest, rows []exportRow, currency string) ([]byte, string, error) {
	headers, records := normativeExportTable(req.Type, rows)
	switch req.Format {
	case "JSON":
		return renderJSON(req, headers, records, rows, currency)
	case "CSV":
		return renderCSV(headers, records, currency)
	case "XLSX":
		return renderXLSX(headers, records, currency)
	default:
		return nil, "", errors.New("unsupported format")
	}
}

func renderJSON(req ExportRequest, headers []string, records [][]string, rows []exportRow, currency string) ([]byte, string, error) {
	doc := map[string]any{
		"schema_id":          fmt.Sprintf("BG_%s_V1", req.Type),
		"official_currency":  currency,
		"interval_semantics": "[from,to)",
		"from":               req.From.Format(time.RFC3339),
		"to":                 req.To.Format(time.RFC3339),
		"columns":            headers,
		"table":              records,
		"rows":               rows,
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, "", err
	}
	return b, "application/json", nil
}

func renderCSV(headers []string, records [][]string, currency string) ([]byte, string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	meta := []string{"official_currency=" + currency}
	_ = w.Write(meta)
	_ = w.Write(headers)
	for _, row := range records {
		_ = w.Write(row)
	}
	w.Flush()
	return buf.Bytes(), "text/csv", nil
}

func renderXLSX(headers []string, records [][]string, currency string) ([]byte, string, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// [Content_Types].xml
	ct, _ := zw.Create("[Content_Types].xml")
	_, _ = ct.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`))
	// _rels/.rels
	rels, _ := zw.Create("_rels/.rels")
	_, _ = rels.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`))
	// xl/_rels/workbook.xml.rels
	wbrels, _ := zw.Create("xl/_rels/workbook.xml.rels")
	_, _ = wbrels.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`))
	// xl/workbook.xml
	wb, _ := zw.Create("xl/workbook.xml")
	_, _ = wb.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`))
	// xl/worksheets/sheet1.xml
	ws, _ := zw.Create("xl/worksheets/sheet1.xml")
	var sbuf bytes.Buffer
	sbuf.WriteString(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	writeXLSXRow(&sbuf, 1, append([]string{"official_currency=" + currency}, headers...))
	for i, row := range records {
		writeXLSXRow(&sbuf, i+2, row)
	}
	sbuf.WriteString(`</sheetData></worksheet>`)
	_, _ = ws.Write(sbuf.Bytes())
	if err := zw.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
}

func writeXLSXRow(buf *bytes.Buffer, rowNum int, cells []string) {
	buf.WriteString(fmt.Sprintf(`<row r="%d">`, rowNum))
	for i, cell := range cells {
		col := xlsxColumnName(i)
		ref := fmt.Sprintf("%s%d", col, rowNum)
		escaped := escapeXML(cell)
		buf.WriteString(fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, escaped))
	}
	buf.WriteString(`</row>`)
}

func escapeXML(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// ensure math is used (needed for potential overflow checks)
var _ = math.MaxInt64

// ensure strings is used
var _ = strings.Contains
