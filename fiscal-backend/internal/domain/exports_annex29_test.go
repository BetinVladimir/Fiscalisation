package domain

import (
	"encoding/json"
	"os"
	"testing"
)

type annex29Schema struct {
	Tables       map[string][]string `json:"tables"`
	TotalColumns int                 `json:"total_columns"`
}

func TestAnnex29Columns(t *testing.T) {
	data, err := os.ReadFile("testdata/annex29_columns.json")
	if err != nil {
		t.Fatalf("cannot read annex29_columns.json: %v", err)
	}

	var schema annex29Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("cannot parse annex29_columns.json: %v", err)
	}

	requiredTables := []string{"18.1", "18.2", "18.3", "18.4", "18.5", "18.9"}
	for _, tbl := range requiredTables {
		cols, ok := schema.Tables[tbl]
		if !ok {
			t.Errorf("missing table %s in annex29_columns.json", tbl)
			continue
		}
		if len(cols) == 0 {
			t.Errorf("table %s has no columns", tbl)
		}
	}

	if schema.TotalColumns < 10 {
		t.Errorf("total_columns=%d, want >= 10", schema.TotalColumns)
	}

	computed := 0
	for _, cols := range schema.Tables {
		computed += len(cols)
	}
	if computed != schema.TotalColumns {
		t.Errorf("total_columns field %d != computed sum %d", schema.TotalColumns, computed)
	}
}
