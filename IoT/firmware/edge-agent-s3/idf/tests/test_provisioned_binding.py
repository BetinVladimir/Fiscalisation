import json
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).parents[1]
SOURCE = (ROOT / "main" / "provisioned_binding.cpp").read_text()
APP = (ROOT / "main" / "app_main.cpp").read_text()


def binding(profile: str) -> dict:
    base = {
        "schema_version": 2,
        "generation": 7,
        "tenant_id": "11111111-1111-4111-8111-111111111111",
        "location_id": "22222222-2222-4222-8222-222222222222",
        "register_id": "33333333-3333-4333-8333-333333333333",
        "edge_device_id": "44444444-4444-4444-8444-444444444444",
        "profile": profile,
        "ble_advertising_identity": "BeeFiscal-edge-44444444",
        "mqtt": {
            "uri": "mqtts://mqtt.example.test:8883",
            "client_id": "edge-44444444",
            "command_topic": "v1/edge/44444444/commands",
            "sync_topic": "v1/edge/44444444/sync",
            "ack_topic": "v1/edge/44444444/acks",
            "root_ca_ref": "mqtt-ca",
            "client_certificate_ref": "mqtt-cert",
            "client_key_ref": "mqtt-key",
        },
        "operational_authority": {
            "command_hmac_ref": "command-key",
            "sync_ack_hmac_ref": "sync-ack-key",
            "transaction_signing_kid": "edge-p256-44444444",
            "unp_prefix": "DT000001-01-",
            "unp_range_start": 1,
            "unp_range_end": 10000,
        },
    }
    if profile == "DATECS_DP150_BLUEPAD50":
        base["fiscal_endpoint"] = {
            "device_id": "55555555-5555-4555-8555-555555555555",
            "vendor": "DATECS", "model": "DP-150 MX", "transport": "RS232",
            "uart": {"baud": 115200, "data_bits": 8, "parity": "N",
                     "stop_bits": 1, "tx_pin": 17, "rx_pin": 18},
        }
        base["payment_endpoint"] = {
            "device_id": "66666666-6666-4666-8666-666666666666",
            "vendor": "DATECS", "model": "BLUEPAD-50 PLUS",
            "transport": "BLE_GATT", "ble_identity": "BP50-123",
            "service_uuid": "0000fe60-0000-1000-8000-00805f9b34fb",
            "tx_characteristic_uuid": "0000fe61-0000-1000-8000-00805f9b34fb",
            "rx_characteristic_uuid": "0000fe62-0000-1000-8000-00805f9b34fb",
        }
    else:
        base["fiscal_endpoint"] = {
            "device_id": "77777777-7777-4777-8777-777777777777",
            "vendor": "DAISY", "model": "COMPACT S 01", "transport": "USB_SERIAL",
            "usb": {"vid": 4660, "pid": 22136, "interface": 0, "serial": "DS-1"},
        }
        base["payment_endpoint"] = None
    return base


class ProvisionedBindingContract(unittest.TestCase):
    def test_golden_profiles_have_closed_shape(self):
        for profile in ("DATECS_DP150_BLUEPAD50", "DAISY_COMPACT_S01"):
            raw = json.dumps(binding(profile), separators=(",", ":"), sort_keys=True)
            self.assertLess(len(raw), 8_192)
            self.assertEqual(json.loads(raw)["profile"], profile)

    def test_install_is_signature_and_generation_gated(self):
        self.assertIn("mbedtls_pk_verify", SOURCE)
        self.assertIn("strcmp(e.key_id,key_id_)!=0", SOURCE)
        self.assertIn("b.generation<=generation_", SOURCE)
        self.assertLess(SOURCE.index("verify(e)"), SOURCE.index("nvs_set_blob(impl_->h,\"active\""))

    def test_active_and_generation_share_one_commit_boundary(self):
        body = re.search(r"ProvisionedBindingStore::install\(.*?\n\}", SOURCE, re.S).group(0)
        self.assertEqual(body.count("nvs_commit"), 1)
        self.assertIn('nvs_set_blob(impl_->h,"previous"', body)

    def test_unprovisioned_runtime_exposes_only_signed_install(self):
        unprovisioned = APP.index("unprovisioned: only signed binding installation")
        mqtt = APP.index("mqtt_runtime_start")
        self.assertLess(unprovisioned, mqtt)
        self.assertIn("ble_provisioning_start", APP)
        self.assertIn("store->install(envelope)", APP)
        self.assertNotIn('BleConfig ble_binding{"BeeFiscal-unprovisioned"}', APP)

    def test_secrets_are_references_not_values(self):
        self.assertIn('nvs_open("edge-secrets",NVS_READONLY', (ROOT / "main" / "runtime_config.cpp").read_text())
        self.assertNotIn("client_key_pem", SOURCE)


if __name__ == "__main__":
    unittest.main()
