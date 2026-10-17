package domain

import "testing"

func TestCompositeExecutionStrictNonSecretProfile(t *testing.T) {
	datecs := map[string]any{"operator_number": 2, "operator_count": 30, "operator_password_ref": "fiscal-pass", "device_configuration_id": "installed-config", "till_number": 24, "payment_mapping": map[string]any{"CASH": 0, "CARD": 2}}
	if _, e := compositeExecution(map[string]any{"execution": datecs}, "DATECS_DP150_BLUEPAD50"); e != nil {
		t.Fatal(e)
	}
	for name, change := range map[string]func(map[string]any){
		"fractional operator":     func(v map[string]any) { v["operator_number"] = 1.5 },
		"secret in signed config": func(v map[string]any) { v["operator_password"] = "12345678" },
		"foreign mapping":         func(v map[string]any) { v["payment_mapping"] = map[string]any{"CASH": 0, "CARD": 3} },
		"missing cash":            func(v map[string]any) { v["payment_mapping"] = map[string]any{"CARD": 1} },
		"missing configuration":   func(v map[string]any) { delete(v, "device_configuration_id") },
		"bad operator count":      func(v map[string]any) { v["operator_count"] = 29 },
		"unsafe reference":        func(v map[string]any) { v["operator_password_ref"] = "../password" },
	} {
		t.Run(name, func(t *testing.T) {
			v := cloneMap(datecs)
			change(v)
			if _, e := compositeExecution(map[string]any{"execution": v}, "DATECS_DP150_BLUEPAD50"); e == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
	daisy := map[string]any{"operator_number": 2, "operator_count": 30, "operator_password_ref": "fiscal-pass", "device_configuration_id": "installed-config", "payment_mapping": map[string]any{"CASH": "P", "CARD": "C"}}
	if _, e := compositeExecution(map[string]any{"execution": daisy}, "DAISY_COMPACT_S01"); e != nil {
		t.Fatal(e)
	}
	daisy["till_number"] = 1
	if _, e := compositeExecution(map[string]any{"execution": daisy}, "DAISY_COMPACT_S01"); e == nil {
		t.Fatal("Datecs field accepted in Daisy")
	}
}
