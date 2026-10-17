package domain

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestFiscalUNPAllocatorSharedOnlineAndOfflineSerialLedger(t *testing.T) {
	store := &testStore{}
	r, _ := NewPersistentRepository(store)
	start, end, err := r.ReserveFiscalUNPRange("tenant", "AB123456", "12345678", 1000)
	if err != nil || start != 1 || end != 1000 {
		t.Fatal(start, end, err)
	}
	sale := Sale{ID: "sale1", TenantID: "tenant", ExternalID: "ext1", OperatorID: "A001", FiscalDevice: FiscalDeviceSnapshot{FiscalMemoryNumber: "12345678", FiscalDeviceNumber: "AB123456"}}
	out, err := r.OpenSaleWithFirstLine(sale, SaleLine{LineID: "line"}, "AB123456")
	if err != nil || out.UNP != "AB123456-A001-0001001" {
		t.Fatal(out.UNP, err)
	}
	r, err = NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	start, end, err = r.ReserveFiscalUNPRange("tenant", "AB123456", "12345678", 10)
	if err != nil || start != 1002 || end != 1011 {
		t.Fatal(start, end, err)
	}
	if r.unp["tenant\n12345678"] != 0 {
		t.Fatal("FM key allocated")
	}
}
func TestFiscalUNPAllocatorKnownSaleHighwaterAcrossOperators(t *testing.T) {
	r := NewMemoryRepository()
	r.sales["old"] = Sale{ID: "old", TenantID: "tenant", UNP: "AB123456-B002-0009000", FiscalDevice: FiscalDeviceSnapshot{FiscalMemoryNumber: "12345678"}}
	start, _, err := r.ReserveFiscalUNPRange("tenant", "AB123456", "12345678", 1)
	if err != nil || start != 9001 {
		t.Fatal(start, err)
	}
}
func TestFiscalUNPAllocatorLegacyAuthorityCannotBeRenamed(t *testing.T) {
	for _, kind := range []string{"fm-ledger", "unscoped-fm", "unscoped-serial", "device-lease", "activation-lease"} {
		t.Run(kind, func(t *testing.T) {
			r := NewMemoryRepository()
			switch kind {
			case "fm-ledger":
				r.unp["tenant\n12345678"] = 1000
			case "unscoped-fm":
				r.unp["12345678"] = 1000
			case "unscoped-serial":
				r.unp["AB123456"] = 1000
			case "device-lease":
				r.resources["device:d"] = ResourceRecord{Kind: "device", TenantID: "tenant", Data: map[string]any{"status": "REVOKED", "unp_prefix": "12345678", "unp_range_end": int64(1000)}}
			case "activation-lease":
				r.activationRequests["a"] = DeviceActivationRequest{ClaimedTenantID: "tenant", UNPPrefix: "12345678", UNPRangeEnd: 1000, State: "REVOKED"}
			}
			if _, _, err := r.ReserveFiscalUNPRange("tenant", "AB123456", "12345678", 1); !errors.Is(err, ErrLegacyUNPLeaseFenceRequired) {
				t.Fatal(err)
			}
			if r.unp["tenant\nAB123456"] != 0 {
				t.Fatal("dirty ledger")
			}
			sale := Sale{ID: "s", TenantID: "tenant", ExternalID: "ext", OperatorID: "A001", FiscalDevice: FiscalDeviceSnapshot{FiscalMemoryNumber: "12345678"}}
			if _, err := r.OpenSaleWithFirstLine(sale, SaleLine{}, "AB123456"); !errors.Is(err, ErrLegacyUNPLeaseFenceRequired) {
				t.Fatal("online bypass", err)
			}
		})
	}
}
func TestFiscalUNPAllocatorIdentityCollisionsFailBeforeMutation(t *testing.T) {
	for _, kind := range []string{"fm-shared", "serial-changed-fm", "foreign-tenant", "sale-foreign"} {
		t.Run(kind, func(t *testing.T) {
			r := NewMemoryRepository()
			serial, fm, tenant := "AB999999", "12345678", "tenant"
			if kind == "serial-changed-fm" {
				serial = "AB123456"
				fm = "87654321"
			}
			if kind == "foreign-tenant" {
				serial = "AB123456"
				tenant = "foreign"
			}
			if kind == "sale-foreign" {
				r.sales["other"] = Sale{TenantID: "foreign", UNP: "AB123456-A001-0000001"}
			} else {
				r.resources["device:other"] = ResourceRecord{Kind: "device", TenantID: tenant, Data: map[string]any{"fiscal_device_number": serial, "fiscal_memory_number": fm}}
			}
			if _, _, err := r.ReserveFiscalUNPRange("tenant", "AB123456", "12345678", 1); !errors.Is(err, ErrFiscalUNPIdentityConflict) {
				t.Fatal(err)
			}
			if r.unp["tenant\nAB123456"] != 0 {
				t.Fatal("dirty ledger")
			}
		})
	}
}
func TestFiscalUNPAllocatorConcurrentDisjointRanges(t *testing.T) {
	r := NewMemoryRepository()
	var wg sync.WaitGroup
	ch := make(chan int64, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start, _, err := r.ReserveFiscalUNPRange("tenant", "AB123456", "12345678", 10)
			if err != nil {
				t.Error(err)
			}
			ch <- start
		}()
	}
	wg.Wait()
	close(ch)
	seen := map[int64]bool{}
	for start := range ch {
		if seen[start] || start < 1 || (start-1)%10 != 0 {
			t.Fatal(start)
		}
		seen[start] = true
	}
	if len(seen) != 20 {
		t.Fatal(seen)
	}
}
func TestFiscalUNPAllocatorIdentityImmutable(t *testing.T) {
	r := NewMemoryRepository()
	if _, _, err := r.ReserveFiscalUNPRange("tenant", "AB123456", "12345678", 1); err != nil {
		t.Fatal(err)
	}
	rows := r.Resources("unp_serial_identity", "tenant")
	if len(rows) != 1 {
		t.Fatal(rows)
	}
	v := rows[0]
	v.Data["fiscal_memory_number"] = "87654321"
	v.Version++
	if err := r.PutResource(v); err == nil {
		t.Fatal("identity mutable")
	}
}
func TestFiscalUNPAllocatorSaveAndReloadFailureRollsBack(t *testing.T) {
	for _, online := range []bool{false, true} {
		t.Run(fmt.Sprint(online), func(t *testing.T) {
			store := &edgeFaultStore{testStore: &testStore{}}
			r, _ := NewPersistentRepository(store)
			store.saveFail = true
			store.loadFail = true
			if online {
				_, err := r.OpenSaleWithFirstLine(Sale{ID: "s", TenantID: "tenant", ExternalID: "ext", OperatorID: "A001", FiscalDevice: FiscalDeviceSnapshot{FiscalMemoryNumber: "12345678"}}, SaleLine{}, "AB123456")
				if err == nil {
					t.Fatal("save ignored")
				}
			} else {
				if _, _, err := r.ReserveFiscalUNPRange("tenant", "AB123456", "12345678", 10); err == nil {
					t.Fatal("save ignored")
				}
			}
			if len(r.sales) != 0 || len(r.audit) != 0 || len(r.Resources("unp_serial_identity", "tenant")) != 0 || r.unp["tenant\nAB123456"] != 0 {
				t.Fatal("dirty failed allocation")
			}
			store.saveFail = false
			store.loadFail = false
			start, _, err := r.ReserveFiscalUNPRange("tenant", "AB123456", "12345678", 10)
			if err != nil || start != 1 {
				t.Fatal(start, err)
			}
		})
	}
}

func TestFiscalUNPAllocatorDoesNotReenableLegacyFMAPI(t *testing.T) {
	r := NewMemoryRepository()
	if _, _, err := r.ReserveFiscalUNPRange("tenant", "AB123456", "12345678", 10); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.ReserveUNPRange("tenant", "12345678", 10); !errors.Is(err, ErrLegacyUNPLeaseFenceRequired) {
		t.Fatal(err)
	}
}
func TestFiscalUNPAllocatorRejectsInvalidOrExhaustedWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		serial, fm  string
		count, high int64
	}{{"bad", "12345678", 1, 0}, {"AB123456", "bad", 1, 0}, {"AB123456", "12345678", 0, 0}, {"AB123456", "12345678", 10001, 0}, {"AB123456", "12345678", 2, 9999998}, {"AB123456", "12345678", 1, -1}} {
		r := NewMemoryRepository()
		r.unp["tenant\nAB123456"] = tc.high
		if _, _, err := r.ReserveFiscalUNPRange("tenant", tc.serial, tc.fm, tc.count); err == nil {
			t.Fatal(tc)
		}
		if r.unp["tenant\nAB123456"] != tc.high || len(r.resources) != 0 {
			t.Fatal("dirty invalid reservation")
		}
	}
}

func TestFiscalUNPAllocatorRetainsAllIssuedSerialHighwater(t *testing.T) {
	for _, source := range []string{"device", "issued_binding", "activation"} {
		t.Run(source, func(t *testing.T) {
			r := NewMemoryRepository()
			switch source {
			case "device":
				r.resources["device:adapter"] = ResourceRecord{Kind: "device", TenantID: "tenant", Data: map[string]any{"unp_prefix": "AB123456", "unp_range_end": int64(8000)}}
			case "issued_binding":
				r.resources["issued_binding:b"] = ResourceRecord{Kind: "issued_binding", TenantID: "tenant", Data: map[string]any{"canonical_payload": map[string]any{"fiscal_endpoint": map[string]any{"fiscal_device_number": "AB123456", "fiscal_memory_number": "12345678"}, "operational_authority": map[string]any{"unp_prefix": "AB123456", "unp_range_end": int64(8000)}}}}
			case "activation":
				r.activationRequests["a"] = DeviceActivationRequest{ClaimedTenantID: "tenant", UNPPrefix: "AB123456", UNPRangeEnd: 8000}
			}
			start, _, err := r.ReserveFiscalUNPRange("tenant", "AB123456", "12345678", 1)
			if err != nil || start != 8001 {
				t.Fatal(start, err)
			}
		})
	}
}
func TestFiscalUNPAllocatorHistoricalBindingLegacyLeaseRequiresFence(t *testing.T) {
	r := NewMemoryRepository()
	r.resources["issued_binding:b"] = ResourceRecord{Kind: "issued_binding", TenantID: "tenant", Data: map[string]any{"canonical_payload": map[string]any{"operational_authority": map[string]any{"unp_prefix": "12345678", "unp_range_end": int64(1000)}}}}
	if _, _, err := r.ReserveFiscalUNPRange("tenant", "AB123456", "12345678", 1); !errors.Is(err, ErrLegacyUNPLeaseFenceRequired) {
		t.Fatal(err)
	}
}
