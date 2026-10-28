package com.beeloy.fiscal.daisy

/** Plan item for a Smart S fiscal sale line. */
data class SmartSaleItem(
    val name: String,
    val priceCents: Long,
    val qtyMilli: Long,
    val vatGroup: String,
    val discountCents: Long = 0L,
    val discountType: String = "amount",
)

/** A payment slot entry from GetPayments. */
data class SmartPaymentSlot(val id: Int, val name: String, val currencyRate: Int, val payTypeNap: Int)

/** Sale payment instruction. */
data class SmartSalePayment(val payTypeCode: Int, val amountCents: Long)

/** Plan for a sale or reversal (storno). */
data class SmartReceiptPlan(
    val unp: String,
    val operationId: String,
    val opCode: Int,
    val opPassword: String,
    val items: List<SmartSaleItem>,
    val payments: List<SmartSalePayment>,
)

/** Parameters for a fiscal reversal. */
data class SmartStornoPlan(
    val unp: String,
    val operationId: String,
    val opPassword: String,
    val reason: Int,
    val docNumLink: String,
    val fmNumLink: String,
    val dateTimeLink: String,
    val items: List<SmartSaleItem>,
    val payments: List<SmartSalePayment>,
)

sealed class DeviceResult {
    data class Committed(val fiscalReference: String?, val noDocNumber: Boolean = false) : DeviceResult()
    data class Rejected(val code: Int, val messages: List<String>) : DeviceResult()
    object Unknown : DeviceResult()
    object NotSent : DeviceResult()
    data class Error(val code: String) : DeviceResult()
}

/**
 * Smart S fiscal port: builds vendor JSON and delegates to SmartFiscalHttpClient.
 * Invariant 1: integer cents → JSON decimal via SmartMoney.
 * Invariant 6: fiscal reference only from FU response; synthetic numbers forbidden.
 * Invariant 7: vatGroup must be Cyrillic (CP1251); Latin tax group rejected before I/O.
 *
 * @param send injectable transport; pass `client::post` for production.
 */
class SmartFiscalPort(
    private val send: (String) -> SmartReply,
    val isLab: Boolean = false,
) {
    constructor(client: SmartFiscalHttpClient, isLab: Boolean = false) : this(client::post, isLab)
    private var paymentSlots: List<SmartPaymentSlot>? = null
    private var seq = 1

    fun sale(plan: SmartReceiptPlan): DeviceResult {
        for (item in plan.items) {
            if (isLatinTaxGroup(item.vatGroup)) return DeviceResult.Error("TAX_GROUP_LATIN")
        }
        val json = buildSaleJson(plan, seq++)
        return when (val reply = send(json)) {
            is SmartReply.Ok -> parseCommitted(reply.data)
            is SmartReply.Rejected -> DeviceResult.Rejected(reply.code, reply.messages)
            is SmartReply.Unknown -> DeviceResult.Unknown
            is SmartReply.NotSent -> DeviceResult.NotSent
            is SmartReply.ConfigMissing -> DeviceResult.Error(reply.reason)
        }
    }

    fun storno(plan: SmartStornoPlan): DeviceResult {
        for (item in plan.items) {
            if (isLatinTaxGroup(item.vatGroup)) return DeviceResult.Error("TAX_GROUP_LATIN")
        }
        val json = buildStornoJson(plan, seq++)
        return when (val reply = send(json)) {
            is SmartReply.Ok -> parseCommitted(reply.data)
            is SmartReply.Rejected -> DeviceResult.Rejected(reply.code, reply.messages)
            is SmartReply.Unknown -> DeviceResult.Unknown
            is SmartReply.NotSent -> DeviceResult.NotSent
            is SmartReply.ConfigMissing -> DeviceResult.Error(reply.reason)
        }
    }

    fun zReport(opPassword: String): DeviceResult {
        val json = """{"command":"PrintDailyReport","data":{"reset":true,"opPassword":"$opPassword"},"seq":${seq++},"errorsLocale":"bg"}"""
        return when (val reply = send(json)) {
            is SmartReply.Ok -> DeviceResult.Committed(null)
            is SmartReply.Rejected -> DeviceResult.Rejected(reply.code, reply.messages)
            is SmartReply.Unknown -> DeviceResult.Unknown
            is SmartReply.NotSent -> DeviceResult.NotSent
            is SmartReply.ConfigMissing -> DeviceResult.Error(reply.reason)
        }
    }

    fun status(): DeviceResult {
        val json = """{"command":"GetStatus","seq":${seq++},"errorsLocale":"bg"}"""
        return when (val reply = send(json)) {
            is SmartReply.Ok -> DeviceResult.Committed(null)
            is SmartReply.Rejected -> DeviceResult.Rejected(reply.code, reply.messages)
            is SmartReply.Unknown -> DeviceResult.Unknown
            is SmartReply.NotSent -> DeviceResult.NotSent
            is SmartReply.ConfigMissing -> DeviceResult.Error(reply.reason)
        }
    }

    fun payments(): List<SmartPaymentSlot> {
        paymentSlots?.let { return it }
        val json = """{"command":"GetPayments","seq":${seq++},"errorsLocale":"bg"}"""
        val reply = send(json)
        if (reply is SmartReply.Ok) {
            val slots = parsePaymentSlots(reply.data)
            paymentSlots = slots
            return slots
        }
        return emptyList()
    }

    fun cashPayTypeCode(): Int {
        val slots = payments()
        return slots.firstOrNull { it.payTypeNap == 0 && it.currencyRate == 1 }?.id
            ?: throw IllegalStateException("PAYMENT_TYPE_UNSUPPORTED")
    }

    private fun parseCommitted(data: String): DeviceResult {
        // Invariant 6: fiscal reference only from FU response.
        val docNum = Regex(""""docNum"\s*:\s*"([^"]*)"""").find(data)?.groupValues?.get(1)
            ?: Regex(""""docNum"\s*:\s*(\d+)""").find(data)?.groupValues?.get(1)
        return if (docNum != null && docNum.isNotEmpty()) {
            DeviceResult.Committed(docNum)
        } else {
            if (isLab) DeviceResult.Committed(null, noDocNumber = true)
            else DeviceResult.Unknown
        }
    }

    private fun parsePaymentSlots(data: String): List<SmartPaymentSlot> {
        val result = mutableListOf<SmartPaymentSlot>()
        val objectPattern = Regex("""\{[^{}]+\}""")
        for (match in objectPattern.findAll(data)) {
            val obj = match.value
            fun field(key: String) = Regex(""""$key"\s*:\s*(-?\d+|"[^"]*")""").find(obj)?.groupValues?.get(1)?.removeSurrounding("\"")
            val id = field("id")?.toIntOrNull() ?: continue
            val name = field("name") ?: continue
            val currencyRate = field("currencyRate")?.toIntOrNull() ?: continue
            val payTypeNap = field("payTypeNap")?.toIntOrNull() ?: continue
            result += SmartPaymentSlot(id, name, currencyRate, payTypeNap)
        }
        return result
    }

    private fun isLatinTaxGroup(vg: String): Boolean =
        vg.length == 1 && vg[0] in 'A'..'Z'

    private fun buildSaleJson(plan: SmartReceiptPlan, seq: Int): String {
        val itemsJson = plan.items.joinToString(",") { item ->
            val base = buildString {
                append("""{"type":"item","item":${jsonString(item.name)},"price":${SmartMoney.centsToJson(item.priceCents)},"qty":${SmartMoney.qtyMilliToJson(item.qtyMilli)},"vatGroup":${jsonString(item.vatGroup)}""")
                if (item.discountCents != 0L) {
                    append(""","discountType":"${item.discountType}","discountValue":${SmartMoney.centsToJson(item.discountCents)}""")
                }
                append("}")
            }
            base
        }
        val paymentsJson = plan.payments.joinToString(",") { p ->
            """{"type":"payment","payTypeCode":${p.payTypeCode},"amount":${SmartMoney.centsToJson(p.amountCents)}}"""
        }
        return """{"command":"PrintFiscalReceipt","data":{"UNP":${jsonString(plan.unp)},"refNum":${jsonString(plan.operationId)},"opCode":${plan.opCode},"opPassword":${jsonString(plan.opPassword)},"items":[$itemsJson],"payments":[$paymentsJson]},"seq":$seq,"errorsLocale":"bg"}"""
    }

    private fun buildStornoJson(plan: SmartStornoPlan, seq: Int): String {
        val itemsJson = plan.items.joinToString(",") { item ->
            val base = buildString {
                append("""{"type":"item","item":${jsonString(item.name)},"price":${SmartMoney.centsToJson(item.priceCents)},"qty":${SmartMoney.qtyMilliToJson(item.qtyMilli)},"vatGroup":${jsonString(item.vatGroup)}""")
                if (item.discountCents != 0L) {
                    append(""","discountType":"${item.discountType}","discountValue":${SmartMoney.centsToJson(item.discountCents)}""")
                }
                append("}")
            }
            base
        }
        val paymentsJson = plan.payments.joinToString(",") { p ->
            """{"type":"payment","payTypeCode":${p.payTypeCode},"amount":${SmartMoney.centsToJson(p.amountCents)}}"""
        }
        return """{"command":"PrintFiscalReceipt","data":{"UNP":${jsonString(plan.unp)},"refNum":${jsonString(plan.operationId)},"opCode":20,"opPassword":${jsonString(plan.opPassword)},"isStorno":true,"reason":${plan.reason},"docNumLink":${jsonString(plan.docNumLink)},"fmNumLink":${jsonString(plan.fmNumLink)},"dateTimeLink":${jsonString(plan.dateTimeLink)},"invoiceLink":"","isInvoice":false,"items":[$itemsJson],"payments":[$paymentsJson]},"seq":$seq,"errorsLocale":"bg"}"""
    }

    private fun jsonString(s: String): String =
        "\"${s.replace("\\", "\\\\").replace("\"", "\\\"")}\""
}
