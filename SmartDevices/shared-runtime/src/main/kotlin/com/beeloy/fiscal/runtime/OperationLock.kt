package com.beeloy.fiscal.runtime

import java.util.concurrent.locks.ReentrantLock
import kotlin.concurrent.withLock

/**
 * Single-operation executor lock — enforces "one FU, one executor" rule.
 *
 * Vendor-neutral wrapper around a ReentrantLock so the locking strategy can be
 * shared between bluecash-app and daisy-smart-app without duplicating the pattern.
 *
 * Usage:
 *   private val lock = OperationLock()
 *   fun execute(command: C): R = lock.withOperation { /* fiscal I/O */ }
 */
class OperationLock {
    private val lock = ReentrantLock()

    /** Executes [block] under the exclusive operation lock. */
    fun <T> withOperation(block: () -> T): T = lock.withLock(block)
}
