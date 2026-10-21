package domain

import (
	"bytes"
	"encoding/json"
)

// Recovery reads may omit historically optional empty maps, but cannot silently
// erase acknowledged collections. Constructor/migration acceptance is unchanged.
func validRecoverySnapshotShape(raw, acknowledged []byte) bool {
	var next, previous map[string]json.RawMessage
	if json.Unmarshal(raw, &next) != nil || next == nil || len(next) == 0 {
		return false
	}
	_ = json.Unmarshal(acknowledged, &previous)
	maps := []string{"products", "employees", "shifts", "orders", "checkouts", "checkout_hashes", "api_replays", "webhook_inbox", "configurations", "tax_groups", "identity_bindings", "operator_sessions"}
	optional := map[string]bool{"tax_groups": true, "identity_bindings": true, "operator_sessions": true}
	recognized := false
	for _, key := range maps {
		old, existed := previous[key]
		value, present := next[key]
		if !present {
			if existed && !(optional[key] && emptyRecoveryMap(old)) {
				return false
			}
			continue
		}
		recognized = true
		var entries map[string]json.RawMessage
		if json.Unmarshal(value, &entries) != nil {
			return false
		}
		if entries == nil && !(optional[key] && emptyRecoveryMap(old) && bytes.Equal(bytes.TrimSpace(value), []byte("null"))) {
			return false
		}
	}
	if _, existed := previous["sequence"]; existed {
		if _, present := next["sequence"]; !present {
			return false
		}
	}
	if value, present := next["sequence"]; present {
		var sequence uint64
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) || json.Unmarshal(value, &sequence) != nil {
			return false
		}
	}
	return recognized
}
func emptyRecoveryMap(raw []byte) bool {
	if len(raw) == 0 {
		return true
	}
	var entries map[string]json.RawMessage
	return json.Unmarshal(raw, &entries) == nil && len(entries) == 0
}
