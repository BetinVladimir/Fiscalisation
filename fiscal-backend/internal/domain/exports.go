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
	Format     string    `json:"format"`
}
type exportRow struct {
	SaleID     string `json:"sale_id"`
	ExternalID string `json:"external_id"`
	UNP        string `json:"unp"`
	RegisterID string `json:"register_id"`
	OperatorID string `json:"operator_id"`
	State      string `json:"state"`
	CreatedAt  string `json:"created_at"`
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
	if tenant == "" || !contains([]string{"SUPTO_18_1", "SUPTO_18_2", "SUPTO_18_3", "SUPTO_18_4", "SUPTO_18_5", "SUPTO_18_9", "KLEN", "FISCAL_MEMORY"}, in.Type) || !contains([]string{"JSON", "CSV", "XLSX"}, in.Format) || in.From.IsZero() || in.To.IsZero() || in.To.Before(in.From) {
		return Operation{}, errors.New("invalid export request")
	}
	rows := make([]exportRow, 0)
	for _, sale := range s.repo.Sales(tenant) {
		if sale.CreatedAt.Before(in.From) || sale.CreatedAt.After(in.To) || (in.RegisterID != "" && sale.RegisterID != in.RegisterID) || (in.OperatorID != "" && sale.OperatorID != in.OperatorID) {
			continue
		}
		rows = append(rows, exportRow{sale.ID, sale.ExternalID, sale.UNP, sale.RegisterID, sale.OperatorID, sale.State, sale.CreatedAt.Format(time.RFC3339Nano)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt < rows[j].CreatedAt })
	var artifact []byte
	media := "application/json"
	var err error
	switch in.Format {
	case "JSON":
		artifact, err = json.Marshal(map[string]any{"schema_version": "2026-08-07", "policy_version": "BG-2026-EUR", "official_currency": "EUR", "type": in.Type, "from": in.From, "to": in.To, "rows": rows})
	case "CSV":
		media = "text/csv"
		artifact, err = exportCSV(rows)
	case "XLSX":
		media = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		artifact, err = exportXLSX(rows)
	}
	if err != nil {
		return Operation{}, err
	}
	exportID, _ := newUUID()
	artifactID, _ := newUUID()
	now := time.Now().UTC()
	sum := sha256.Sum256(artifact)
	digest := hex.EncodeToString(sum[:])
	if err = s.repo.PutArtifact(artifactID, tenant, artifact); err != nil {
		return Operation{}, err
	}
	manifest := map[string]any{"artifact_id": artifactID, "media_type": media, "sha256": digest, "size": len(artifact), "created_at": now}
	data := map[string]any{"export_id": exportID, "state": "COMPLETED", "type": in.Type, "requested_at": now, "completed_at": now, "artifact": manifest, "official_currency": "EUR"}
	if err = s.repo.PutResource(ResourceRecord{Kind: "export", TenantID: tenant, ID: exportID, Version: 1, Data: data, CreatedAt: now, UpdatedAt: now}); err != nil {
		return Operation{}, err
	}
	op := Operation{ID: newID("op"), TenantID: tenant, Type: "COMPLIANCE_EXPORT", State: "FISCALIZED", Version: 2, FiscalReference: exportID, Simulated: true, AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	return op, s.repo.PutOperation(op)
}

// CreatePeriodizedExport is the additive BG-020 export path. Its interval is
// deliberately half-open [from,to), so the legal BGN/EUR boundary can never
// duplicate or omit a sale. The locked canonical ComplianceExport remains
// EUR-only and is not widened with undocumented fields.
func (s *Service) CreatePeriodizedExport(in ExportRequest, tenant string) (Operation, error) {
	if tenant == "" || !contains([]string{"SUPTO_18_1", "SUPTO_18_2", "SUPTO_18_3", "SUPTO_18_4", "SUPTO_18_5", "SUPTO_18_9", "KLEN", "FISCAL_MEMORY"}, in.Type) || !contains([]string{"JSON", "CSV", "XLSX"}, in.Format) || in.From.IsZero() || in.To.IsZero() || !in.To.After(in.From) {
		return Operation{}, errors.New("invalid periodized export request")
	}
	periods := splitOfficialCurrencyPeriods(in.From, in.To)
	exportID, _ := newUUID()
	now := time.Now().UTC()
	periodManifests := make([]map[string]any, 0, len(periods))
	for _, period := range periods {
		rows := s.exportRows(in, tenant, period.From, period.To)
		artifact, media, err := renderExportArtifact(in, rows, period.Currency, period.From, period.To)
		if err != nil {
			return Operation{}, err
		}
		artifactID, _ := newUUID()
		if err = s.repo.PutArtifact(artifactID, tenant, artifact); err != nil {
			return Operation{}, err
		}
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
	if err := s.repo.PutResource(ResourceRecord{Kind: "export_periods", TenantID: tenant, ID: exportID, Version: 1, Data: data, CreatedAt: now, UpdatedAt: now}); err != nil {
		return Operation{}, err
	}
	op := Operation{ID: newID("op"), TenantID: tenant, Type: "COMPLIANCE_EXPORT", State: "FISCALIZED", Version: 2, FiscalReference: exportID, Simulated: true, AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	return op, s.repo.PutOperation(op)
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

func (s *Service) exportRows(in ExportRequest, tenant string, from, to time.Time) []exportRow {
	rows := make([]exportRow, 0)
	for _, sale := range s.repo.Sales(tenant) {
		if sale.CreatedAt.Before(from) || !sale.CreatedAt.Before(to) || (in.RegisterID != "" && sale.RegisterID != in.RegisterID) || (in.OperatorID != "" && sale.OperatorID != in.OperatorID) {
			continue
		}
		rows = append(rows, exportRow{sale.ID, sale.ExternalID, sale.UNP, sale.RegisterID, sale.OperatorID, sale.State, sale.CreatedAt.Format(time.RFC3339Nano)})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt == rows[j].CreatedAt {
			return rows[i].SaleID < rows[j].SaleID
		}
		return rows[i].CreatedAt < rows[j].CreatedAt
	})
	return rows
}

func renderExportArtifact(in ExportRequest, rows []exportRow, currency string, from, to time.Time) ([]byte, string, error) {
	switch in.Format {
	case "JSON":
		artifact, err := json.Marshal(map[string]any{
			"schema_version": "2026-08-07", "policy_version": "BG-EUR-TRANSITION-2026-01-01",
			"official_currency": currency, "type": in.Type, "from_inclusive": from.UTC(), "to_exclusive": to.UTC(), "rows": rows,
		})
		return artifact, "application/json", err
	case "CSV":
		artifact, err := exportPeriodCSV(rows, currency, from, to)
		return artifact, "text/csv", err
	case "XLSX":
		artifact, err := exportPeriodXLSX(rows, currency, from, to)
		return artifact, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", err
	default:
		return nil, "", errors.New("invalid export format")
	}
}

func exportPeriodCSV(rows []exportRow, currency string, from, to time.Time) ([]byte, error) {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"official_currency", "period_from_inclusive", "period_to_exclusive", "sale_id", "external_id", "unp", "register_id", "operator_id", "state", "created_at"})
	if len(rows) == 0 {
		_ = w.Write([]string{currency, from.UTC().Format(time.RFC3339Nano), to.UTC().Format(time.RFC3339Nano), "", "", "", "", "", "", ""})
	}
	for _, r := range rows {
		_ = w.Write([]string{currency, from.UTC().Format(time.RFC3339Nano), to.UTC().Format(time.RFC3339Nano), r.SaleID, r.ExternalID, r.UNP, r.RegisterID, r.OperatorID, r.State, r.CreatedAt})
	}
	w.Flush()
	return b.Bytes(), w.Error()
}

func exportPeriodXLSX(rows []exportRow, currency string, from, to time.Time) ([]byte, error) {
	// The period metadata is represented as a deterministic first record so an
	// empty period remains independently auditable in spreadsheet tooling.
	metadata := exportRow{SaleID: "official_currency=" + currency, ExternalID: "from=" + from.UTC().Format(time.RFC3339Nano), UNP: "to=" + to.UTC().Format(time.RFC3339Nano)}
	return exportXLSX(append([]exportRow{metadata}, rows...))
}
func exportCSV(rows []exportRow) ([]byte, error) {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"sale_id", "external_id", "unp", "register_id", "operator_id", "state", "created_at"})
	for _, r := range rows {
		_ = w.Write([]string{r.SaleID, r.ExternalID, r.UNP, r.RegisterID, r.OperatorID, r.State, r.CreatedAt})
	}
	w.Flush()
	return b.Bytes(), w.Error()
}
func exportXLSX(rows []exportRow) ([]byte, error) {
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	files := map[string]string{"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`, "_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`, "xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sales" sheetId="1" r:id="rId1"/></sheets></workbook>`, "xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`}
	values := [][]string{{"sale_id", "external_id", "unp", "register_id", "operator_id", "state", "created_at"}}
	for _, r := range rows {
		values = append(values, []string{r.SaleID, r.ExternalID, r.UNP, r.RegisterID, r.OperatorID, r.State, r.CreatedAt})
	}
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
