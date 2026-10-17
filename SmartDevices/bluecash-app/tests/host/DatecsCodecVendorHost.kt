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
fun main() {
  // Literal PM v2.11.4 section4.1 command33 request/answer; independent oracle.
  val tx=hex("01 30 30 32 3A 24 30 30 32 31 05 30 31 3B 38 03")
  val rx=hex("01 30 30 33 35 24 30 30 32 31 30 09 04 80 80 80 80 86 9A 80 80 05 30 36 31 31 03")
  test("vendor TX") {check(DatecsFrameCodec.encode(0x24,33,byteArrayOf()).contentEquals(tx))}
  test("vendor RX") {val result=DatecsFrameCodec.decode(rx);check(result.sequence==0x24 && result.command==33);check(result.data.contentEquals(byteArrayOf(0x30,9)));check(result.status.contentEquals(hex("80 80 80 80 86 9A 80 80")))}
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
