package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	BGUNPV1             = "BG_UNP_V1"
	BGProfileVersion    = "2026-08-10.1"
	BGUNPSequenceMax    = int64(9_999_999)
	BGReadinessLeaseMax = 2 * time.Hour
)

var (
	bgFMINPattern     = regexp.MustCompile(`^[A-Za-z0-9]{8}$`)
	bgOperatorPattern = regexp.MustCompile(`^[A-Za-z0-9]{4}$`)
	bgUNPPattern      = regexp.MustCompile(`^([A-Za-z0-9]{8})-([A-Za-z0-9]{4})-([0-9]{7})$`)
)

// CountryFiscalProfile is an immutable effective-dated authority snapshot.
// Regulatory formats live in country adapters, never in POS code.
type CountryFiscalProfile struct {
	CountryCode       string    `json:"country_code"`
	Version           string    `json:"version"`
	IdentifierScheme  string    `json:"identifier_scheme"`
	Currency          string    `json:"currency"`
	EffectiveFrom     time.Time `json:"effective_from"`
	ReadinessLeaseMax string    `json:"readiness_lease_max"`
}

type RegulatoryIdentifier struct {
	Type           string `json:"type"`
	Scheme         string `json:"scheme"`
	Value          string `json:"value"`
	CountryCode    string `json:"country_code"`
	ProfileVersion string `json:"profile_version"`
}

type BGUNP struct {
	FiscalDeviceNumber string
	OperatorCode       string
	Sequence           int64
}

func NewBGUNP(fmin, operator string, sequence int64) (BGUNP, error) {
	if !bgFMINPattern.MatchString(fmin) || !bgOperatorPattern.MatchString(operator) || sequence < 1 || sequence > BGUNPSequenceMax {
		return BGUNP{}, errors.New("invalid BG_UNP_V1 components")
	}
	return BGUNP{FiscalDeviceNumber: fmin, OperatorCode: operator, Sequence: sequence}, nil
}

func ParseBGUNP(value string) (BGUNP, error) {
	if strings.TrimSpace(value) != value {
		return BGUNP{}, errors.New("invalid BG_UNP_V1 value")
	}
	m := bgUNPPattern.FindStringSubmatch(value)
	if m == nil {
		return BGUNP{}, errors.New("invalid BG_UNP_V1 value")
	}
	sequence, err := strconv.ParseInt(m[3], 10, 64)
	if err != nil {
		return BGUNP{}, errors.New("invalid BG_UNP_V1 sequence")
	}
	return NewBGUNP(m[1], m[2], sequence)
}

func (u BGUNP) String() string {
	return fmt.Sprintf("%s-%s-%07d", u.FiscalDeviceNumber, u.OperatorCode, u.Sequence)
}

func (u BGUNP) RegulatoryIdentifier() RegulatoryIdentifier {
	return RegulatoryIdentifier{Type: "SALE", Scheme: BGUNPV1, Value: u.String(), CountryCode: "BG", ProfileVersion: BGProfileVersion}
}

func DefaultBGFiscalProfile() CountryFiscalProfile {
	return CountryFiscalProfile{CountryCode: "BG", Version: BGProfileVersion, IdentifierScheme: BGUNPV1, Currency: "EUR", EffectiveFrom: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC), ReadinessLeaseMax: BGReadinessLeaseMax.String()}
}
