import unittest
from pathlib import Path

ROOT=Path(__file__).parents[1]
MAIN=ROOT/"main"

class PhysicalProfiles(unittest.TestCase):
 def test_idf_links_authoritative_protocol_core(self):
  cmake=(MAIN/"CMakeLists.txt").read_text()
  for name in ("CommandPayload.cpp","FrameCodec.cpp","AllCommands.cpp"):
   self.assertIn(name,cmake)
 def test_pending_executor_is_removed(self):
  app=(MAIN/"app_main.cpp").read_text()
  self.assertNotIn("PendingPhysicalExecutor",app)
  self.assertIn("ProfileExecutor",app)
 def test_datecs_uart_and_daisy_usb_are_binding_derived(self):
  executor=(MAIN/"profile_executor.cpp").read_text()
  uart=(MAIN/"uart_runtime.cpp").read_text()
  for token in ("buildDatecsOpenReceipt","buildDatecsSaleItem","buildDatecsPayment",
                "buildDaisyOpenReceipt","buildDaisySaleItem","buildDaisyPayment"):
   self.assertIn(token,executor)
  # Actual serial codec/correlation/error semantics are run by run_native_serial.sh.
  for token in ("uart_baud","uart_parity","uart_tx_pin","uart_rx_pin"):
   self.assertIn(token,uart)
 def test_unknown_after_send_is_not_blindly_retried(self):
  executor=(MAIN/"profile_executor.cpp").read_text()
  self.assertNotIn("for(int retry",executor)
  self.assertIn("FISCAL_CLOSE_UNKNOWN",executor)
 # Receipt-proof semantics are exercised by actual receipt_host/run.py, not helper-name tokens.
 def test_bluepad_is_a_real_binding_derived_central_not_a_readiness_stub(self):
  executor=(MAIN/"profile_executor.cpp").read_text();central=(MAIN/"bluepad_ble_central.cpp").read_text();cmake=(MAIN/"CMakeLists.txt").read_text()
  self.assertNotIn("payment_ready(){/*",executor);self.assertIn("bluepad_ble_ready",executor)
  for token in ("BLE_GAP_EVENT_CONNECT","BLE_GAP_EVENT_DISCONNECT","ble_gattc_disc_svc_by_uuid"):
   self.assertIn(token,central)
  self.assertIn("bluepad_ble_central.cpp",cmake)
  # Purchase correlation, receipt tags and asynchronous failures execute the
  # production central in mandatory bluepad_host/run.py; helper names are not
  # evidence that unverified lookup/reversal commands are supported.
  make=(ROOT.parents[3]/"Makefile").read_text()
  self.assertIn("tests/bluepad_host/run.py",make)

if __name__=="__main__":unittest.main()
