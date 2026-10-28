package com.beeloy.fiscal.bluecash

import java.security.MessageDigest
import java.util.Base64
import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec
import org.json.JSONArray
import org.json.JSONObject

/** Typed Go SyncAck order is distinct from sorted maps, including nil slices. */
object BlueCashSyncWire {
  internal fun eventWire(id: String, operationId: String, deviceId: String, sequence: Long,
    type: String, occurredAt: String, payload: JSONObject, previous: String?, hash: String = "", signature: String? = null): String =
    "{\"event_id\":${CanonicalJson.quote(id)},\"operation_id\":${CanonicalJson.quote(operationId)},\"device_id\":${CanonicalJson.quote(deviceId)},\"journal_seq\":$sequence,\"event_type\":${CanonicalJson.quote(type)},\"occurred_at\":${CanonicalJson.quote(occurredAt)},\"payload\":${CanonicalJson.encode(payload)},\"prev_hash\":${previous?.let { CanonicalJson.quote(it) } ?: "null"},\"event_hash\":${CanonicalJson.quote(hash)},\"signature\":${signature?.let { CanonicalJson.quote(it) } ?: "null"}}"

  internal fun batchWire(deviceId: String, first: Long, last: Long, previous: String?,
    eventWire: List<String>, hash: String = "", signature: String = ""): String =
    "{\"edge_id\":${CanonicalJson.quote(deviceId)},\"schema_version\":\"2026-08-07\",\"first_seq\":$first,\"last_seq\":$last,\"previous_acknowledged_hash\":${previous?.let { CanonicalJson.quote(it) } ?: "null"},\"events\":[${eventWire.joinToString(",")}],\"batch_sha256\":${CanonicalJson.quote(hash)},\"signature\":${CanonicalJson.quote(signature)}}"

  internal fun unsignedAck(ack: JSONObject): String {
    val resultsValue = ack.get("operation_results")
    val resultWire = if (resultsValue === JSONObject.NULL) "null" else {
      require(resultsValue is JSONArray) { "MQTT_ACK_RESULTS" }
      (0 until resultsValue.length()).joinToString(",", "[", "]") { i ->
        val v = resultsValue.getJSONObject(i)
        "{\"operation_id\":${CanonicalJson.quote(v.getString("operation_id"))},\"state\":${CanonicalJson.quote(v.getString("state"))},\"version\":${CanonicalJson.int64(v.get("version"))}}"
      }
    }
    val rejected = ack.get("rejected")
    require(rejected === JSONObject.NULL || rejected is JSONArray) { "MQTT_ACK_REJECTED" }
    return "{\"ack_id\":${CanonicalJson.quote(ack.getString("ack_id"))},\"edge_id\":${CanonicalJson.quote(ack.getString("edge_id"))},\"committed_through_seq\":${CanonicalJson.int64(ack.get("committed_through_seq"))},\"committed_event_hash\":${CanonicalJson.quote(ack.getString("committed_event_hash"))},\"committed_at\":${CanonicalJson.quote(ack.getString("committed_at"))},\"operation_results\":$resultWire,\"rejected\":${CanonicalJson.encode(rejected)},\"signature\":\"\"}"
  }
  fun verifyAck(ack: JSONObject, expectedDeviceId: String, key: ByteArray): Boolean = runCatching {
    require(ack.getString("edge_id") == expectedDeviceId)
    val supplied = Base64.getUrlDecoder().decode(ack.getString("signature"))
    val expected = Mac.getInstance("HmacSHA256").run {
      init(SecretKeySpec(key, "HmacSHA256"))
      doFinal(unsignedAck(ack).toByteArray(Charsets.UTF_8))
    }
    MessageDigest.isEqual(expected, supplied)
  }.getOrDefault(false)
}
