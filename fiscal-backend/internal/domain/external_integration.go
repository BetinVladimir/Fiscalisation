package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ApplyExternalResource projects a source-owned integration resource into the
// normal Fiscal domain repository. Source markers make the operation
// idempotent even when a worker crashes after the repository commit.
func (s *Service) ApplyExternalResource(tenant, system, method, kind, source string, sourceVersion int64, payload map[string]any) (map[string]any, error) {
	prepared, err := s.PrepareExternalResource(tenant, system, method, kind, source, sourceVersion, payload)
	if err != nil {
		return nil, err
	}
	record, ok := prepared["_projection_record"].(ResourceRecord)
	if !ok {
		return prepared, nil
	}
	if err = s.repo.PutResource(record); err != nil {
		return nil, err
	}
	delete(prepared, "_projection_record")
	return prepared, nil
}

// PrepareExternalResource validates and constructs the canonical domain row
// without persisting it. The Rabbit consumer commits this row together with the
// command result, integration audit and webhook delivery in one SQL transaction.
func (s *Service) PrepareExternalResource(tenant, system, method, kind, source string, sourceVersion int64, payload map[string]any) (map[string]any, error) {
	if tenant == "" || system == "" || source == "" || sourceVersion < 1 {
		return nil, errors.New("invalid external resource identity")
	}
	if kind != "organization" && kind != "location" && kind != "register" && kind != "operator" {
		return nil, errors.New("unsupported external resource")
	}
	var current map[string]any
	for _, item := range s.ListResources(kind, tenant) {
		if stringField(item, "external_system_id") == system && stringField(item, "external_source_id") == source {
			current = item
			break
		}
	}
	if current != nil {
		if v, ok := numberInt64(current["external_source_version"]); ok && v >= sourceVersion {
			return current, nil
		}
	}
	data, err := s.externalResourceData(tenant, system, method, kind, source, sourceVersion, payload, current)
	if err != nil {
		return nil, err
	}
	if err = validateResource(kind, data); err != nil {
		return nil, err
	}
	// Register references were resolved above by external source identity. The
	// repository cache may intentionally lag the atomically committed SQL row;
	// re-reading by internal ID here would incorrectly reject a valid dependency.
	if kind != "register" {
		if err = s.validateResourceReferences(kind, tenant, data); err != nil {
			return nil, err
		}
	}
	excludeID := ""
	if current != nil {
		excludeID = stringField(current, "id")
	}
	if err = s.ensureUniqueResource(kind, tenant, data, excludeID); err != nil {
		return nil, err
	}
	id, err := newUUID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	record := ResourceRecord{Kind: kind, TenantID: tenant, ID: id, Version: 1, Data: cloneMap(data), CreatedAt: now, UpdatedAt: now}
	if current != nil {
		record.ID = stringField(current, "id")
		record.CreatedAt, _ = current["created_at"].(time.Time)
		if record.CreatedAt.IsZero() {
			record.CreatedAt = now
		}
		expected, ok := numberInt64(current["version"])
		if !ok {
			return nil, errors.New("invalid stored resource version")
		}
		record.Version = expected + 1
	}
	out := publicResource(record)
	out["_projection_record"] = record
	return out, nil
}

func numberInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), n == float64(int64(n))
	default:
		return 0, false
	}
}
func firstString(v map[string]any, keys ...string) string {
	for _, k := range keys {
		if x := stringField(v, k); x != "" {
			return x
		}
	}
	return ""
}

func (s *Service) externalResourceData(tenant, system, method, kind, source string, version int64, payload, current map[string]any) (map[string]any, error) {
	data := map[string]any{"external_system_id": system, "external_source_id": source, "external_source_version": version}
	inactive := strings.EqualFold(method, "DELETE")
	switch kind {
	case "organization":
		company := payload
		if nested, ok := payload["company"].(map[string]any); ok {
			company = nested
		}
		data["legal_name"] = firstString(company, "legal_name", "name")
		data["eik"] = firstString(company, "eik", "tax_identifier", "tax_value")
		if tax, ok := company["tax_identifier"].(map[string]any); ok {
			data["eik"] = firstString(tax, "value")
			data["tax_identifier_type"] = strings.ToUpper(firstString(tax, "type"))
			data["country"] = strings.ToUpper(firstString(tax, "country"))
			data["tax_identifier_normalized"] = strings.NewReplacer("-", "", ".", "", "/", "", " ", "").Replace(strings.ToUpper(firstString(tax, "value")))
		}
		data["country"] = strings.ToUpper(firstString(company, "country", "tax_country"))
		if data["country"] == "" {
			data["country"] = "BG"
		}
		data["status"] = map[bool]string{true: "SUSPENDED", false: "ACTIVE"}[inactive]
	case "location":
		data["code"] = firstString(payload, "code")
		if data["code"] == "" {
			data["code"] = source
		}
		data["name"] = firstString(payload, "name", "display_name")
		data["address"] = firstString(payload, "address")
		if data["address"] == "" {
			data["address"] = "Not provided"
		}
		data["status"] = map[bool]string{true: "INACTIVE", false: "ACTIVE"}[inactive]
	case "register":
		locationSource := firstString(payload, "location_source_id", "location_id")
		var locationID string
		for _, item := range s.ListResources("location", tenant) {
			if stringField(item, "external_system_id") == system && stringField(item, "external_source_id") == locationSource {
				locationID = stringField(item, "id")
				break
			}
		}
		if locationID == "" {
			return nil, fmt.Errorf("referenced location %q not synchronized", locationSource)
		}
		data["location_id"] = locationID
		data["code"] = firstString(payload, "code", "name")
		if data["code"] == "" {
			data["code"] = source
		}
		data["status"] = map[bool]string{true: "INACTIVE", false: "ACTIVE"}[inactive]
	case "operator":
		data["code"] = firstString(payload, "code", "operator_code")
		data["first_name"] = firstString(payload, "first_name")
		data["last_name"] = firstString(payload, "last_name")
		data["roles"] = payload["roles"]
		if data["roles"] == nil {
			data["roles"] = []any{"CASHIER"}
		}
		data["active_from"] = firstString(payload, "active_from", "created_at")
		if data["active_from"] == "" {
			data["active_from"] = time.Now().UTC().Format(time.RFC3339)
		}
		if inactive {
			data["active_to"] = time.Now().UTC().Format(time.RFC3339)
		} else if current != nil && current["active_to"] != nil {
			data["active_to"] = current["active_to"]
		}
	}
	return data, nil
}
