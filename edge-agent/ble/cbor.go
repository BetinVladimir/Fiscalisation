package ble

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"time"

	"github.com/fxamacker/cbor/v2"
)

type DeviceCommandEnvelope struct {
	OperationID   string         `cbor:"operation_id" json:"operation_id"`
	TenantID      string         `cbor:"tenant_id" json:"tenant_id"`
	RegisterID    string         `cbor:"register_id" json:"register_id"`
	DeviceID      string         `cbor:"device_id" json:"device_id"`
	FencingToken  int64          `cbor:"fencing_token" json:"fencing_token"`
	CommandType   string         `cbor:"command_type" json:"command_type"`
	IssuedAt      string         `cbor:"issued_at" json:"issued_at"`
	ExpiresAt     string         `cbor:"expires_at" json:"expires_at"`
	Payload       map[string]any `cbor:"payload" json:"payload"`
	PayloadSHA256 string         `cbor:"payload_sha256" json:"payload_sha256"`
}

var canonicalEncoder cbor.EncMode
var strictDecoder cbor.DecMode

func init() {
	canonicalEncoder, _ = cbor.CanonicalEncOptions().EncMode()
	strictDecoder, _ = (cbor.DecOptions{DupMapKey: cbor.DupMapKeyEnforcedAPF, IndefLength: cbor.IndefLengthForbidden, TagsMd: cbor.TagsForbidden, DefaultMapType: reflect.TypeOf(map[string]any{})}).DecMode()
}

func CanonicalMarshal(v any) ([]byte, error) { return canonicalEncoder.Marshal(v) }
func StrictUnmarshal(b []byte, v any) error  { return strictDecoder.Unmarshal(b, v) }

func PayloadHash(payload map[string]any) (string, error) {
	b, err := CanonicalMarshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (v DeviceCommandEnvelope) Validate(now time.Time) error {
	if v.OperationID == "" || v.TenantID == "" || v.RegisterID == "" || v.DeviceID == "" || v.CommandType == "" || v.FencingToken < 1 || v.Payload == nil {
		return errors.New("incomplete command envelope")
	}
	issued, err := time.Parse(time.RFC3339Nano, v.IssuedAt)
	if err != nil {
		return errors.New("invalid issued_at")
	}
	expires, err := time.Parse(time.RFC3339Nano, v.ExpiresAt)
	if err != nil || !expires.After(issued) || !now.Before(expires) {
		return errors.New("expired command")
	}
	hash, err := PayloadHash(v.Payload)
	if err != nil || hash != v.PayloadSHA256 {
		return errors.New("payload hash mismatch")
	}
	return nil
}
