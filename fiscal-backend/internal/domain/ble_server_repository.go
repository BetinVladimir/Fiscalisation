package domain

import (
	"errors"
	"reflect"
	"time"
)

// All methods use the repository's existing atomic snapshot/CAS and uncertainty
// barrier. The immutable certificate is also the one-time consumption record.
func (r *MemoryRepository) bleAuthorityCurrentLocked(sources []bleCertSource, activation DeviceActivationRequest) error {
	if r.store == nil {
		return errors.New("BLE issuance requires durable repository")
	}
	current, ok := r.activationRequests[activation.ID]
	if !ok || current.State != "ACTIVE" || !reflect.DeepEqual(current, activation) {
		return errors.New("BLE activation changed")
	}
	if len(sources) != 6 {
		return errors.New("BLE authority source set invalid")
	}
	matches := 0
	for _, resource := range r.resources {
		if resource.Kind == "platform_device" && resource.TenantID == platformDeviceTenant && stringField(resource.Data, "serial") == activation.Serial && stringField(resource.Data, "tenant_id") == activation.ClaimedTenantID && oneOf(stringField(resource.Data, "state"), "ASSIGNED", "DEPLOYED") {
			matches++
		}
	}
	if matches != 1 {
		return errors.New("BLE enrollment no longer unique")
	}
	for _, source := range sources {
		current, ok := r.resources[resourceKey(source.Kind, source.ID)]
		if !ok || current.TenantID != source.Tenant || current.Version != source.Version {
			return errors.New("BLE authority generation changed")
		}
	}
	return nil
}
func (r *MemoryRepository) commitBLEServerChallenge(challenge ResourceRecord, sources []bleCertSource, activation DeviceActivationRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.persistenceReadyLocked(); e != nil {
		return e
	}
	if e := r.bleAuthorityCurrentLocked(sources, activation); e != nil {
		return e
	}
	if _, ok := r.resources[resourceKey(challenge.Kind, challenge.ID)]; ok {
		return errors.New("BLE challenge conflict")
	}
	r.resources[resourceKey(challenge.Kind, challenge.ID)] = challenge
	return r.persistLocked()
}
func (r *MemoryRepository) confirmedBLEServerCertificate(id, tenant string, sources []bleCertSource, activation DeviceActivationRequest) (ResourceRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.persistenceReadyLocked(); e != nil {
		return ResourceRecord{}, e
	}
	if e := r.bleAuthorityCurrentLocked(sources, activation); e != nil {
		return ResourceRecord{}, e
	}
	result, ok := r.resources[resourceKey("ble_server_certificate", id)]
	if !ok {
		return ResourceRecord{}, ErrNotFound
	}
	if result.TenantID != tenant {
		return ResourceRecord{}, errors.New("BLE certificate scope mismatch")
	}
	result.Data = edgeDeepMap(result.Data)
	return result, nil
}
func (r *MemoryRepository) commitBLEServerCertificate(challenge, result ResourceRecord, sources []bleCertSource, activation DeviceActivationRequest, now time.Time) (ResourceRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.persistenceReadyLocked(); e != nil {
		return ResourceRecord{}, e
	}
	if e := r.bleAuthorityCurrentLocked(sources, activation); e != nil {
		return ResourceRecord{}, e
	}
	current, ok := r.resources[resourceKey("ble_server_cert_challenge", challenge.ID)]
	if !ok || !immutableResourceEqual(current, challenge) {
		return ResourceRecord{}, errors.New("BLE challenge changed")
	}
	if old, ok := r.resources[resourceKey("ble_server_certificate", challenge.ID)]; ok {
		if old.TenantID != result.TenantID || stringField(old.Data, "proof_sha256") != stringField(result.Data, "proof_sha256") || stringField(old.Data, "challenge_sha256") != stringField(result.Data, "challenge_sha256") {
			return ResourceRecord{}, errors.New("BLE certificate replay conflict")
		}
		old.Data = edgeDeepMap(old.Data)
		return old, nil
	}
	var c BLEServerCertChallenge
	if decodeBLECanonical(stringField(current.Data, "canonical_json"), &c) != nil || !validBLEChallenge(c) || now.Unix() < c.IssuedAt || now.Unix() >= c.ExpiresAt {
		return ResourceRecord{}, errors.New("BLE challenge expired before commit")
	}
	r.resources[resourceKey("ble_server_certificate", result.ID)] = result
	if e := r.persistLocked(); e != nil {
		return ResourceRecord{}, e
	}
	result.Data = edgeDeepMap(result.Data)
	return result, nil
}
