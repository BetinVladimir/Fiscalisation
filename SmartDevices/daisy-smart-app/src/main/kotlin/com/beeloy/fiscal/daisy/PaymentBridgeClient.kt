package com.beeloy.fiscal.daisy

import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL

sealed class PaymentResult {
    data class Committed(val rrn: String) : PaymentResult()
    object Rejected : PaymentResult()
    object NotCommitted : PaymentResult()
    object Unknown : PaymentResult()
    /** Q-P4: card refund via bridge not yet implemented. Invariant 3: storno before refund. */
    object RefundManualRequired : PaymentResult()
}

/**
 * SMS-09: PaymentBridgeClient — DSK/Viva payment bridge.
 * Invariant 2: timeout/disconnect after send → Unknown + immediate check_status, no retry.
 * Invariant 3: refund is not automatic — RefundManualRequired until Q-P4 is resolved.
 */
class PaymentBridgeClient(
    private val baseUrl: String,
    private val connectTimeoutMs: Int = 10_000,
    private val readTimeoutMs: Int = 60_000,
) {

    fun pay(operationId: String, amountCents: Long, bank: String): PaymentResult {
        val referenceId = "$operationId-card"
        return try {
            val body = buildPayJson(referenceId, amountCents, bank)
            val response = httpPost("$baseUrl/payment_request", body)
            when {
                response == null -> checkStatus(referenceId)
                response.contains("\"status\":\"SUCCESS\"") -> {
                    val rrn = Regex(""""rrn"\s*:\s*"([^"]*)"""").find(response)?.groupValues?.get(1) ?: ""
                    PaymentResult.Committed(rrn)
                }
                response.contains("\"status\":\"REJECTED\"") -> PaymentResult.Rejected
                response.contains("\"status\":\"NOT_FOUND\"") -> PaymentResult.NotCommitted
                else -> checkStatus(referenceId)
            }
        } catch (e: IOException) {
            checkStatus(referenceId)
        }
    }

    fun refund(@Suppress("UNUSED_PARAMETER") referenceId: String): PaymentResult =
        PaymentResult.RefundManualRequired

    private fun checkStatus(referenceId: String): PaymentResult {
        return try {
            val response = httpPost("$baseUrl/check_status", """{"reference_id":"$referenceId"}""")
            when {
                response == null -> PaymentResult.Unknown
                response.contains("\"status\":\"SUCCESS\"") -> {
                    val rrn = Regex(""""rrn"\s*:\s*"([^"]*)"""").find(response)?.groupValues?.get(1) ?: ""
                    PaymentResult.Committed(rrn)
                }
                response.contains("\"status\":\"NOT_FOUND\"") -> PaymentResult.NotCommitted
                response.contains("\"status\":\"REJECTED\"") -> PaymentResult.Rejected
                else -> PaymentResult.Unknown
            }
        } catch (e: IOException) {
            PaymentResult.Unknown
        }
    }

    private fun buildPayJson(referenceId: String, amountCents: Long, bank: String): String {
        val amount = SmartMoney.centsToJson(amountCents)
        return """{"reference_id":"$referenceId","amount":"$amount","currency":"EUR","bank":"$bank"}"""
    }

    private fun httpPost(url: String, body: String): String? {
        val conn = URL(url).openConnection() as HttpURLConnection
        conn.requestMethod = "POST"
        conn.setRequestProperty("Content-Type", "application/json; charset=UTF-8")
        conn.connectTimeout = connectTimeoutMs
        conn.readTimeout = readTimeoutMs
        conn.doOutput = true
        conn.connect()
        conn.outputStream.use { it.write(body.toByteArray(Charsets.UTF_8)) }
        return try {
            conn.inputStream.use { it.readBytes() }.toString(Charsets.UTF_8)
        } catch (e: IOException) {
            null
        } finally {
            conn.disconnect()
        }
    }
}
