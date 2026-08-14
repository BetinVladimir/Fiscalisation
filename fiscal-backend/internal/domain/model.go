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
	Discount    *Money `json:"discount,omitempty"`
	TaxGroup    string `json:"tax_group"`
}
type PaymentRecord struct {
	PaymentID       string    `json:"payment_id"`
	Type            string    `json:"type"`
	Amount          Money     `json:"amount"`
	FiscalReference string    `json:"fiscal_reference,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}
type FiscalDeviceSnapshot struct {
	DeviceID           string `json:"device_id"`
	BindingVersion     int64  `json:"binding_version,omitempty"`
	Serial             string `json:"serial,omitempty"`
	FiscalDeviceNumber string `json:"fiscal_device_number,omitempty"`
	FiscalMemoryNumber string `json:"fiscal_memory_number,omitempty"`
	Vendor             string `json:"vendor,omitempty"`
	Model              string `json:"model,omitempty"`
	Firmware           string `json:"firmware,omitempty"`
}
type Sale struct {
	ID, TenantID, ExternalID, LocationID, RegisterID, OperatorID, UNP, State string
	FiscalOperationID, ReceiptArtifactID                                     string
	RegulatoryIdentifiers                                                    []RegulatoryIdentifier
	Version                                                                  int64
	Lines                                                                    []SaleLine
	Payments                                                                 []PaymentRecord
	FiscalDevice                                                             FiscalDeviceSnapshot
	CreatedAt, UpdatedAt                                                     time.Time
}
type saleJSON struct {
	ID                    string                 `json:"sale_id"`
	TenantID              string                 `json:"tenant_id"`
	ExternalID            string                 `json:"external_id"`
	LocationID            string                 `json:"location_id,omitempty"`
	RegisterID            string                 `json:"register_id"`
	OperatorID            string                 `json:"operator_id"`
	UNP                   string                 `json:"unp,omitempty"`
	State                 string                 `json:"state"`
	Version               int64                  `json:"version"`
	Lines                 []SaleLine             `json:"lines"`
	Payments              []PaymentRecord        `json:"payments"`
	FiscalOperationID     string                 `json:"fiscal_operation_id,omitempty"`
	ReceiptArtifactID     string                 `json:"receipt_artifact_id,omitempty"`
	FiscalDevice          FiscalDeviceSnapshot   `json:"fiscal_device"`
	RegulatoryIdentifiers []RegulatoryIdentifier `json:"regulatory_identifiers"`
	AllowedActions        []string               `json:"allowed_actions"`
	Totals                map[string]Money       `json:"totals"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
}

func (s Sale) MarshalJSON() ([]byte, error) {
	total := Money{Amount: "0.00", Currency: "EUR"}
	if cents, err := saleTotal(s); err == nil {
		total.Amount = formatFixed(cents)
	}
	return marshal(saleJSON{ID: s.ID, TenantID: s.TenantID, ExternalID: s.ExternalID, LocationID: s.LocationID, RegisterID: s.RegisterID, OperatorID: s.OperatorID, UNP: s.UNP, State: s.State, Version: s.Version, Lines: s.Lines, Payments: s.Payments, FiscalOperationID: s.FiscalOperationID, ReceiptArtifactID: s.ReceiptArtifactID, FiscalDevice: s.FiscalDevice, RegulatoryIdentifiers: s.RegulatoryIdentifiers, AllowedActions: saleActions(s.State), Totals: map[string]Money{"gross": total}, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt})
}
func saleActions(state string) []string {
	switch state {
	case "DRAFT":
		return []string{"ADD_LINE", "CANCEL"}
	case "OPEN":
		return []string{"ADD_LINE", "CHANGE_LINE", "CANCEL_LINE", "PAY", "CANCEL"}
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
	*s = Sale{ID: v.ID, TenantID: v.TenantID, ExternalID: v.ExternalID, LocationID: v.LocationID, RegisterID: v.RegisterID, OperatorID: v.OperatorID, UNP: v.UNP, State: v.State, Version: v.Version, Lines: v.Lines, Payments: v.Payments, FiscalOperationID: v.FiscalOperationID, ReceiptArtifactID: v.ReceiptArtifactID, FiscalDevice: v.FiscalDevice, RegulatoryIdentifiers: v.RegulatoryIdentifiers, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
	return nil
}

type PaymentRequest struct {
	PaymentID      string `json:"payment_id"`
	Type           string `json:"type"`
	Amount         Money  `json:"amount"`
	TerminalPolicy string `json:"terminal_policy,omitempty"`
}
type SaleFinalizeRequest struct {
	ClientOperationID string           `json:"client_operation_id"`
	ReceiptSessionID  string           `json:"receipt_session_id"`
	Payments          []PaymentRequest `json:"payments"`
	ExpectedTotal     Money            `json:"expected_total"`
	Metadata          map[string]any   `json:"metadata,omitempty"`
}
type Operation struct {
	ID                      string    `json:"operation_id"`
	ClientOperationID       string    `json:"client_operation_id,omitempty"`
	ReceiptSessionID        string    `json:"receipt_session_id,omitempty"`
	TenantID                string    `json:"tenant_id"`
	SaleID                  string    `json:"sale_id,omitempty"`
	RegisterID              string    `json:"register_id,omitempty"`
	Type                    string    `json:"type"`
	State                   string    `json:"state"`
	Version                 int64     `json:"version"`
	FiscalReference         string    `json:"fiscal_reference,omitempty"`
	OriginalFiscalReference string    `json:"original_fiscal_reference,omitempty"`
	ReasonCode              string    `json:"reason_code,omitempty"`
	OriginalDocumentNumber  int64     `json:"original_document_number,omitempty"`
	OriginalDocumentAt      time.Time `json:"original_document_at,omitempty"`
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
	LocationID      string    `json:"location_id"`
	RegisterID      string    `json:"register_id"`
	OperatorID      string    `json:"operator_id"`
	AppInstanceID   string    `json:"app_instance_id"`
	ActorSubject    string    `json:"actor_subject"`
	ClientPublicKey string    `json:"client_public_key"`
	DeviceID        string    `json:"device_id"`
	FiscalDeviceID  string    `json:"fiscal_device_id"`
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
type ReadinessLease struct {
	LeaseID            string    `json:"lease_id"`
	TenantID           string    `json:"tenant_id,omitempty"`
	WorkstationID      string    `json:"workstation_id"`
	FiscalDeviceID     string    `json:"fiscal_device_id"`
	FiscalDeviceNumber string    `json:"fiscal_device_number"`
	ProfileVersion     string    `json:"profile_version"`
	Ready              bool      `json:"ready"`
	CheckedAt          time.Time `json:"checked_at"`
	ValidUntil         time.Time `json:"valid_for_open_sale_until"`
	Signature          string    `json:"signature"`
}
type DeviceClockSync struct {
	EventID       string    `json:"event_id"`
	TenantID      string    `json:"tenant_id,omitempty"`
	WorkstationID string    `json:"workstation_id"`
	DeviceID      string    `json:"device_id"`
	BusinessDate  string    `json:"business_date"`
	TrustedTime   time.Time `json:"trusted_time"`
	DeviceTime    time.Time `json:"device_time"`
	DriftSeconds  int64     `json:"drift_seconds"`
	SetPerformed  bool      `json:"set_performed"`
	Verified      bool      `json:"verified"`
	OccurredAt    time.Time `json:"occurred_at"`
}
type WorkstationSession struct {
	SessionID     string    `json:"session_id"`
	TenantID      string    `json:"tenant_id,omitempty"`
	WorkstationID string    `json:"workstation_id"`
	OperatorID    string    `json:"operator_id"`
	OperatorCode  string    `json:"operator_code"`
	AppInstanceID string    `json:"app_instance_id"`
	ActorSubject  string    `json:"-"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
}
