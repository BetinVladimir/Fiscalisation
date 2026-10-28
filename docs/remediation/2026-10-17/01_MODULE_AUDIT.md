# Охват репозитория и исходное evidence

[Навигация](README.md) · [Реестр заданий](02_GAP_REGISTER.md)

## Карта модулей

| Модуль | Реальная роль / результат исследования | Что ещё доказать |
|---|---|---|
| `fiscal-backend/cmd`, `internal/api`, `domain` | REST, registry/composite binding, operations, idempotency, authority, sync materialization; обнаружены расхождения mapping и sale identity | T-01, T-05, T-13, T-19: реальные device envelopes и tenant boundaries |
| `fiscal-backend/internal/mqttclient` | Публикация command и приём sync; другой namespace command topic относительно физических получателей | GAP-001/002, T-01 |
| `fiscal-backend/internal/persistence`, `startup`, `config`, `auth`, `webhook`, `integration` | Есть unit/integration suites; Go suite прошёл. Это не отдельное подтверждение real PostgreSQL/RLS и broker mTLS | T-16/19/20: чистая БД, рестарт, конкуренция, WebHook delivery |
| `minipos/beeminipos-backend` | Автономные orders/shifts/catalog, webhook inbox, Fiscal checkout; три API regression failures | GAP-028; T-15/16/19; у fiscalintegration/mqttclient нет собственных Go test files |
| `minipos/BeeMiniPOS` | Expo Android/iOS, BLE transport, crypto/CBOR, checkout/session; native BLE фактически принимает OPEN_MVP | GAP-007/008/027; T-03/04/14/21 |
| `minipos/miniposweb` | React/Vite POS, IndexedDB, route controller, offline cache; риск повторной отправки UNKNOWN подтверждён ветвлением кода | GAP-026; T-14/17/21 |
| `BeeFiscalApp` | Администрирование, registry и activation; Edge V2 onboarding не завершён в native implementation | GAP-003; T-02/18/21 |
| `AdminApp` | Отдельная административная Expo app; typecheck проходит, самостоятельного `test` script нет | T-18: auth expiry/roles/tenant and secret-redaction; UI e2e отдельно |
| Удалённый Go `edge-agent/` | Исторический объект исходного аудита; модуль удалён по запросу пользователя | Исторические тесты не являются текущим evidence |

| `IoT/firmware/edge-agent-s3/idf` | Основной ESP32-S3 runtime: SD/SQLite, UART/USB, NimBLE peripheral+central, MQTT, local HTTP, SPA manager | Основная концентрация GAP-003…025; host tests должны компилировать именно эти реализации |
| `IoT/firmware/edge-agent-s3` Arduino track | Scaffold/другой composition; не считать взаимозаменяемым с IDF | Закрепить один target и reproducible image, T-22 |
| `IoT/common-modules/{datecs,daisy,datecspay}` | Отдельные драйверы и transport-neutral BLE profile; их тесты проходят | GAP-009…013/029: правильные функции не гарантируют включение в firmware |
| `IoT/common-modules/termol` | Дополнительный Arduino Tremol driver с собственным framing и командами; не включён в два основных S3 physical profiles | GAP-032; T-24, без заявления аппаратной готовности |
| `IoT/protocol-abstraction` | Facade adapters и **отдельные** `FrameCodec.cpp`, `CommandPayload.cpp`; IDF использует последние | GAP-009/010; independent golden vectors обязательны |
| `IoT/device-manufacturing` | Python flash/hash и registration client; нет полного network/secret provisioning lifecycle | GAP-004/031; T-02/22 |
| `SmartDevices/bluecash-app` | Kotlin приложение с vendor JAR, raw fiscal/pinpad bridge, SQLite processor, activation, MQTT/HTTP/BLE | GAP-001/010/011/029; фактический APK/HIL не построен в этом аудите |
| `SmartDevices/daisy-smart-app`, `shared` | Общий transport interface; Daisy smart app явно STUB с production guard | GAP-033. Не путать с внешним Daisy USB принтером на S3 |
| `SmartDevices/apps/cashbox-a`, `cashbox-b`, `shared-sdk` | Документированные placeholders, самостоятельной готовой integration app нет | Исключить из поддерживаемой аппаратной матрицы до реализации |
| Корневой `BeeMiniPOS` | В просмотренном каталоге только `node_modules`; не самостоятельный проверяемый source module | Использовать `minipos/BeeMiniPOS`, убрать неоднозначность путей в заданиях |
| `database/fiscal`, `database/minipos`, `dbmigrate` | SQL migrations, typed rows, tenant roles/RLS; связанные embedded migration loaders в backend | T-16: реальный PostgreSQL, consistency SQL/embedded, upgrade/rollback drill |
| `contracts` и generated artifacts | OpenAPI, schemas, trace/security matrices описывают целевой контракт, но не runtime compatibility | GAP-030; T-01/05/13/23, normative sources с версиями |
| `deploy/fiscalisation`, `deploy/minipos`, `device-pki`, root Compose | Deployment templates/runbooks, CA, broker и изоляция сервисов | T-20/22: реально запущенные TLS/ACL/revocation, не только наличие конфигурации |
| `scripts`, `Makefile`, `tests/e2e`, release evidence | Software gates, driver coverage и signed HIL verifier; часть проверок структурная | GAP-029/030/034; T-22/23 |
| `docs`, `docs/MVP1`, `docs/SUPTO`, ADR, integration kit | Есть противоречащие текущему runtime claims. SUPTO документы не заменяют vendor/HIL evidence | Этот пакет supersedes readiness claims; внешние правовые gates сохраняются |

## Исполнено при исследовании 2026-10-17

| Проверка | Результат | Что результат действительно покрывает |
|---|---|---|
| `make iot-test` | PASS | 33 Python tests и native/common driver/protocol suites; многие IDF проверки проверяют строки исходника/модель SQLite, не исполняют весь physical runtime |
| `fiscal-backend`: `go test ./...` | PASS | Доступный локальный Go contour; не замена отдельному `make postgres-integration` |
| `edge-agent`: `go test ./...` | PASS | Go runtime, включая reference BLE/journal/sync; не ESP32-S3 |
| `minipos/beeminipos-backend`: `go test ./...` | FAIL | 3 failures в `internal/api`, остальные доступные пакеты проходят |
| `minipos/BeeMiniPOS`: `npm test` | PASS: 47 | TS module tests, не native radio или IPA/APK |
| `minipos/miniposweb`: `npm test -- --cache=false` | PASS: 10 файлов / 78 tests | Vitest suites на текущем срезе; без физического broker/радио |
| `BeeFiscalApp`: `npm test` | PASS: 16 | Module tests; не hardware onboarding |
| `AdminApp`: `tsc --noEmit` | PASS | Статическая проверка типов |
| `ruby scripts/verify_bluecash_release_composition.rb` | PASS | Структурная composition-проверка; не APK build или протокол в устройстве |
| Независимый C++ probe, actual `FrameCodec.cpp` | **FAIL относительно vendor vector** | Daisy vendor RX → `DAISY_LENGTH`; Datecs request LEN смещён на 4 |

Точные MiniPOS API failures: `TestEditorsShiftOrder` (`handler_test.go:479`) и `TestShiftRecoveryFiltersByEmployeeRegisterAndState` (`:499`) получают 403 `operator identity does not match employee`; `TestPublicContractAliasAndOptimisticUpdate` (`:637`) сообщает `version must be mandatory`. Требуется выяснить, устарел fixture или нарушен контракт; ослаблять operator binding ради зелёного теста нельзя.

Локальные диагностические логи: `/tmp/fiscal-deep-audit-20261017/{iot-tests,fiscal-go,edge-go,minipos-go,native-tests,miniposweb-tests,beefiscalapp-tests,admin-ts}.log`. Это временные материалы, не переносимое release evidence. Перед приёмкой сохранить новые результаты в CI с commit/build IDs и реальными exit codes.

## Границы результата

Не запускались hardware I/O, fiscal/card transactions, firmware flash, store publish, полноценная Gradle/IPA/IDF сборка, 72-hour physical soak, production deployment и отдельный PostgreSQL integration gate. Вердикта о готовности этих направлений нет. Наличие старых PASS в документах не перенесено в результаты сегодняшнего исследования. Часть Go результатов использовала cache; повторный acceptance-run должен быть без cache на фиксированном чистом срезе.
