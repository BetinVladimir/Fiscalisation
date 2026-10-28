package com.beeloy.fiscal.daisy

import java.io.IOException
import java.net.HttpURLConnection
import java.net.SocketTimeoutException
import java.net.URL

sealed class SmartReply {
    data class Ok(val data: String, val messages: List<String>, val seq: Int) : SmartReply()
    data class Rejected(val code: Int, val messages: List<String>) : SmartReply()
    /** Sent to device but outcome unknown (disconnect after write). Invariant 2: no retry. */
    object Unknown : SmartReply()
    /** Connection failed before body was written. */
    object NotSent : SmartReply()
    data class ConfigMissing(val reason: String) : SmartReply()
}

class SmartFiscalHttpClient(private val baseUrl: String? = null) {

    private val connectTimeoutMs = 3_000
    private val readTimeoutMs = 15_000

    fun post(json: String): SmartReply {
        val url = baseUrl ?: return SmartReply.ConfigMissing("CONFIG_MISSING: baseUrl not configured")
        val conn: HttpURLConnection
        try {
            conn = URL("$url/fiscal").openConnection() as HttpURLConnection
            conn.requestMethod = "POST"
            conn.setRequestProperty("Content-Type", "application/json; charset=UTF-8")
            conn.connectTimeout = connectTimeoutMs
            conn.readTimeout = readTimeoutMs
            conn.doOutput = true
            conn.connect()
        } catch (e: IOException) {
            return SmartReply.NotSent
        }
        // After body write, any disconnect → Unknown (invariant 2).
        var bodySent = false
        try {
            conn.outputStream.use { it.write(json.toByteArray(Charsets.UTF_8)) }
            bodySent = true
            val body = conn.inputStream.use { it.readBytes() }.toString(Charsets.UTF_8)
            return parseReply(body)
        } catch (e: SocketTimeoutException) {
            return if (bodySent) SmartReply.Unknown else SmartReply.NotSent
        } catch (e: IOException) {
            return if (bodySent) SmartReply.Unknown else SmartReply.NotSent
        } finally {
            conn.disconnect()
        }
    }

    private fun parseReply(body: String): SmartReply {
        val ok = Regex(""""ok"\s*:\s*(true|false)""").find(body)?.groupValues?.get(1) == "true"
        val data = Regex(""""data"\s*:\s*"([^"]*)"""").find(body)?.groupValues?.get(1) ?: ""
        val seq = Regex(""""seq"\s*:\s*(\d+)""").find(body)?.groupValues?.get(1)?.toIntOrNull() ?: 0
        val code = Regex(""""code"\s*:\s*(\d+)""").find(body)?.groupValues?.get(1)?.toIntOrNull() ?: 0
        val messages = Regex(""""messages"\s*:\s*\[([^\]]*)]""").find(body)?.groupValues?.get(1)
            ?.split(",")
            ?.map { it.trim().removeSurrounding("\"") }
            ?.filter { it.isNotEmpty() }
            ?: emptyList()
        return if (ok) SmartReply.Ok(data, messages, seq)
        else SmartReply.Rejected(code, messages)
    }
}
