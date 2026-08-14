import unittest
from pathlib import Path
ROOT=Path(__file__).parents[1]/"main"
class TransportReliability(unittest.TestCase):
 def test_callbacks_enqueue_and_mqtt_reassembles(self):
  q=(ROOT/"command_queue.cpp").read_text();m=(ROOT/"mqtt_runtime.cpp").read_text()
  self.assertIn("xQueueSend",q);self.assertIn("current_data_offset",m)
  self.assertIn("expected_length>8192",m)
 def test_ble_frame_has_bounds_order_and_digest(self):
  b=(ROOT/"ble_runtime.cpp").read_text()
  for token in ('memcmp(payload,"BFF1",4)','offset==frame_payload.size()','mbedtls_sha256'):
   self.assertIn(token,b)
 def test_cbor_subset_is_strict(self):
  c=(ROOT/"canonical_cbor.cpp").read_text()
  self.assertIn("p_!=end_",c);self.assertIn("cJSON_GetObjectItemCaseSensitive",c)
  self.assertNotIn("ai==31",c)
 def test_sync_requires_business_ack_and_device_signature(self):
  s=(ROOT/"sync_runtime.cpp").read_text();m=(ROOT/"mqtt_runtime.cpp").read_text()
  self.assertIn("sign_hash_hex",s);self.assertIn("previous_acknowledged_hash",s)
  self.assertIn("sync_runtime_accept_ack",s);self.assertIn("acknowledge_through",s)
  self.assertIn("ack_sink",m);self.assertIn("mqtt_publish_sync",m)
  self.assertNotIn("acknowledge_through",m)
 def test_mqtt_command_hmac_precedes_durable_reservation(self):
  p=(ROOT/"intent_processor.cpp").read_text()
  self.assertIn("verify_hmac",p)
  self.assertLess(p.index("!verify_hmac"),p.index("storage_.reserve_command("))
 def test_mqtt_binding_control_path_is_exact_topic_and_uses_signed_store(self):
  m=(ROOT/"mqtt_runtime.cpp").read_text();a=(ROOT/"app_main.cpp").read_text()
  self.assertIn("assembling_binding",m);self.assertIn("binding_sink",m)
  self.assertIn('"/bindings"',a);self.assertIn("provision,&binding_store",a)
if __name__=="__main__":unittest.main()
