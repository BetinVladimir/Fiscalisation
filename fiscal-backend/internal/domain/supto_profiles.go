package domain

import (
	"encoding/json"
	"fmt"
	"os"
)

// SutpoProfile describes the audit/operator capabilities for one fiscal device profile.
// The JSON fixture lives in docs/MVP1/fixtures/supto_profiles.json.
type SutpoProfile struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	DisabledOperator  bool     `json:"disabled_operator"`
	ReadOnlyAuditor   bool     `json:"read_only_auditor"`
	AuditEvents       []string `json:"audit_events"`
	Unsupported       []string `json:"unsupported"`
	UnsupportedReason string   `json:"unsupported_reason"`
}

type sutpoProfilesFile struct {
	Profiles []SutpoProfile `json:"profiles"`
}

// LoadSutpoProfiles reads the SUPTO profiles fixture and validates required fields.
func LoadSutpoProfiles(path string) ([]SutpoProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadSutpoProfiles: %w", err)
	}
	var f sutpoProfilesFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("LoadSutpoProfiles: %w", err)
	}
	if len(f.Profiles) == 0 {
		return nil, fmt.Errorf("LoadSutpoProfiles: no profiles in %s", path)
	}
	for i, p := range f.Profiles {
		if p.ID == "" {
			return nil, fmt.Errorf("LoadSutpoProfiles: profile %d missing id", i)
		}
		if len(p.AuditEvents) == 0 {
			return nil, fmt.Errorf("LoadSutpoProfiles: profile %s has no audit_events", p.ID)
		}
	}
	return f.Profiles, nil
}
