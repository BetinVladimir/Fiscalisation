package domain

import "testing"

func TestBindingDriverResolverFencesTenantRegisterDeviceAndGeneration(t *testing.T) {
	r := NewBindingDriverResolver()
	d := FiscalDeviceSnapshot{DeviceID: "edge-1", BindingVersion: 7}
	driver := NewSimulator(true)
	if err := r.Bind("tenant-a", "register-a", d, driver); err != nil {
		t.Fatal(err)
	}
	if got, err := r.Resolve("tenant-a", "register-a", d); err != nil || got != driver {
		t.Fatal("exact binding did not resolve")
	}
	for _, mismatch := range []struct {
		tenant, register string
		device           FiscalDeviceSnapshot
	}{
		{"tenant-b", "register-a", d}, {"tenant-a", "register-b", d},
		{"tenant-a", "register-a", FiscalDeviceSnapshot{DeviceID: "edge-2", BindingVersion: 7}},
		{"tenant-a", "register-a", FiscalDeviceSnapshot{DeviceID: "edge-1", BindingVersion: 8}},
	} {
		if _, err := r.Resolve(mismatch.tenant, mismatch.register, mismatch.device); err == nil {
			t.Fatal("unsafe fallback resolved")
		}
	}
	r.Unbind("tenant-a", "register-a", d)
	if _, err := r.Resolve("tenant-a", "register-a", d); err == nil {
		t.Fatal("unbound route resolved")
	}
}
