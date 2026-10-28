package dossier

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// EvidenceRule is a reviewed allowlist, not a general redactor. Unknown fields
// never cross the PostgreSQL connection: SQL projection precedes Go validation.
type EvidenceRule struct {
	Type   string                  `json:"type"`
	Fields map[string]EvidenceRule `json:"fields,omitempty"`
	Items  *EvidenceRule           `json:"items,omitempty"`
}

var EvidenceTables = []string{"fiscal_runtime_audit", "fiscal_runtime_resources"}
var excludedEvidenceKinds = []string{"provisioning_session", "provisioning_delivery", "device_command_outbox", "webhook_endpoint"}

func erObject() EvidenceRule { return EvidenceRule{Type: "object", Fields: map[string]EvidenceRule{}} }
func erFields(text string) EvidenceRule {
	r := erObject()
	for _, field := range strings.Fields(text) {
		r.Fields[field] = EvidenceRule{Type: "string"}
	}
	return r
}
func erTypes(r EvidenceRule, typ, fields string) EvidenceRule {
	for _, field := range strings.Fields(fields) {
		r.Fields[field] = EvidenceRule{Type: typ}
	}
	return r
}
func erArray(r EvidenceRule) EvidenceRule { return EvidenceRule{Type: "array", Items: &r} }
func evidenceRules() map[string]EvidenceRule {
	money := erFields("amount currency")
	line := erFields("line_id product_code name quantity tax_group")
	line.Fields["unit_price"] = money
	line.Fields["discount"] = money
	payment := erFields("payment_id type fiscal_reference created_at")
	payment.Fields["amount"] = money
	fiscal := erFields("adapter_device_id device_id serial fiscal_device_number fiscal_memory_number vendor model firmware")
	fiscal = erTypes(fiscal, "integer", "binding_generation binding_version")
	sale := erFields("sale_id tenant_id external_id location_id register_id operator_id location_code location_name register_code operator_code operator_name unp state fiscal_operation_id receipt_artifact_id created_at updated_at completed_at cancelled_at reversed_at operation_id command_type edge_id server_sale_id client_sale_surrogate_id receipt_session_id payload_sha256 binding_sha256")
	sale = erTypes(sale, "integer", "version binding_generation")
	sale.Fields["lines"] = erArray(line)
	sale.Fields["payments"] = erArray(payment)
	sale.Fields["fiscal_device"] = fiscal
	totals := erObject()
	totals.Fields["total"] = money
	sale.Fields["totals"] = totals
	operation := erFields("operation_id client_operation_id receipt_session_id tenant_id sale_id register_id type state fiscal_reference original_fiscal_reference reason_code original_document_at error_code created_at updated_at")
	operation = erTypes(operation, "integer", "version original_document_number")
	operation = erTypes(operation, "boolean", "simulated")
	operator := erFields("code first_name last_name active_from active_to actor_subject")
	operator.Fields["roles"] = erArray(EvidenceRule{Type: "role"})
	register := erFields("location_id code status fiscal_device_id payment_terminal_id composite_binding_id fiscal_device_active_from payment_terminal_active_from")
	device := erFields("kind vendor model serial firmware status environment fiscal_device_number fiscal_memory_number device_key_thumbprint credential_id transaction_signing_kid unp_prefix")
	device = erTypes(device, "integer", "binding_version unp_range_start unp_range_end")
	device = erTypes(device, "boolean", "simulated")
	binding := erFields("register_id location_id adapter_device_id fiscal_device_id payment_device_id profile status payload_sha256 bound_at activated_at revoked_at")
	binding = erTypes(binding, "integer", "generation")
	authority := erFields("unp_prefix transaction_signing_kid")
	authority = erTypes(authority, "integer", "unp_range_start unp_range_end")
	endpoint := erFields("device_id fiscal_device_number fiscal_memory_number vendor model firmware protocol_profile")
	canonical := erFields("tenant_id location_id register_id edge_device_id profile")
	canonical = erTypes(canonical, "integer", "schema_version generation")
	canonical.Fields["fiscal_endpoint"] = endpoint
	canonical.Fields["operational_authority"] = authority
	issued := erFields("payload_sha256 key_id issued_at")
	issued.Fields["canonical_payload"] = canonical
	envelope := erFields("algorithm key_id sha256") // payload/signature together are an execution grant; never exported.
	execution := erFields("issued_binding_sha256 issuance_sale_id issuance_operation_id")
	execution = erTypes(execution, "integer", "issuance_sale_version issuance_operation_version")
	execution.Fields["envelope"] = envelope
	reconAuth := erFields("issuance_pending_digest issuance_operation_id issuance_binding_id")
	reconAuth = erTypes(reconAuth, "integer", "issuance_binding_version")
	reconAuth.Fields["envelope"] = envelope
	reconPayload := erFields("reconciliation_id terminal_id payment_id operation_id actor_id evidence_reference evidence_sha256 authorization_sha256 old_state new_state amount_minor terminal_reference rrn auth_code reversal_reference reversal_rrn reversal_auth_code")
	event := erFields("event_id operation_id device_id event_type occurred_at event_hash")
	event = erTypes(event, "integer", "journal_seq")
	event.Fields["payload"] = reconPayload
	recon := erFields("grant_sha256 issued_resource_digest issuance_pending_digest original_operation_id")
	recon.Fields["event"] = event
	artifact := erFields("artifact_id media_type sha256 created_at")
	artifact = erTypes(artifact, "integer", "size")
	report := erFields("register_id type state requested_at completed_at policy_version official_currency")
	report.Fields["artifacts"] = erArray(artifact)
	session := erFields("session_id tenant_id location_id workstation_id operator_id operator_code app_instance_id actor_subject status started_at expires_at revoked_at")
	rules := map[string]EvidenceRule{
		"sale": sale, "operation": operation, "organization": erFields("legal_name eik country status"), "location": erFields("code name address status"), "register": register, "operator": operator, "device": device, "composite_binding": binding, "issued_binding": issued, "execution_authorization": execution, "execution_authorization_revocation": erFields("effective_at revoked_at actor_id reason_code"), "payment_reconciliation_authorization": reconAuth, "payment_reconciliation": recon, "report": report, "workstation_session": session,
		"delegated_sale_request": erFields("system_id operator_id register_id source_shift_id plan_sha256"), "delegated_shift": erTypes(erFields("system_id source_shift_id fiscal_shift_id source_assertion operator_id register_id"), "boolean", "physical_verified"), "unp_serial_identity": erFields("fiscal_device_number fiscal_memory_number"),
	}
	return rules
}
func evidenceStatus(kind string) string {
	if _, ok := evidenceRules()[kind]; ok {
		return "PROJECTED_ALLOWLIST_V1"
	}
	for _, excluded := range excludedEvidenceKinds {
		if kind == excluded {
			return "EXCLUDED_CREDENTIAL_KIND"
		}
	}
	return "EXCLUDED_UNREVIEWED_KIND"
}
func sqlLiteral(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// SQL pruning occurs inside PostgreSQL: raw private-key/token/envelope fields are
// not selected into the export process. Invalid known types trigger SQL/Go refusal.
func ruleSQL(expr string, rule EvidenceRule, depth int) string {
	alias := fmt.Sprintf("ev%d", depth)
	var valid string
	switch rule.Type {
	case "object":
		keys := []string{}
		for key := range rule.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		branches := []string{}
		quoted := []string{}
		for _, key := range keys {
			quoted = append(quoted, sqlLiteral(key))
			branches = append(branches, "WHEN "+sqlLiteral(key)+" THEN "+ruleSQL(alias+".value", rule.Fields[key], depth+1))
		}
		valid = "(SELECT coalesce(jsonb_object_agg(" + alias + ".key, CASE " + alias + ".key " + strings.Join(branches, " ") + " END),'{}'::jsonb) FROM jsonb_each(" + expr + ") AS " + alias + " WHERE " + alias + ".key IN (" + strings.Join(quoted, ",") + "))"
	case "array":
		valid = "(SELECT coalesce(jsonb_agg(" + ruleSQL(alias+".value", *rule.Items, depth+1) + " ORDER BY " + alias + ".ordinality),'[]'::jsonb) FROM jsonb_array_elements(" + expr + ") WITH ORDINALITY AS " + alias + "(value,ordinality))"
	default:
		valid = expr
	}
	expected := rule.Type
	if expected == "integer" {
		expected = "number"
	}
	if expected == "role" {
		expected = "string"
	}
	// Invalid known types yield a type-only marker; Go rejects it without ever receiving unapproved contents.
	return "CASE WHEN " + expr + " IS NULL OR " + expr + "='null'::jsonb THEN " + expr + " WHEN jsonb_typeof(" + expr + ")=" + sqlLiteral(expected) + " THEN " + valid + " ELSE jsonb_build_object('__invalid_type__',jsonb_typeof(" + expr + ")) END"
}
func auditSnapshotSQL(expr string, rule EvidenceRule) string {
	wrapper := erFields("kind tenant_id id created_at updated_at")
	wrapper = erTypes(wrapper, "integer", "version")
	wrapper.Fields["data"] = rule
	return "CASE WHEN jsonb_typeof(" + expr + ")='object' AND " + expr + " ? 'data' AND " + expr + " ? 'kind' THEN " + ruleSQL(expr, wrapper, 1) + " ELSE " + ruleSQL(expr, rule, 1) + " END"
}
func evidenceExpression(table, column string) string {
	if (table != "fiscal_runtime_resources" || column != "data") && (table != "fiscal_runtime_audit" || column != "payload") {
		return identifier(column) + "::text"
	}
	kind := "kind"
	if table == "fiscal_runtime_audit" {
		kind = "object_type"
	}
	rules := evidenceRules()
	keys := []string{}
	for key := range rules {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	branches := []string{}
	for _, key := range keys {
		fields := ruleSQL("data", rules[key], 0)
		if table == "fiscal_runtime_audit" {
			fields = "jsonb_build_object('unp'," + ruleSQL("payload->'unp'", EvidenceRule{Type: "string"}, 0) + ",'previous_hash'," + ruleSQL("payload->'previous_hash'", EvidenceRule{Type: "string"}, 0) + ",'prev_hash'," + ruleSQL("payload->'prev_hash'", EvidenceRule{Type: "string"}, 0) + ",'before'," + auditSnapshotSQL("payload->'before'", rules[key]) + ",'after'," + auditSnapshotSQL("payload->'after'", rules[key]) + ")"
		}
		branches = append(branches, "WHEN "+sqlLiteral(key)+" THEN jsonb_build_object('projection_status','PROJECTED_ALLOWLIST_V1','fields',"+fields+")")
	}
	excluded := []string{}
	for _, key := range excludedEvidenceKinds {
		excluded = append(excluded, sqlLiteral(key))
	}
	fallback := "jsonb_build_object('projection_status',CASE WHEN " + kind + " IN (" + strings.Join(excluded, ",") + ") THEN 'EXCLUDED_CREDENTIAL_KIND' ELSE 'EXCLUDED_UNREVIEWED_KIND' END,'fields',null)"
	result := "(CASE " + identifier(kind) + " " + strings.Join(branches, " ") + " ELSE " + fallback + " END)"
	if table == "fiscal_runtime_audit" {
		result = "(" + result + " || jsonb_build_object('source_metadata'," + ruleSQL("payload", erFields("unp previous_hash prev_hash"), 0) + "))"
	}
	return result + "::text"
}
func validateEvidence(raw json.RawMessage, rule EvidenceRule) error {
	if string(raw) == "null" {
		return nil
	}
	switch rule.Type {
	case "object":
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return errors.New("expected projected object")
		}
		if object == nil {
			return errors.New("expected projected object")
		}
		for key, value := range object {
			child, ok := rule.Fields[key]
			if !ok {
				return errors.New("unexpected projected field")
			}
			if err := validateEvidence(value, child); err != nil {
				return fmt.Errorf("invalid projected field %s: %w", key, err)
			}
		}
	case "array":
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return errors.New("expected projected array")
		}
		for _, item := range items {
			if err := validateEvidence(item, *rule.Items); err != nil {
				return err
			}
		}
	case "string", "role":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return errors.New("expected projected string")
		}
		if rule.Type == "role" {
			switch value {
			case "CASHIER", "SUPERVISOR", "ADMIN", "AUDITOR", "SERVICE":
			default:
				return errors.New("unreviewed operator role")
			}
		}
	case "integer":
		if !regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`).Match(raw) {
			return errors.New("expected exact integer")
		}
	case "boolean":
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return errors.New("expected projected boolean")
		}
	default:
		return errors.New("unknown projection rule")
	}
	return nil
}
func evidenceProjectRow(table string, names []string, cells []*string) (string, error) {
	values := map[string]*string{}
	for i, name := range names {
		values[name] = cells[i]
	}
	kindColumn, payloadColumn := "kind", "data"
	if table == "fiscal_runtime_audit" {
		kindColumn, payloadColumn = "object_type", "payload"
	}
	if values[kindColumn] == nil || values[payloadColumn] == nil {
		return "", errors.New("missing evidence identity/projection")
	}
	kind := *values[kindColumn]
	status := evidenceStatus(kind)
	var wrapper struct {
		Status         string          `json:"projection_status"`
		Fields         json.RawMessage `json:"fields"`
		SourceMetadata json.RawMessage `json:"source_metadata"`
	}
	if err := json.Unmarshal([]byte(*values[payloadColumn]), &wrapper); err != nil {
		return "", err
	}
	if wrapper.Status != status {
		return "", errors.New("evidence status mismatch")
	}
	if table == "fiscal_runtime_audit" {
		if err := validateEvidence(wrapper.SourceMetadata, erFields("unp previous_hash prev_hash")); err != nil {
			return "", err
		}
	}
	if status != "PROJECTED_ALLOWLIST_V1" {
		if string(wrapper.Fields) != "null" {
			return "", errors.New("excluded evidence payload present")
		}
		return table + ":" + kind + ":" + status, nil
	}
	rule := evidenceRules()[kind]
	if table == "fiscal_runtime_audit" {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(wrapper.Fields, &fields); err != nil {
			return "", err
		}
		for _, name := range []string{"unp", "previous_hash", "prev_hash"} {
			if err := validateEvidence(fields[name], EvidenceRule{Type: "string"}); err != nil {
				return "", err
			}
		}
		for _, name := range []string{"before", "after"} {
			raw := fields[name]
			if string(raw) == "null" {
				continue
			}
			var object map[string]json.RawMessage
			if err := json.Unmarshal(raw, &object); err != nil {
				return "", err
			}
			snapshotRule := rule
			if _, wrapped := object["data"]; wrapped {
				snapshotRule = erFields("kind tenant_id id created_at updated_at")
				snapshotRule = erTypes(snapshotRule, "integer", "version")
				snapshotRule.Fields["data"] = rule
				var actualKind string
				if err := json.Unmarshal(object["kind"], &actualKind); err != nil || actualKind != kind {
					return "", errors.New("audit resource wrapper kind mismatch")
				}
			}
			if err := validateEvidence(raw, snapshotRule); err != nil {
				return "", err
			}
		}
		pointer := ""
		for _, name := range []string{"previous_hash", "prev_hash"} {
			var v string
			if string(fields[name]) != "null" {
				if err := json.Unmarshal(fields[name], &v); err != nil {
					return "", err
				}
			}
			if v != "" {
				if pointer != "" && pointer != v {
					return table + ":" + kind + ":PROJECTED_AMBIGUOUS_HASH_POINTER", nil
				}
				pointer = v
			}
		}
		if values["previous_hash"] != nil && *values["previous_hash"] != pointer {
			return table + ":" + kind + ":PROJECTED_HASH_POINTER_MISMATCH", nil
		}
	} else if err := validateEvidence(wrapper.Fields, rule); err != nil {
		return "", err
	}
	return table + ":" + kind + ":" + status, nil
}
