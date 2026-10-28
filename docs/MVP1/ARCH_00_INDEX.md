# ARCH-MVP1 — архитектурный аудит перед тестами с реальными устройствами

Дата: 2026-10-22 · Роль: IT-архитектор · Корни: `Fiscalisation/` (далее FISC) и `POS/` (далее POS)
Метод: **статическое чтение кода** (Go, Kotlin, C++, TypeScript, SQL в запросах). Тесты и сборки **не запускались**, код **не менялся**.
Цель: MVP и тесты фискализации на реальных устройствах (BlueCash-50, S3+DP-150/Daisy), соответствие СУПТО / Наредба Н-18 / Приложение 29 (НАП).

## Состав пакета

| Файл | Содержание |
|---|---|
| [MVP_GOAL.md](MVP_GOAL.md) | **ЦЕЛЬ MVP (источник истины)**: POS+MiniPOS → MQTT/BLE → BlueCash-50, Datecs (S3+COM), BluePay (S3+BLE), Daisy (S3+USB), СУПТО |
| [ARCH_17_MVP_GOAL_PATHS_GAPS.md](ARCH_17_MVP_GOAL_PATHS_GAPS.md) | Разрывы по путям P-A…P-E, факты Daisy, HIL-матрица, задачи DSY/S3D/BP/MQ/CL |
| [ARCH_01_BUGS.md](ARCH_01_BUGS.md) | Очевидные баги: место, исправление, тесты, критерии приёмки |
| [ARCH_02_GAPS_TASKS.md](ARCH_02_GAPS_TASKS.md) | Технологические разрывы и недоработки, декомпозиция на мелкие задачи для слабой модели |
| [ARCH_03_TESTS_E2E.md](ARCH_03_TESTS_E2E.md) | Какие unit/интеграционные/E2E/HIL тесты ещё нужно написать |
| [ARCH_04_ARCHITECTURE_CODE_STYLE_DOCS.md](ARCH_04_ARCHITECTURE_CODE_STYLE_DOCS.md) | Архитектурные замечания, код, стиль, недостаточная документация кода |
| [ARCH_05_POS_INTEGRATION.md](ARCH_05_POS_INTEGRATION.md) | Интеграция POS ↔ pos-gateway ↔ RabbitMQ ↔ fiscal-backend ↔ устройства |
| [ARCH_06_SUPTO_NAP_READINESS.md](ARCH_06_SUPTO_NAP_READINESS.md) | Матрица СУПТО/НАП: что закрыто, что блокирует HIL, что внешнее |
| [ARCH_07_PLAN70_CRITICAL_REVIEW.md](ARCH_07_PLAN70_CRITICAL_REVIEW.md) | Критическая оценка плана 70 |
| [ARCH_08_SUPTO_NAP_COMPLIANCE_STUDY.md](ARCH_08_SUPTO_NAP_COMPLIANCE_STUDY.md) | Исследование соответствия Приложению 29 / Н-18 |
| [ARCH_09_VENDOR_DOCS_AND_PROTOCOL_GAPS.md](ARCH_09_VENDOR_DOCS_AND_PROTOCOL_GAPS.md) | Вендорная документация (инвентарь, чего не хватает) и 20 разрывов «спецификация ↔ реализация» (VG-01…VG-20) |
| [ARCH_10_NAP_LATEST_CHANGES.md](ARCH_10_NAP_LATEST_CHANGES.md) | Последние изменения НАП (евро, ДВ 42/2025, ДВ 115/2025, ДВ 82/2026) и влияние (NAP-16…23, NAP-N0) |
| [ARCH_11_MVP_COMPLETENESS_REVIEW.md](ARCH_11_MVP_COMPLETENESS_REVIEW.md) | Проверка полноты: определение MVP, трассировка возможностей → задачи → тесты → ворота, что ещё открыто |
| [DECISIONS.md](DECISIONS.md) | Решения D-1…D-12 (DEFAULT по репозиторию и спецификации), статус DOC-V, интерим-правила NORM, HIL-пробы PRB-01…05 |
| [ARCH_12_ACCEPTANCE_SCRIPTS.md](ARCH_12_ACCEPTANCE_SCRIPTS.md) | Спецификации 36 отсутствующих скриптов приёмки SCR-01…36 |
| [ARCH_13_COMPONENTS_MINIPOS_ADMIN_ERP.md](ARCH_13_COMPONENTS_MINIPOS_ADMIN_ERP.md) | Статический разбор minipos, AdminApp, BeeFiscalApp, erp-connector, public-api-core (24 находки) |
| [ARCH_15_TEST_SPECS_WITH_LOGIC.md](ARCH_15_TEST_SPECS_WITH_LOGIC.md) | 20 E2E сценариев Given/When/Then с числами, 12 задач TST-DEEP; фикстуры в `fixtures/` |
| [ARCH_16_PLAN_REVIEW_ROUND4.md](ARCH_16_PLAN_REVIEW_ROUND4.md) | Ревью против реальной реализации и ожиданий `docs/`; пригодность для модели 35B/35k; условия достижения MVP |
| [TRACE_EXISTING_REGISTERS.md](TRACE_EXISTING_REGISTERS.md) | Транзитивная трассировка GAP-001…036 → карточки плана 70 → задачи MVP1 |
| [execution/decomp/](execution/decomp/) | Декомпозиция до функций: backend.md, android.md, pos.md |
| [vendor/datecs_golden_vectors.json](vendor/datecs_golden_vectors.json) | Эталонные кадры Datecs из примеров спецификации (проверены независимым энкодером) |
| [../vendor/DAISY_SMART_PROTOCOLS.md](../vendor/DAISY_SMART_PROTOCOLS.md) | Разбор протоколов Smart S (фискальный JSON + Payment Bridge): факты, пробелы F-1…F-6, вопросы вендору |
| [../vendor/DATECSPAY_PROTOCOL.md](../vendor/DATECSPAY_PROTOCOL.md) | Разбор протокола DatecsPay (BluePad/BlueCash-50): кадры, транзакции, прокси сокета, восстановление, вопросы Q-B* |
| [ARCH_18_TEST_LOGIC_SPECS.md](ARCH_18_TEST_LOGIC_SPECS.md) | Логика 60 тестов ARCH_03 в Given/When/Then/R с числами (части 1–3, включая TST-VENDOR) |
| [ARCH_19_HIL_SCENARIOS.md](ARCH_19_HIL_SCENARIOS.md) | 48 сценариев S1…S8 для 6 путей/транспортов: шаги, ожидания, инжекция сбоев, evidence |
| [ARCH_20_EMULATOR_SPECS.md](ARCH_20_EMULATOR_SPECS.md) | Спецификации эмуляторов: Datecs, Daisy, Smart S SDK, BluePad, MQTT-брокер, BLE-периферия (задачи EMU-01…06) |
| [ARCH_21_DELIVERY_PIPELINE.md](ARCH_21_DELIVERY_PIPELINE.md) | Конвейер: qwen кодит → codex ревью → тесты+багфикс qwen → e2e codex; маршрутизация, чек-лист, сроки (~14 недель) |
| [ARCH_22_TEST_E2E_REVIEW_ADDENDUM.md](ARCH_22_TEST_E2E_REVIEW_ADDENDUM.md) | Ревью тестов/e2e: 12 находок (в т.ч. дефект SEQ Datecs на S3), сценарии TST-E2E-21…26, правила 3 из 3 |
| [OPEN_WORK.md](OPEN_WORK.md) | Актуальные работы после сверки с кодом (2026-10-24): закрыто / частично / подтверждённо открыто / не проверялось; генерируется `tools/gen-open-work.py` из `execution/reverify.json` |
| [tools/](tools/) | `task-pack.py` (пакет задачи ≤6000 ток.), `check-doc-refs.py` (ссылки/статус/сценарии), `extract_vendor_frames.py` (проверка golden-кадров), `datecs_frame.py`, `route-tasks.py` (маршрут qwen/codex/human), `run-accept.py` (прогон accept), `check-test-matrix.py`, `apply_reverify.py`, `gen-open-work.py` |
| [MASTER_PLAN.md](MASTER_PLAN.md), [execution/status.json](execution/status.json) | Общий план (волны, ворота, решения D-1…D-10) и машинный трекер |

Существующие документы (`docs/remediation/2026-10-17/*`, `docs/fiscal-contour-review-20261021/*`, `MVP_BLOCKERS.md`) **не дублируются**: где находка уже есть в реестре, дана ссылка `GAP-0xx` и отмечено «подтверждено в коде». Новые идентификаторы: `BUG-*`, `GAPX-*`, `TST-*`, `ARCH-*`, `POS-*`, `SUP-*`.

## Главные выводы (коротко)

1. **Blocker для HIL BlueCash:** в `BlueCashEngine.kt` (a) при сбое после открытия чека он не отменяется; (b) при UNKNOWN фискальном результате карта всё равно сторнируется; (c) нет таймаута чтения и обработки SYN/NAK (GAP-011 подтверждён). Это BUG-020, 021, 029.
2. **Сломана цепочка POS → шлюз → fiscal-backend:** шлюз финализирует любой чек как `CASH` с суммой `0.00` по умолчанию (BUG-055), `reconcile` вызывается по несуществующему URL (BUG-050), приложение использует один base URL для двух разных сервисов (BUG-051).
3. **Фейковая синхронизация часов ФУ** при MQTT-маршруте: `Bridge.DeviceTime()` возвращает часы сервера (BUG-005) — требование СУПТО п.5 формально «выполнено», фактически нет.
4. **Денежная арифметика не единая** в 4 местах (Go backend: half-up; BlueCash: без округления; POS: half-even; шлюз: НДС `/1.2`) → отказы на дробных количествах (BUG-023, 056, 061) и `receiptMoneyCents` принимает `-0.50` как `+0.50` (BUG-001).
5. Все операции помечаются `simulated:true`, даже на реальных устройствах (BUG-003) — искажает аудит и экспорт для НАП.
6. Состояние backend хранится одним JSON-снимком, сериализуемым на каждую запись под глобальным мьютексом (ARCH-001) — не масштабируется и не годится для production.
7. Секреты: `POS/pos-app/credentials.json` (пароли Android keystore) **находится под контролем git** (BUG-060). Ротировать.

8. **Вендорная сверка (ARCH_09):** BlueCash шлёт текст в UTF-8, а ФУ ждёт windows-1251 (кириллица); пустой ответ и статус-байты ФУ не проверяются; после рестарта SEQ снова `0x20` и ФУ может молча проигнорировать команду (кэш ответа); нет чтения состояния ФУ (cmd 74/76/103, параметр `UNP`) для разрешения UNKNOWN; режим валюты ФУ (евро) не проверяется.
9. **Нормативный baseline (ARCH_10):** последняя подтверждённая поправка Н-18 — ДВ 42/2025 (евро: параметр валюты ФУ 0–3); найдены ЗИД ЗДДС ДВ 115/2025 и евро-поправка ДВ 82/2026, первичные тексты которых нужно прочитать (NAP-N0).

## Приоритеты

- **P0** — блокирует начало HIL или может привести к денежному/фискальному расхождению.
- **P1** — исправить до пилота/первого реального клиента.
- **P2** — качество, поддерживаемость, документация.

## Как исполнять задачи слабой моделью (qwen3.5:9b и т.п.)

1. Брать **одну** задачу `T-…` за сессию. Читать только перечисленные в ней файлы и строки.
2. Правило «красный → зелёный»: сначала написать тест из раздела «Тест», запустить, убедиться, что он **падает** по описанной причине; затем править код; затем тест **зелёный**.
3. Не рефакторить смежный код, не переименовывать, не менять публичные контракты (OpenAPI, CBOR/JSON-схемы) без явной задачи.
4. После правки выполнить команды из блока «Приёмка (автоматическая)». Если любая команда ≠ код 0 — задача не закрыта.
5. Номера строк даны на коммите `b33afe8` (Fiscalisation) и `7a3c0db` (POS). Если строки сместились — искать по имени функции.

## Общий Definition of Done (CDOD) — применяется к каждой задаче

- [ ] Добавлен/изменён тест, который **падал до** правки и **проходит после** (в сообщении коммита указать имя теста).
- [ ] Команда(ы) из «Приёмка (автоматическая)» завершаются кодом 0.
- [ ] Go: `cd fiscal-backend && go vet ./... && go test -race ./...` (для POS: `cd pos-gateway && go vet ./... && go test -race ./...`).
- [ ] Kotlin: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest` (при расхождении — смотреть цель `smart-device-test` в `Makefile`).
- [ ] TS (POS): `cd pos-app && npx tsc --noEmit && npx jest`.
- [ ] C++ S3: `make iot-test` из корня FISC.
- [ ] Если затронут контракт: `sh scripts/verify-contract-lock.sh` не хуже, чем до правки (сейчас падает — FISC-DOC-001, см. `docs/fiscal-contour-review-20261021/01-review-and-status.md`).
- [ ] Статус задачи обновлён в таблице `ARCH_02_GAPS_TASKS.md` / `ARCH_01_BUGS.md` (колонка Status: OPEN → FIXED + SHA).
- [ ] Нет изменений вне файлов, перечисленных в задаче (проверка: `git diff --stat` содержит только эти файлы + тесты).
- [ ] Для денежных путей: добавлен случай UNKNOWN/timeout (результат **не** считается NOT_SENT).

## Ограничения анализа (честно)

- Не читались целиком: `handler.go` (2089 строк), `repository.go`, `postgres.go`, `exports.go`, BLE-слой, `minipos/*`, `AdminApp`, `BeeFiscalApp`, остальные C++ файлы S3. Они помечены в `ARCH_03` как «слепые зоны, требуют второго прохода».
- Утверждения со статусом **TO VERIFY** требуют подтверждения тестом, прежде чем править код.
- Протокол Datecs читался по реализации и по пересказам в `docs/remediation/.../03_DEVICE_PROTOCOLS.md`; PDF вендора не открывался.
- Юридические выводы — не заключение юриста; перед декларацией СУПТО нужна проверка болгарским специалистом.

## Дополнение
- [ARCH_07_PLAN70_CRITICAL_REVIEW.md](ARCH_07_PLAN70_CRITICAL_REVIEW.md) — критическая оценка плана 70_FULL_PRE_HIL…

## Существующие Make-цели и файлы (учитывать при исполнении; проверено `grep`/`ls`)
- Уже есть: `make test`, `test-race`, `vet`, `contract-test` (`verify-contract-lock.sh` + генерация OpenAPI + tsc), `security-test` (`verify_security_regression.rb`, `verify_sensitive_data.sh`), `supto-time-test` (`go test ./internal/domain -run 'Test.*(Clock|Time)'`), `supto-export-test` (`-run 'Test.*Export'`), `postgres-integration`, `compose-e2e`, `fault-regression-test`, `soak-regression-test`, `smart-device-test` (Gradle prod/demo), `iot-test`, `supto-trace-test` (`scripts/verify_supto_trace.rb`).
- **Правило именования:** новые Go-тесты часов/времени называть `Test…Clock…`/`Test…Time…`, экспорта — `Test…Export…`, чтобы их подхватили существующие цели без правки Makefile.
- Уже существуют тестовые файлы (дополнять, не создавать заново): `fiscal-backend/internal/config/config_test.go`, `internal/auth/{auth_test,oidc_test}.go`, `internal/domain/{unp_allocator_test,reversal_policy_test,receipt_saga_test,exports_test,readiness_time_test}.go`, `SmartDevices/bluecash-app/src/test/kotlin/.../{BlueCashEngineTest,BlueCashComplianceIntentTest,DatecsProtocolTest}.kt`.
- Перед созданием любого теста из ARCH_03 выполнить `grep -rn "<TestName>"` — часть кейсов может уже существовать в других формулировках.
- Цели, которых нет и которые предлагается создать (помечены «создать» в ARCH): `fuzz-short`, `plan-check`, `lint` (проверить: в Makefile нет `lint`), скрипты `check-test-matrix.sh`, `check-no-secrets.sh`, `check-prod-artifact.sh` и т.п.
- [ARCH_08_SUPTO_NAP_COMPLIANCE_STUDY.md](ARCH_08_SUPTO_NAP_COMPLIANCE_STUDY.md) — исследование соответствия СУПТО/Н-18 по официальному тексту: требования ↔ реализация ↔ разрывы (NAP-01…15, NORM-09…12)
- [MASTER_PLAN.md](MASTER_PLAN.md) — **общий план для агента исправлений** (волны W0–W9, ворота, цикл работы); трекер [execution/status.json](execution/status.json)
