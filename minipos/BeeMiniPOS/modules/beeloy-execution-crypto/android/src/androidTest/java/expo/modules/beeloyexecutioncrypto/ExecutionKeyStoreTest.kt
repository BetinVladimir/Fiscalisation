package expo.modules.beeloyexecutioncrypto

import android.util.Base64
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import java.security.KeyFactory
import java.security.KeyStore
import java.security.Signature
import java.security.spec.X509EncodedKeySpec
import java.util.UUID

@RunWith(AndroidJUnit4::class)
class ExecutionKeyStoreTest {
  @Test fun authorityInterop() {
    val args=InstrumentationRegistry.getArguments()
    val scope=args.getString("authority_scope") ?: return
    val context=InstrumentationRegistry.getInstrumentation().targetContext
    val keys=ExecutionKeyStore(context)
    val publicKey=keys.prepare(scope)
    val message=args.getString("authority_message")
    val signature=if(message!=null)keys.sign(scope,publicKey,message) else ""
    context.openFileOutput("native-authority-evidence.json",0).use { it.write(("{\"public_key\":\""+publicKey+"\",\"message\":\""+(message?:"")+"\",\"signature\":\""+signature+"\"}").toByteArray()) }
  }
  @Test fun persistentNonExportableKeySignScopeAndRevocation() {
    val context = InstrumentationRegistry.getInstrumentation().targetContext
    val scope = "https://api.example|tenant|actor|app|" + UUID.randomUUID().toString()
    val first = ExecutionKeyStore(context)
    val publicKey = first.prepare(scope)
    assertEquals(publicKey, ExecutionKeyStore(context).prepare(scope))
    val otherScope = scope + ":other"
    val otherPublicKey = first.prepare(otherScope)
    assertNotEquals(publicKey, otherPublicKey)
    val transcript = "BEELOY-EXECUTION-INTENT-PROOF-V1\n{\"operation_id\":\"actual-android-keystore\"}".toByteArray()
    val flags = Base64.URL_SAFE or Base64.NO_WRAP or Base64.NO_PADDING
    val signature = Base64.decode(first.sign(scope, publicKey, Base64.encodeToString(transcript, flags)), flags)
    val key = KeyFactory.getInstance("EC").generatePublic(X509EncodedKeySpec(Base64.decode(publicKey, flags)))
    assertTrue(Signature.getInstance("SHA256withECDSA").run { initVerify(key); update(transcript); verify(signature) })
    assertFalse(Signature.getInstance("SHA256withECDSA").run { initVerify(key); update(transcript + 1); verify(signature) })
    try { first.sign(scope, otherPublicKey, Base64.encodeToString(transcript, flags)); fail("cross-scope key accepted") } catch (_: IllegalStateException) {}
    val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
    val alias = keyStore.aliases().toList().single { name ->
      name.startsWith("beeloy.execution.v1.") && Base64.encodeToString(keyStore.getCertificate(name).publicKey.encoded, flags) == publicKey
    }
    assertNull("private key must not be exportable", keyStore.getKey(alias, null).encoded)
    context.openFileOutput("native-key-evidence.json", 0).use { it.write(("{\"platform\":\"AndroidKeyStore\",\"public_key\":\"" + publicKey + "\",\"message\":\"" + Base64.encodeToString(transcript, flags) + "\",\"signature\":\"" + Base64.encodeToString(signature, flags) + "\"}").toByteArray()) }
    first.revoke(scope)
    assertFalse(keyStore.containsAlias(alias))
    try { ExecutionKeyStore(context).prepare(scope); fail("revoked scope regenerated") } catch (_: IllegalStateException) {}
    try { first.sign(scope, publicKey, Base64.encodeToString(transcript, flags)); fail("revoked scope signed") } catch (_: IllegalStateException) {}
    assertEquals(otherPublicKey, first.prepare(otherScope))
    first.revoke(otherScope)
  }
}
