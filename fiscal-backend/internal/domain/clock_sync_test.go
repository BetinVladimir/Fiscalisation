package domain

import (
	"errors"
	"testing"
	"time"
)

// fixedDriftDriver returns a device time that is driftSec seconds ahead of host.
type fixedDriftDriver struct {
	errorCodeDriver
	driftSec int64
}

func (d *fixedDriftDriver) DeviceTime() (time.Time, error) {
	return time.Now().UTC().Add(time.Duration(d.driftSec) * time.Second), nil
}
func (d *fixedDriftDriver) SetDeviceTime(_ time.Time) error { return nil }

// failingClockDriver implements clockCapableDriver but returns error from DeviceTime.
type failingClockDriver struct{ errorCodeDriver }

func (d *failingClockDriver) DeviceTime() (time.Time, error) {
	return time.Time{}, errors.New("DEVICE_CLOCK_NOT_MEASURED")
}
func (d *failingClockDriver) SetDeviceTime(_ time.Time) error { return nil }

// TestClockSyncFailsWhenDriverLacksClockCapability verifies that
// SyncWorkstationClock returns an error when the driver does not implement
// the clockCapableDriver interface; no fallback to the host clock.
func TestClockSyncFailsWhenDriverLacksClockCapability(t *testing.T) {
	s := NewService(NewMemoryRepository(), &errorCodeDriver{})
	_, err := s.SyncWorkstationClock("register-1", "")
	if err == nil {
		t.Fatal("want error when driver lacks clock capability, got nil")
	}
}

// TestClockSyncFallbackToHostClockRejected verifies that when DeviceTime()
// returns an error, SyncWorkstationClock fails rather than falling back to
// the host clock. Invariant: DEVICE_CLOCK_NOT_MEASURED blocks money I/O.
func TestClockSyncFallbackToHostClockRejected(t *testing.T) {
	s := NewService(NewMemoryRepository(), &failingClockDriver{})
	_, err := s.SyncWorkstationClock("register-1", "")
	if err == nil {
		t.Fatal("want error on DeviceTime failure, got nil (host clock fallback must not occur)")
	}
}

// TestClockSyncDurabilityAndBlocksWithoutSync verifies:
// 1. HasDailyClockSync is false before any sync.
// 2. SyncWorkstationClock succeeds and records the sync durably.
// 3. HasDailyClockSync is true after a successful sync.
// This is the foundation for the invariant that financial I/O is blocked
// without a trusted device clock measurement.
func TestClockSyncDurabilityAndBlocksWithoutSync(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewService(repo, NewSimulator(true))
	register, _ := prepareBLERegister(t, s, "clock-t1")

	if s.HasDailyClockSync(register, "clock-t1", time.Now().UTC()) {
		t.Fatal("daily clock sync must not exist before SyncWorkstationClock")
	}

	event, err := s.SyncWorkstationClock(register, "clock-t1")
	if err != nil {
		t.Fatal(err)
	}
	if !event.Verified {
		t.Fatalf("clock sync event not verified: drift=%d", event.DriftSeconds)
	}

	if !s.HasDailyClockSync(register, "clock-t1", event.OccurredAt) {
		t.Fatal("HasDailyClockSync must return true after successful sync")
	}
	if len(repo.Resources("device_clock_sync_event", "clock-t1")) != 1 {
		t.Fatal("clock sync event not durable in repository")
	}
}

// TestClockSyncRejectsHostClockAsDeviceClock verifies that SyncWorkstationClock
// returns DEVICE_CLOCK_NOT_MEASURED when the fiscal device clock is unavailable,
// never substituting the host clock. Required by BUG-005 invariant.
func TestClockSyncRejectsHostClockAsDeviceClock(t *testing.T) {
	s := NewService(NewMemoryRepository(), &failingClockDriver{})
	register, _ := prepareBLERegister(t, s, "reject-t1")
	_, err := s.SyncWorkstationClock(register, "reject-t1")
	if err == nil {
		t.Fatal("want DEVICE_CLOCK_NOT_MEASURED when device unavailable, got nil (host clock fallback must be rejected)")
	}
	if err.Error() != "DEVICE_CLOCK_NOT_MEASURED" {
		t.Fatalf("want error DEVICE_CLOCK_NOT_MEASURED, got %q", err)
	}
}

// TestHasDailyClockSyncUsesSofiaBusinessDate verifies that business_date is
// computed in Europe/Sofia timezone, not UTC. At 2026-10-21T22:30Z (UTC),
// Sofia time is 2026-10-22T01:30+03:00 → business_date must be 2026-10-22.
func TestHasDailyClockSyncUsesSofiaBusinessDate(t *testing.T) {
	sofia, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Fatalf("load Europe/Sofia: %v", err)
	}
	at := time.Date(2026, 10, 21, 22, 30, 0, 0, time.UTC)
	wantDate := at.In(sofia).Format("2006-01-02") // "2026-10-22"
	if wantDate != "2026-10-22" {
		t.Fatalf("fixture check: expected 2026-10-22, got %s", wantDate)
	}
	repo := NewMemoryRepository()
	s := NewService(repo, NewSimulator(true))
	// Inject a verified sync record with the Sofia business date.
	_ = repo.PutResource(ResourceRecord{
		Kind: "device_clock_sync_event", TenantID: "tz-t1",
		ID: "ws-tz|2026-10-22", Version: 1,
		Data: map[string]any{"workstation_id": "ws-tz", "business_date": "2026-10-22", "verified": true},
	})
	if !s.HasDailyClockSync("ws-tz", "tz-t1", at) {
		t.Fatal("HasDailyClockSync must return true for Sofia date 2026-10-22 when checking at 2026-10-21T22:30Z")
	}
	// Verify that UTC date lookup (2026-10-21) finds nothing — no record stored for that date.
	utcAt := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC) // Sofia 13:00 → date 2026-10-21
	if s.HasDailyClockSync("ws-tz", "tz-t1", utcAt) {
		t.Fatal("HasDailyClockSync must return false for a different Sofia date (no record for 2026-10-21)")
	}
}

// TestClockSyncWarnDriftOver5Min verifies that drift between 5 min and 2 hours
// is accepted (Verified=true) but flagged with DriftWarn=true.
func TestClockSyncWarnDriftOver5Min(t *testing.T) {
	s := NewService(NewMemoryRepository(), &fixedDriftDriver{driftSec: 360}) // +6 min
	register, _ := prepareBLERegister(t, s, "warn-t1")
	event, err := s.SyncWorkstationClock(register, "warn-t1")
	if err != nil {
		t.Fatalf("+6min drift should not block clock sync, got error: %v", err)
	}
	if !event.Verified {
		t.Fatal("+6min drift: want Verified=true")
	}
	if !event.DriftWarn {
		t.Fatal("+6min drift: want DriftWarn=true")
	}
}

// TestClockSyncBlockDriftOver2Hours verifies that drift exceeding 2 hours
// returns an error and does not store a verified clock sync event.
func TestClockSyncBlockDriftOver2Hours(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewService(repo, &fixedDriftDriver{driftSec: 10800}) // +3 hours
	register, _ := prepareBLERegister(t, s, "block-t1")
	_, err := s.SyncWorkstationClock(register, "block-t1")
	if err == nil {
		t.Fatal("+3h drift should block clock sync, got nil error")
	}
	if s.HasDailyClockSync(register, "block-t1", time.Now().UTC()) {
		t.Fatal("+3h drift: HasDailyClockSync must be false after blocked sync")
	}
}
