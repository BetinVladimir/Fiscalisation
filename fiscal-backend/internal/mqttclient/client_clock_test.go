package mqttclient

import (
	"errors"
	"testing"
	"time"
)

var fixedTime = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

// TestBridgeDeviceTimeFailsClosedWithoutMeasurement verifies that DeviceTime()
// returns ErrDeviceClockNotMeasured when no DEVICE_TIME result has been received.
// Host-clock fallback is forbidden (BUG-005).
func TestBridgeDeviceTimeFailsClosedWithoutMeasurement(t *testing.T) {
	b := NewBridge("")
	_, err := b.DeviceTime()
	if !errors.Is(err, ErrDeviceClockNotMeasured) {
		t.Fatalf("DeviceTime() = %v, want ErrDeviceClockNotMeasured", err)
	}
}

// TestBridgeSetDeviceTimeUnsupportedForBC50 verifies that SetDeviceTime returns
// ErrDeviceClockSetUnsupported for the BC-50 MQTT bridge. BC-50 reads time via
// cmd62 but cmd61 (set) is not available on this profile.
func TestBridgeSetDeviceTimeUnsupportedForBC50(t *testing.T) {
	b := NewBridge("")
	err := b.SetDeviceTime(fixedTime)
	if !errors.Is(err, ErrDeviceClockSetUnsupported) {
		t.Fatalf("SetDeviceTime() = %v, want ErrDeviceClockSetUnsupported", err)
	}
}
