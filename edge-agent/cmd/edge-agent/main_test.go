package main

import "testing"

func TestLoadConfigGuardsProduction(t *testing.T) {
	base := map[string]string{"APP_ENV": "prod", "EDGE_ID": "edge", "DEVICE_ID": "device", "FISCAL_EDGE_SYNC_URL": "https://fiscal.example/public/v1/edge-sync/batches", "EDGE_SYNC_HMAC_KEY": "12345678901234567890123456789012", "WEBHOOK_VERIFICATION_KEY": "12345678901234567890123456789012", "FISCAL_AUTH_TOKEN": "jwt"}
	get := func(k string) string { return base[k] }
	if _, err := loadConfig(get); err != nil {
		t.Fatal(err)
	}
	base["FISCAL_EDGE_SYNC_URL"] = "http://fiscal/public/v1/edge-sync/batches"
	if _, err := loadConfig(get); err == nil {
		t.Fatal("HTTP accepted in PROD")
	}
}

func TestLoadConfigRejectsWeakAndInvalidInterval(t *testing.T) {
	base := map[string]string{"EDGE_ID": "edge", "DEVICE_ID": "device", "FISCAL_EDGE_SYNC_URL": "http://fiscal/public/v1/edge-sync/batches", "EDGE_SYNC_HMAC_KEY": "short", "WEBHOOK_VERIFICATION_KEY": "1234567890123456"}
	get := func(k string) string { return base[k] }
	if _, err := loadConfig(get); err == nil {
		t.Fatal("weak key accepted")
	}
	base["EDGE_SYNC_HMAC_KEY"] = "1234567890123456"
	base["SYNC_INTERVAL_MS"] = "1"
	if _, err := loadConfig(get); err == nil {
		t.Fatal("unsafe interval accepted")
	}
}

func TestLoadConfigRejectsSimulatorAndLocalAPIInProd(t *testing.T) {
	base := map[string]string{"APP_ENV": "prod", "EDGE_ID": "edge", "DEVICE_ID": "device", "FISCAL_EDGE_SYNC_URL": "https://fiscal.example/public/v1/edge-sync/batches", "EDGE_SYNC_HMAC_KEY": "12345678901234567890123456789012", "WEBHOOK_VERIFICATION_KEY": "12345678901234567890123456789012", "FISCAL_AUTH_TOKEN": "jwt", "DEVICE_ADAPTER": "simulator"}
	if _, err := loadConfig(func(k string) string { return base[k] }); err == nil {
		t.Fatal("simulator accepted in PROD")
	}
	base["APP_ENV"] = "dev"
	base["EDGE_LOCAL_API_TOKEN"] = "short"
	if _, err := loadConfig(func(k string) string { return base[k] }); err == nil {
		t.Fatal("weak local API token accepted")
	}
}

func TestLoadConfigRejectsInvalidAuthorityRange(t *testing.T) {
	base := map[string]string{"EDGE_ID": "edge", "DEVICE_ID": "device", "FISCAL_EDGE_SYNC_URL": "http://fiscal/public/v1/edge-sync/batches", "EDGE_SYNC_HMAC_KEY": "1234567890123456", "WEBHOOK_VERIFICATION_KEY": "1234567890123456", "OPERATION_FROM": "10", "OPERATION_TO": "9"}
	if _, err := loadConfig(func(k string) string { return base[k] }); err == nil {
		t.Fatal("invalid authority range accepted")
	}
}
