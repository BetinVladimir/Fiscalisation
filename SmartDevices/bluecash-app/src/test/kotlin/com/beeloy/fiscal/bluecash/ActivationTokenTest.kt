package com.beeloy.fiscal.bluecash

import java.time.Instant
import java.util.Base64
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.fail
import org.junit.Test

class ActivationTokenTest {
  private fun token(payload: String) =
    "e30.${Base64.getUrlEncoder().withoutPadding().encodeToString(payload.toByteArray())}.signature"

  @Test
  fun bindsOrganizationLocationAndDevice() {
    val jwt =
      token(
        """{"aud":"beefiscal-bluecash-activation","organization_id":"org-1","location_id":"loc-1","device_id":"dev-1","app_instance_id":"app-1","exp":2000000000}"""
      )
    val claims =
      ActivationToken.parseAndBind(
        jwt,
        "org-1",
        "loc-1",
        "dev-1",
        Instant.ofEpochSecond(1900000000)
      )
    assertEquals("org-1", claims.organizationId)
    assertEquals("loc-1", claims.locationId)
  }

  @Test(expected = IllegalArgumentException::class)
  fun rejectsForeignLocation() {
    val jwt =
      token(
        """{"aud":"beefiscal-bluecash-activation","organization_id":"org-1","location_id":"loc-2","device_id":"dev-1","app_instance_id":"app-1","exp":2000000000}"""
      )
    ActivationToken.parseAndBind(jwt, "org-1", "loc-1", "dev-1", Instant.ofEpochSecond(1900000000))
  }

  @Test
  fun loginIsRequiredBeforeToken() {
    val controller = BlueCashActivationController {}
    val result = runCatching { controller.acceptToken("x.y.z") }
    assertEquals("LOGIN_REQUIRED", result.exceptionOrNull()?.message)
  }
}

class TokenChunkAssemblerTest {
  @Test
  fun reassemblesOrderedFramesAndRejectsConflicts() {
    val assembler = TokenChunkAssembler()
    assertNull(assembler.accept("peer", "BFA1|transfer-1|1|2|abc"))
    assertEquals("abcdef", assembler.accept("peer", "BFA1|transfer-1|2|2|def"))
    assertNull(assembler.accept("peer", "BFA1|transfer-2|1|2|abc"))
    try {
      assembler.accept("peer", "BFA1|transfer-2|1|2|xyz")
      fail("conflicting duplicate chunk accepted")
    } catch (_: IllegalArgumentException) {}
  }
}
