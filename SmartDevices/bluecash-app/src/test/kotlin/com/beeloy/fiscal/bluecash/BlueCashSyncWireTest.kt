package com.beeloy.fiscal.bluecash

import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.nio.charset.StandardCharsets
import java.util.Base64
import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec

class BlueCashSyncWireTest {
 @Test fun verifiesExactGoAckAndRejectsTampering() {
  val key="0123456789abcdef0123456789abcdef".toByteArray()
  val unsigned="{\"ack_id\":\"ack-1\",\"edge_id\":\"edge-1\",\"committed_through_seq\":3,\"committed_event_hash\":\"abc\",\"committed_at\":\"2026-08-11T10:11:12.123456789Z\",\"operation_results\":[{\"operation_id\":\"op-1\",\"state\":\"FISCALIZED\",\"version\":3}],\"rejected\":[],\"signature\":\"\"}"
  val signature=Base64.getUrlEncoder().withoutPadding().encodeToString(Mac.getInstance("HmacSHA256").run{init(SecretKeySpec(key,"HmacSHA256"));doFinal(unsigned.toByteArray(StandardCharsets.UTF_8))})
  val ack=JSONObject().put("ack_id","ack-1").put("edge_id","edge-1").put("committed_through_seq",3).put("committed_event_hash","abc").put("committed_at","2026-08-11T10:11:12.123456789Z").put("operation_results",JSONArray().put(JSONObject().put("operation_id","op-1").put("state","FISCALIZED").put("version",3))).put("rejected",JSONArray()).put("signature",signature)
  assertTrue(BlueCashSyncWire.verifyAck(ack,"edge-1",key))
  ack.put("committed_through_seq",4)
  assertFalse(BlueCashSyncWire.verifyAck(ack,"edge-1",key))
 }
}
