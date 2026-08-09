# Pinned MVP toolchain

Validated baseline on 2026-08-07. Runtime images and JavaScript dependencies are pinned in their owning manifests; upgrades require `make regression`, Compose render and contract-lock verification.

| Layer | Pinned version |
|---|---|
| Go | 1.26.3 |
| Node.js | 22.14.0 |
| npm | 11.5.2 |
| Expo | 53.x (`~53.0.0`) |
| React / React Native | 19.0.0 / 0.79.0 |
| TypeScript | 5.8.x (`~5.8.3`) |
| Browser interaction tests | Playwright 1.62.1; Playwright Chromium when installed, otherwise explicit/system Chrome executable |
| OpenAPI TypeScript generation/client | openapi-typescript 7.13.0 / openapi-fetch 0.17.0 |
| Native BLE | react-native-ble-plx 3.5.1 |
| Portable BLE crypto | @noble/ciphers 2.3.0; @noble/curves 2.3.0; @noble/hashes 2.3.0; expo-crypto 14.1.5 |
| PostgreSQL image | 16.10 |
| Caddy image | 2.10.0 |
| E2E HTTP client image | curlimages/curl 8.12.1 |
| Java for Android toolchain | 21.0.10 |
| Gradle wrapper / Android SDK | Gradle 8.13; compile/target SDK 35; min SDK 24 |
| Android Gradle Plugin / Kotlin | 8.8.2 / 2.0.21 |
| Xcode / iOS deployment target | Xcode 26.0 (build 17A324); iOS 15.1 |
| C++ language level | C++17; validated with Apple clang 17.0.0 |
| Canonical CBOR | fxamacker/cbor v2.9.2 |
| Web/React Native CBOR | cbor-x 1.6.4 |
| CSS processing | PostCSS 8.5.23 (direct pin and npm override in both Expo apps) |
| Docker / Compose validation host | 24.0.2 / 2.19.1 |

Android Gradle Plugin and Kotlin are resolved by the Expo 53 / React Native 0.79 generated native build and its lockfiles; compile/target/min SDK and Gradle wrapper are fixed above and verified by `assembleDebug`. ESP32 Arduino/ESP-IDF/PlatformIO versions remain blocked until the confirmed vendor hardware baseline is introduced; they must not be inferred from example vendor projects.

Node.js `crypto` from the pinned Node 22 runtime is also the canonical release-signature implementation. It generates/loads standard Ed25519 PKCS#8/SPKI PEM keys and signs the exact bytes of `release-manifest.json`; this avoids dependence on the macOS system LibreSSL, which does not expose Ed25519 key generation in the validated environment.

## Native build evidence

- `make native-bundle` creates Android, iOS and Web Expo production bundles without hardware.
- `make ui-interaction-test` exports both Web applications against isolated mock public APIs and executes the MiniPOS sale/UNKNOWN/reconcile/admin and BeeFiscal readiness/report/admin journeys in Playwright Chromium.
- `make android-build` creates `BeeMiniPOS/android/app/build/outputs/apk/debug/app-debug.apk` and proves native BLE autolinking/compilation.
- `make ios-build` installs locked CocoaPods and creates an unsigned universal iOS Simulator app with the native BLE pod linked.
- `make native-regression` runs all three checks. Signed store artifacts and BLE behavior on physical devices are separate HIL/release gates.

## Dependency-security baseline

The 2026-08-09 refresh pins the mutually compatible noble 2.3.0 set and the fixed `js-yaml` 4.3.1 transitive override. `npm audit` still reports 12 moderate findings in the Expo 53 build-tool chain and two root High DoS advisories for `image-size` (expanded to six affected graph nodes through Metro/React Native). Every published `image-size` version is currently inside the affected `<=2.0.2` range and npm has no fixed release; npm's advertised React Native 0.72/Expo 46 downgrade is incompatible and prohibited. These build-time findings keep release evidence `PROD_NO_GO`; they are not waived or misreported as a clean scan.
