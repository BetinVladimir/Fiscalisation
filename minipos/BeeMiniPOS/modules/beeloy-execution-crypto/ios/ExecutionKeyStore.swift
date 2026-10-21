import Foundation
import Security
import CryptoKit

public final class ExecutionKeyStore {
  private static let lock = NSLock()
  private let service = "beeloy.execution.keys.v1"
  public init() {}
  private func tag(_ scope: String) throws -> Data {
    guard !scope.isEmpty, scope.utf8.count <= 4096 else { throw failure("EXECUTION_SCOPE_INVALID") }
    return Data((service + ".").utf8) + Data(SHA256.hash(data: Data(scope.utf8)))
  }
  private func failure(_ code: String) -> NSError { NSError(domain: code, code: 1) }
  private func tombstone(_ tag: Data) throws -> Bool {
    let query: [String: Any] = [kSecClass as String: kSecClassGenericPassword,
      kSecAttrService as String: service, kSecAttrAccount as String: tag.base64EncodedString(),
      kSecReturnData as String: true]
    let status = SecItemCopyMatching(query as CFDictionary, nil)
    if status == errSecItemNotFound { return false }
    guard status == errSecSuccess else { throw failure("EXECUTION_REVOCATION_READ_FAILED_\(status)") }
    return true
  }
  private func key(_ tag: Data) throws -> SecKey? {
    let query: [String: Any] = [kSecClass as String: kSecClassKey,
      kSecAttrApplicationTag as String: tag, kSecAttrKeyType as String: kSecAttrKeyTypeECSECPrimeRandom,
      kSecAttrKeyClass as String: kSecAttrKeyClassPrivate, kSecReturnRef as String: true]
    var result: CFTypeRef?
    let status = SecItemCopyMatching(query as CFDictionary, &result)
    if status == errSecItemNotFound { return nil }
    guard status == errSecSuccess, let result else { throw failure("EXECUTION_KEY_READ_FAILED_\(status)") }
    return (result as! SecKey)
  }
  private func publicSPKI(_ key: SecKey) throws -> String {
    guard let pub = SecKeyCopyPublicKey(key) else { throw failure("EXECUTION_PUBLIC_KEY_FAILED") }
    var error: Unmanaged<CFError>?
    guard let raw = SecKeyCopyExternalRepresentation(pub, &error) as Data?, raw.count == 65, raw[0] == 4 else {
      throw failure("EXECUTION_PUBLIC_KEY_FAILED")
    }
    // RFC5480 id-ecPublicKey + prime256v1; only public X9.63 bytes leave Security.framework.
    let prefix: [UInt8] = [0x30,0x59,0x30,0x13,0x06,0x07,0x2a,0x86,0x48,0xce,0x3d,0x02,0x01,0x06,0x08,0x2a,0x86,0x48,0xce,0x3d,0x03,0x01,0x07,0x03,0x42,0x00]
    return encode(Data(prefix) + raw)
  }
  private func encode(_ data: Data) -> String { data.base64EncodedString().replacingOccurrences(of: "+", with: "-").replacingOccurrences(of: "/", with: "_").replacingOccurrences(of: "=", with: "") }
  public func prepare(_ scope: String) throws -> String {
    Self.lock.lock(); defer { Self.lock.unlock() }
    let id = try tag(scope)
    guard try !tombstone(id) else { throw failure("AUTHENTICATED_SESSION_REVOKED") }
    if let existing = try key(id) { return try publicSPKI(existing) }
    let attributes: [String: Any] = [kSecAttrKeyType as String: kSecAttrKeyTypeECSECPrimeRandom,
      kSecAttrKeySizeInBits as String: 256,
      kSecPrivateKeyAttrs as String: [kSecAttrIsPermanent as String: true,
        kSecAttrApplicationTag as String: id, kSecAttrIsExtractable as String: false,
        kSecAttrAccessible as String: kSecAttrAccessibleWhenUnlockedThisDeviceOnly]]
    var error: Unmanaged<CFError>?
    guard let created = SecKeyCreateRandomKey(attributes as CFDictionary, &error) else { throw failure("EXECUTION_KEY_GENERATION_FAILED") }
    return try publicSPKI(created)
  }
  public func sign(_ scope: String, expectedPublicKey: String, messageBase64: String) throws -> String {
    Self.lock.lock(); defer { Self.lock.unlock() }
    let id = try tag(scope)
    guard try !tombstone(id) else { throw failure("AUTHENTICATED_SESSION_REVOKED") }
    guard let privateKey = try key(id), try publicSPKI(privateKey) == expectedPublicKey else { throw failure("EXECUTION_KEY_CHANGED") }
    guard messageBase64.count <= 131072 else { throw failure("EXECUTION_MESSAGE_TOO_LARGE") }
    var base64 = messageBase64.replacingOccurrences(of: "-", with: "+").replacingOccurrences(of: "_", with: "/")
    base64 += String(repeating: "=", count: (4 - base64.count % 4) % 4)
    guard let message = Data(base64Encoded: base64), !message.isEmpty, message.count <= 65536 else { throw failure("EXECUTION_MESSAGE_INVALID") }
    var error: Unmanaged<CFError>?
    guard let signature = SecKeyCreateSignature(privateKey, .ecdsaSignatureMessageX962SHA256, message as CFData, &error) as Data? else { throw failure("EXECUTION_SIGN_FAILED") }
    return encode(signature)
  }
  public func revoke(_ scope: String) throws {
    Self.lock.lock(); defer { Self.lock.unlock() }
    let id = try tag(scope)
    let query: [String: Any] = [kSecClass as String: kSecClassGenericPassword,
      kSecAttrService as String: service, kSecAttrAccount as String: id.base64EncodedString(),
      kSecAttrAccessible as String: kSecAttrAccessibleWhenUnlockedThisDeviceOnly,
      kSecValueData as String: Data([1])]
    let status = SecItemAdd(query as CFDictionary, nil)
    guard status == errSecSuccess || status == errSecDuplicateItem else { throw failure("EXECUTION_REVOCATION_PERSIST_FAILED") }
    let deletion: [String: Any] = [kSecClass as String: kSecClassKey, kSecAttrApplicationTag as String: id,
      kSecAttrKeyType as String: kSecAttrKeyTypeECSECPrimeRandom, kSecAttrKeyClass as String: kSecAttrKeyClassPrivate]
    let deleted = SecItemDelete(deletion as CFDictionary)
    guard deleted == errSecSuccess || deleted == errSecItemNotFound else { throw failure("EXECUTION_KEY_DELETE_FAILED") }
  }
}
