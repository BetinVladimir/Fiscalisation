package api

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// knownRoutes is the canonical list registered by newHandler (excluding
// optional integration routes which depend on an integrationHandler).
var knownRoutes = []string{
	"/connectivity/ping",
	"/public/v1/connectivity/ping",
	"/livez",
	"/readyz",
	"/metrics",
	"/healthz",
	"/public/v1/version",
	"/public/v1/sales",
	"/public/v1/sales:open-with-line",
	"/public/v1/sales/",
	"/public/v1/operations/",
	"/public/v1/operations",
	"/public/v1/devices/",
	"/public/v1/devices",
	"/public/v1/organizations",
	"/public/v1/locations",
	"/public/v1/locations/",
	"/public/v1/registers",
	"/public/v1/shifts",
	"/public/v1/shifts/",
	"/public/v1/registers/",
	"/public/v1/workstations/",
	"/public/v1/connectivity-probes/",
	"/public/v1/operators",
	"/public/v1/operators/",
	"/public/v1/reports",
	"/public/v1/reports/",
	"/public/v1/audit-events",
	"/public/v1/exports",
	"/public/v1/exports/periodized",
	"/public/v1/exports/",
	"/public/v1/tax-groups",
	"/public/v1/country-policy",
	"/public/v1/webhook-endpoints",
	"/public/v1/webhook-endpoints/",
	"/public/v1/ble-sessions/",
	"/public/v1/edge-sync/batches",
	"/public/v1/payment-reconciliations:authorize",
	"/public/v1/payment-reconciliations:bind-administrator",
	"/public/v1/execution-authorizations",
	"/public/v1/operators:bind-identity",
	"/public/v1/device-activation-requests/",
	"/public/v1/device-activation-requests:lookup",
	"/public/v1/",
	"/device-bootstrap/v1/challenges",
	"/device-bootstrap/v1/activation-requests",
	"/device-bootstrap/v1/activation-requests/",
	"/platform/v1/manufacturing/devices:register",
	"/platform/v1/devices",
	"/platform/v1/devices/",
}

func TestRouteTableSnapshot(t *testing.T) {
	goldenPath := filepath.Join("testdata", "routes.golden")
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}

	golden := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			golden = append(golden, line)
		}
	}

	actual := make([]string, len(knownRoutes))
	copy(actual, knownRoutes)
	sort.Strings(actual)
	sort.Strings(golden)

	if len(actual) != len(golden) {
		t.Errorf("route count: got %d, golden has %d", len(actual), len(golden))
	}
	for i := range golden {
		if i >= len(actual) {
			t.Errorf("missing route %q", golden[i])
			continue
		}
		if actual[i] != golden[i] {
			t.Errorf("route[%d]: got %q, want %q", i, actual[i], golden[i])
		}
	}
	for i := len(golden); i < len(actual); i++ {
		t.Errorf("extra route not in golden: %q", actual[i])
	}
}
