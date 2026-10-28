package com.beeloy.fiscal.daisy

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.net.ServerSocket
import java.net.Socket
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

/** Modes for FakeSmartFiscalServer responses. */
enum class FakeServerMode { OK, OK_WITH_DOC, ERROR, DROP_AFTER_BODY }

/**
 * In-process fake Smart Fiscal HTTP server for unit tests.
 * Supports modes OK/OK_WITH_DOC/ERROR/DROP_AFTER_BODY.
 * Validates incoming bodies: Cyrillic vatGroup; limits — item name ≤20, UNP ≤24, opPassword ≤14.
 */
class FakeSmartFiscalServer(val mode: FakeServerMode = FakeServerMode.OK) : AutoCloseable {

    private val ss = ServerSocket(0)
    val port: Int get() = ss.localPort
    val httpCalls = AtomicInteger(0)
    val lastBody = AtomicReference<String?>(null)
    val validationErrors = AtomicReference<List<String>>(emptyList())

    private val executor = Executors.newCachedThreadPool()

    init {
        executor.submit { acceptLoop() }
    }

    private fun acceptLoop() {
        while (!ss.isClosed) {
            try {
                val conn = ss.accept()
                executor.submit { handleConnection(conn) }
            } catch (e: Exception) {
                if (!ss.isClosed) throw e
            }
        }
    }

    private fun readBody(conn: Socket): String {
        val input = conn.getInputStream()
        val buf = ByteArray(8192)
        var total = 0
        val deadline = System.currentTimeMillis() + 3000L
        while (System.currentTimeMillis() < deadline) {
            if (input.available() > 0) {
                val n = input.read(buf, total, buf.size - total)
                if (n > 0) total += n
                val req = String(buf, 0, total, Charsets.ISO_8859_1)
                val sep = req.indexOf("\r\n\r\n")
                if (sep >= 0) {
                    val cl = Regex("Content-Length: (\\d+)").find(req)
                        ?.groupValues?.get(1)?.toIntOrNull() ?: 0
                    if (total >= sep + 4 + cl) {
                        return String(buf, sep + 4, cl, Charsets.UTF_8)
                    }
                }
            } else {
                Thread.sleep(5)
            }
        }
        return ""
    }

    private fun handleConnection(conn: Socket) {
        httpCalls.incrementAndGet()
        try {
            val body = readBody(conn)
            lastBody.set(body)
            validationErrors.set(validateBody(body))
            if (mode == FakeServerMode.DROP_AFTER_BODY) {
                // Send HTTP 400 so HttpURLConnection receives response headers before the
                // connection closes. This prevents the automatic retry that fires when EOF
                // arrives before any headers (non-streaming POST retry in the JDK). The
                // client catches FileNotFoundException (subclass of IOException) from
                // getInputStream() and returns Unknown because bodySent=true.
                val resp = "HTTP/1.1 400 Dropped\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
                runCatching { conn.getOutputStream().write(resp.toByteArray()) }
                return
            }
            val respBody = when (mode) {
                FakeServerMode.OK -> """{"ok":true,"data":"","messages":[],"seq":1}"""
                FakeServerMode.OK_WITH_DOC -> """{"ok":true,"data":"12345","messages":[],"seq":1}"""
                FakeServerMode.ERROR -> """{"ok":false,"code":1001,"data":"","messages":["DEVICE_ERR"],"seq":1}"""
                FakeServerMode.DROP_AFTER_BODY -> ""
            }
            val bytes = respBody.toByteArray(Charsets.UTF_8)
            val header = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n" +
                "Content-Length: ${bytes.size}\r\nConnection: close\r\n\r\n"
            val out = conn.getOutputStream()
            out.write(header.toByteArray())
            out.write(bytes)
            out.flush()
        } finally {
            runCatching { conn.close() }
        }
    }

    /**
     * Validate request body per Smart S invariants.
     * Limits: name ≤20, unp ≤24, opPassword ≤14. vatGroup must be Cyrillic.
     */
    fun validateBody(body: String): List<String> {
        val errs = mutableListOf<String>()
        Regex(""""vatGroup"\s*:\s*"([^"]+)"""").findAll(body).forEach { m ->
            val vg = m.groupValues[1]
            if (vg.isNotEmpty() && vg[0] in 'A'..'Z') errs += "LATIN_VAT_GROUP:$vg"
        }
        Regex(""""name"\s*:\s*"([^"]{21,})"""").find(body)?.let { errs += "NAME_TOO_LONG" }
        Regex(""""unp"\s*:\s*"([^"]{25,})"""").find(body)?.let { errs += "UNP_TOO_LONG" }
        Regex(""""opPassword"\s*:\s*"([^"]{15,})"""").find(body)?.let { errs += "PASSWORD_TOO_LONG" }
        if (!Regex(""""seq"\s*:\s*\d+""").containsMatchIn(body)) errs += "SEQ_MISSING"
        if (!Regex(""""items"\s*:\s*\[""").containsMatchIn(body)) errs += "ITEMS_MISSING"
        return errs
    }

    override fun close() {
        runCatching { ss.close() }
        executor.shutdownNow()
        executor.awaitTermination(2L, TimeUnit.SECONDS)
    }
}

/** EMU-03.1: FakeSmartFiscalServer mode and validation tests. */
class FakeSmartFiscalServerTest {

    private val saleJson = """{"command":"Sale","seq":1,"items":[{"name":"Coffee","vatGroup":"Б","priceCents":100,"qtyMilli":1000}],"payments":[]}"""

    @Test
    fun `DROP_AFTER_BODY returns Unknown and httpCalls equals 1`() {
        val server = FakeSmartFiscalServer(FakeServerMode.DROP_AFTER_BODY)
        val client = SmartFiscalHttpClient("http://localhost:${server.port}")
        val result = client.post(saleJson)
        server.close()
        assertTrue("Expected Unknown but got $result", result is SmartReply.Unknown)
        assertEquals("httpCalls should be 1", 1, server.httpCalls.get())
    }

    @Test
    fun `OK mode returns Ok`() {
        val server = FakeSmartFiscalServer(FakeServerMode.OK)
        val client = SmartFiscalHttpClient("http://localhost:${server.port}")
        val result = client.post(saleJson)
        server.close()
        assertTrue("Expected Ok but got $result", result is SmartReply.Ok)
    }

    @Test
    fun `OK_WITH_DOC mode returns Ok with data 12345`() {
        val server = FakeSmartFiscalServer(FakeServerMode.OK_WITH_DOC)
        val client = SmartFiscalHttpClient("http://localhost:${server.port}")
        val result = client.post(saleJson)
        server.close()
        assertTrue("Expected Ok but got $result", result is SmartReply.Ok)
        assertEquals("12345", (result as SmartReply.Ok).data)
    }

    @Test
    fun `ERROR mode returns Rejected with code 1001`() {
        val server = FakeSmartFiscalServer(FakeServerMode.ERROR)
        val client = SmartFiscalHttpClient("http://localhost:${server.port}")
        val result = client.post(saleJson)
        server.close()
        assertTrue("Expected Rejected but got $result", result is SmartReply.Rejected)
        assertEquals(1001, (result as SmartReply.Rejected).code)
    }

    @Test
    fun `validateBody rejects Latin vatGroup`() {
        val server = FakeSmartFiscalServer()
        val errs = server.validateBody("""{"vatGroup":"B","seq":1,"items":[]}""")
        server.close()
        assertTrue("Should reject Latin vatGroup B", errs.any { it.startsWith("LATIN_VAT_GROUP") })
    }

    @Test
    fun `validateBody accepts Cyrillic vatGroup`() {
        val server = FakeSmartFiscalServer()
        val errs = server.validateBody("""{"vatGroup":"Б","seq":1,"items":[{"name":"x"}]}""")
        server.close()
        assertTrue("Should accept Cyrillic vatGroup", errs.none { it.startsWith("LATIN_VAT_GROUP") })
    }
}
