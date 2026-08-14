package domain

import (
	"errors"
	"sync"
)

// BindingDriverResolver maps a frozen tenant/register/device tuple to a driver.
// The key includes the binding generation, so rebind immediately fences stale
// commands and there is no global-driver fallback.
type BindingDriverResolver struct {
	mu     sync.RWMutex
	routes map[string]Driver
}

func NewBindingDriverResolver() *BindingDriverResolver {
	return &BindingDriverResolver{routes: map[string]Driver{}}
}

func routeKey(tenant, register string, device FiscalDeviceSnapshot) string {
	return tenant + "\x00" + register + "\x00" + device.DeviceID + "\x00" +
		fmtInt64(device.BindingVersion)
}

func fmtInt64(v int64) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	negative := v < 0
	if negative {
		v = -v
	}
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if negative {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func (r *BindingDriverResolver) Bind(tenant, register string, device FiscalDeviceSnapshot, driver Driver) error {
	if r == nil || tenant == "" || register == "" || device.DeviceID == "" ||
		device.BindingVersion < 1 || driver == nil {
		return errors.New("invalid driver binding")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[routeKey(tenant, register, device)] = driver
	return nil
}

func (r *BindingDriverResolver) Unbind(tenant, register string, device FiscalDeviceSnapshot) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.routes, routeKey(tenant, register, device))
}

func (r *BindingDriverResolver) Resolve(tenant, register string, device FiscalDeviceSnapshot) (Driver, error) {
	if r == nil {
		return nil, errors.New("driver resolver unavailable")
	}
	r.mu.RLock()
	driver := r.routes[routeKey(tenant, register, device)]
	r.mu.RUnlock()
	if driver == nil {
		return nil, errors.New("bound device route unavailable")
	}
	return driver, nil
}
