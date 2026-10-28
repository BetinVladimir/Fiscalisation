package com.beeloy.fiscal.daisy

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.net.ServerSocket
import java.util.concurrent.LinkedBlockingQueue
import java.util.concurrent.TimeUnit

/** SMS-09.1/9.2: PaymentBridgeClient — SUCCESS/REJECTED/NOT_FOUND/FAILED scenarios, refund. */
class PaymentBridgeClientTest {

    /** Queue-based mini HTTP server: pre-load responses (empty string = close without reply). */
    class QueueServer(responses: List<String?>) : AutoCloseable {
        companion object { const val CLOSE_EMPTY = "\u0000" }
        private val queue = LinkedBlockingQueue<String>()
        private val ss = ServerSocket(0)
        val port = ss.localPort

        init {
            responses.forEach { queue.offer(it ?: CLOSE_EMPTY) }
            val t = Thread {
                while (!ss.isClosed) {
                    try {
                        val conn = ss.accept()
                        Thread {
                            try {
                                val buf = ByteArray(4096)
                                var total = 0
                                val dl = System.currentTimeMillis() + 3000
                                while (System.currentTimeMillis() < dl) {
                                    val n = conn.getInputStream().read(buf, total, buf.size - total)
                                    if (n < 0) break
                                    if (n > 0) total += n
                                    val req = String(buf, 0, total)
                                    val bs = req.indexOf("\r\n\r\n")
                                    if (bs >= 0) {
                                        val cl = Regex("Content-Length: (\\d+)").find(req)?.groupValues?.get(1)?.toIntOrNull() ?: 0
                                        if (total >= bs + 4 + cl) break
                                    }
                                }
                                val resp = queue.poll(2, TimeUnit.SECONDS) ?: CLOSE_EMPTY
                                if (resp != CLOSE_EMPTY) {
                                    val bodyBytes = resp.toByteArray(Charsets.UTF_8)
                                    val header = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: ${bodyBytes.size}\r\nConnection: close\r\n\r\n"
                                    conn.getOutputStream().apply { write(header.toByteArray()); write(bodyBytes); flush() }
                                }
                                conn.close()
                            } catch (_: Exception) {}
                        }.also { it.isDaemon = true; it.start() }
                    } catch (_: Exception) { break }
                }
            }
            t.isDaemon = true
            t.start()
        }

        override fun close() = ss.close()
    }

    private fun client(port: Int) = PaymentBridgeClient("http://localhost:$port", 3_000, 5_000)

    @Test fun `SUCCESS response returns Committed with rrn`() {
        QueueServer(listOf("""{"status":"SUCCESS","rrn":"RRN-12345"}""")).use { srv ->
            val result = client(srv.port).pay("op-1", 1367, "dsk")
            assertTrue("Expected Committed, got $result", result is PaymentResult.Committed)
            assertEquals("RRN-12345", (result as PaymentResult.Committed).rrn)
        }
    }

    @Test fun `REJECTED response returns Rejected`() {
        QueueServer(listOf("""{"status":"REJECTED"}""")).use { srv ->
            val result = client(srv.port).pay("op-2", 500, "viva")
            assertTrue("Expected Rejected, got $result", result is PaymentResult.Rejected)
        }
    }

    @Test fun `NOT_FOUND status after pay timeout returns NotCommitted`() {
        // pay endpoint: no response (server closes) → client calls checkStatus
        // checkStatus endpoint: returns NOT_FOUND
        QueueServer(listOf(null, """{"status":"NOT_FOUND"}""")).use { srv ->
            val result = client(srv.port).pay("op-3", 867, "dsk")
            assertTrue("Expected NotCommitted, got $result", result is PaymentResult.NotCommitted)
        }
    }

    @Test fun `disconnect after pay calls check_status returning Committed`() {
        QueueServer(listOf(null, """{"status":"SUCCESS","rrn":"RRN-99"}""")).use { srv ->
            val result = client(srv.port).pay("op-4", 200, "dsk")
            assertTrue("Expected Committed from check_status, got $result", result is PaymentResult.Committed)
        }
    }

    @Test fun `refund returns REFUND_MANUAL_REQUIRED with no network calls`() {
        val closedPort = ServerSocket(0).use { it.localPort }
        val result = PaymentBridgeClient("http://localhost:$closedPort", 3_000, 5_000).refund("op-1-card")
        assertEquals(PaymentResult.RefundManualRequired, result)
    }
}
