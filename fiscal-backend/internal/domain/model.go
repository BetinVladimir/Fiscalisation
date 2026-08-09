package domain

import (
	"encoding/json"
	"time"
)

type Money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}
type SaleLine struct {
	LineID      string `json:"line_id"`
	ProductCode string `json:"product_code,omitempty"`
	Name        string `json:"name"`
	Quantity    string `json:"quantity"`
	UnitPrice   Money  `json:"unit_price"`
	TaxGroup    string `json:"tax_group"`
}
type PaymentRecord struct {
	PaymentID       string    `json:"payment_id"`
	Type            string    `json:"type"`
	Amount          Money     `json:"amount"`
	FiscalReference string    `json:"fiscal_reference,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}
type Sale struct {
	ID, TenantID, ExternalID, RegisterID, OperatorID, UNP, State string
	FiscalOperationID, ReceiptArtifactID                         string
	Version                                                      int64
	Lines                                                        []SaleLine
	Payments                                                     []PaymentRecord
	CreatedAt, UpdatedAt                                         time.Time
}
type saleJSON struct {
	ID                string          `json:"sale_id"`
	TenantID          string          `json:"tenant_id"`
	ExternalID        string          `json:"external_id"`
	RegisterID        string          `json:"register_id"`
	OperatorID        string          `json:"operator_id"`
	UNP               string          `json:"unp,omitempty"`
	State             string          `json:"state"`
	Version           int64           `json:"version"`
	Lines             []SaleLine      `json:"lines"`
	Payments          []PaymentRecord `json:"payments"`
	FiscalOperationID string          `json:"fiscal_operation_id,omitempty"`
	ReceiptArtifactID string          `json:"receipt_artifact_id,omitempty"`
	AllowedActions    []string        `json:"allowed_actions"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

func (s Sale) MarshalJSON() ([]byte, error) {
	return marshal(saleJSON{ID: s.ID, TenantID: s.TenantID, ExternalID: s.ExternalID, RegisterID: s.RegisterID, OperatorID: s.OperatorID, UNP: s.UNP, State: s.State, Version: s.Version, Lines: s.Lines, Payments: s.Payments, FiscalOperationID: s.FiscalOperationID, ReceiptArtifactID: s.ReceiptArtifactID, AllowedActions: saleActions(s.State), CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt})
}
func saleActions(state string) []string {
	switch state {
	case "DRAFT":
		return []string{"ADD_LINE", "CANCEL"}
	case "OPEN":
		return []string{"ADD_LINE", "PAY", "CANCEL"}
	case "UNKNOWN":
		return []string{"RECONCILE", "READ"}
	case "COMPLETED":
		return []string{"REVERSE", "RECEIPT"}
	default:
		return []string{}
	}
}
func (s *Sale) UnmarshalJSON(b []byte) error {
	var v saleJSON
	if e := json.Unmarshal(b, &v); e != nil {
		return e
	}
	*s = Sale{ID: v.ID, TenantID: v.TenantID, ExternalID: v.ExternalID, RegisterID: v.RegisterID, OperatorID: v.OperatorID, UNP: v.UNP, State: v.State, Version: v.Version, Lines: v.Lines, Payments: v.Payments, FiscalOperationID: v.FiscalOperationID, ReceiptArtifactID: v.ReceiptArtifactID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
	return nil
}

type PaymentRequest struct {
	PaymentID      string `json:"payment_id"`
	Type           string `json:"type"`
	Amount         Money  `json:"amount"`
	TerminalPolicy string `json:"terminal_policy,omitempty"`
}
type Operation struct {
	ID                      string    `json:"operation_id"`
	TenantID                string    `json:"tenant_id"`
	SaleID                  string    `json:"sale_id,omitempty"`
	Type                    string    `json:"type"`
	State                   string    `json:"state"`
	Version                 int64     `json:"version"`
	FiscalReference         string    `json:"fiscal_reference,omitempty"`
	OriginalFiscalReference string    `json:"original_fiscal_reference,omitempty"`
	ReasonCode              string    `json:"reason_code,omitempty"`
	Simulated               bool      `json:"simulated"`
	ErrorCode               string    `json:"error_code,omitempty"`
	AllowedActions          []string  `json:"allowed_actions"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}
type Device struct {
	ID            string `json:"id"`
	Vendor        string `json:"vendor"`
	Model         string `json:"model"`
	Serial        string `json:"serial"`
	Firmware      string `json:"firmware,omitempty"`
	Status        string `json:"status"`
	Simulated     bool   `json:"simulated"`
	EvidenceState string `json:"evidence_state"`
}
type Shift struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id,omitempty"`
	RegisterID string     `json:"register_id"`
	OperatorID string     `json:"operator_id"`
	State      string     `json:"state"`
	Version    int64      `json:"version"`
	OpenedAt   time.Time  `json:"opened_at"`
	ClosedAt   *time.Time `json:"closed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
type WebhookEvent struct {
	EventID         string    `json:"event_id"`
	EventType       string    `json:"event_type"`
	APIVersion      string    `json:"api_version"`
	TenantID        string    `json:"tenant_id"`
	ResourceID      string    `json:"resource_id"`
	ResourceVersion int64     `json:"resource_version"`
	OccurredAt      time.Time `json:"occurred_at"`
	Data            any       `json:"data"`
}
type BLESessionRecord struct {
	SessionID       string    `json:"session_id"`
	TenantID        string    `json:"tenant_id,omitempty"`
	RegisterID      string    `json:"register_id"`
	OperatorID      string    `json:"operator_id"`
	AppInstanceID   string    `json:"app_instance_id"`
	ActorSubject    string    `json:"actor_subject"`
	ClientPublicKey string    `json:"client_public_key"`
	DeviceID        string    `json:"device_id"`
	Scopes          []string  `json:"scopes"`
	FencingToken    int64     `json:"fencing_token"`
	ExpiresAt       time.Time `json:"expires_at"`
	Revoked         bool      `json:"revoked"`
	Nonce           string    `json:"nonce"`
}
type SyncAck struct {
	AckID               string                `json:"ack_id"`
	EdgeID              string                `json:"edge_id"`
	CommittedThroughSeq int64                 `json:"committed_through_seq"`
	CommittedEventHash  string                `json:"committed_event_hash"`
	CommittedAt         time.Time             `json:"committed_at"`
	OperationResults    []SyncOperationResult `json:"operation_results"`
	Rejected            []map[string]any      `json:"rejected"`
	Signature           string                `json:"signature"`
}
type SyncOperationResult struct {
	OperationID string `json:"operation_id"`
	State       string `json:"state"`
	Version     int64  `json:"version"`
}
type EdgePendingCommand struct {
	OperationID       string         `json:"operation_id"`
	TenantID          string         `json:"tenant_id"`
	RegisterID        string         `json:"register_id"`
	DeviceID          string         `json:"device_id"`
	CommandType       string         `json:"command_type"`
	Payload           map[string]any `json:"payload"`
	OperationSequence int64          `json:"operation_sequence"`
	UNPSequence       int64          `json:"unp_sequence"`
	AcceptedAt        time.Time      `json:"accepted_at"`
}
type ConnectivityProbe struct {
	ProbeID              string                    `json:"probe_id"`
	TenantID             string                    `json:"tenant_id,omitempty"`
	RegisterID           string                    `json:"register_id"`
	State                string                    `json:"state"`
	ObservedAt           time.Time                 `json:"observed_at"`
	Hops                 map[string]map[string]any `json:"hops"`
	RecommendedTransport string                    `json:"recommended_transport"`
}
