# Текущий статус фискальных контуров

Дата: 2026-10-21. Граница: статическое ревью выбранных production paths и существующих отчётов; одна фактически выполненная проверка contract-lock. Полный повтор suite и demo фискальные операции не запускались.

## 1. Архитектура

| Контур | Источники реализации | Что обнаружено | Текущий доказанный предел |
| --- | --- | --- | --- |
| Fiscal backend | fiscal-backend/, contracts/ | Отдельный Go backend, операции, identity, repository, authoritative projection | SOURCE_PRESENT; предыдущие PostgreSQL/HTTP отчёты — REPORTED_PASS |
| MiniPOS backend | minipos/beeminipos-backend/internal/fiscalintegration/ | Вызовы публичного Fiscal API, owner/operation сверки, enrollment, webhook inbox | SOURCE_PRESENT; реальный demo денежный цикл NOT_RUN |
| MiniPOS WEB | minipos/miniposweb/src/fiscal/ | cloud recovery, route controller, intent builder, browser GATT | SOURCE_PRESENT; browser evidence не заменяет устройство |
| MiniPOS native | minipos/BeeMiniPOS/ | Native приложение, identity и native transport helpers | SOURCE_PRESENT; полный актуальный APK/IPA остаётся отдельным gate |
| Fiscal/Admin UI | BeeFiscalApp/, AdminApp/ | Expo UI и provisioning/management | SOURCE_PRESENT; проверять каждую платформу отдельно |
| S3 runtime | IoT/firmware/edge-agent-s3/idf/main/ | intent processor, durable storage, payment reconcile, serial/USB/BLE/MQTT/sync | SOURCE_PRESENT; основной runtime именно IDF |
| Fiscal driver facade | IoT/protocol-abstraction/ | Раздельные IFiscalDevice и IPaymentTerminal | SOURCE_PRESENT; device-model/firmware/region HIL_PENDING |
| Smart Android | SmartDevices/bluecash-app/ и apps/ | Vendor adapters, activation, journal | BlueCash build/unit — REPORTED_PASS, физическая касса/эквайринг HIL_PENDING |
| Учёт и логистика | Соседние BeeloyBackend контуры | Требуются sale/payment/receipt → accounting и связи источников | DEMO_PENDING; Paid/чек не подтверждают автоматически проводки |

Удалённый корневой Go edge-agent не использовать как production runtime. Arduino skeleton рядом с IDF не доказывает работу IDF. Ключи S3 по текущему плану находятся во внутренней NVS; внешний ATECC608 не является обязательным компонентом.

## 2. Подтверждённый новый FAIL: FISC-DOC-001

Команда из корня Fiscalisation: `sh scripts/verify-contract-lock.sh`.

Результат текущего ревью: exit1, `contract drift: .../contracts/openapi-runtime-v1.yaml`.

| Источник | runtime SHA-256 |
| --- | --- |
| Фактический файл | `6e0f350e740977ec87d682d7f85f151c26aea4d6962ab36455a5d37a11d16960` |
| Ожидает verify-contract-lock.sh | `d21c47241323915eb67d17d924eb2261915edf416c43991c338e9d18f7640397` |
| Записан в CONTRACT_LOCK.md | `46f90ac046f6b62bc73ece42712d55240856d758319a0db4241c90d2d5e1517b` |

Canonical public OpenAPI, AsyncAPI и corrections overlay совпали с ожидаемыми хешами при отдельной сверке. Это не закрывает generated runtime drift или request/response compatibility.

Доработка: review diff runtime против утверждённого baseline → проверка handlers/request/success schemas → regenerate typed clients/contracts → целевые и общие contract gates → единое обновление lock/script/docs. Нельзя просто скопировать текущий SHA в lock ради зелёного результата. Статус VERIFIED_FAIL до полного повторного доказательства.

## 3. Что ранее проверялось, но здесь не перезапускалось

| Историческое доказательство | Заявленная проверка | Что не выводить из неё |
| --- | --- | --- |
| ../remediation/2026-10-17/37_POSTGRES_CANONICAL_INTEGRATION.md | SQL/persistence/two-compose integration | Не опубликованный demo и не физическая фискализация |
| ../remediation/2026-10-17/51_FISCAL_REPOSITORY_RECOVERY.md | Persistence uncertainty и защищённые методы repository | Не весь текущий demo failover и не новые последующие изменения |
| ../remediation/2026-10-17/67_SELF_CONTAINED_TRANSPORT_CI.md | Реальный software producer/oracle/browser transport | Не физический радиообмен и не device firmware |
| ../remediation/2026-10-17/68_INSTALLED_TIME_LIFECYCLE.md | Time lifecycle hooks и native sanitizer cases | Не принятый remote time provider и не измеренный осциллятор |
| ../remediation/2026-10-17/69_NATIVE_APPLICATION_BUILD_CHECKPOINT.md | BlueCash APK/unit и hashes | Не полный актуальный MiniPOS/FiscalApp APK/IPA |

Перед переносом любого REPORTED_PASS в VERIFIED_PASS проверить actual source fingerprint и rerun scope. Числа тестов старых документов не складывать и не выдавать за текущую общую полноту.

## 4. Реализации и проверки, требующие завершения

| ID этого среза | Состояние | Требование / следующий шаг |
| --- | --- | --- |
| FISC-DOC-001 | VERIFIED_FAIL | Согласовать runtime contract lock и generated surfaces |
| FISC-EL-001 | IMPLEMENTATION_REQUIRED по документу30 | Полный жизненный цикл электронного фискального чека; бумажный/программный документ не заменяет |
| FISC-TIME-001 | SOURCE_PRESENT / DEMO_PENDING | Замкнуть trusted-time provider/issuer/bootstrap в реальный demo; исторические hooks не закрывают весь vertical |
| FISC-NATIVE-001 | DEMO_PENDING | Собрать актуальные полные native приложения, проверить встроенный demo endpoint и source/artifact hashes |
| FISC-BLE-001 | DEMO_PENDING / HIL_PENDING | Secure binding/PoP/reliable GATT и весь native/offline цикл проверить по актуальному коду; прошлые OPEN перепроверить, не считать автоматически исправленными |
| FISC-OFF-001 | DEMO_PENDING | Полная offline-equivalence/expiry/revoke/sync, без повторного debit/receipt |
| FISC-DEV-001 | HIL_PENDING / EXTERNAL_PENDING | Конкретная model/firmware/SDK/acquirer matrix, электрический интерфейс и настоящий fiscal proof |
| FISC-ACC-001 | DEMO_PENDING | Сверить документы и события Fiscal с бухгалтерией, оплатой и учётом исходного бизнеса |
| FISC-RELEASE-001 | EXTERNAL_PENDING | Нормативная применимость, разрешения и независимые release evidence; не выводить из software PASS |

Эти ID — навигация к существующим GAP/T/UI, не замена их более подробных критериев. Непроверенный текущий runtime нельзя объявлять дефектным только по старому OPEN: сначала воспроизвести. И наоборот, новое наличие файла не закрывает исторический разрыв.

## 5. Приоритет исправлений

1. Согласованность contracts и воспроизводимые артефакты — база для остальных тестов.
2. Durable-before-device, owner/tenant fences, UNKNOWN и per-tender recovery.
3. Readiness/time/provisioning и transport security.
4. Полный cash/card/split/reversal цикл с независимым stateful device peer.
5. Published demo WEB → Android → iOS с одним и тем же бизнес-результатом и отдельными operationId.
6. Электронный чек и accounting/source-posting цепочки.
7. Полные device/HIL/external gates отдельно от demo и unit.
