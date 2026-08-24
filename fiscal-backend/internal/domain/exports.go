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
		row, rowErr := detailedExportRow(sale, "EUR")
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
		artifact, err = json.Marshal(map[string]any{"schema_version": "2026-08-24", "schema_id": exportSchemaID(in.Type), "columns": headers, "policy_version": "BG-N18-APP29", "official_currency": "EUR", "interval_semantics": "[from,to)", "type": in.Type, "from": in.From, "to": in.To, "records": records})
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
		row, err := detailedExportRow(sale, currency)
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

func detailedExportRow(sale Sale, currency string) (exportRow, error) {
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
	return exportRow{
		SaleID: sale.ID, ExternalID: sale.ExternalID, UNP: sale.UNP,
		LocationID: sale.LocationID, RegisterID: sale.RegisterID, OperatorID: sale.OperatorID, State: sale.State,
		FiscalOperationID: sale.FiscalOperationID, ReceiptArtifactID: sale.ReceiptArtifactID,
		FiscalDevice:     sale.FiscalDevice,
		OfficialCurrency: currency, Total: Money{Amount: formatFixed(total), Currency: currency},
		Lines: lines, Payments: payments, CreatedAt: sale.CreatedAt.Format(time.RFC3339Nano),
	}, nil
}

func renderExportArtifact(in ExportRequest, rows []exportRow, currency string, from, to time.Time) ([]byte, string, error) {
	switch in.Format {
	case "JSON":
		headers, records := normativeExportTable(in.Type, rows)
		artifact, err := json.Marshal(map[string]any{
			"schema_version": "2026-08-24", "schema_id": exportSchemaID(in.Type), "columns": headers, "policy_version": "BG-N18-APP29",
			"official_currency": currency, "type": in.Type, "from_inclusive": from.UTC(), "to_exclusive": to.UTC(), "records": records,
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
	headers,records:=normativeExportTable(exportType,rows)
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
	var headers []string
	var records [][]string
	switch exportType {
	case "SUPTO_18_1":
		headers=[]string{"sale_id","external_id","unp","location_id","register_id","operator_id","state","created_at"}
		for _,r:=range rows{records=append(records,[]string{r.SaleID,r.ExternalID,r.UNP,r.LocationID,r.RegisterID,r.OperatorID,r.State,r.CreatedAt})}
	case "SUPTO_18_2":
		headers=[]string{"sale_id","unp","payment_id","payment_type","amount_json","fiscal_reference","created_at"}
		for _,r:=range rows{for _,payment:=range r.Payments{amount,_:=json.Marshal(payment.Amount);records=append(records,[]string{r.SaleID,r.UNP,payment.PaymentID,payment.Type,string(amount),payment.FiscalReference,payment.CreatedAt.UTC().Format(time.RFC3339Nano)})}}
	case "SUPTO_18_3":
		headers=[]string{"sale_id","unp","line_id","name","quantity","unit_price_json","discount_json","tax_group"}
		for _,r:=range rows{for _,line:=range r.Lines{price,_:=json.Marshal(line.UnitPrice);discount,_:=json.Marshal(line.Discount);records=append(records,[]string{r.SaleID,r.UNP,line.LineID,line.Name,line.Quantity,string(price),string(discount),line.TaxGroup})}}
	case "SUPTO_18_4":
		headers=[]string{"sale_id","unp","fiscal_operation_id","receipt_artifact_id","fiscal_device_json","state"}
		for _,r:=range rows{device,_:=json.Marshal(r.FiscalDevice);records=append(records,[]string{r.SaleID,r.UNP,r.FiscalOperationID,r.ReceiptArtifactID,string(device),r.State})}
	case "SUPTO_18_5":
		headers=[]string{"sale_id","unp","operator_id","location_id","register_id","created_at"}
		for _,r:=range rows{records=append(records,[]string{r.SaleID,r.UNP,r.OperatorID,r.LocationID,r.RegisterID,r.CreatedAt})}
	case "SUPTO_18_6":
		headers=[]string{"delivery_id","delivery_date","delivery_time","operator_code","supplier_code","supplier_name","invoice_number","invoice_date","net_amount","discount_amount","vat_amount","gross_amount","payment_type"}
	case "SUPTO_18_7":
		headers=[]string{"delivery_id","product_code","product_name","quantity","unit_price","discount_amount","vat_amount","gross_amount"}
	case "SUPTO_18_8":
		headers=[]string{"product_code","product_name","opening_quantity","opening_value","debit_quantity","debit_value","credit_quantity","credit_value","closing_quantity","closing_value"}
	case "SUPTO_18_9":
		headers=exportCSVHeader();for _,r:=range rows{records=append(records,exportCSVRecord(r))}
	default:
		headers=exportCSVHeader();for _,r:=range rows{records=append(records,exportCSVRecord(r))}
	}
	return headers,records
}

func exportNormativeCSV(exportType string,rows []exportRow)([]byte,error){headers,records:=normativeExportTable(exportType,rows);var b bytes.Buffer;w:=csv.NewWriter(&b);_ = w.Write(headers);for _,record:=range records{_ = w.Write(record)};w.Flush();return b.Bytes(),w.Error()}

func exportNormativeXLSX(exportType string,rows []exportRow)([]byte,error){headers,records:=normativeExportTable(exportType,rows);return exportTableXLSX(headers,records)}

func exportPeriodXLSX(exportType string,rows []exportRow, currency string, from, to time.Time) ([]byte, error) {
	headers,records:=normativeExportTable(exportType,rows);prefix:=[]string{"period_official_currency","period_from_inclusive","period_to_exclusive"};values:=make([][]string,0,len(records));for _,record:=range records{values=append(values,append([]string{currency,from.UTC().Format(time.RFC3339Nano),to.UTC().Format(time.RFC3339Nano)},record...))};if len(values)==0{values=append(values,append([]string{currency,from.UTC().Format(time.RFC3339Nano),to.UTC().Format(time.RFC3339Nano)},make([]string,len(headers))...))};return exportTableXLSX(append(prefix,headers...),values)
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
	values:=make([][]string,0,len(rows));for _,r:=range rows{values=append(values,exportCSVRecord(r))};return exportTableXLSX(exportCSVHeader(),values)
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
			sheet.WriteString(fmt.Sprintf(`<c r="%c%d" t="inlineStr"><is><t>%s</t></is></c>`, 'A'+j, i+1, html.EscapeString(v)))
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
