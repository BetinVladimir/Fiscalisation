# Native execution authority

Native monetary commands use the same root-issued execution authorization and domain-separated client proof as the web fiscal client. Android uses AndroidKeyStore P-256/SHA256withECDSA. iOS uses permanent Security.framework SecKey P-256 with WhenUnlockedThisDeviceOnly access. JavaScript receives public SPKI and DER signatures only; the module has no private-key export or software fallback. The Android instrumentation test additionally proves `PrivateKey.encoded == null`; simulator tests do not establish hardware-backed custody or Secure Enclave use.

A key scope includes source backend, trusted root tenant/actor, app instance and source JWT token hash. Native stores serialize key creation/signing/revocation; a persisted native tombstone precedes deletion. JS also persists a scope tombstone to fence in-flight issuance. A new authenticated session requires a new scope. Changing actor, app instance or source session cannot restore an old authorization. Root/S3 cryptographic verification remains authoritative; client inspection validates canonical bytes, hash, scope, key and validity before signing.

Online checkout prepares a persisted MiniPOS source order through `/execution-authorizations`, then uses its exact root canonical payload, server sale, operation and receipt IDs. The source order ID is never a root sale ID. The issuer's SERVER_ASSIGNED UNP is retained. Offline authorization has a bounded leased range, null server sale and a client surrogate; only an unexpired cached lease may authorize it. An online grant does not replace the offline lease cache.

The checkout journal persists AUTHORIZING and a fixed issuance request key before requesting online preparation. A lost response cannot fall back to a different leased operation. The one transition to SIGNED freezes the exact signed intent and returned operation/receipt IDs before transport. Pending sent operations are read/reconciled, never rebuilt. Existing root-only carts/journals retain their original IDs and require explicit recovery; they are not cloned into source orders.

The current S3 protocol is canonical CBOR inside BFF1, protected by root authorization and per-intent proof. The unused legacy X25519 HELLO/CHALLENGE/AUTH_PROOF client is not a supported S3 monetary handshake. Encrypted BLE transport remains a separate provisioning/HIL acceptance item; this change does not label OPEN_MVP transport as encrypted. Native transport checks negotiated ATT MTU (61..517), bounds every fragment by MTU-3, serializes one monetary command, fences connection callbacks by generation, and treats write failure after an attempt as uncertain. Correlated operation IDs and physical fiscal proof are required for success.

## Automated evidence

- `npm test` exercises Go-compatible canonicalization, exact prepared payload/IDs, null offline IDs, proof DER, scope/expiry/revocation, immutable journal and actual production BLE transport with fake adapter (MTU61/185/247/517, wrong/stale callbacks, partial writes and concurrency).
- `android/gradlew :beeloy-execution-crypto:connectedDebugAndroidTest` uses Android KeyStore on an emulator/device; it tests persistent key reopening, public-key isolation, DER verification, non-exportability and durable revocation. Test-only `authorityInterop` accepts a message to prove actual issuer/native-signature/S3 interop without a production debug endpoint.
- `tests/native-crypto-ios/Main.swift` is a separate simulator test application using the production SecKey store; it checks reopening, DER verification, scope and durable revocation. It is not compiled into the production app.
- Full Android/iOS binary builds and OS provider tests do not replace real Android/iOS BLE HIL, keychain locked-state/reinstallation policy, radio disconnect tests, fiscal device tests or acquirer certification.

Official API references: [Android KeyGenParameterSpec](https://developer.android.com/reference/android/security/keystore/KeyGenParameterSpec), [Apple WhenUnlockedThisDeviceOnly](https://developer.apple.com/documentation/security/ksecattraccessiblewhenunlockedthisdeviceonly), [Expo local module autolinking](https://docs.expo.dev/modules/autolinking/).

## Reproduce actual OS provider interoperability

The separate OS test application uses the production `ExecutionKeyStore` source. Its prepare/sign argument interface is compiled only into test executables, never the production Expo module. `tests/native-authority-interop/run.py` obtains a real Go-issued authorization for the OS public key, uses production `NativeExecutionAuthority.signIntent` to construct the PoP transcript, terminates/restarts the iOS test application before signing, and submits its actual DER signature to the production S3 `IntentProcessor` + mbedTLS + SQLite host executable. The S3 suite must accept the valid command and reject missing proof, altered signature/binding/payload and expired authorization.

For iOS, generate a standard Xcode test application (requires the `xcodeproj` Ruby gem installed with CocoaPods), build and install it into a booted simulator:

```sh
ruby tests/native-crypto-ios/project.rb "$PWD" /tmp/native-ios-project
xcodebuild -project /tmp/native-ios-project/NativeCrypto.xcodeproj -scheme NativeCrypto -configuration Debug -sdk iphonesimulator -destination 'generic/platform=iOS Simulator' -derivedDataPath /tmp/native-ios-build build
xcrun simctl install "$IOS_SIMULATOR_ID" /tmp/native-ios-build/Build/Products/Debug-iphonesimulator/NativeCrypto.app
python3 tests/native-authority-interop/run.py ios --device "$IOS_SIMULATOR_ID" --output /tmp/native-ios-evidence
```

For Android, build the local module instrumentation APK using `:beeloy-execution-crypto:assembleDebugAndroidTest`, install it with `adb install -r -t`, and run:

```sh
python3 tests/native-authority-interop/run.py android --device "$ANDROID_DEVICE_ID" --output /tmp/native-android-evidence
```

The default repository root is discovered from the app location; `--repo` selects another complete repository checkout. Add `--online` for the actual server-assigned authorization fixture. Both variants require the Go toolchain, Node with TypeScript stripping, C/C++ host compiler and the same crypto dependencies as `authority_host/run.py`. Outputs contain public keys, signatures and test grants, never private keys.

Verified 2026-10-19: actual iOS 26.5 simulator SecKey → actual Go issuer → production JS PoP → actual S3 verifier/SQLite, valid plus five negative cases PASS. Android AndroidKeyStore instrumentation and complete Android application build PASS; full Android issuer vertical is tracked separately until its runner completes. These are OS simulator checks, not claims of secure hardware, radio HIL, acquirer approval or physical fiscal device acceptance. Encrypted BLE transport remains a separate GAP005 requirement; mandatory per-command execution authorization is retained on the BFF1 path.
