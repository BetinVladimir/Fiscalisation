package dossier

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Verify checks accidental archive damage/inventory, not manifest authenticity.
func Verify(directory string) error {
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("ordinary archive directory required")
	}
	raw, err := readRegular(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return err
	}
	var m Manifest
	if err = json.Unmarshal(raw, &m); err != nil {
		return err
	}
	profileName := "fiscal"
	if m.Format == "beefiscal-evidence-pg-extract-v1" {
		profileName = "fiscal-evidence"
	}
	if m.Format == "beeminipos-financial-pg-extract-v1" {
		profileName = "minipos"
	}
	p, _ := selectProfile(profileName)
	if m.Format != p.archiveFormat || m.Tenant == "" {
		return errors.New("invalid manifest")
	}
	expected := map[string]bool{"dictionary.json": true}
	for _, name := range p.tables {
		expected[name+".jsonl"] = true
	}
	if len(m.Files) != len(expected) {
		return errors.New("archive inventory mismatch")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	if len(entries) != len(expected)+1 {
		return errors.New("extra/missing archive artifact")
	}
	coverage := map[string]int64{}
	var contract Contract
	for name, meta := range m.Files {
		if !expected[name] || strings.ContainsAny(name, "/\\") {
			return errors.New("unexpected artifact")
		}
		body, e := readRegular(filepath.Join(directory, name))
		if e != nil {
			return e
		}
		if digest(body) != meta.SHA256 {
			return errors.New("artifact hash mismatch")
		}
		if name == "dictionary.json" {
			if e = json.Unmarshal(body, &contract); e != nil {
				return e
			}
			continue
		}
		var count int64
		if len(body) > 0 {
			if body[len(body)-1] != '\n' {
				return errors.New("truncated JSONL")
			}
			for _, line := range strings.Split(string(body[:len(body)-1]), "\n") {
				var row []*string
				if e = json.Unmarshal([]byte(line), &row); e != nil {
					return e
				}
				count++
			}
		}
		if count != meta.Rows {
			return errors.New("row count mismatch")
		}
	}
	if contract.Schema != m.Schema || contract.Format != p.schemaFormat || len(contract.Tables) != len(p.tables) {
		return errors.New("dictionary mismatch")
	}
	for i, t := range contract.Tables {
		if t.Name != p.tables[i] {
			return errors.New("table inventory mismatch")
		}
		if p.columns != nil {
			want, _ := json.Marshal(p.exportColumns(t))
			got, _ := json.Marshal(t.ExportColumns)
			if string(want) != string(got) {
				return errors.New("projection inventory mismatch")
			}
		}
		body, e := readRegular(filepath.Join(directory, t.Name+".jsonl"))
		if e != nil {
			return e
		}
		if len(body) == 0 {
			continue
		}
		for _, line := range strings.Split(strings.TrimSuffix(string(body), "\n"), "\n") {
			var row []*string
			if e = json.Unmarshal([]byte(line), &row); e != nil {
				return e
			}
			if len(row) != len(p.exportColumns(t)) {
				return errors.New("column count mismatch")
			}
			if p.name == "fiscal-evidence" {
				key, e := evidenceProjectRow(t.Name, p.exportColumns(t), row)
				if e != nil {
					return e
				}
				coverage[key]++
			}
		}
	}
	if p.name == "fiscal-evidence" {
		a, _ := json.Marshal(coverage)
		b, _ := json.Marshal(m.Coverage)
		if string(a) != string(b) {
			return errors.New("evidence coverage inventory mismatch")
		}
	}
	return nil
}
func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("regular non-symlink artifact required")
	}
	return os.ReadFile(path)
}
