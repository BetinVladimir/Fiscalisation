package com.beeloy.fiscal.runtime

/**
 * Vendor-neutral command processor strategy.
 *
 * Invariant 4: duplicate operationId → return existing result without re-executing.
 * Invariant 5: storage errors are propagated upward; the monetary side effect must
 * not proceed if the journal cannot be written.
 */
interface CommandProcessor<C, R> {
    /**
     * Execute a command.
     * Implementations must check the journal for an existing entry before issuing any
     * monetary I/O, and record the outcome durably afterward.
     */
    fun execute(command: C): R
}
