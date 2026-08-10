package domain

import "testing"

func TestReadinessRequiresSignatureAndInvalidatesOnRebind(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo, NewSimulator(true))
	register, _ := prepareBLERegister(t, svc, "tenant-ready")
	if _, err := svc.RefreshReadiness(register, "tenant-ready"); err == nil {
		t.Fatal("unsigned readiness accepted")
	}
	svc.SetBLESigningKey("01234567890123456789012345678901")
	lease, err := svc.RefreshReadiness(register, "tenant-ready")
	if err != nil || !lease.Ready || lease.ValidUntil.Sub(lease.CheckedAt) > BGReadinessLeaseMax {
		t.Fatal(lease, err)
	}
	if _, err = svc.CurrentReadiness(register, "tenant-ready"); err != nil {
		t.Fatal(err)
	}
	other, err := svc.CreateResource("device", "tenant-ready", map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Datecs", "model": "DP-150 MX", "serial": "SECOND", "fiscal_device_number": "BL000002", "status": "DRAFT", "environment": "DEV", "simulated": true})
	if err != nil {
		t.Fatal(err)
	}
	other = activateTestDevice(t, svc, "tenant-ready", other)
	if _, err = svc.BindRegister(register, "tenant-ready", other["id"].(string), "FISCAL_DEVICE", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.CurrentReadiness(register, "tenant-ready"); err == nil {
		t.Fatal("lease survived device rebind")
	}
}

func TestDailyClockSyncIsDurable(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo, NewSimulator(true))
	register, _ := prepareBLERegister(t, svc, "tenant-clock")
	event, err := svc.SyncWorkstationClock(register, "tenant-clock")
	if err != nil || !event.Verified || !svc.HasDailyClockSync(register, "tenant-clock", event.OccurredAt) {
		t.Fatal(event, err)
	}
	if len(repo.Resources("device_clock_sync_event", "tenant-clock")) != 1 {
		t.Fatal("clock event not durable")
	}
}
