package expo.modules.beeloyexecutioncrypto
import expo.modules.kotlin.modules.Module
import expo.modules.kotlin.modules.ModuleDefinition
class BeeloyExecutionCryptoModule : Module() {
  private fun keys() = ExecutionKeyStore(requireNotNull(appContext.reactContext))
  override fun definition() = ModuleDefinition {
    Name("BeeloyExecutionCrypto")
    AsyncFunction("prepare") { scope: String -> keys().prepare(scope) }
    AsyncFunction("sign") { scope: String, publicKey: String, message: String -> keys().sign(scope, publicKey, message) }
    AsyncFunction("revoke") { scope: String -> keys().revoke(scope) }
  }
}
