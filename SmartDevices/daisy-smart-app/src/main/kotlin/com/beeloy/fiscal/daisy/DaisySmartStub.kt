package com.beeloy.fiscal.daisy

import com.beeloy.fiscal.Command
import com.beeloy.fiscal.FiscalDriver
import com.beeloy.fiscal.Result

/** Vendor documentation is pending; this adapter is intentionally debug-only. */
class DaisySmartStub(
    environment: String,
    private val cardTerminalAvailable: Boolean = true,
    private val fiscalDeviceAvailable: Boolean = true,
) : FiscalDriver {
    init {
        require(environment.lowercase() != "prod") { "Daisy SMART S STUB is forbidden in PROD" }
        require(BuildConfig.STUB_ADAPTER && BuildConfig.DEBUG) { "Daisy SMART S STUB is debug-only" }
    }
    override fun probe() = mapOf("edge" to "READY", "driver" to "READY", "fiscal_device" to if (fiscalDeviceAvailable) "SIMULATED" else "UNREACHABLE", "payment_terminal" to if (cardTerminalAvailable) "SIMULATED" else "UNAVAILABLE")
    override fun capabilities() = listOf("FISCAL_SALE", "FISCAL_REVERSAL", "CASH_IN", "CASH_OUT", "X_REPORT", "Z_REPORT", "KLEN_REPORT", "FISCAL_MEMORY_REPORT", "CARD_PAYMENT", "BARCODE_SCAN", "DEVICE_PROBE").associateWith { "STUB_ONLY" }
    override fun execute(command: Command): Result {
        if (command.operationId.isBlank()) return failed(command, "INVALID_OPERATION_ID")
        if (!capabilities().containsKey(command.type)) return failed(command, "UNSUPPORTED_CAPABILITY")
        if (!fiscalDeviceAvailable && command.type != "BARCODE_SCAN") return Result(command.operationId, "BLOCKED", errorCode = "FISCAL_DEVICE_UNREACHABLE", simulated = true)
        return when (command.payload["scenario"] as? String ?: "success") {
            "success" -> success(command)
            "card_decline" -> failed(command, "CARD_DECLINED")
            "card_timeout", "timeout_after_execution", "disconnect_after_execution" -> Result(command.operationId, "FISCAL_RESULT_UNKNOWN", errorCode = "DEVICE_OUTCOME_UNKNOWN", simulated = true)
            "disconnect_before_execution" -> Result(command.operationId, "BLOCKED", errorCode = "FISCAL_DEVICE_UNREACHABLE", simulated = true)
            "paper_out" -> failed(command, "PAPER_OUT")
            "fiscal_error" -> failed(command, "FISCAL_DEVICE_ERROR")
            else -> failed(command, "INVALID_STUB_SCENARIO")
        }
    }
    private fun success(command: Command): Result {
        if (command.type == "FISCAL_SALE" && hasCardPayment(command) && !cardTerminalAvailable) return failed(command, "PAYMENT_TERMINAL_UNAVAILABLE")
        if (command.type == "BARCODE_SCAN") return Result(command.operationId, "BARCODE_READ", simulated = true, data = mapOf("barcode" to "3800000000017"))
        if (command.type == "DEVICE_PROBE") return Result(command.operationId, "READY", simulated = true)
        return Result(command.operationId, "FISCALIZED", "DAISY-STUB-${command.operationId}", simulated = true)
    }
    private fun hasCardPayment(command: Command) = (command.payload["payments"] as? List<*>)?.any { (it as? Map<*, *>)?.get("type") == "CARD" } == true
    private fun failed(command: Command, code: String) = Result(command.operationId, "FAILED", errorCode = code, simulated = true)
}
