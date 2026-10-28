package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// compositeExecution validates the complete non-secret schema3 execution profile.
// Passwords are delivered only inside the device-bound encrypted package.
func compositeExecution(data map[string]any, profile string) (map[string]any, error) {
	raw, e := json.Marshal(data["execution"])
	if e != nil {
		return nil, e
	}
	var v struct {
		Operator      int64                      `json:"operator_number"`
		Count         int64                      `json:"operator_count"`
		PasswordRef   string                     `json:"operator_password_ref"`
		Configuration string                     `json:"device_configuration_id"`
		Till          *int64                     `json:"till_number,omitempty"`
		Mapping       map[string]json.RawMessage `json:"payment_mapping"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(&v); e != nil {
		return nil, errors.New("invalid fiscal execution configuration")
	}
	var tail any
	if d.Decode(&tail) != io.EOF {
		return nil, errors.New("trailing execution configuration")
	}
	if v.Operator < 1 || v.Count < 1 || v.Count > 30 || v.Operator > v.Count || v.PasswordRef == "" || len(v.PasswordRef) > 15 || v.Configuration == "" || len(v.Configuration) > 128 {
		return nil, errors.New("incomplete fiscal execution configuration")
	}
	for _, c := range v.PasswordRef {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return nil, errors.New("invalid fiscal password reference")
		}
	}
	for k := range v.Mapping {
		if k != "CASH" && k != "CARD" {
			return nil, errors.New("unknown payment mapping")
		}
	}
	if _, ok := v.Mapping["CASH"]; !ok {
		return nil, errors.New("cash mapping required")
	}
	if profile == "DATECS_DP150_BLUEPAD50" {
		var cash int64
		if json.Unmarshal(v.Mapping["CASH"], &cash) != nil || cash != 0 || v.Count != 30 || v.Till == nil || *v.Till < 1 || *v.Till > 99999 {
			return nil, errors.New("invalid Datecs execution configuration")
		}
		if card, ok := v.Mapping["CARD"]; ok {
			var mode int64
			if json.Unmarshal(card, &mode) != nil || (mode != 1 && mode != 2) {
				return nil, errors.New("invalid Datecs card mapping")
			}
		}
	} else if profile == "DAISY_COMPACT_S01" {
		var cash string
		if json.Unmarshal(v.Mapping["CASH"], &cash) != nil || cash != "P" || v.Till != nil {
			return nil, errors.New("invalid Daisy execution configuration")
		}
		if card, ok := v.Mapping["CARD"]; ok {
			var code string
			if json.Unmarshal(card, &code) != nil || len(code) != 1 || !strings.Contains("NCDUBE", code) {
				return nil, errors.New("invalid Daisy card mapping")
			}
		}
	} else {
		return nil, errors.New("schema3 S3 profile unsupported")
	}
	canonical, _ := json.Marshal(v)
	var result map[string]any
	if e = json.Unmarshal(canonical, &result); e != nil {
		return nil, e
	}
	return result, nil
}
