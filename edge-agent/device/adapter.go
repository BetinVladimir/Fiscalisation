package device

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	edgeruntime "fiscalisation/edge-agent/runtime"
)

var ErrUnavailable = errors.New("fiscal device unavailable")

// Simulator is deterministic and is strictly a DEV/test adapter. Scenarios are
// carried in metadata so the canonical command payload stays identical across
// REST, BLE and hardware adapters.
type Simulator struct {
	mu         sync.Mutex
	reachable  bool
	executions map[string]int
}

func NewSimulator(reachable bool) *Simulator {
	return &Simulator{reachable: reachable, executions: make(map[string]int)}
}

func (s *Simulator) SetReachable(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reachable = v
}

func (s *Simulator) Probe() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.reachable {
		return ErrUnavailable
	}
	return nil
}

func (s *Simulator) Execute(command edgeruntime.Command) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.reachable {
		return "", ErrUnavailable
	}
	s.executions[command.CommandID]++
	var payload struct {
		Metadata map[string]any `json:"metadata"`
	}
	if len(command.Payload) > 0 && json.Unmarshal(command.Payload, &payload) != nil {
		return "", edgeruntime.KnownDeviceFailure("INVALID_COMMAND_PAYLOAD")
	}
	scenario, _ := payload.Metadata["simulator_scenario"].(string)
	switch scenario {
	case "timeout_after_execution", "disconnect_after_execution":
		return "", edgeruntime.UnknownDeviceFailure("SIMULATED_OUTCOME_UNKNOWN")
	case "paper_out":
		return "", edgeruntime.KnownDeviceFailure("PAPER_OUT")
	case "":
		return fmt.Sprintf("SIM-%s", command.CommandID), nil
	default:
		return "", edgeruntime.KnownDeviceFailure("INVALID_SIMULATOR_SCENARIO")
	}
}

func (s *Simulator) Executions(commandID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.executions[commandID]
}

// Unsupported is a fail-closed adapter used when a documented hardware track
// has not passed its vendor/HIL gate. It can never report the final device live.
type Unsupported struct{ Capability string }

func (u Unsupported) Probe() error { return fmt.Errorf("%w: %s", ErrUnavailable, u.Capability) }
func (u Unsupported) Execute(edgeruntime.Command) (string, error) {
	return "", fmt.Errorf("unsupported capability: %s", u.Capability)
}
