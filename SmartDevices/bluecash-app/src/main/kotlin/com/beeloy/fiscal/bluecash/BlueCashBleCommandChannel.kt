package com.beeloy.fiscal.bluecash

import java.util.UUID

/** Transport-independent transaction channel used by the Android GATT callback. */
class BlueCashBleCommandChannel(private val handshake:BlueCashBleServerHandshake,private val execute:(Map<String,Any?>)->Map<String,Any?>,private val attMtu:Int=185){
 private val reassembler=BlueCashBleReassembler()
 fun control(raw:ByteArray):ByteArray=when(handshake.state){BlueCashBleServerHandshake.State.NEW->handshake.hello(raw);BlueCashBleServerHandshake.State.CHALLENGED->handshake.authenticate(raw);else->error("BLE_HANDSHAKE_STATE")}
 fun command(rawFrame:ByteArray):List<ByteArray>{require(handshake.state==BlueCashBleServerHandshake.State.READY){"BLE_NOT_READY"};val session=handshake.frames?:error("BLE_NOT_READY");val frame=session.open(rawFrame);val complete=reassembler.accept(frame)?:return emptyList();val intent=BlueCashCanonicalCbor.decodeMap(complete);val result=execute(intent);val encoded=BlueCashCanonicalCbor.encode(result);val chunks=BlueCashBleFrameSession.chunks(encoded,attMtu);return chunks.mapIndexed{index,bytes->session.seal(frame.messageId,index,chunks.size,1,bytes)}}
 fun close(){reassembler.clear();handshake.close()}
}
