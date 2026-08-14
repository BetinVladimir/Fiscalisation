package com.beeloy.fiscal.bluecash

import android.content.Context
import com.android.fiscal.FiscalManager
import com.android.pinpad.PinpadManager

class DatecsAndroidFiscalPort(context: Context) : DatecsFiscalPort {
  private val manager = FiscalManager.getInstance(context.applicationContext)
  private var socket: com.android.fiscal.FiscalSocket? = null
  private var protocol: DatecsSocketProtocol? = null
  @Synchronized
  private fun connected(): DatecsSocketProtocol {
    if (manager.status != FiscalManager.STATUS_ON) manager.turnOn()
    val existing = protocol
    if (existing != null) return existing
    val next = manager.openDevice() ?: error("DATECS_FISCAL_OPEN_FAILED")
    socket = next
    return DatecsSocketProtocol(next.inputStream, next.outputStream).also { protocol = it }
  }
  override fun reachable() =
    runCatching {
        manager.status == FiscalManager.STATUS_ON ||
          run {
            manager.turnOn()
            manager.status == FiscalManager.STATUS_ON
          }
      }
      .getOrDefault(false)
  override fun execute(command: Int, payload: ByteArray) =
    try {
      connected().execute(command, payload)
    } catch (e: Throwable) {
      close()
      throw e
    }
  @Synchronized
  fun close() {
    runCatching { socket?.close() }
    socket = null
    protocol = null
  }
}

/**
 * Opens the real BlueCash pinpad IPC socket. Transaction TLV is delegated to a supplied audited
 * codec.
 */
interface DatecsPinpadCodec {
  fun purchase(
    amountMinor: Long,
    operationId: String,
    input: java.io.InputStream,
    output: java.io.OutputStream
  ): Map<String, String>
  fun reverse(
    operationId: String,
    input: java.io.InputStream,
    output: java.io.OutputStream
  ): Map<String, String>
}

class DatecsAndroidPaymentPort(context: Context, private val codec: DatecsPinpadCodec) :
  DatecsPaymentPort {
  private val manager = PinpadManager.getInstance(context.applicationContext)
  override fun reachable() =
    runCatching {
        manager.status == PinpadManager.STATUS_ON ||
          run {
            manager.turnOn()
            manager.status == PinpadManager.STATUS_ON
          }
      }
      .getOrDefault(false)
  private fun <T> withSocket(block: (java.io.InputStream, java.io.OutputStream) -> T): T {
    require(reachable()) { "PAYMENT_TERMINAL_UNREACHABLE" }
    val socket = manager.openPinpad() ?: error("DATECS_PINPAD_OPEN_FAILED")
    return try {
      block(socket.inputStream, socket.outputStream)
    } finally {
      socket.close()
    }
  }
  override fun purchaseEur(amountMinor: Long, operationId: String) = withSocket { i, o ->
    codec.purchase(amountMinor, operationId, i, o)
  }
  override fun reverse(operationId: String) = withSocket { i, o ->
    codec.reverse(operationId, i, o)
  }
}

class MissingDatecsPinpadCodec : DatecsPinpadCodec {
  override fun purchase(
    amountMinor: Long,
    operationId: String,
    input: java.io.InputStream,
    output: java.io.OutputStream
  ): Map<String, String> = error("DATECS_PINPAD_TLV_CODEC_NOT_INSTALLED")
  override fun reverse(
    operationId: String,
    input: java.io.InputStream,
    output: java.io.OutputStream
  ): Map<String, String> = error("DATECS_PINPAD_TLV_CODEC_NOT_INSTALLED")
}
