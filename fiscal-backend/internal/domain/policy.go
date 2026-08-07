package domain

import (
	"errors"
	"sort"
	"time"
)

// CountryPolicy is an immutable, effective-dated compliance configuration.
// Rates are deliberately seeded only when they have reviewed source evidence;
// missing groups are rejected instead of being guessed from a vendor profile.
type CountryPolicy struct {
	Country          string     `json:"country"`
	Version          string     `json:"version"`
	ValidFrom        time.Time  `json:"valid_from"`
	ValidTo          *time.Time `json:"valid_to"`
	OfficialCurrency string     `json:"official_currency"`
	Profiles         []string   `json:"profiles"`
	SourceSHA256     string     `json:"source_sha256"`
}

type TaxGroup struct {
	Code          string     `json:"code"`
	Rate          string     `json:"rate"`
	ValidFrom     time.Time  `json:"valid_from"`
	ValidTo       *time.Time `json:"valid_to"`
	PolicyVersion string     `json:"policy_version"`
}

type PolicyCatalog struct {
	policies []CountryPolicy
	groups   []TaxGroup
}

func DefaultBGPolicyCatalog() PolicyCatalog {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	version := "bg-2026.08.07"
	return PolicyCatalog{
		policies: []CountryPolicy{{
			Country: "BG", Version: version, ValidFrom: from,
			OfficialCurrency: "EUR", Profiles: []string{"FISCAL_DEVICE", "SUPTO"},
			SourceSHA256: "4eb9b863e14f85f1de0d25643000e98829b963bce38ab02473dca98b07a1a3fa",
		}},
		// B/20.00 is the reviewed baseline used by the MVP golden sale. Other
		// A-H mappings must enter through a reviewed policy update, not inference.
		groups: []TaxGroup{{Code: "B", Rate: "20.00", ValidFrom: from, PolicyVersion: version}},
	}
}

func (c PolicyCatalog) Policy(at time.Time) (CountryPolicy, error) {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	var found *CountryPolicy
	for i := range c.policies {
		v := c.policies[i]
		if !at.Before(v.ValidFrom) && (v.ValidTo == nil || at.Before(*v.ValidTo)) && (found == nil || v.ValidFrom.After(found.ValidFrom)) {
			copy := v
			copy.Profiles = append([]string(nil), v.Profiles...)
			found = &copy
		}
	}
	if found == nil {
		return CountryPolicy{}, errors.New("no effective country policy")
	}
	return *found, nil
}

func (c PolicyCatalog) TaxGroups(at time.Time) ([]TaxGroup, error) {
	p, err := c.Policy(at)
	if err != nil {
		return nil, err
	}
	result := make([]TaxGroup, 0)
	for _, v := range c.groups {
		if v.PolicyVersion == p.Version && !at.Before(v.ValidFrom) && (v.ValidTo == nil || at.Before(*v.ValidTo)) {
			result = append(result, v)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Code < result[j].Code })
	return result, nil
}

func (c PolicyCatalog) AllowsTaxGroup(code string, at time.Time) bool {
	groups, err := c.TaxGroups(at)
	if err != nil {
		return false
	}
	for _, group := range groups {
		if group.Code == code {
			return true
		}
	}
	return false
}
