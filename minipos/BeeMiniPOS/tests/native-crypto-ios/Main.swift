import UIKit
import Security
import CryptoKit
@main final class NativeCryptoTestApp: UIResponder, UIApplicationDelegate {
  var window: UIWindow?
  func application(_ application: UIApplication, didFinishLaunchingWithOptions options: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
    window = UIWindow(frame: UIScreen.main.bounds); window?.rootViewController = UIViewController(); window?.makeKeyAndVisible()
    DispatchQueue.global().async {
      let output = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].appendingPathComponent("native-key-evidence.json")
      do {
        let arguments=ProcessInfo.processInfo.arguments
        if let at=arguments.firstIndex(of:"--authority-scope"),at+1<arguments.count {
          let scope=arguments[at+1],keys=ExecutionKeyStore(),publicKey=try keys.prepare(scope)
          let messageAt=arguments.firstIndex(of:"--authority-message")
          let message=messageAt.flatMap{$0+1<arguments.count ? arguments[$0+1] : nil} ?? ""
          let signature=message.isEmpty ? "" : try keys.sign(scope,expectedPublicKey:publicKey,messageBase64:message)
          let evidence=["public_key":publicKey,"message":message,"signature":signature]
          let path=output.deletingLastPathComponent().appendingPathComponent("native-authority-evidence.json")
          try JSONSerialization.data(withJSONObject:evidence,options:[.sortedKeys]).write(to:path,options:.atomic)
          return
        }
        let keys = ExecutionKeyStore(),scope = "https://api.example|tenant|actor|app|" + UUID().uuidString
        let publicKey = try keys.prepare(scope)
        guard publicKey == (try ExecutionKeyStore().prepare(scope)) else { throw NSError(domain:"REOPEN_KEY_MISMATCH",code:1) }
        let other = try keys.prepare(scope + ":other")
        guard other != publicKey else { throw NSError(domain:"SCOPE_COLLISION",code:1) }
        let message = Data("BEELOY-EXECUTION-INTENT-PROOF-V1\n{\"operation_id\":\"actual-ios-seckey\"}".utf8)
        func url(_ d: Data) -> String { d.base64EncodedString().replacingOccurrences(of:"+",with:"-").replacingOccurrences(of:"/",with:"_").replacingOccurrences(of:"=",with:"") }
        let signature = try keys.sign(scope, expectedPublicKey: publicKey, messageBase64: url(message))
        func decode(_ s: String) -> Data { let b=s.replacingOccurrences(of:"-",with:"+").replacingOccurrences(of:"_",with:"/");return Data(base64Encoded:b+String(repeating:"=",count:(4-b.count%4)%4))! }
        let spki=decode(publicKey),raw=spki.suffix(65)
        let verify=try P256.Signing.PublicKey(x963Representation:raw)
        guard verify.isValidSignature(try P256.Signing.ECDSASignature(derRepresentation:decode(signature)),for:message) else { throw NSError(domain:"INVALID_DER_SIGNATURE",code:1) }
        do { _=try keys.sign(scope,expectedPublicKey:other,messageBase64:url(message));throw NSError(domain:"CROSS_SCOPE_SIGNED",code:1) } catch { if (error as NSError).domain == "CROSS_SCOPE_SIGNED" { throw error } }
        try keys.revoke(scope)
        do { _=try ExecutionKeyStore().prepare(scope);throw NSError(domain:"REVOKED_REGENERATED",code:1) } catch { if (error as NSError).domain == "REVOKED_REGENERATED" { throw error } }
        do { _=try keys.sign(scope,expectedPublicKey:publicKey,messageBase64:url(message));throw NSError(domain:"REVOKED_SIGNED",code:1) } catch { if (error as NSError).domain == "REVOKED_SIGNED" { throw error } }
        guard other == (try keys.prepare(scope+":other")) else { throw NSError(domain:"FOREIGN_SCOPE_REVOKED",code:1) }
        try keys.revoke(scope+":other")
        let evidence:[String:Any]=["platform":"iOS Security.framework","public_key":publicKey,"message":url(message),"signature":signature,"status":"PASS"]
        try JSONSerialization.data(withJSONObject:evidence,options:[.sortedKeys]).write(to:output,options:.atomic)
      } catch { try? Data("{\"status\":\"FAIL\",\"error\":\"\(error)\"}".utf8).write(to:output,options:.atomic) }
    }
    return true
  }
}
