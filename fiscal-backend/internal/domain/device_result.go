package domain

import "encoding/json"

// DeviceResultStatus is the outcome of a fiscal device operation.
// COMMITTED means the document is confirmed recorded; NOT_COMMITTED means it
// definitively was not recorded; UNKNOWN means the recording status cannot be
// determined (e.g. timeout after the command was sent). UNKNOWN must never be
// treated as FAILED or NOT_COMMITTED — no retry write and no automatic refund.
type DeviceResultStatus string

const (
	DeviceResultCommitted    DeviceResultStatus = "COMMITTED"
	DeviceResultNotCommitted DeviceResultStatus = "NOT_COMMITTED"
	DeviceResultUnknown      DeviceResultStatus = "UNKNOWN"
)

// DeviceResult captures the outcome of a fiscal device operation together with
// any opaque evidence string returned by the device (document number, raw
// response, etc.).
type DeviceResult struct {
	Status   DeviceResultStatus `json:"status"`
	Evidence string             `json:"evidence,omitempty"`
}

// MarshalJSON encodes DeviceResult to JSON. UNKNOWN must be preserved as-is;
// an empty Status defaults to UNKNOWN so callers cannot accidentally lose it.
func (r DeviceResult) MarshalJSON() ([]byte, error) {
	type alias DeviceResult
	if r.Status == "" {
		r.Status = DeviceResultUnknown
	}
	return json.Marshal(alias(r))
}
