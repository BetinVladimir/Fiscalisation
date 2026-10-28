package com.beeloy.fiscal.bluecash

import java.io.File
import java.math.BigInteger
import org.json.JSONObject

private fun nullable(v: JSONObject, key: String): String? = if (v.isNull(key)) null else v.getString(key)
private fun eventWire(v: JSONObject) = BlueCashSyncWire.eventWire(v.getString("event_id"),
  v.getString("operation_id"), v.getString("device_id"), v.getLong("journal_seq"),
  v.getString("event_type"), v.getString("occurred_at"), v.getJSONObject("payload"),
  nullable(v,"prev_hash"),v.getString("event_hash"),nullable(v,"signature"))
fun main(args: Array<String>) {
  var count = 0
  File(args.single()).forEachLine { line ->
    val row = JSONObject(line)
    val input = row.getString("input")
    val value: Any = when (row.getString("kind")) {
      "string" -> input
      "map" -> JSONObject(input)
      "int64" -> input.toLong()
      "float64" -> Double.fromBits(java.lang.Long.parseUnsignedLong(input, 16))
      "float32" -> Float.fromBits(java.lang.Integer.parseUnsignedInt(input, 16))
      "event", "batch" -> {
        val v = JSONObject(input)
        val actual = if (row.getString("kind") == "event") eventWire(v) else {
          val events = v.getJSONArray("events")
          BlueCashSyncWire.batchWire(v.getString("edge_id"), v.getLong("first_seq"), v.getLong("last_seq"),
            nullable(v,"previous_acknowledged_hash"),(0 until events.length()).map { eventWire(events.getJSONObject(it)) },
            v.getString("batch_sha256"),v.getString("signature"))
        }
        check(actual == row.getString("want")) { "Typed Go wire mismatch: $actual" }
        count++
        return@forEachLine
      }
      "ack" -> {
        val ack = JSONObject(input)
        val key = "0123456789abcdef0123456789abcdef".toByteArray()
        check(BlueCashSyncWire.verifyAck(ack, "edge😀", key)) { "Go typed ACK HMAC mismatch: $input" }
        ack.put("committed_through_seq", Long.MAX_VALUE - 1)
        check(!BlueCashSyncWire.verifyAck(ack, "edge😀", key))
        ack.put("committed_through_seq", 1.5)
        check(!BlueCashSyncWire.verifyAck(ack, "edge😀", key))
        count++
        return@forEachLine
      }
      else -> error("Unknown oracle row")
    }
    val actual = CanonicalJson.encode(value)
    check(actual == row.getString("want")) {
      "Go byte mismatch kind=${row.getString("kind")} input=$input expected=${row.getString("want")} actual=$actual"
    }
    count++
  }
  // Additional strict type/Unicode rejection checks.
  for (value in listOf(Double.NaN, Double.POSITIVE_INFINITY, Float.NEGATIVE_INFINITY, Any())) {
    check(runCatching { CanonicalJson.encode(value) }.isFailure)
  }
  check(CanonicalJson.quote("\ud800") == "\"\\ufffd\"")
  check(CanonicalJson.quote("\udfff") == "\"\\ufffd\"")
  check(CanonicalJson.encode(BigInteger("9223372036854775807")) == "9223372036854775807")
  check(CanonicalJson.int64(BigInteger("-9223372036854775808")) == Long.MIN_VALUE)
  check(runCatching { CanonicalJson.int64(BigInteger("9223372036854775808")) }.isFailure)
  check(runCatching { CanonicalJson.int64(1.0) }.isFailure)
  println("Go encoding/json -> actual Kotlin CanonicalJson/typed ACK: $count oracle vectors PASS")
}
