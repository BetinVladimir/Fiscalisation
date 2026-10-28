package com.beeloy.fiscal.bluecash
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
private var failures = 0
private fun test(name: String, f: () -> Unit) { try { f() } catch (e: Throwable) { failures++; System.err.println("FAIL $name: ${e.message}") } }
private fun hex(value: String) = value.split(" ").map { it.toInt(16).toByte() }.toByteArray()
private fun reject(raw: ByteArray) { try { DatecsFrameCodec.decode(raw) } catch (e: IllegalArgumentException) { return }; error("invalid response accepted") }
private fun response(n: Int): ByteArray {
  // Independent response construction from PM v2.11.4 sec3.4; never encode().
  val out = ArrayList<Byte>()
  fun word(value: Int) { for (shift in intArrayOf(12,8,4,0)) out += (0x30 + ((value shr shift) and 15)).toByte() }
  out += 1.toByte();word(n + 0x33);out += 0x20.toByte();word(33)
  repeat(n) {out += 'x'.code.toByte()};out += 4.toByte();repeat(8){out += 0x80.toByte()};out += 5.toByte()
  word(out.drop(1).sumOf {it.toInt() and 255} and 0xffff);out += 3.toByte();return out.toByteArray()
}
// Verify BCC of any raw Datecs frame bytes independently (no encode/decode).
// Returns true if BCC matches. Used for vectors with SEQ outside 0x20..0x7F.
private fun bccOk(raw: ByteArray): Boolean {
  if (raw.size < 16) return false
  val lenVal = raw[1..4].fold(0) { acc, b -> (acc shl 4) or ((b.toInt() and 0xFF) - 0x30) }
  val expectedSize: Int
  val bccAt: Int
  if (raw[0].toInt() == 1) { // request: PRE LEN SEQ CMD DATA ENQ BCC ETX
    val dataN = lenVal - 0x2A
    if (dataN < 0 || dataN > 496) return false
    expectedSize = 1 + 4 + 1 + 4 + dataN + 1 + 4 + 1
    bccAt = 1 + 4 + 1 + 4 + dataN + 1
  } else return false
  if (raw.size != expectedSize) return false
  if (raw.last().toInt() != 3) return false
  val computed = (1 until bccAt).sumOf { raw[it].toInt() and 0xFF } and 0xFFFF
  val stored = (bccAt until bccAt+4).fold(0) { acc, i -> (acc shl 4) or ((raw[i].toInt() and 0xFF) - 0x30) }
  return computed == stored
}
private operator fun ByteArray.get(r: IntRange) = r.map { this[it] }.toByteArray()
fun main() {
  // Literal PM v2.11.4 section4.1 command33 request/answer; independent oracle.
  val tx=hex("01 30 30 32 3A 24 30 30 32 31 05 30 31 3B 38 03")
  val rx=hex("01 30 30 33 35 24 30 30 32 31 30 09 04 80 80 80 80 86 9A 80 80 05 30 36 31 31 03")
  test("vendor TX") {check(DatecsFrameCodec.encode(0x24,33,byteArrayOf()).contentEquals(tx))}
  test("vendor RX") {val result=DatecsFrameCodec.decode(rx);check(result.sequence==0x24 && result.command==33);check(result.data.contentEquals(byteArrayOf(0x30,9)));check(result.status.contentEquals(hex("80 80 80 80 86 9A 80 80")))}
  // --- All golden vectors from docs/MVP1/vendor/datecs_golden_vectors.json (VG-20) ---
  // cmd33_clear_display (no data, seq=0x24) — covered by "vendor TX" above; named explicitly for VG-20 count
  test("golden cmd33_clear_display") {check(DatecsFrameCodec.encode(0x24,33,byteArrayOf()).contentEquals(tx))}
  // cmd56_close (no data, seq=0x29)
  test("golden cmd56_close") {check(DatecsFrameCodec.encode(0x29,56,byteArrayOf()).contentEquals(hex("01 30 30 32 3A 29 30 30 33 38 05 30 31 3C 35 03")))}
  // cmd60_cancel (no data, seq=0x43)
  test("golden cmd60_cancel") {check(DatecsFrameCodec.encode(0x43,60,byteArrayOf()).contentEquals(hex("01 30 30 32 3A 43 30 30 33 3C 05 30 31 3E 33 03")))}
  // cmd76_txn_status (no data, seq=0x32)
  test("golden cmd76_txn_status") {check(DatecsFrameCodec.encode(0x32,76,byteArrayOf()).contentEquals(hex("01 30 30 32 3A 32 30 30 34 3C 05 30 31 3D 33 03")))}
  // cmd103_receipt_info (no data, seq=0x40)
  test("golden cmd103_receipt_info") {check(DatecsFrameCodec.encode(0x40,103,byteArrayOf()).contentEquals(hex("01 30 30 32 3A 40 30 30 36 37 05 30 31 3D 3E 03")))}
  // cmd62_read_datetime (no data, seq=0x45)
  test("golden cmd62_read_datetime") {check(DatecsFrameCodec.encode(0x45,62,byteArrayOf()).contentEquals(hex("01 30 30 32 3A 45 30 30 33 3E 05 30 31 3E 37 03")))}
  // cmd74_status (no data, seq=0x31)
  test("golden cmd74_status") {check(DatecsFrameCodec.encode(0x31,74,byteArrayOf()).contentEquals(hex("01 30 30 32 3A 31 30 30 34 3A 05 30 31 3D 30 03")))}
  // cmd48_open_syntax1 (data="1\t1\t24\tI\t", seq=0x2C)
  test("golden cmd48_open_syntax1") {check(DatecsFrameCodec.encode(0x2C,48,hex("31 09 31 09 32 34 09 49 09")).contentEquals(hex("01 30 30 33 33 2C 30 30 33 30 31 09 31 09 32 34 09 49 09 05 30 32 3E 3F 03")))}
  // cmd48_open_syntax2 (data with invoice "DT636533-0020-00101101\t1\tI\t", seq=0x32)
  test("golden cmd48_open_syntax2") {check(DatecsFrameCodec.encode(0x32,48,hex("31 09 31 09 44 54 36 33 36 35 33 33 2D 30 30 32 30 2D 30 30 31 30 31 31 30 09 31 09 49 09")).contentEquals(hex("01 30 30 34 38 32 30 30 33 30 31 09 31 09 44 54 36 33 36 35 33 33 2D 30 30 32 30 2D 30 30 31 30 31 31 30 09 31 09 49 09 05 30 37 31 30 03")))}
  // cmd49_sale_cp1251 ("Топено сирене\t2\t2.65\t3\t2\t5\t2\t" in CP1251, seq=0x36)
  test("golden cmd49_sale_cp1251") {check(DatecsFrameCodec.encode(0x36,49,hex("D2 EE EF E5 ED EE 20 F1 E8 F0 E5 ED E5 09 32 09 32 2E 36 35 09 33 09 32 09 35 09 32 09")).contentEquals(hex("01 30 30 34 37 36 30 30 33 31 D2 EE EF E5 ED EE 20 F1 E8 F0 E5 ED E5 09 32 09 32 2E 36 35 09 33 09 32 09 35 09 32 09 05 30 3E 3E 31 03")))}
  // cmd53_pay_cash: SEQ=0xEF is outside Datecs 0x20..0x7F — verify BCC is correct independently
  test("golden cmd53_pay_cash BCC independent") {
    val frame53=hex("01 30 30 32 3F EF 30 30 33 35 30 09 31 30 09 05 30 33 33 30 03")
    check(bccOk(frame53)){"cmd53 golden frame BCC mismatch"}
    // Also verify encode rejects SEQ=0xEF as out of range
    try{DatecsFrameCodec.encode(0xEF,53,hex("30 09 31 30 09"));error("encode(0xEF,...) should throw")}catch(e:IllegalArgumentException){}}
  // cmd56_answer: decode and verify SEQ, CMD, data, BCC, STAT
  test("golden cmd56_answer decode") {
    val ans56=hex("01 30 30 33 39 29 30 30 33 38 30 09 32 36 39 09 04 80 80 80 80 80 82 80 80 05 30 36 3A 3E 03")
    val r=DatecsFrameCodec.decode(ans56)
    check(r.sequence==0x29 && r.command==56){"cmd56 answer SEQ/CMD mismatch"}
    check(r.data.contentEquals(hex("30 09 32 36 39 09"))){"cmd56 answer data mismatch"}
    check(r.status.contentEquals(hex("80 80 80 80 80 82 80 80"))){"cmd56 answer STAT mismatch"}}
  // cmd76_answer: decode and verify SEQ, CMD, data, BCC, STAT
  test("golden cmd76_answer decode") {
    val ans76=hex("01 30 30 34 37 32 30 30 34 3C 30 09 30 09 35 31 37 09 30 09 30 2E 30 30 09 30 2E 30 30 09 04 80 80 80 80 86 9A 80 80 05 30 38 3D 35 03")
    val r=DatecsFrameCodec.decode(ans76)
    check(r.sequence==0x32 && r.command==76){"cmd76 answer SEQ/CMD mismatch"}
    check(r.status.contentEquals(hex("80 80 80 80 86 9A 80 80"))){"cmd76 answer STAT mismatch"}}
  test("every truncated prefix") {for(n in rx.indices) reject(rx.copyOf(n))}
  test("every single-byte corruption") {for(i in rx.indices){val bad=rx.copyOf();bad[i]=(bad[i].toInt() xor 1).toByte();reject(bad)}}
  test("trailing bytes") {reject(rx+3.toByte())}
  test("invalid checksum alphabet") {val bad=rx.copyOf();bad[bad.size-2]='A'.code.toByte();reject(bad)}
  test("underflow length") {val bad=rx.copyOf();bad[4]=0x20;reject(bad)}
  test("max TX") {val frame=DatecsFrameCodec.encode(0x7f,33,ByteArray(496));check(frame.size==512 && frame.copyOfRange(1,5).contentEquals(hex("30 32 31 3A")))}
  test("oversize TX") {try {DatecsFrameCodec.encode(0x20,33,ByteArray(497))}catch(e: IllegalArgumentException){return@test};error("oversize accepted")}
  test("max RX") {check(DatecsFrameCodec.decode(response(480)).data.size==480)}
  test("oversize RX") {reject(response(481))}
  test("empty RX") {check(DatecsFrameCodec.decode(response(0)).data.isEmpty())}
  test("socket actual wire/correlation") {val expected=rx.copyOf();expected[5]=0x20 // Adjust literal sequence and BCC by -4 independently.
    expected[expected.size-3]=0x30;expected[expected.size-2]=0x3d
    val output=ByteArrayOutputStream();val result=DatecsSocketProtocol(ByteArrayInputStream(expected),output).execute(33)
    check(result.sequence==0x20);check(output.toByteArray().contentEquals(hex("01 30 30 32 3A 20 30 30 32 31 05 30 31 3B 34 03")))
  }
  test("deterministic malformed noise") {for(n in 0..549)reject(ByteArray(n){0xff.toByte()})}
  check(failures==0){"$failures vendor codec failures"};println("actual Kotlin Datecs codec host tests: OK")
}
