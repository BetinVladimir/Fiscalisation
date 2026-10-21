# Actual native application build checkpoint

Дата:2026-10-21. Это working-tree snapshot/evidence конкретных приложений; source hashes важнее общего HEAD с посторонними UI edits. Полный mobile/radio/financial HIL и store/release acceptance не заявлены. ESP32-S3 identity остаётся в NVS; ATECC не используется.

## BlueCash Android: APK и unit PASS

Worker из isolated SmartDevices copy с private SDK/Gradle cache выполнил собственным Gradle8.11.1/JDK17/API36 `:bluecash-app:testProdDebugUnitTest :bluecash-app:assembleProdDebug`. Root независимо сверил все69 snapshot entries с оригиналом, APK SHA256, реальные14 JUnit XML suites/43tests/0fail/0error/0skip, а также aapt manifest package `org.beeloy.bluecash.prod`, actual launcher `com.beeloy.fiscal.bluecash.MainActivity`, debuggable=true. Inventory69 включает один существующий .kotlin/errors log hash; это не69 compiled translation units, содержимое лога не публикуется.

[Safe summary/source hashes](evidence/bluecash-actual-apk-2026-10-21/summary.json). APK сохранён локально в ignored `.store-build/bluecash-prod-debug-20261021/bluecash-app-prod-debug.apk`; SHA256 `c7549031baecfa5a854d6401da389b134038152bda7e42fd82c4eab3ede85da3`. Это debug-signed build, не production release. Vendor Datecs JAR hashes зафиксированы; реальные Datecs Binder services/BlueCash device нужны для приёмки.

`make smart-device-test` исправлен: теперь используется SmartDevices/gradlew вместо wrapper соседнего MiniPOS/android. Набор задач сохранён. Actual BlueCash gate прошёл своим wrapper; общий много-модульный Make target после изменения отдельно не выполнялся.

## MiniPOS/FiscalApp: работа продолжается

Actual Expo AppEntry → App.tsx сборки подготавливаются с source/dependency inventory соседнего shared UI/design tokens. MiniPOS Metro910modules bundle и native modules compilation состоялись, но release lint pipeline встретил java.lang.OutOfMemoryError: Metaspace при512MiB. Повтор с большим task-local memory budget допустим только после завершения текущего процесса, lint не отключается. Итоговый APK пока не подтверждён.

Tracked MiniPOS iOS Podfile.lock содержит Expo53/RN0.79, тогда как actual package/installed deps Expo54/RN0.81.5. Старый native tree не доказывает текущую iOS сборку; нужны fresh prebuild/pod resolution и actual simulator/device build. FiscalApp tracked iOS tree отсутствует. Эти разрывы OPEN; key interoperability harness не заменяет full application artifact. Устройства не прошивались и приложения на них не устанавливались.
