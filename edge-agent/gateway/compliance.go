package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	edgeruntime "fiscalisation/edge-agent/runtime"
)

var fminPattern = regexp.MustCompile(`^[A-Za-z0-9]{8}$`)
var operatorPattern = regexp.MustCompile(`^[A-Za-z0-9]{4}$`)

type CountryPolicyBundle struct {
	CountryCode, ProfileVersion, IdentifierScheme, FiscalDeviceNumber, Signature string
	ExpiresAt                                                                    time.Time
}
type IntentLine struct {
	LineID      string `json:"line_id"`
	ProductCode string `json:"product_code"`
	Name        string `json:"name"`
	Quantity    string `json:"quantity"`
	UnitPrice   string `json:"unit_price"`
	Discount    string `json:"discount,omitempty"`
	TaxGroup    string `json:"tax_group"`
}
type ComplianceIntent struct {
	IntentID              string         `json:"intent_id"`
	Action                string         `json:"action"`
	ClientSaleSurrogateID string         `json:"client_sale_surrogate_id"`
	ServerSaleID          string         `json:"server_sale_id,omitempty"`
	OperatorCode          string         `json:"operator_code"`
	AppInstanceID         string         `json:"app_instance_id"`
	ExpectedVersion       int64          `json:"expected_version"`
	Line                  *IntentLine    `json:"line,omitempty"`
	Payment               map[string]any `json:"payment,omitempty"`
	ReasonCode            string         `json:"reason_code,omitempty"`
}
type IntentResult struct {
	OperationID           string              `json:"operation_id"`
	State                 string              `json:"state"`
	ServerSaleID          string              `json:"server_sale_id,omitempty"`
	Version               int64               `json:"version"`
	RegulatoryIdentifiers []map[string]string `json:"regulatory_identifiers,omitempty"`
	FiscalReference       string              `json:"fiscal_reference,omitempty"`
	ErrorCode             string              `json:"error_code,omitempty"`
}

type ComplianceGateway struct {
	runtime *edgeruntime.Runtime
	binding SessionBinding
	policy  CountryPolicyBundle
	now     func() time.Time
}

func NewComplianceGateway(runtime *edgeruntime.Runtime, binding SessionBinding, policy CountryPolicyBundle) (*ComplianceGateway, error) {
	if runtime == nil || binding.TenantID == "" || binding.RegisterID == "" || binding.DeviceID == "" || binding.SessionID == "" || !operatorPattern.MatchString(binding.OperatorCode) || binding.AppInstanceID == "" || binding.FencingToken < 1 || !fminPattern.MatchString(policy.FiscalDeviceNumber) || policy.CountryCode != "BG" || policy.IdentifierScheme != "BG_UNP_V1" || policy.ProfileVersion == "" || policy.Signature == "" || policy.ExpiresAt.IsZero() {
		return nil, errors.New("invalid compliance gateway configuration")
	}
	g := &ComplianceGateway{runtime: runtime, binding: binding, policy: policy, now: time.Now}
	runtime.SetCommandEnricher(func(c edgeruntime.Command, sequence int64) (edgeruntime.Command, error) {
		if !operatorPattern.MatchString(c.OperatorCode) {
			return c, errors.New("invalid operator code")
		}
		var payload map[string]any
		if json.Unmarshal(c.Payload, &payload) != nil {
			return c, errors.New("invalid intent payload")
		}
		action, _ := payload["action"].(string)
		surrogate, _ := payload["client_sale_surrogate_id"].(string)
		unp := ""
		if action == "OPEN_WITH_LINE" {
			if existing, found := runtime.RegulatoryIdentifierForSurrogate(surrogate); found {
				unp = existing
			} else {
				unp = fmt.Sprintf("%s-%s-%07d", policy.FiscalDeviceNumber, c.OperatorCode, sequence)
			}
		} else {
			var found bool
			unp, found = runtime.RegulatoryIdentifierForSurrogate(surrogate)
			if !found {
				return c, errors.New("sale regulatory binding not found")
			}
		}
		payload["unp"] = unp
		payload["country_code"] = policy.CountryCode
		payload["profile_version"] = policy.ProfileVersion
		payload["identifier_scheme"] = policy.IdentifierScheme
		c.Payload, _ = json.Marshal(payload)
		return c, nil
	})
	return g, nil
}

func (g *ComplianceGateway) Execute(intent ComplianceIntent) (IntentResult, error) {
	now := g.now().UTC()
	if !now.Before(g.binding.ExpiresAt) || g.binding.IsRevoked(g.binding.SessionID, now) || !now.Before(g.policy.ExpiresAt) {
		return IntentResult{}, errors.New("offline authority inactive")
	}
	if intent.IntentID == "" || intent.ClientSaleSurrogateID == "" || intent.AppInstanceID == "" || !operatorPattern.MatchString(intent.OperatorCode) || intent.ExpectedVersion < 0 {
		return IntentResult{}, errors.New("invalid compliance intent")
	}
	if intent.OperatorCode != g.binding.OperatorCode || intent.AppInstanceID != g.binding.AppInstanceID {
		return IntentResult{}, errors.New("compliance intent session binding mismatch")
	}
	types := map[string]string{"OPEN_WITH_LINE": "FISCAL_SALE_OPEN", "ADD_LINE": "FISCAL_SALE_LINE", "CHANGE_LINE": "FISCAL_SALE_LINE_CHANGE", "CANCEL_LINE": "FISCAL_SALE_LINE_CANCEL", "CANCEL_SALE": "FISCAL_SALE_CANCEL", "PAYMENT": "FISCAL_SALE_PAYMENT", "REVERSE": "FISCAL_SALE_REVERSAL"}
	commandType, ok := types[intent.Action]
	if !ok {
		return IntentResult{}, errors.New("unsupported compliance intent")
	}
	if intent.Action == "OPEN_WITH_LINE" && intent.Line == nil {
		return IntentResult{}, errors.New("first line required")
	}
	payload, _ := json.Marshal(intent)
	result, err := g.runtime.Execute(edgeruntime.Command{CommandID: intent.IntentID, TenantID: g.binding.TenantID, RegisterID: g.binding.RegisterID, DeviceID: g.binding.DeviceID, Type: commandType, FencingToken: g.binding.FencingToken, OperatorCode: intent.OperatorCode, Payload: payload})
	if err != nil {
		return IntentResult{}, err
	}
	serverSaleID := intent.ServerSaleID
	if intent.Action == "OPEN_WITH_LINE" && serverSaleID == "" {
		serverSaleID = intent.ClientSaleSurrogateID
	}
	out := IntentResult{OperationID: result.CommandID, State: result.State, ServerSaleID: serverSaleID, Version: intent.ExpectedVersion + 1, FiscalReference: result.FiscalReference, ErrorCode: result.ErrorCode}
	if intent.Action == "OPEN_WITH_LINE" {
		value, found := g.runtime.RegulatoryIdentifierForSurrogate(intent.ClientSaleSurrogateID)
		if !found {
			return IntentResult{}, errors.New("regulatory binding not durable")
		}
		out.RegulatoryIdentifiers = []map[string]string{{"type": "SALE", "scheme": g.policy.IdentifierScheme, "value": value, "country_code": g.policy.CountryCode, "profile_version": g.policy.ProfileVersion}}
	}
	return out, nil
}
