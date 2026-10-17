import unittest
from pathlib import Path

ROOT=Path(__file__).parents[1]
SOURCE=(ROOT/"main"/"intent_processor.cpp").read_text()
COMPACT="".join(SOURCE.split())
APP=(ROOT/"main"/"app_main.cpp").read_text()

class IntentProcessorContract(unittest.TestCase):
 def test_same_processor_is_used_for_mqtt_and_ble(self):
  self.assertIn("mqtt_runtime_start(mqtt_binding,",APP)
  self.assertIn("ble_runtime_start(ble_binding,",APP)
  self.assertEqual(APP.count("queued_command_sink"),2)
  self.assertNotIn("ESP_ERR_NOT_FINISHED",APP)
 def test_binding_and_digest_precede_reservation(self):
  self.assertLess(COMPACT.index("tenant!=binding_.tenant_id"),COMPACT.index("storage_.reserve_command("))
  self.assertLess(COMPACT.index("p.payload_digest!=provided"),COMPACT.index("storage_.reserve_command("))
 def test_reservation_precedes_executor(self):
  self.assertLess(SOURCE.index("storage_.reserve_command("),SOURCE.index("executor_.execute"))
  self.assertLess(SOURCE.index("accepted_json"),SOURCE.index("executor_.execute"))
  self.assertIn("ReservationStatus::PayloadConflict",SOURCE)
 # Durability is exercised against actual IntentProcessor + SQLite by
 # run_storage_host.sh (included in make iot-test), not inferred from tokens.
 # Nullable offline server_sale_id and distinct client surrogate are exercised
 # by authority_host/run.py against Go-issued commands, actual IntentProcessor
 # and SQLite. Non-sale execution is covered by run_storage_host.sh.
 def test_startup_recovery_is_called_before_transports(self):
  self.assertLess(APP.index("recover_pending"),APP.index("mqtt_runtime_start"))

if __name__=="__main__":unittest.main()
