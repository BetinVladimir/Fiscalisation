package com.beeloy.fiscal.bluecash

/**
 * ARCH-007: Pure Kotlin runtime coordinator — no android.* imports.
 * Platform-specific Android wiring (Context, Service lifecycle) lives in
 * RuntimeHost / BlueCashRuntimeService. This class owns the testable,
 * platform-independent execution state.
 */
class BlueCashRuntime(
  val processor: BlueCashCommandProcessor,
  val store: BleLocalSaleStore,
  val unp: BleUnpAllocator,
  val journal: TransactionJournal
) {
  fun executorFor(operator: Int, password: String, till: Int): BlueCashComplianceIntentExecutor =
    BlueCashComplianceIntentExecutor(store, unp, processor, operator, password, till)

  /** ARCH-008: rebuild this runtime's aggregate from a durable backing store. */
  fun rebuildFrom(durable: BleLocalSaleStore) = rebuildAggregateFromJournal(durable, store)
}
