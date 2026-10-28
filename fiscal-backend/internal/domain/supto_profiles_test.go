package domain

import (
	"testing"
)

const sutpoFixturePath = "../../../docs/MVP1/fixtures/supto_profiles.json"

func TestLoadSutpoProfilesHas4Profiles(t *testing.T) {
	profiles, err := LoadSutpoProfiles(sutpoFixturePath)
	if err != nil {
		t.Fatalf("LoadSutpoProfiles: %v", err)
	}
	if len(profiles) < 4 {
		t.Fatalf("want >=4 profiles, got %d", len(profiles))
	}
}

func TestSutpoProfilesDisabledOperatorAndReadOnlyAuditor(t *testing.T) {
	profiles, err := LoadSutpoProfiles(sutpoFixturePath)
	if err != nil {
		t.Fatalf("LoadSutpoProfiles: %v", err)
	}
	for _, p := range profiles {
		if !p.ReadOnlyAuditor {
			t.Errorf("profile %s: read_only_auditor must be true", p.ID)
		}
		if len(p.AuditEvents) == 0 {
			t.Errorf("profile %s: audit_events must not be empty", p.ID)
		}
		if len(p.Unsupported) == 0 {
			t.Errorf("profile %s: unsupported must not be empty", p.ID)
		}
		if p.UnsupportedReason == "" {
			t.Errorf("profile %s: unsupported_reason must be set", p.ID)
		}
	}
}

func TestSutpoProfileBC50HasDisabledOperator(t *testing.T) {
	profiles, err := LoadSutpoProfiles(sutpoFixturePath)
	if err != nil {
		t.Fatalf("LoadSutpoProfiles: %v", err)
	}
	var bc50 *SutpoProfile
	for i := range profiles {
		if profiles[i].ID == "BC50" {
			bc50 = &profiles[i]
			break
		}
	}
	if bc50 == nil {
		t.Fatal("BC50 profile not found")
	}
	if !bc50.DisabledOperator {
		t.Error("BC50: disabled_operator must be true")
	}
}

func TestSutpoProfilesAuditEventsIncludeStornoAndCancel(t *testing.T) {
	profiles, err := LoadSutpoProfiles(sutpoFixturePath)
	if err != nil {
		t.Fatalf("LoadSutpoProfiles: %v", err)
	}
	for _, p := range profiles {
		hasStorno, hasCancel := false, false
		for _, ev := range p.AuditEvents {
			if ev == "storno" {
				hasStorno = true
			}
			if ev == "cancel" {
				hasCancel = true
			}
		}
		if !hasStorno {
			t.Errorf("profile %s: audit_events must include 'storno'", p.ID)
		}
		if !hasCancel {
			t.Errorf("profile %s: audit_events must include 'cancel'", p.ID)
		}
	}
}
