package com.beeloy.fiscal.daisy

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.net.ServerSocket
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit

/** SMS-08.1: SmartFiscalHttpClient — CONFIG_MISSING, ok, Rejected(1008), NotSent, Unknown. */
class SmartFiscalHttpClientTest {

    private fun withHttpServer(
        responseBody: String?,
        statusCode: Int = 200,
        closeBeforeResponse: Boolean = false,
        block: (port: Int) -> SmartReply,
    ): SmartReply {
        val ss = ServerSocket(0)
        val port = ss.localPort
        val ready = CountDownLatch(1)
        val pool = Executors.newSingleThreadExecutor()
        pool.submit {
            ready.countDown()
            try {
                val conn = ss.accept()
                val input = conn.getInputStream()
                val out = conn.getOutputStream()
                // Drain request.
                val buf = ByteArray(4096)
                var total = 0
                val deadline = System.currentTimeMillis() + 2000
                while (System.currentTimeMillis() < deadline) {
                    if (input.available() > 0) {
                        val n = input.read(buf, total, buf.size - total)
                        if (n > 0) total += n
                        // Look for double CRLF and Content-Length to know body end.
                        val req = String(buf, 0, total)
                        val bodyStart = req.indexOf("\r\n\r\n")
                        if (bodyStart >= 0) {
                            val clMatch = Regex("Content-Length: (\\d+)").find(req)
                            val cl = clMatch?.groupValues?.get(1)?.toIntOrNull() ?: 0
                            if (total >= bodyStart + 4 + cl) break
                        }
                    } else {
                        Thread.sleep(5)
                    }
                }
                if (closeBeforeResponse) {
                    conn.close()
                    return@submit
                }
                if (responseBody != null) {
                    val bodyBytes = responseBody.toByteArray(Charsets.UTF_8)
                    val response = "HTTP/1.1 $statusCode OK\r\nContent-Type: application/json\r\nContent-Length: ${bodyBytes.size}\r\nConnection: close\r\n\r\n"
                    out.write(response.toByteArray())
                    out.write(bodyBytes)
                    out.flush()
                }
                conn.close()
            } catch (_: Exception) {}
        }
        ready.await(5, TimeUnit.SECONDS)
        try {
            return block(port)
        } finally {
            ss.close()
            pool.shutdown()
        }
    }

    @Test fun `baseUrl missing returns CONFIG_MISSING`() {
        val client = SmartFiscalHttpClient(null)
        val reply = client.post("{}")
        assertTrue("Expected ConfigMissing, got $reply", reply is SmartReply.ConfigMissing)
        assertTrue((reply as SmartReply.ConfigMissing).reason.contains("CONFIG_MISSING"))
    }

    @Test fun `ok true response returns Ok`() {
        val reply = withHttpServer("""{"ok":true,"data":"FD-001","messages":[],"seq":1}""") { port ->
            SmartFiscalHttpClient("http://localhost:$port").post("""{"type":"SALE"}""")
        }
        assertTrue("Expected Ok, got $reply", reply is SmartReply.Ok)
        assertEquals("FD-001", (reply as SmartReply.Ok).data)
        assertEquals(1, reply.seq)
    }

    @Test fun `ok false code 1008 returns Rejected 1008`() {
        val reply = withHttpServer("""{"ok":false,"code":1008,"messages":["DUPLICATE"]}""") { port ->
            SmartFiscalHttpClient("http://localhost:$port").post("""{"type":"SALE"}""")
        }
        assertTrue("Expected Rejected, got $reply", reply is SmartReply.Rejected)
        assertEquals(1008, (reply as SmartReply.Rejected).code)
    }

    @Test fun `connection refused before write returns NotSent`() {
        // No server on a closed port → connection refused before body is sent.
        val closedPort = ServerSocket(0).use { it.localPort }
        val reply = SmartFiscalHttpClient("http://localhost:$closedPort").post("{}")
        assertTrue("Expected NotSent, got $reply", reply is SmartReply.NotSent)
    }

    @Test fun `disconnect after body write returns Unknown`() {
        val reply = withHttpServer(null, closeBeforeResponse = true) { port ->
            SmartFiscalHttpClient("http://localhost:$port").post("""{"type":"SALE"}""")
        }
        // After body is written, disconnect → Unknown (invariant 2: no retry).
        assertTrue("Expected Unknown, got $reply", reply is SmartReply.Unknown)
    }
}
