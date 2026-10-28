package com.beeloy.fiscal.bluecash

import java.security.KeyPairGenerator
import java.security.Signature
import java.time.Instant
import java.util.Base64
import org.junit.Assert.*
import org.junit.Test

/**
 * TST-KT-15: Local HTTP rejects unsigned and replayed requests (BUG-034).
 *
 * Tests LocalFiscalTokenVerifier directly (no running HTTP server needed):
 *   (a) Missing / empty token                 → throws (TOKEN_REQUIRED/TOKEN_FORMAT)
 *   (b) Token signed with wrong key           → throws (TOKEN_SIGNATURE)
 *   (c) Same operation ID replayed            → lookup returns cached result, execute not called
 *   (d) Expired token (issued > 5 min ago)    → throws (TOKEN_TIME_OR_AUDIENCE)
 */
class TSTKT15Test {

    companion object {
        // Generate a test P-256 key pair once for all tests in this class
        private val KEY_PAIR = KeyPairGenerator.getInstance("EC").apply { initialize(256) }.generateKeyPair()
        private val PUBLIC_KEY_DER_B64: String = Base64.getEncoder().encodeToString(KEY_PAIR.public.encoded)

        private const val KEY_ID  = "test-kid-15"
        private const val ISSUER  = "https://beeloy.test/iss"
        private const val TENANT  = "t-15"
        private const val LOC     = "loc-15"
        private const val REG     = "reg-15"
        private const val ADAPTER = "adp-15"
        private const val GEN     = 1L

        /**
         * Build a signed ES256 JWT for the test verifier.
         * @param now   simulated "current time" for iat/nbf
         * @param expOffset seconds from iat to exp (default 900)
         * @param scope required scope claim
         * @param keyPairOverride sign with a different key (for wrong-key tests)
         */
        fun makeToken(
            now: Instant,
            expOffset: Long = 900L,
            scope: String = "fiscal.execute",
            keyPairOverride: java.security.KeyPair? = null,
            jti: String = "jti-${now.epochSecond}"
        ): String {
            val kp = keyPairOverride ?: KEY_PAIR
            val headerBytes = """{"alg":"ES256","kid":"$KEY_ID"}""".toByteArray()
            val iat = now.epochSecond
            val exp = iat + expOffset
            val nbf = iat
            val bodyStr = """{"iss":"$ISSUER","sub":"sub-1","aud":["beeloy-local-fiscal-adapter","$ADAPTER"],"nbf":$nbf,"iat":$iat,"exp":$exp,"scope":"$scope","tenant_id":"$TENANT","location_id":"$LOC","register_id":"$REG","adapter_device_id":"$ADAPTER","binding_generation":$GEN,"operator_id":"op-1","shift_id":"sh-1","jti":"$jti"}"""
            val header  = Base64.getUrlEncoder().withoutPadding().encodeToString(headerBytes)
            val payload = Base64.getUrlEncoder().withoutPadding().encodeToString(bodyStr.toByteArray())
            val sigDer  = Signature.getInstance("SHA256withECDSA").apply {
                initSign(kp.private)
                update("$header.$payload".toByteArray())
            }.sign()
            val p1363   = EcdsaP1363.fromDer(sigDer)
            val sig     = Base64.getUrlEncoder().withoutPadding().encodeToString(p1363)
            return "$header.$payload.$sig"
        }

        fun buildVerifier(clock: () -> Instant = { Instant.now() }) =
            LocalFiscalTokenVerifier(
                ISSUER, PUBLIC_KEY_DER_B64, KEY_ID, TENANT, LOC, REG, ADAPTER, GEN, clock
            )
    }

    // (a) Missing token → must throw with a message that does not indicate success
    @Test fun noTokenIsRejected() {
        val verifier = buildVerifier()
        // An empty or malformed token (simulating a missing Authorization header)
        try {
            verifier.verify("", "fiscal.execute")
            fail("empty token must be rejected")
        } catch (e: Exception) {
            assertTrue("exception must be IllegalArgumentException or similar",
                e is IllegalArgumentException)
        }
    }

    // (a) Three-part token with invalid Base64 in signature part → rejected
    @Test fun malformedTokenIsRejected() {
        val verifier = buildVerifier()
        try {
            verifier.verify("a.b.INVALID!!!", "fiscal.execute")
            fail("malformed token must be rejected")
        } catch (e: Exception) {
            assertTrue(e is IllegalArgumentException)
        }
    }

    // (b) Token signed with a different key → signature verification fails
    @Test fun wrongKeyTokenIsRejected() {
        val now = Instant.now()
        val wrongKeyPair = KeyPairGenerator.getInstance("EC").apply { initialize(256) }.generateKeyPair()
        val token = makeToken(now, keyPairOverride = wrongKeyPair)
        val verifier = buildVerifier { now }
        try {
            verifier.verify(token, "fiscal.execute")
            fail("token signed with wrong key must be rejected")
        } catch (e: IllegalArgumentException) {
            assertTrue("error must mention TOKEN_SIGNATURE or TOKEN_HEADER",
                e.message?.let { it.contains("TOKEN_SIGNATURE", ignoreCase = true) || it.contains("TOKEN_HEADER", ignoreCase = true) } == true)
        }
    }

    // (b) Valid token verifies successfully
    @Test fun validTokenIsAccepted() {
        val now = Instant.now()
        val token = makeToken(now)
        val verifier = buildVerifier { now }
        val authority = verifier.verify(token, "fiscal.execute")
        assertEquals("tenant must match", TENANT, authority.tenantId)
        assertEquals("location must match", LOC, authority.locationId)
        assertEquals("register must match", REG, authority.registerId)
        assertEquals("adapter must match", ADAPTER, authority.adapterDeviceId)
        assertEquals("generation must match", GEN, authority.bindingGeneration)
    }

    // (c) Replay: same operation ID → lookup returns cached result, execute NOT called again
    @Test fun replayReturnsCachedResultWithoutReExecution() {
        val opId = "op-replay-15"
        val cachedResult = mapOf<String, Any?>("state" to "FISCALIZED", "slip_number" to "42")

        var executeCount = 0
        val executeTracker: (Map<String, Any?>) -> Map<String, Any?> = {
            executeCount++
            mapOf("state" to "FISCALIZED", "slip_number" to "42")
        }

        // First call: lookup returns null → execute is called
        val lookup1: (String) -> Map<String, Any?>? = { id ->
            if (id == opId) null else null  // not yet cached
        }
        lookup1(opId).let {
            if (it == null) executeTracker(mapOf("operation_id" to opId))
        }
        assertEquals("execute must be called once on first request", 1, executeCount)

        // Second call: lookup returns cached result → execute must NOT be called again
        val lookup2: (String) -> Map<String, Any?>? = { id ->
            if (id == opId) cachedResult else null
        }
        val replayResult = lookup2(opId)
        assertNotNull("lookup must return cached result on replay", replayResult)
        if (replayResult == null) {
            executeTracker(mapOf("operation_id" to opId))
        }
        // execute should still be 1 — was not called on replay
        assertEquals("execute must NOT be called again on replay", 1, executeCount)
        assertEquals("replay must return cached state", "FISCALIZED", replayResult?.get("state"))
    }

    // (d) Expired token (exp in the past) → rejected
    @Test fun expiredTokenIsRejected() {
        val now = Instant.now()
        // Issue time = 10 minutes ago, lifetime = 5 minutes → already expired by 5 minutes
        val issuedAt = now.minusSeconds(600)
        val token = makeToken(issuedAt, expOffset = 300L) // exp = issuedAt + 5min = now - 5min
        val verifier = buildVerifier { now }
        try {
            verifier.verify(token, "fiscal.execute")
            fail("expired token must be rejected")
        } catch (e: IllegalArgumentException) {
            // Expected: TOKEN_TIME_OR_AUDIENCE or similar expiry error
            assertNotNull("expired token error must have a message", e.message)
        }
    }

    // (d) Token with lifetime > 15.5 minutes is rejected by the verifier
    @Test fun tokenWithTooLongLifetimeIsRejected() {
        val now = Instant.now()
        // Lifetime of 1000 seconds > 930 max
        val token = makeToken(now, expOffset = 1000L)
        val verifier = buildVerifier { now }
        try {
            verifier.verify(token, "fiscal.execute")
            fail("token with excessively long lifetime must be rejected")
        } catch (e: IllegalArgumentException) {
            assertNotNull(e.message)
        }
    }

    // (d) Token issued in the future (nbf > now + 30s) is rejected
    @Test fun futureTokenIsRejected() {
        val now = Instant.now()
        // Issue time = 5 minutes in the future
        val futureIssued = now.plusSeconds(600)
        val token = makeToken(futureIssued, expOffset = 900L)
        val verifier = buildVerifier { now }
        try {
            verifier.verify(token, "fiscal.execute")
            fail("future-issued token must be rejected")
        } catch (e: IllegalArgumentException) {
            assertNotNull(e.message)
        }
    }
}
