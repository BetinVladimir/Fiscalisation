package com.beeloy.fiscal.bluecash

import org.junit.Assert.*
import org.junit.Test

class DatecsProtocolTest {
 @Test fun `command 43 storno payload follows vendor field order`(){val d=DatecsStornoDocument(1,428,"24-04-19 08:36:27","02636571","DT636497-0021-0010001");assertEquals("21\t9876\t24\t1\t428\t24-04-19 08:36:27\t02636571\t\t\t\tDT636497-0021-0010001\t",String(DatecsPayloads.stornoOpen(21,"9876",24,d)))}
 @Test fun `codec matches locked C++ vector`() { val frame=DatecsFrameCodec.encode(0x20,48,"1\t1".toByteArray()); assertEquals(1,frame.first().toInt()); assertEquals(3,frame.last().toInt()); assertEquals(19,frame.size) }
 @Test fun `payloads match Datecs documentation vectors`() {
  assertEquals("1\t1\tDY000600-OP01-0000001\t24\t\t",String(DatecsPayloads.open(1,"1","DY000600-OP01-0000001",24)))
  assertEquals("Coffee\t2\t2.65\t3.000\t2\t5.00\t2\tpcs\t",String(DatecsPayloads.line(FiscalLine("Coffee",'B',"2.65","3.000",2,"5.00",2,"pcs"))))
  assertEquals("0\t10.00\t",String(DatecsPayloads.payment(0,"10.00")))
  assertEquals("Z\t",String(DatecsPayloads.report(true))); assertEquals("1\t50.00\t",String(DatecsPayloads.cash(true,"50.00")))
 }
 @Test fun `codec rejects tampering`() { val frame=DatecsFrameCodec.encode(0x20,48,byteArrayOf()); frame[frame.lastIndex-2]=(frame[frame.lastIndex-2].toInt() xor 1).toByte(); assertThrows(IllegalArgumentException::class.java){DatecsFrameCodec.decode(frame)} }
}
