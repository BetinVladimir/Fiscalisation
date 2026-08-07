package device

import (
	"encoding/json"
	"testing"

	edgeruntime "fiscalisation/edge-agent/runtime"
)

func TestSimulatorProbeAndDeterministicExecution(t *testing.T) {
	s := NewSimulator(true)
	if err := s.Probe(); err != nil {
		t.Fatal(err)
	}
	command := edgeruntime.Command{CommandID: "op-1", Payload: json.RawMessage(`{"metadata":{}}`)}
	ref, err := s.Execute(command)
	if err != nil || ref != "SIM-op-1" || s.Executions("op-1") != 1 {
		t.Fatalf("unexpected simulator result %q %v", ref, err)
	}
	s.SetReachable(false)
	if s.Probe() == nil {
		t.Fatal("unreachable simulator reported ready")
	}
}

func TestUnsupportedAlwaysFailsClosed(t *testing.T) {
	u := Unsupported{Capability: "DATECS_DP150_HARDWARE"}
	if u.Probe() == nil {
		t.Fatal("unsupported adapter reported ready")
	}
	if _, err := u.Execute(edgeruntime.Command{}); err == nil {
		t.Fatal("unsupported adapter executed command")
	}
}
