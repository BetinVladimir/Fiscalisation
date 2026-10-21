import ExpoModulesCore
public class BeeloyExecutionCryptoModule: Module {
  public func definition() -> ModuleDefinition {
    Name("BeeloyExecutionCrypto")
    AsyncFunction("prepare") { (scope: String) -> String in try ExecutionKeyStore().prepare(scope) }
    AsyncFunction("sign") { (scope: String, publicKey: String, message: String) -> String in
      try ExecutionKeyStore().sign(scope, expectedPublicKey: publicKey, messageBase64: message)
    }
    AsyncFunction("revoke") { (scope: String) in try ExecutionKeyStore().revoke(scope) }
  }
}
