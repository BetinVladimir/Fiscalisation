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
| Native BLE | react-native-ble-plx 3.5.1 |
| Portable BLE crypto | @noble/ciphers 1.3.0; @noble/curves 1.9.7; @noble/hashes 1.8.0; expo-crypto 14.1.5 |
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

## Native build evidence

- `make native-bundle` creates Android, iOS and Web Expo production bundles without hardware.
- `make android-build` creates `BeeMiniPOS/android/app/build/outputs/apk/debug/app-debug.apk` and proves native BLE autolinking/compilation.
- `make ios-build` installs locked CocoaPods and creates an unsigned universal iOS Simulator app with the native BLE pod linked.
- `make native-regression` runs all three checks. Signed store artifacts and BLE behavior on physical devices are separate HIL/release gates.

## Dependency-security baseline

`npm audit` on 2026-08-07 reports no critical or high findings in either Expo application after the PostCSS pin. Ten moderate findings remain in the Expo 53 build-tool chain (`@expo/cli`, config/plugins, Metro, `expo-asset`, `expo-constants`, `xcode` and transitive `uuid`). npm's advertised automatic fix is an incompatible Expo 46 downgrade and is therefore prohibited. Resolution requires a planned Expo SDK upgrade with Android/iOS/Web release regression; until then these findings are a recorded release risk, not silently accepted production clearance.
