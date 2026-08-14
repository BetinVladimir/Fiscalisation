package com.beeloy.fiscal.bluecash

/**
 * Boundary for Datecs FiscalDeviceSDK_MultiPlatform 1.0.1 / Com.Android.Fiscal. The supplied
 * package is a .NET MAUI binding, not an AAR, so production must provide a separately built bridge
 * implementing this contract.
 */
interface BlueCashFiscalAdapter {
  fun connect(): Map<String, String>
  fun executeCanonicalFiscalCommand(command: ByteArray): ByteArray
  fun disconnect()
}

/** Boundary for DatecsPaySDK_MultiPlatform_Net8 1.0.4 / Com.Android.Pinpad. */
interface BlueCashCardAdapter {
  fun connect(): Map<String, String>
  fun authorizeEur(amountMinor: Long, operationId: String): Map<String, String>
  fun reverse(operationId: String): Map<String, String>
  fun disconnect()
}

class MissingVendorFiscalAdapter : BlueCashFiscalAdapter {
  override fun connect(): Map<String, String> = error("DATECS_FISCAL_VENDOR_BRIDGE_NOT_INSTALLED")
  override fun executeCanonicalFiscalCommand(command: ByteArray): ByteArray =
    error("DATECS_FISCAL_VENDOR_BRIDGE_NOT_INSTALLED")
  override fun disconnect() = Unit
}

class MissingVendorCardAdapter : BlueCashCardAdapter {
  override fun connect(): Map<String, String> = error("DATECS_PINPAD_VENDOR_BRIDGE_NOT_INSTALLED")
  override fun authorizeEur(amountMinor: Long, operationId: String): Map<String, String> =
    error("DATECS_PINPAD_VENDOR_BRIDGE_NOT_INSTALLED")
  override fun reverse(operationId: String): Map<String, String> =
    error("DATECS_PINPAD_VENDOR_BRIDGE_NOT_INSTALLED")
  override fun disconnect() = Unit
}
