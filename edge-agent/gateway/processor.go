package gateway

import (
	"encoding/json"
	"errors"
	"time"

	"fiscalisation/edge-agent/ble"
	edgeruntime "fiscalisation/edge-agent/runtime"
)

type ExecuteFunc func(edgeruntime.Command) (edgeruntime.Result, error)
type SessionBinding struct {
	TenantID, RegisterID, DeviceID string
	FencingToken                   int64
}

type Processor struct {
	session     *ble.Session
	reassembler *ble.Reassembler
	execute     ExecuteFunc
	now         func() time.Time
	attMTU      int
	probe       func() error
	binding     SessionBinding
}

func (p *Processor) SetFinalDeviceProbe(probe func() error) { p.probe = probe }

type AcceptResult struct {
	Flow           ble.FlowStatus
	ResponseFrames [][]byte
}

func NewProcessor(session *ble.Session, binding SessionBinding, execute ExecuteFunc, attMTU int) (*Processor, error) {
	if session == nil || execute == nil || binding.TenantID == "" || binding.RegisterID == "" || binding.DeviceID == "" || binding.FencingToken < 1 || (attMTU != 185 && attMTU != 247 && attMTU != 517) {
		return nil, errors.New("invalid BLE processor configuration")
	}
	return &Processor{session: session, reassembler: ble.NewReassembler(1024), execute: execute, now: time.Now, attMTU: attMTU, binding: binding}, nil
}

// AcceptCommandFrame is the transport-neutral GATT command handler. The OS
// adapter only forwards characteristic bytes and publishes returned frames.
func (p *Processor) AcceptCommandFrame(raw []byte) (AcceptResult, error) {
	frame, plain, err := p.session.OpenFrame(raw)
	if err != nil {
		return AcceptResult{}, err
	}
	if frame.Flags != 0 {
		return AcceptResult{}, errors.New("unexpected BLE frame type")
	}
	flow, complete, err := p.reassembler.Add(frame, plain)
	if err != nil || complete == nil {
		return AcceptResult{Flow: flow}, err
	}
	var envelope ble.DeviceCommandEnvelope
	if err = ble.StrictUnmarshal(complete, &envelope); err != nil {
		return AcceptResult{Flow: flow}, errors.New("invalid command CBOR")
	}
	if err = envelope.Validate(p.now().UTC()); err != nil {
		return AcceptResult{Flow: flow}, err
	}
	if envelope.TenantID != p.binding.TenantID || envelope.RegisterID != p.binding.RegisterID || envelope.DeviceID != p.binding.DeviceID || envelope.FencingToken != p.binding.FencingToken {
		return AcceptResult{Flow: flow}, errors.New("BLE session binding mismatch")
	}
	payload, err := json.Marshal(envelope.Payload)
	if err != nil {
		return AcceptResult{Flow: flow}, err
	}
	response := map[string]any{"operation_id": envelope.OperationID}
	if envelope.CommandType == "DEVICE_PROBE" {
		if p.probe == nil || p.probe() != nil {
			response["state"] = "BLOCKED"
			response["error_code"] = "FISCAL_DEVICE_UNREACHABLE"
		} else {
			response["state"] = "READY"
		}
	} else {
		result, execErr := p.execute(edgeruntime.Command{CommandID: envelope.OperationID, TenantID: envelope.TenantID, RegisterID: envelope.RegisterID, DeviceID: envelope.DeviceID, Type: envelope.CommandType, FencingToken: envelope.FencingToken, Payload: payload})
		response = map[string]any{"operation_id": result.CommandID, "state": result.State, "fiscal_reference": result.FiscalReference, "error_code": result.ErrorCode, "operation_sequence": result.OperationSequence, "unp_sequence": result.UNPSequence}
		if execErr != nil {
			response["state"] = "REJECTED"
			response["error_code"] = "EDGE_EXECUTION_REJECTED"
		}
	}
	encoded, err := ble.CanonicalMarshal(response)
	if err != nil {
		return AcceptResult{Flow: flow}, err
	}
	chunks, err := ble.ChunkPlaintext(encoded, p.attMTU)
	if err != nil {
		return AcceptResult{Flow: flow}, err
	}
	frames := make([][]byte, 0, len(chunks))
	for i, chunk := range chunks {
		sealed, sealErr := p.session.SealFrame(frame.MessageID, uint16(i), uint16(len(chunks)), 1, chunk)
		if sealErr != nil {
			return AcceptResult{Flow: flow}, sealErr
		}
		frames = append(frames, sealed)
	}
	return AcceptResult{Flow: flow, ResponseFrames: frames}, nil
}
