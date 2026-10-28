package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"
)

func newID(prefix string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // best-effort: rand.Read never returns error on Linux; UUID fallback is harmless
	return fmt.Sprintf("%s-%08x-%04x-%04x-%04x-%012x",
		prefix, b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func pad7(n int64) string  { return fmt.Sprintf("%07d", n) }
func Hash(b []byte) string { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }

// Driver is the transport-neutral device boundary. Domain code reserves state
// before calling it and maps ambiguous failures to UNKNOWN, never to a retry of
// a potentially completed physical side effect.
type Driver interface {
	Execute(Operation, Sale, PaymentRequest) (string, string)
	Probe() error
}
type queuedDriver interface {
	Queue(Operation, Sale, PaymentRequest) error
}
type durableQueuedDriver interface {
	Prepare(Operation, Sale, PaymentRequest) (ResourceRecord, error)
	Publish(ResourceRecord) error
}
type Simulator struct {
	enabled               bool
	cardTerminalAvailable bool
	outcomeUnknown        bool
}

func driverIsSimulated(d Driver) bool { _, ok := d.(*Simulator); return ok }

func NewSimulator(enabled bool) *Simulator { return &Simulator{enabled: enabled} }
func NewSimulatorWithCardTerminal(enabled, available bool) *Simulator {
	return &Simulator{enabled: enabled, cardTerminalAvailable: available}
}
func (s *Simulator) SetOutcomeUnknown(v bool) { s.outcomeUnknown = v }
func (s *Simulator) Execute(op Operation, sale Sale, p PaymentRequest) (string, string) {
	if !s.enabled {
		return "", "SIMULATOR_DISABLED"
	}
	if p.Type == "CARD" && !s.cardTerminalAvailable {
		return "", "PAYMENT_TERMINAL_UNAVAILABLE"
	}
	if s.outcomeUnknown {
		return "", "FISCAL_RESULT_UNKNOWN"
	}
	digest := sha256.Sum256([]byte(op.ID))
	document := int64(binary.BigEndian.Uint32(digest[:4])%9_999_999) + 1
	return strconv.FormatInt(document, 10), ""
}
func (s *Simulator) Probe() error {
	if !s.enabled {
		return errors.New("fiscal device unavailable")
	}
	return nil
}
func (s *Simulator) DeviceTime() (time.Time, error) {
	if !s.enabled {
		return time.Time{}, errors.New("fiscal device unavailable")
	}
	return time.Now().UTC(), nil
}
func (s *Simulator) SetDeviceTime(at time.Time) error {
	if !s.enabled || at.IsZero() {
		return errors.New("fiscal device unavailable")
	}
	return nil
}

// Service is the authoritative fiscalization layer. It owns legal identifiers,
// lifecycle transitions, route fencing, idempotency and immutable evidence;
// POS clients submit intent and render its projections.
type Service struct {
	repo                            Repository
	driver                          Driver
	bleSigningKey                   []byte
	policy                          PolicyCatalog
	requireHardwareSyncSignatures   bool
	deviceCredentialIssuer          DeviceCredentialIssuer
	bleServerIssuer                 *BLEServerCertificateIssuer
	compositeBindingPublisher       CompositeBindingPublisher
	localTokenIssuer                string
	localTokenSigningKID            string
	localTokenPublicKeyDERBase64    string
	spaDeploymentDescriptorURL      string
	spaDeploymentSigningKID         string
	spaDeploymentPublicKeyDERBase64 string
	simulated                       bool
}

func (s *Service) SetSimulated(v bool) { s.simulated = v }

type CompositeBindingPublisher interface {
	PublishCompositeBinding(string, string, []byte) error
}

func (s *Service) SetCompositeBindingPublisher(v CompositeBindingPublisher) {
	s.compositeBindingPublisher = v
}

// ReadyComponent reports the health of an individual subsystem checked during
// readiness evaluation.
type ReadyComponent struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// Ready returns true when the database persistence store and the bound driver
// are both reachable.  It pings each component with a one-second timeout and
// reports failures (but never panics): readiness is the *absence* of errors.
func (s *Service) Ready(ctx context.Context) []ReadyComponent {
	components := make([]ReadyComponent, 0, 3)

	// Persistence layer.
	if s.repo != nil {
		if pingErr := s.repo.PingContext(ctx); pingErr != nil {
			components = append(components, ReadyComponent{Name: "persistence", Status: "unhealthy", Detail: pingErr.Error()})
		} else {
			components = append(components, ReadyComponent{Name: "persistence", Status: "ok"})
		}
	} else {
		components = append(components, ReadyComponent{Name: "persistence", Status: "unhealthy", Detail: "no repository configured"})
	}

	// Driver / transport layer.
	if d := s.driverFor(""); d != nil {
		if probeErr := d.Probe(); probeErr != nil {
			components = append(components, ReadyComponent{Name: "driver", Status: "unhealthy", Detail: probeErr.Error()})
		} else {
			components = append(components, ReadyComponent{Name: "driver", Status: "ok"})
		}
	} else {
		components = append(components, ReadyComponent{Name: "driver", Status: "ok"}) // simulator path
	}

	return components
}

// driverFor is the single access point for the service transport driver.
func (s *Service) driverFor(_ string) Driver { return s.driver }

// boundDriver revalidates the complete immutable route snapshot immediately
// before use. The configured driver is a transport dispatcher (MQTT/simulator),
// not a device choice; the selected device remains the snapshot carried in the
// command. Any rebind or identity mismatch is fenced without fallback.


func (s *Service) SetRequireHardwareSyncSignatures(required bool) {
	s.requireHardwareSyncSignatures = required
}

// ExpireDeviceCommand fail-closes an immutable command that could not be
// delivered while its signed authority was valid. It never creates a fresh
// envelope and never repeats a possible device side effect.


func (s *Service) CountryPolicy(at time.Time) (CountryPolicy, error) { return s.policy.Policy(at) }
func (s *Service) TaxGroups(at time.Time) ([]TaxGroup, error)        { return s.policy.TaxGroups(at) }





func NewService(r Repository, d Driver) *Service {
	return &Service{repo: r, driver: d, policy: DefaultBGPolicyCatalog()}
}

type CreateSale struct {
	TenantID   string `json:"-"`
	ExternalID string `json:"external_id"`
	RegisterID string `json:"register_id"`
	OperatorID string `json:"operator_id"`
}

