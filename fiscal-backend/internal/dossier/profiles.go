package dossier

import (
	"encoding/json"
	"errors"
	"fmt"
)

// MiniPOSTables is separate from Fiscal's tenant namespace and credentials.
var MiniPOSTables = []string{"minipos_runtime_checkout_hashes", "minipos_runtime_checkout_results", "minipos_runtime_employees", "minipos_runtime_orders", "minipos_runtime_products", "minipos_runtime_shifts", "minipos_runtime_tax_groups"}

type profile struct {
	name, schemaFormat, archiveFormat, setting, tenantColumn string
	tables, excluded                                         []string
	columns                                                  map[string][]string
}

func selectProfile(name string) (profile, error) {
	switch name {
	case "", "fiscal":
		return profile{name: "fiscal", schemaFormat: "beefiscal-financial-pg-schema-v1", archiveFormat: "beefiscal-financial-pg-extract-v1", setting: "app.tenant_id", tenantColumn: "tenant_id", tables: Tables, excluded: []string{"generic resources and credentials", "audit/outbox/replay payloads pending secret-safe export contract", "legacy/reporting typed tables", "outbound email and auth stores", "external artifact object bodies", "MiniPOS PostgreSQL", "S3/BlueCash/browser/native stores"}}, nil
	case "fiscal-evidence":
		return profile{name: "fiscal-evidence", schemaFormat: "beefiscal-evidence-pg-schema-v1", archiveFormat: "beefiscal-evidence-pg-extract-v1", setting: "app.tenant_id", tenantColumn: "tenant_id", tables: EvidenceTables, columns: map[string][]string{
			"fiscal_runtime_audit":     {"event_id", "tenant_id", "actor_id", "action", "object_type", "object_id", "occurred_at", "event_hash", "previous_hash", "payload"},
			"fiscal_runtime_resources": {"kind", "id", "tenant_id", "version", "data", "created_at", "updated_at"},
		}, excluded: []string{"credential/private-key/token/envelope payload/signature fields", "raw resource payload duplicate and unreviewed data keys", "unreviewed/credential kind payloads (metadata inventory retained)", "legacy audit_events/reporting/compatibility/activation/auth stores", "Fiscal financial tables require independent fiscal profile", "MiniPOS PostgreSQL/external objects/local/native stores"}}, nil
	case "minipos":
		return profile{name: "minipos", schemaFormat: "beeminipos-financial-pg-schema-v1", archiveFormat: "beeminipos-financial-pg-extract-v1", setting: "app.organization_id", tenantColumn: "organization_id", tables: MiniPOSTables, columns: map[string][]string{
			"minipos_runtime_checkout_hashes":  {"replay_key", "organization_id", "request_hash"},
			"minipos_runtime_checkout_results": {"replay_key", "organization_id", "order_id", "state", "version", "fiscal_sale_id", "fiscal_operation_id", "fiscal_reference"},
			"minipos_runtime_employees":        {"id", "organization_id", "first_name", "last_name", "operator_code", "active", "status", "version", "created_at", "updated_at"},
			"minipos_runtime_orders":           {"id", "organization_id", "external_id", "shift_id", "register_id", "operator_code", "state", "total", "currency", "lines", "fiscal_sale_id", "fiscal_operation_id", "fiscal_version", "version", "created_at", "updated_at", "reversal_operation_id", "reversal_fiscal_reference", "reversal_reason_code", "fiscal_reference", "payments"},
			"minipos_runtime_products":         {"id", "organization_id", "sku", "name", "unit", "amount", "currency", "tax_group", "active", "status", "version", "created_at", "updated_at", "barcode"},
			"minipos_runtime_shifts":           {"id", "organization_id", "register_id", "employee_id", "state", "version", "opened_at", "closed_at", "created_at", "updated_at", "z_operation_id", "z_fiscal_reference"},
			"minipos_runtime_tax_groups":       {"id", "organization_id", "code", "name", "rate", "status", "version", "created_at", "updated_at"},
		}, excluded: []string{"generic payload columns; unknown line/payment JSON keys", "employee roles and shift close_error", "identity bindings/operator sessions/auth credentials", "configurations and adapter endpoints", "API replays/webhook inbox/outbox", "legacy/reporting/compatibility tables", "Fiscal PostgreSQL and external artifact stores", "S3/BlueCash/browser/native stores"}}, nil
	default:
		return profile{}, errors.New("unknown extraction profile")
	}
}
func (p profile) exportColumns(t Table) []string {
	if p.columns != nil {
		return p.columns[t.Name]
	}
	var out []string
	for _, c := range t.Columns {
		out = append(out, c.Name)
	}
	return out
}
func (p profile) project(table, column string, cell *string) (*string, error) {
	if p.name != "minipos" || table != "minipos_runtime_orders" || (column != "lines" && column != "payments") || cell == nil {
		return cell, nil
	}
	var elements []json.RawMessage
	if err := json.Unmarshal([]byte(*cell), &elements); err != nil {
		return nil, err
	}
	fields := map[string]bool{"line_id": false, "product_id": false, "name": false, "quantity": false, "unit_price": true, "discount": true, "tax_group": false}
	if column == "payments" {
		fields = map[string]bool{"payment_id": false, "type": false, "amount": true, "terminal_policy": false}
	}
	out := make([]json.RawMessage, 0, len(elements))
	for _, element := range elements {
		clean, err := projectObject(element, fields)
		if err != nil {
			return nil, fmt.Errorf("%s.%s JSON projection: %w", table, column, err)
		}
		out = append(out, clean)
	}
	body, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	value := string(body)
	return &value, nil
}
func projectObject(raw json.RawMessage, fields map[string]bool) (json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, errors.New("object required")
	}
	clean := map[string]json.RawMessage{}
	for key, money := range fields {
		value, present := object[key]
		if !present {
			continue
		}
		if string(value) == "null" {
			clean[key] = value
			continue
		}
		if money {
			nested, err := projectObject(value, map[string]bool{"amount": false, "currency": false})
			if err != nil {
				return nil, err
			}
			clean[key] = nested
		} else {
			var text string
			if err := json.Unmarshal(value, &text); err != nil {
				return nil, fmt.Errorf("string field required: %s", key)
			}
			clean[key] = value
		}
	}
	return json.Marshal(clean)
}
