package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fiscalisation/edge-agent/authority"
	"fiscalisation/edge-agent/journal"
)

type Command struct {
	CommandID    string          `json:"command_id"`
	TenantID     string          `json:"tenant_id"`
	RegisterID   string          `json:"register_id"`
	DeviceID     string          `json:"device_id"`
	Type         string          `json:"type"`
	FencingToken int64           `json:"fencing_token"`
	Payload      json.RawMessage `json:"payload"`
}
type Result struct {
	CommandID         string `json:"command_id"`
	State             string `json:"state"`
	FiscalReference   string `json:"fiscal_reference,omitempty"`
	ErrorCode         string `json:"error_code,omitempty"`
	OperationSequence int64  `json:"operation_sequence,omitempty"`
	UNPSequence       int64  `json:"unp_sequence,omitempty"`
}
type Device interface {
	Probe() error
	Execute(Command) (string, error)
}

// DeviceFailure distinguishes a confirmed device rejection from an ambiguous
// transport loss after bytes may have reached the fiscal device.
type DeviceFailure struct {
	Code         string
	OutcomeKnown bool
}

func (e *DeviceFailure) Error() string { return e.Code }

func KnownDeviceFailure(code string) error {
	return &DeviceFailure{Code: code, OutcomeKnown: true}
}

func UnknownDeviceFailure(code string) error {
	return &DeviceFailure{Code: code, OutcomeKnown: false}
}

type Runtime struct {
	journal           *journal.Journal
	authority         *authority.Manager
	device            Device
	now               func() time.Time
	storageQuotaBytes int64
}

func (r *Runtime) SetStorageQuota(bytes int64) { r.storageQuotaBytes = bytes }
func (r *Runtime) StorageStatus() (journal.StorageStatus, error) {
	return r.journal.Storage(r.storageQuotaBytes)
}

func New(j *journal.Journal, a *authority.Manager, d Device) *Runtime {
	var maxOperation, maxUNP int64
	for _, e := range j.Events() {
		if e.Type != "COMMAND_DURABLE" {
			continue
		}
		var v struct {
			OperationSequence int64 `json:"operation_sequence"`
			UNPSequence       int64 `json:"unp_sequence"`
		}
		if json.Unmarshal(e.Payload, &v) == nil {
			if v.OperationSequence > maxOperation {
				maxOperation = v.OperationSequence
			}
			if v.UNPSequence > maxUNP {
				maxUNP = v.UNPSequence
			}
		}
	}
	if maxOperation > 0 {
		a.Restore(maxOperation+1, maxUNP+1)
	}
	return &Runtime{journal: j, authority: a, device: d, now: time.Now}
}

// ProbeFinalDevice checks the actual fiscal device path without allocating a
// fiscal sequence or appending a command to the journal.
func (r *Runtime) ProbeFinalDevice() error { return r.device.Probe() }

func (r *Runtime) Execute(c Command) (Result, error) {
	if c.CommandID == "" || c.TenantID == "" || c.RegisterID == "" || c.DeviceID == "" || c.Type == "" {
		return Result{}, errors.New("invalid command")
	}
	if previous, found, unknown := r.lookup(c.CommandID); found {
		return previous, nil
	} else if unknown {
		return Result{CommandID: c.CommandID, State: "FISCAL_RESULT_UNKNOWN", ErrorCode: "RECONCILIATION_REQUIRED"}, nil
	}
	storage, err := r.StorageStatus()
	if err != nil {
		return Result{}, fmt.Errorf("storage status: %w", err)
	}
	if storage.State == "CRITICAL" || storage.State == "FULL" {
		return Result{CommandID: c.CommandID, State: "BLOCKED", ErrorCode: "EDGE_STORAGE_CRITICAL"}, nil
	}
	// The end device, not only the cloud route, must be reachable before a sale.
	if err := r.device.Probe(); err != nil {
		return Result{CommandID: c.CommandID, State: "BLOCKED", ErrorCode: "FISCAL_DEVICE_UNREACHABLE"}, nil
	}
	op, unp, err := r.authority.Allocate(r.now().UTC(), c.FencingToken)
	if err != nil {
		return Result{}, err
	}
	accepted := map[string]any{"command": c, "operation_sequence": op, "unp_sequence": unp}
	if _, err = r.journal.Append(c.CommandID, "COMMAND_DURABLE", accepted); err != nil {
		return Result{}, err
	}
	ref, execErr := r.device.Execute(c)
	result := Result{CommandID: c.CommandID, State: "FISCALIZED", FiscalReference: ref, OperationSequence: op, UNPSequence: unp}
	if execErr != nil {
		var failure *DeviceFailure
		if errors.As(execErr, &failure) && failure.OutcomeKnown {
			result.State = "FAILED"
			result.ErrorCode = failure.Code
		} else {
			result.State = "FISCAL_RESULT_UNKNOWN"
			result.ErrorCode = "DEVICE_OUTCOME_UNKNOWN"
		}
	}
	if _, err = r.journal.Append(c.CommandID, "COMMAND_RESULT", result); err != nil {
		return Result{}, fmt.Errorf("result not durable: %w", err)
	}
	return result, nil
}

func (r *Runtime) lookup(id string) (Result, bool, bool) {
	accepted := false
	for _, e := range r.journal.Events() {
		if e.OperationID != id {
			continue
		}
		if e.Type == "COMMAND_DURABLE" {
			accepted = true
		}
		if e.Type == "COMMAND_RESULT" {
			var v Result
			if json.Unmarshal(e.Payload, &v) == nil {
				return v, true, false
			}
		}
	}
	return Result{}, false, accepted
}

type Simulator struct {
	Reachable       bool
	FailAfterAccept bool
	Executions      int
}

func (s *Simulator) Probe() error {
	if !s.Reachable {
		return errors.New("offline")
	}
	return nil
}
func (s *Simulator) Execute(c Command) (string, error) {
	s.Executions++
	if s.FailAfterAccept {
		return "", errors.New("transport lost")
	}
	return "SIM-" + c.CommandID, nil
}
