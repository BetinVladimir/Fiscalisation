package domain

import (
	"errors"
	"time"
)

var ErrLegacyUNPLeaseFenceRequired = errors.New("LEGACY_UNP_LEASE_FENCE_REQUIRED")
var ErrFiscalUNPIdentityConflict = errors.New("FISCAL_UNP_IDENTITY_CONFLICT")

// ReserveFiscalUNPRange reserves from the same serial ledger as online sales.
// Legacy FM-keyed leases have no expiry: a backend status change cannot prove
// their cached physical authority was erased. Such migration remains fenced.
func (r *MemoryRepository) ReserveFiscalUNPRange(tenant, serial, fm string, count int64) (int64, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if tenant == "" || !bgFMINPattern.MatchString(serial) || !bgFMINPattern.MatchString(fm) || count < 1 || count > 10000 {
		return 0, 0, errors.New("invalid fiscal UNP reservation")
	}
	high, identity, err := r.fiscalUNPHighwaterLocked(tenant, serial, fm)
	if err != nil {
		return 0, 0, err
	}
	if high > BGUNPSequenceMax-count {
		return 0, 0, errors.New("UNP range exhausted")
	}
	key := tenant + "\n" + serial
	oldCounter, hadCounter := r.unp[key]
	oldIdentity, hadIdentity := r.resources[resourceKey(identity.Kind, identity.ID)]
	r.unp[key] = high + count
	r.resources[resourceKey(identity.Kind, identity.ID)] = identity
	if err = r.persistLocked(); err != nil {
		// persistLocked normally reloads durable state; restore locally as well when
		// the store is simultaneously unavailable for reload.
		if r.unp[key] == high+count {
			if hadCounter {
				r.unp[key] = oldCounter
			} else {
				delete(r.unp, key)
			}
			if hadIdentity {
				r.resources[resourceKey(identity.Kind, identity.ID)] = oldIdentity
			} else {
				delete(r.resources, resourceKey(identity.Kind, identity.ID))
			}
		}
		return 0, 0, err
	}
	return high + 1, high + count, nil
}
func (r *MemoryRepository) fiscalUNPHighwaterLocked(tenant, serial, fm string) (int64, ResourceRecord, error) {
	empty := ResourceRecord{}
	if tenant == "" || !bgFMINPattern.MatchString(serial) || !bgFMINPattern.MatchString(fm) {
		return 0, empty, ErrFiscalUNPIdentityConflict
	}
	id := edgeDigest(map[string]any{"tenant_id": tenant, "fiscal_device_number": serial})
	observedHigh := int64(0)
	identity := ResourceRecord{Kind: "unp_serial_identity", ID: id, TenantID: tenant, Version: 1, Data: map[string]any{"allocator": "SERIAL_V1", "fiscal_device_number": serial, "fiscal_memory_number": fm}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	for _, record := range r.resources {
		if record.TenantID != tenant {
			if stringAny(record.Data, "fiscal_device_number") == serial {
				return 0, empty, ErrFiscalUNPIdentityConflict
			}
			if record.Kind == "issued_binding" {
				b, _ := record.Data["canonical_payload"].(map[string]any)
				e, _ := b["fiscal_endpoint"].(map[string]any)
				if stringAny(e, "fiscal_device_number") == serial {
					return 0, empty, ErrFiscalUNPIdentityConflict
				}
			}
			continue
		}
		switch record.Kind {
		case "unp_serial_identity":
			otherSerial := stringAny(record.Data, "fiscal_device_number")
			otherFM := stringAny(record.Data, "fiscal_memory_number")
			if (otherSerial == serial && otherFM != fm) || (otherFM == fm && otherSerial != serial) {
				return 0, empty, ErrFiscalUNPIdentityConflict
			}
			if record.ID == id {
				if otherSerial != serial || otherFM != fm || stringAny(record.Data, "allocator") != "SERIAL_V1" {
					return 0, empty, ErrFiscalUNPIdentityConflict
				}
				identity = record
			}
		case "device":
			if stringAny(record.Data, "unp_prefix") == serial && intAny(record.Data, "unp_range_end") > observedHigh {
				observedHigh = intAny(record.Data, "unp_range_end")
			}
			otherSerial := stringAny(record.Data, "fiscal_device_number")
			otherFM := stringAny(record.Data, "fiscal_memory_number")
			if otherSerial != "" && otherFM != "" && ((otherSerial == serial && otherFM != fm) || (otherFM == fm && otherSerial != serial)) {
				return 0, empty, ErrFiscalUNPIdentityConflict
			}
		case "issued_binding":
			binding, _ := record.Data["canonical_payload"].(map[string]any)
			authority, _ := binding["operational_authority"].(map[string]any)
			if stringAny(authority, "unp_prefix") == fm && fm != serial && intAny(authority, "unp_range_end") > 0 {
				return 0, empty, ErrLegacyUNPLeaseFenceRequired
			}
			if stringAny(authority, "unp_prefix") == serial && intAny(authority, "unp_range_end") > observedHigh {
				observedHigh = intAny(authority, "unp_range_end")
			}
			endpoint, _ := binding["fiscal_endpoint"].(map[string]any)
			otherSerial := stringAny(endpoint, "fiscal_device_number")
			otherFM := stringAny(endpoint, "fiscal_memory_number")
			if otherSerial != "" && otherFM != "" && ((otherSerial == serial && otherFM != fm) || (otherFM == fm && otherSerial != serial)) {
				return 0, empty, ErrFiscalUNPIdentityConflict
			}
		}
	}
	// Neither high-water copying nor key renaming invalidates already issued
	// legacy leases. Require a separate verified migration/physical fencing flow.
	if fm != serial && (r.unp[tenant+"\n"+fm] > 0 || r.unp[fm] > 0) {
		return 0, empty, ErrLegacyUNPLeaseFenceRequired
	}
	for _, request := range r.activationRequests {
		if request.ClaimedTenantID == tenant && request.UNPPrefix == serial && request.UNPRangeEnd > observedHigh {
			observedHigh = request.UNPRangeEnd
		}
		if request.ClaimedTenantID == tenant && request.UNPRangeEnd > 0 && request.UNPPrefix == fm && fm != serial {
			return 0, empty, ErrLegacyUNPLeaseFenceRequired
		}
	}
	for _, record := range r.resources {
		if record.TenantID == tenant && record.Kind == "device" && stringAny(record.Data, "unp_prefix") == fm && fm != serial && intAny(record.Data, "unp_range_end") > 0 {
			return 0, empty, ErrLegacyUNPLeaseFenceRequired
		}
	}
	if r.unp[serial] > 0 {
		return 0, empty, ErrLegacyUNPLeaseFenceRequired
	}
	high := r.unp[tenant+"\n"+serial]
	if high < 0 {
		return 0, empty, errors.New("invalid fiscal UNP highwater")
	}
	if observedHigh > high {
		high = observedHigh
	}
	for _, sale := range r.sales {
		u, e := ParseBGUNP(sale.UNP)
		if e == nil && u.FiscalDeviceNumber == serial && sale.TenantID != tenant {
			return 0, empty, ErrFiscalUNPIdentityConflict
		}
		if sale.TenantID != tenant {
			continue
		}
		if e != nil || u.FiscalDeviceNumber != serial {
			continue
		}
		if sale.FiscalDevice.FiscalMemoryNumber != "" && sale.FiscalDevice.FiscalMemoryNumber != fm {
			return 0, empty, ErrFiscalUNPIdentityConflict
		}
		if u.Sequence > high {
			high = u.Sequence
		}
	}
	if high < 0 || high > BGUNPSequenceMax {
		return 0, empty, errors.New("invalid fiscal UNP highwater")
	}
	return high, identity, nil
}
