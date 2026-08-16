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
	if current == nil {
		if kind == "organization" {
			return s.UpsertOrganization(tenant, 0, data)
		}
		return s.CreateResource(kind, tenant, data)
	}
	expected, ok := numberInt64(current["version"])
	if !ok {
		return nil, errors.New("invalid stored resource version")
	}
	if kind == "organization" {
		return s.UpsertOrganization(tenant, expected, data)
	}
	return s.UpdateResource(kind, stringField(current, "id"), tenant, expected, data)
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
