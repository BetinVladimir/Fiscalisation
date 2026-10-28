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

// SpecialUNPKind identifies the legal basis for a reversal without an original SUPTO document.
type SpecialUNPKind string

const (
	// SpecialUNPMarketWithdrawal corresponds to чл. 31(11): изъятие товара с рынка.
	// Format prefix: "XX" (two Latin capital X letters).
	SpecialUNPMarketWithdrawal SpecialUNPKind = "MARKET_WITHDRAWAL"
	// SpecialUNPPreSUPTOSale corresponds to чл. 31(12) / Прил. 29 п.10: продажа до внедрения СУПТО.
	// Format prefix: "OO" (two Latin capital O letters, not digit zero).
	SpecialUNPPreSUPTOSale SpecialUNPKind = "PRE_SUPTO_SALE"
)

// SpecialUNP is a UNP for reversals that have no corresponding SUPTO sale document.
type SpecialUNP struct {
	Kind   SpecialUNPKind
	Prefix string // two-letter Latin prefix (XX or OO)
}

// NewSpecialUNP constructs a SpecialUNP for the given reversal kind.
// fmPrefix must be the first two characters of the fiscal-memory number (FMIN),
// used to identify the fiscal device; it is not used in the value format but
// is recorded in the audit log.
//
// The resulting UNP value follows Наредба Н-18 Прил. 29 encoding:
//
//	MARKET_WITHDRAWAL → "XX000000-0000-0000000"
//	PRE_SUPTO_SALE    → "OO000000-0000-0000000"
//
// where the letters are Latin (not Cyrillic), per regulatory requirement.
func NewSpecialUNP(kind SpecialUNPKind, fmPrefix string) (SpecialUNP, error) {
	switch kind {
	case SpecialUNPMarketWithdrawal:
		return SpecialUNP{Kind: kind, Prefix: "XX"}, nil
	case SpecialUNPPreSUPTOSale:
		return SpecialUNP{Kind: kind, Prefix: "OO"}, nil
	default:
		return SpecialUNP{}, fmt.Errorf("unknown special UNP kind: %q", kind)
	}
}

// Value returns the canonical UNP string for this special reversal.
func (u SpecialUNP) Value() string {
	return u.Prefix + "000000-0000-0000000"
}

// ReversalBasisAllowed returns true when the given reason is a valid legal basis
// for a reversal that uses a special UNP (no original SUPTO document).
func ReversalBasisAllowed(reason string) bool {
	return reason == string(SpecialUNPMarketWithdrawal) || reason == string(SpecialUNPPreSUPTOSale)
}
