package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// clockCapableDriver extends Driver with time-reading and time-setting ability.
type clockCapableDriver interface {
	DeviceTime() (time.Time, error)
	SetDeviceTime(time.Time) error
}

func (s *Service) RefreshReadiness(workstation, tenant string) (ReadinessLease, error) {
	if len(s.bleSigningKey) < 16 {
		return ReadinessLease{}, errors.New("readiness signing unavailable")
	}
	now := time.Now().UTC()
	device, err := s.activeFiscalDeviceSnapshot(workstation, tenant, now)
	d := s.driverFor(workstation)
	if err != nil || !bgFMINPattern.MatchString(device.FiscalDeviceNumber) || d == nil || d.Probe() != nil {
		return ReadinessLease{}, errors.New("fiscal device not ready")
	}
	lease := ReadinessLease{LeaseID: newID("ready"), TenantID: tenant, WorkstationID: workstation, FiscalDeviceID: device.DeviceID, FiscalDeviceNumber: device.FiscalDeviceNumber, ProfileVersion: BGProfileVersion, Ready: true, CheckedAt: now, ValidUntil: now.Add(BGReadinessLeaseMax)}
	lease.Signature = s.signReadiness(lease)
	err = s.repo.PutResource(ResourceRecord{Kind: "readiness_lease", TenantID: tenant, ID: lease.LeaseID, Version: 1, Data: asMap(lease), CreatedAt: now, UpdatedAt: now})
	return lease, err
}
func (s *Service) CurrentReadiness(workstation, tenant string) (ReadinessLease, error) {
	var newest ReadinessLease
	for _, resource := range s.repo.Resources("readiness_lease", tenant) {
		var lease ReadinessLease
		b, _ := json.Marshal(resource.Data)
		if json.Unmarshal(b, &lease) == nil && lease.WorkstationID == workstation && lease.CheckedAt.After(newest.CheckedAt) {
			newest = lease
		}
	}
	device, err := s.activeFiscalDeviceSnapshot(workstation, tenant, time.Now().UTC())
	if err != nil || !newest.Ready || !time.Now().UTC().Before(newest.ValidUntil) || newest.FiscalDeviceID != device.DeviceID || newest.FiscalDeviceNumber != device.FiscalDeviceNumber || newest.ProfileVersion != BGProfileVersion || !hmac.Equal([]byte(newest.Signature), []byte(s.signReadiness(newest))) {
		return ReadinessLease{}, errors.New("readiness lease unavailable")
	}
	return newest, nil
}
func (s *Service) signReadiness(v ReadinessLease) string {
	mac := hmac.New(sha256.New, s.bleSigningKey)
	fmt.Fprintf(mac, "%s\n%s\n%s\n%s\n%s\n%s\n%d", v.TenantID, v.WorkstationID, v.FiscalDeviceID, v.FiscalDeviceNumber, v.ProfileVersion, v.CheckedAt.UTC().Format(time.RFC3339Nano), v.ValidUntil.UnixNano())
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (s *Service) SyncWorkstationClock(workstation, tenant string) (DeviceClockSync, error) {
	driver, ok := s.driverFor(workstation).(clockCapableDriver)
	if !ok {
		return DeviceClockSync{}, errors.New("device clock capability unavailable")
	}
	now := time.Now().UTC()
	device, err := s.activeFiscalDeviceSnapshot(workstation, tenant, now)
	if err != nil {
		return DeviceClockSync{}, err
	}
	deviceTime, err := driver.DeviceTime()
	if err != nil {
		return DeviceClockSync{}, err
	}
	drift := int64(deviceTime.Sub(now).Seconds())
	sofia, _ := time.LoadLocation("Europe/Sofia")
	businessDate := now.In(sofia).Format("2006-01-02")
	driftAbs := drift
	if driftAbs < 0 {
		driftAbs = -driftAbs
	}
	event := DeviceClockSync{
		EventID: newID("clock"), TenantID: tenant, WorkstationID: workstation, DeviceID: device.DeviceID,
		BusinessDate: businessDate, TrustedTime: now, DeviceTime: deviceTime, DriftSeconds: drift,
		DriftWarn: driftAbs > 300, SetPerformed: false,
		Verified: driftAbs <= 7200, OccurredAt: now,
	}
	if !event.Verified {
		return DeviceClockSync{}, fmt.Errorf("DEVICE_CLOCK_DRIFT_BLOCKED: drift %ds exceeds 2h limit", drift)
	}
	syncKey := workstation + "|" + businessDate
	err = s.repo.PutResource(ResourceRecord{Kind: "device_clock_sync_event", TenantID: tenant, ID: syncKey, Version: 1, Data: asMap(event), CreatedAt: now, UpdatedAt: now})
	return event, err
}
func (s *Service) HasDailyClockSync(workstation, tenant string, at time.Time) bool {
	sofia, _ := time.LoadLocation("Europe/Sofia")
	date := at.In(sofia).Format("2006-01-02")
	r, err := s.repo.Resource("device_clock_sync_event", workstation+"|"+date)
	return err == nil && r.TenantID == tenant && r.Data["verified"] == true
}
func (s *Service) Readiness(device string) map[string]any {
	d := s.driverFor(device)
	ready := d != nil && d.Probe() == nil
	state, transport := "READY", "REST"
	if !ready {
		state, transport = "UNAVAILABLE", "BLOCK"
	}
	return map[string]any{"ready": ready, "device_id": device, "observed_at": time.Now().UTC(), "components": map[string]string{"driver": state, "fiscal_device": state}, "recommended_transport": transport}
}
func (s *Service) ReadinessForTenant(device, tenant string) map[string]any {
	registryReady := s.activeDeviceForTenant(device, tenant)
	d := s.driverFor(device)
	driverReady := registryReady && d != nil && d.Probe() == nil
	state, transport := "READY", "REST"
	if !driverReady {
		state, transport = "UNAVAILABLE", "BLOCK"
	}
	registryState := "ACTIVE"
	if !registryReady {
		registryState = "UNAVAILABLE"
	}
	return map[string]any{"ready": driverReady, "device_id": device, "observed_at": time.Now().UTC(), "components": map[string]string{"registry": registryState, "driver": state, "fiscal_device": state}, "recommended_transport": transport}
}
func (s *Service) activeDeviceForTenant(device, tenant string) bool {
	if device == "" || tenant == "" {
		return false
	}
	v, err := s.repo.Resource("device", device)
	if err != nil || v.TenantID != tenant || stringField(v.Data, "status") != "ACTIVE" {
		return false
	}
	return oneOf(stringField(v.Data, "kind"), "FISCAL_DEVICE", "SMART_DEVICE")
}
