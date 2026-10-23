# ARCH_12 — Скрипты и цели Make приёмки: задачи на написание

Дата: 2026-10-22 · Источник: ARCH_11 §6.8 + автоматический обход `docs/MVP1/*.md` и `execution/status.json` (отсутствующие `scripts/*`, `make *`).
Найдено **36** ссылок на несуществующие скрипты/цели. Пока они не написаны, команды приёмки соответствующих задач **нерабочие** (ложный «красный» или, хуже, «зелёный» при `|| true`).

## 0. Общие соглашения (обязательны для каждого скрипта)
- Расположение: `scripts/`; bash — `#!/usr/bin/env bash` + `set -euo pipefail`; Ruby/Python — без внешних зависимостей сверх stdlib (Ruby ≥ 3, Python ≥ 3.9).
- Корень репозитория определяется `ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"`; рабочий каталог не предполагается.
- Коды выхода: `0` — всё хорошо; `1` — найдены нарушения; `2` — ошибка использования/входных данных; `3` — не выполнены предусловия среды (нет БД, нет инструмента). **SKIP запрещён**: недоступный инструмент = код 3.
- Вывод нарушения: одна строка `FAIL <КОД> <путь>[:<строка>] <сообщение>`; итог: `OK <N> checks` (stdout) или `FAILED <M> of <N>` (stderr).
- Детерминизм: без обращения к сети, кроме явно описанного; без зависимости от времени (кроме `--max-age-days`).
- Самотест: для каждого скрипта `scripts/tests/test_<имя>.sh` запускает скрипт на `scripts/tests/fixtures/<имя>/good` (ожидает 0) и `…/bad` (ожидает 1 и строку с указанным кодом). Скрипт без самотеста не считается готовым.
- Регистрация: после готовности добавить в `make plan-check` (для документных проверок) или соответствующую цель, и в `.PHONY` Makefile.
- Подзадачи каждой SCR-задачи (одинаковый шаблон): `.1` самотест и фикстуры (красный: скрипта ещё нет), `.2` реализация проверок по списку, `.3` подключение в Makefile и `scripts/README.md`.

## 1. Реестр
| Задача | Файл/цель | Приоритет | Кто использует |
|---|---|---|---|
| SCR-01 | `scripts/check_plan_coverage.rb` | P0 | MASTER_PLAN, ARCH_05/06/07/11 (make plan-check) |
| SCR-02 | `scripts/check_plan_acceptance.rb` | P0 | ARCH_07 (make plan-check) |
| SCR-03 | `scripts/check-test-matrix.sh` | P1 | ARCH_03, ARCH_06, ARCH_07 (make plan-check) |
| SCR-04 | `scripts/check-no-secrets.sh` | P0 | ARCH_01, ARCH_02, ARCH_06, ARCH_11 |
| SCR-05 | `scripts/check-prod-artifact.sh` | P0 | ARCH_06, ARCH_08 |
| SCR-06 | `scripts/check-migrations.sh` | P0 | ARCH_02, ARCH_06, MASTER_PLAN |
| SCR-07 | `scripts/check-ops-docs.sh` | P1 | ARCH_06, MASTER_PLAN |
| SCR-08 | `scripts/check-vendor-docs.sh` | P1 | ARCH_03, ARCH_09, VG-19 |
| SCR-09 | `scripts/check-golden-vectors.sh` | P0 | ARCH_09 VG-20, MASTER_PLAN |
| SCR-10 | `scripts/check-datecs-error-catalog.sh` | P1 | ARCH_03, ARCH_09 VG-02 |
| SCR-11 | `scripts/gen-datecs-error-catalog.py` | P1 | VG-02 |
| SCR-12 | `scripts/check-nap-baseline.sh` | P1 | ARCH_10, DECISIONS, NAP-N0 |
| SCR-13 | `scripts/check-norm-decisions.sh` | P1 | ARCH_06 |
| SCR-14 | `scripts/check-decisions.sh` | P1 | DECISIONS |
| SCR-15 | `scripts/check-adr-links.sh` | P1 | ARCH_04 |
| SCR-16 | `scripts/check-doc-drift.sh` | P1 | ARCH_04, ARCH_05, MASTER_PLAN |
| SCR-17 | `scripts/check-doc-invariants.sh` | P1 | ARCH_04, MASTER_PLAN |
| SCR-18 | `scripts/check-lint-rules.sh` | P1 | ARCH_04 |
| SCR-19 | `scripts/check-swallowed-errors.sh` | P1 | ARCH_04 |
| SCR-20 | `scripts/check-rounding-golden.sh` | P1 | ARCH_04, D-4 |
| SCR-21 | `scripts/check-storno-coverage.sh` | P1 | ARCH_02 |
| SCR-22 | `scripts/check-db-schema.sh` | P1 | ARCH_02 TG-001/002 |
| SCR-23 | `scripts/check-mqtt-tls.sh` | P1 | ARCH_02 |
| SCR-24 | `scripts/check-rabbit-topology.sh` | P1 | ARCH_01, ARCH_05 |
| SCR-25 | `scripts/check-data-location.sh` | P1 | ARCH_08 |
| SCR-26 | `scripts/check-version-bump.sh` | P1 | ARCH_08, NAP-22 |
| SCR-27 | `scripts/e2e-trace-check.sh` | P1 | ARCH_05 |
| SCR-28 | `scripts/gen-annex29-columns.py` | P1 | ARCH_06 |
| SCR-29 | `scripts/verify_trace_tests_exist.rb` | P1 | ARCH_06, ARCH_08 |
| SCR-30 | `scripts/hil/run.sh` | P0 | ARCH_02, ARCH_11, MASTER_PLAN |
| SCR-31 | `scripts/loadtest/run.sh` | P1 | ARCH_02 TG-016 |
| SCR-32 | `scripts/mutate-vector.sh` | P1 | ARCH_03 TST-VENDOR |
| SCR-33 | `make lint` | P1 | ARCH_04, ARCH_11 |
| SCR-34 | `make plan-check` | P0 | ARCH_07, ARCH_09, ARCH_11 |
| SCR-35 | `make fuzz-short` | P1 | ARCH_03 |
| SCR-36 | `make contract-lock` | P1 | ARCH_09 |

## 2. Спецификации

### SCR-01 · P0 · `scripts/check_plan_coverage.rb`
- **Язык:** ruby · **Нужен для:** MASTER_PLAN, ARCH_05/06/07/11 (make plan-check)
- **Назначение:** Целостность плана: markdown-карточки ↔ status.json.
- **Проверки (каждая — отдельный код FAIL):**
  1. Прочитать docs/MVP1/execution/status.json; собрать множество ID задач.
  2. Из docs/MVP1/ARCH_*.md и DECISIONS.md собрать ID по regexp ^###? (?:[A-Z]+-[0-9A-Z.-]+) · ; каждый ID из md обязан быть в status.json (код FAIL PLAN-001).
  3. Каждый ID из status.json обязан иметь поле doc, указанный файл существует и содержит ID (PLAN-002).
  4. depends ссылаются на существующие ID (PLAN-003); граф без циклов — алгоритм Кана (PLAN-004).
  5. wave(dep) ≤ wave(task) (PLAN-005).
  6. Для status != NOT_A_BUG: поле accept или subtasks[*].accept непустое (PLAN-006).
  7. Число задач в docs/MVP1/MASTER_PLAN.md (строка «Всего задач: N») равно len(tasks) (PLAN-007).
- **Красный фикстур (самотест):** status.json с depends на несуществующий ID → exit 1, строка PLAN-003.
- **Приёмка:** `bash scripts/tests/test_check_plan_coverage.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check_plan_coverage.rb" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-02 · P0 · `scripts/check_plan_acceptance.rb`
- **Язык:** ruby · **Нужен для:** ARCH_07 (make plan-check)
- **Назначение:** Каждая команда приёмки исполнима: ссылается на существующий файл/цель.
- **Проверки (каждая — отдельный код FAIL):**
  1. Для каждого accept/subtasks[*].accept разбить командную строку shellwords.
  2. Первый токен ∈ {go,cd,make,bash,python3,ruby,jq,rg,test,./gradlew,npm,npx,psql,curl,grep,sed,diff,ls} или существующий файл (ACC-001).
  3. Для make X — X есть в Makefile (.PHONY или цель) (ACC-002); для scripts/… — файл существует или в status.json есть задача SCR-* с этим path (ACC-003).
  4. Запрещены плейсхолдеры <…>, TODO, XXX, «…» (ACC-004).
  5. Команда go test обязана содержать -run или путь пакета, gradlew — --tests (ACC-005; защита от «зелёного ничего»).
- **Красный фикстур (самотест):** accept: "make nonexistent" → ACC-002.
- **Приёмка:** `bash scripts/tests/test_check_plan_acceptance.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check_plan_acceptance.rb" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-03 · P1 · `scripts/check-test-matrix.sh`
- **Язык:** bash · **Нужен для:** ARCH_03, ARCH_06, ARCH_07 (make plan-check)
- **Назначение:** Матрица тестов: каждый TST-* имеет файл/функцию, которая реально существует после реализации.
- **Проверки (каждая — отдельный код FAIL):**
  1. Прочитать docs/MVP1/test-matrix.tsv (колонки: id, level, file, symbol, status).
  2. Для status=IMPLEMENTED: файл существует (rg -q symbol file) (MAT-001).
  3. Для status=PLANNED: файл НЕ обязан существовать, но поле symbol непустое (MAT-002).
  4. Каждый TST-E2E-01..20 из ARCH_03 §7 присутствует в матрице (MAT-003).
  5. Нет двух строк с одним id (MAT-004).
- **Красный фикстур (самотест):** строка IMPLEMENTED с несуществующим symbol → MAT-001.
- **Приёмка:** `bash scripts/tests/test_check-test-matrix.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-test-matrix.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-04 · P0 · `scripts/check-no-secrets.sh`
- **Язык:** bash · **Нужен для:** ARCH_01, ARCH_02, ARCH_06, ARCH_11
- **Назначение:** Нет секретов в репозитории и артефактах.
- **Проверки (каждая — отдельный код FAIL):**
  1. git ls-files, исключая node_modules, dist-*, vendor-PDF.
  2. rg по паттернам: -----BEGIN (RSA |EC )?PRIVATE KEY-----, AKIA[0-9A-Z]{16}, (?i)(password|secret|hmac_key|api_key)\s*[:=]\s*["\x27][^"\x27$]{8,}, postgres://[^:]+:[^@$]+@ (SEC-001).
  3. Белый список scripts/secrets-allowlist.txt (путь:regexp), каждая запись с комментарием-причиной (SEC-002).
  4. В *.env.example значения пустые или помечены CHANGE_ME (SEC-003).
  5. credentials.json в BeeFiscalApp/ — проверить, что не содержит приватных ключей (SEC-004).
- **Красный фикстур (самотест):** временный файл с "password = \"abcdefgh1\"" → SEC-001.
- **Приёмка:** `bash scripts/tests/test_check-no-secrets.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-no-secrets.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-05 · P0 · `scripts/check-prod-artifact.sh`
- **Язык:** bash · **Нужен для:** ARCH_06, ARCH_08
- **Назначение:** В прод-сборке нет симуляторов, stub-адаптеров и тестовых ключей.
- **Проверки (каждая — отдельный код FAIL):**
  1. Собрать go build ./cmd/fiscal-backend в tmp; strings бинарника (BIN-001: нет «SimulatorCardTerminal», «StubAdapter»).
  2. APP_ENV=prod ALLOW_STUB_ADAPTERS=true ./bin → завершается с ошибкой конфигурации (BIN-002).
  3. Для Android: apk/aab не содержит класса com.beeloy.*.Simulator* (unzip -l | rg) (BIN-003).
  4. dist-cloudflare не содержит sourceMappingURL на *.map с исходниками (BIN-004).
- **Красный фикстур (самотест):** бинарник со stub-строкой → BIN-001.
- **Приёмка:** `bash scripts/tests/test_check-prod-artifact.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-prod-artifact.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-06 · P0 · `scripts/check-migrations.sh`
- **Язык:** bash · **Нужен для:** ARCH_02, ARCH_06, MASTER_PLAN
- **Назначение:** Миграции database/fiscal: нумерация, идемпотентность, откат.
- **Проверки (каждая — отдельный код FAIL):**
  1. Имена NNN_*.sql: номера уникальны в пределах database/fiscal/ (дубликаты 004_*, 012_* допустимы только из allowlist scripts/migrations-allowlist.txt) (MIG-001).
  2. Поднять postgres (docker или PG_URL), применить все миграции дважды подряд — второй прогон без ошибок (MIG-002).
  3. Для новых миграций (git diff base) должен быть файл NNN_*.down.sql или пометка -- IRREVERSIBLE: причина (MIG-003).
  4. После применения: psql -c "\d fiscal.unp_allocations" содержит UNIQUE по (tenant_id, fiscal_device_number, unp_sequence) (MIG-004).
- **Красный фикстур (самотест):** две новые миграции с одинаковым номером → MIG-001.
- **Приёмка:** `bash scripts/tests/test_check-migrations.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-migrations.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-07 · P1 · `scripts/check-ops-docs.sh`
- **Язык:** bash · **Нужен для:** ARCH_06, MASTER_PLAN
- **Назначение:** Эксплуатационные документы полны.
- **Проверки (каждая — отдельный код FAIL):**
  1. Файлы docs/operations-runbook.md, docs/rollback-plan.md, docs/support-guide.md существуют (OPS-001).
  2. В каждом обязательные заголовки (из scripts/ops-docs-required.tsv: файл → список H2) (OPS-002).
  3. Нет текста TODO|TBD|FIXME (OPS-003).
  4. Каждая ссылка [..](path) на локальный файл разрешается (OPS-004).
- **Красный фикстур (самотест):** удалить H2 «Откат миграции» → OPS-002.
- **Приёмка:** `bash scripts/tests/test_check-ops-docs.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-ops-docs.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-08 · P1 · `scripts/check-vendor-docs.sh`
- **Язык:** bash · **Нужен для:** ARCH_03, ARCH_09, VG-19
- **Назначение:** SHA256-манифест вендорных документов.
- **Проверки (каждая — отдельный код FAIL):**
  1. docs/vendor/manifest.json (jq): массив {id,file,sha256,source_url,version,license_note} (VND-001).
  2. Для файла в репозитории sha256sum равен значению (VND-002); отсутствующие файлы допустимы только с "external":true (VND-003).
  3. Каждый DOC-V* из ARCH_09 §1.2 присутствует в манифесте со status ∈ {received,missing} (VND-004).
- **Красный фикстур (самотест):** изменить 1 байт PDF → VND-002.
- **Приёмка:** `bash scripts/tests/test_check-vendor-docs.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-vendor-docs.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-09 · P0 · `scripts/check-golden-vectors.sh`
- **Язык:** bash · **Нужен для:** ARCH_09 VG-20, MASTER_PLAN
- **Назначение:** Golden-векторы вендора воспроизводятся тестами.
- **Проверки (каждая — отдельный код FAIL):**
  1. jq длина docs/MVP1/vendor/datecs_golden_vectors.json == N (GLD-001).
  2. Запустить Kotlin host-тест и C++ тест; в их выводе число строк «VECTOR OK <id>» равно N для каждого (GLD-002).
  3. Пересчёт BCC в самом скрипте (python3) для каждого кадра совпадает с JSON (GLD-003).
- **Красный фикстур (самотест):** удалить вектор из теста → GLD-002.
- **Приёмка:** `bash scripts/tests/test_check-golden-vectors.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-golden-vectors.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-10 · P1 · `scripts/check-datecs-error-catalog.sh`
- **Язык:** bash · **Нужен для:** ARCH_03, ARCH_09 VG-02
- **Назначение:** Каталог ошибок Datecs §7 полон.
- **Проверки (каждая — отдельный код FAIL):**
  1. scripts/gen-datecs-error-catalog.py извлекает коды из docs/vendor/text/datecs_pm.txt в docs/MVP1/vendor/datecs_errors.json (ERR-001).
  2. Kotlin `DatecsErrorCatalog` содержит все коды из JSON, каждому сопоставлена категория RETRYABLE|FATAL|UNKNOWN_OUTCOME (ERR-002, сверка rg).
  3. Нет кода без категории (ERR-003).
- **Красный фикстур (самотест):** убрать код -111015 из Kotlin → ERR-002.
- **Приёмка:** `bash scripts/tests/test_check-datecs-error-catalog.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-datecs-error-catalog.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-11 · P1 · `scripts/gen-datecs-error-catalog.py`
- **Язык:** python3 · **Нужен для:** VG-02
- **Назначение:** Генератор каталога ошибок из текста PM.
- **Проверки (каждая — отдельный код FAIL):**
  1. Вход: путь к datecs_pm.txt (argv[1]); выход: JSON в stdout.
  2. Регэксп ^\s*(-\d{6})\s+(.+)$ внутри раздела 7; дубликаты кодов → ошибка.
  3. Детерминизм: сортировка по коду, ensure_ascii=False.
- **Красный фикстур (самотест):** файл без раздела 7 → exit 2.
- **Приёмка:** `bash scripts/tests/test_gen-datecs-error-catalog.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "gen-datecs-error-catalog.py" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-12 · P1 · `scripts/check-nap-baseline.sh`
- **Язык:** bash · **Нужен для:** ARCH_10, DECISIONS, NAP-N0
- **Назначение:** Заморозка нормативной базы НАП.
- **Проверки (каждая — отдельный код FAIL):**
  1. docs/MVP1/nap_baseline.json: список {id,title,url,retrieved,sha256_text} (NAP-001).
  2. Все ссылки в ARCH_08/ARCH_10 вида «ДВ NN/YYYY» присутствуют в baseline (NAP-002).
  3. Возраст retrieved ≤ 30 дней перед HIL (параметр --max-age-days) (NAP-003).
  4. Каждое правило NORM_PENDING_<id> в коде (rg) присутствует в DECISIONS.md §3 (NAP-004).
- **Красный фикстур (самотест):** удалить запись ДВ 82/2026 → NAP-002.
- **Приёмка:** `bash scripts/tests/test_check-nap-baseline.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-nap-baseline.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-13 · P1 · `scripts/check-norm-decisions.sh`
- **Язык:** bash · **Нужен для:** ARCH_06
- **Назначение:** Каждый NORM-xx имеет временное правило или ответ.
- **Проверки (каждая — отдельный код FAIL):**
  1. Из ARCH_06/08/10 собрать NORM-NN.
  2. Каждый присутствует в DECISIONS.md §3 (NRM-001).
  3. Для NORM со статусом ANSWERED — ссылка на файл ответа существует (NRM-002).
- **Красный фикстур (самотест):** добавить NORM-17 без правила → NRM-001.
- **Приёмка:** `bash scripts/tests/test_check-norm-decisions.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-norm-decisions.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-14 · P1 · `scripts/check-decisions.sh`
- **Язык:** bash · **Нужен для:** DECISIONS
- **Назначение:** D-1…D-11 заполнены.
- **Проверки (каждая — отдельный код FAIL):**
  1. Таблица §1 DECISIONS.md содержит ровно 11 строк D-1…D-11 (DEC-001).
  2. В колонке DEFAULT нет пустых значений и слов TBD (DEC-002).
  3. Каждое D-n из MASTER_PLAN присутствует (DEC-003).
- **Красный фикстур (самотест):** удалить строку D-7 → DEC-001.
- **Приёмка:** `bash scripts/tests/test_check-decisions.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-decisions.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-15 · P1 · `scripts/check-adr-links.sh`
- **Язык:** bash · **Нужен для:** ARCH_04
- **Назначение:** ADR-ссылки живые.
- **Проверки (каждая — отдельный код FAIL):**
  1. Все «ADR-[A-Z0-9-]+» из docs/MVP1 имеют файл docs/decisions/ADR-*.md или запись в плане DOC-002 (ADR-001).
  2. Каждый ADR-файл содержит разделы Status, Context, Decision, Consequences (ADR-002).
- **Красный фикстур (самотест):** ссылка на несуществующий ADR → ADR-001.
- **Приёмка:** `bash scripts/tests/test_check-adr-links.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-adr-links.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-16 · P1 · `scripts/check-doc-drift.sh`
- **Язык:** bash · **Нужен для:** ARCH_04, ARCH_05, MASTER_PLAN
- **Назначение:** Документы не расходятся с кодом по числам/путям.
- **Проверки (каждая — отдельный код FAIL):**
  1. Для каждого пути файла в обратных кавычках в docs/MVP1/*.md с расширением .go|.kt|.cpp|.ts|.sql — файл существует (DRF-001; исключение: помечено «(создать)»).
  2. Для «file:line» — строка существует в файле (DRF-002, допускается ±5 строк с предупреждением).
  3. Числа эндпоинтов в docs/README.md равны числу операций в OpenAPI (DRF-003).
- **Красный фикстур (самотест):** путь к удалённому файлу → DRF-001.
- **Приёмка:** `bash scripts/tests/test_check-doc-drift.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-doc-drift.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-17 · P1 · `scripts/check-doc-invariants.sh`
- **Язык:** bash · **Нужен для:** ARCH_04, MASTER_PLAN
- **Назначение:** Инварианты документации.
- **Проверки (каждая — отдельный код FAIL):**
  1. Каждый ARCH_*.md начинается с H1 и содержит строку «Дата:» (INV-001).
  2. status.json schema == 2 (INV-002).
  3. Нет двух файлов с одинаковым H1 (INV-003).
- **Красный фикстур (самотест):** убрать H1 → INV-001.
- **Приёмка:** `bash scripts/tests/test_check-doc-invariants.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-doc-invariants.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-18 · P1 · `scripts/check-lint-rules.sh`
- **Язык:** bash · **Нужен для:** ARCH_04
- **Назначение:** Пользовательские правила линтера (не покрываются golangci).
- **Проверки (каждая — отдельный код FAIL):**
  1. rg: panic( в internal/ вне _test.go и вне allowlist (LNT-001).
  2. rg: time.Now() в internal/domain вне clock.go (LNT-002) — время только через Clock.
  3. rg: float64 в файлах с money|amount|cents (LNT-003).
  4. Kotlin: GlobalScope.launch в bluecash-app (LNT-004).
- **Красный фикстур (самотест):** добавить time.Now() в service.go → LNT-002.
- **Приёмка:** `bash scripts/tests/test_check-lint-rules.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-lint-rules.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-19 · P1 · `scripts/check-swallowed-errors.sh`
- **Язык:** bash · **Нужен для:** ARCH_04
- **Назначение:** Не глотать ошибки.
- **Проверки (каждая — отдельный код FAIL):**
  1. rg "_ = \w+\.(Close|Save|Persist|Publish)" в Go вне тестов (SWL-001).
  2. rg "catch \(.*\) \{\s*\}" в Kotlin/TS (SWL-002).
  3. Белый список — scripts/swallow-allowlist.txt.
- **Красный фикстур (самотест):** `_ = repo.Save(x)` → SWL-001.
- **Приёмка:** `bash scripts/tests/test_check-swallowed-errors.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-swallowed-errors.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-20 · P1 · `scripts/check-rounding-golden.sh`
- **Язык:** bash · **Нужен для:** ARCH_04, D-4
- **Назначение:** Эталоны округления.
- **Проверки (каждая — отдельный код FAIL):**
  1. Читает docs/MVP1/fixtures/rounding_golden.json (cases: unit_cents, qty_milli, expect_cents).
  2. Запускает Go `go test ./internal/domain -run RoundingGolden` и Kotlin `--tests *RoundingGolden*`; оба выводят «ROUND OK <n>» (RND-001).
  3. n равно числу кейсов (RND-002).
- **Красный фикстур (самотест):** изменить expect_cents → RND-001.
- **Приёмка:** `bash scripts/tests/test_check-rounding-golden.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-rounding-golden.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-21 · P1 · `scripts/check-storno-coverage.sh`
- **Язык:** bash · **Нужен для:** ARCH_02
- **Назначение:** Покрытие сторно по матрице причин.
- **Проверки (каждая — отдельный код FAIL):**
  1. docs/MVP1/fixtures/storno_matrix.json: reason × payment × period (лев/евро) (STO-001).
  2. Каждой строке соответствует тест с именем из поля test (rg) (STO-002).
- **Красный фикстур (самотест):** убрать тест → STO-002.
- **Приёмка:** `bash scripts/tests/test_check-storno-coverage.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-storno-coverage.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-22 · P1 · `scripts/check-db-schema.sh`
- **Язык:** bash · **Нужен для:** ARCH_02 TG-001/002
- **Назначение:** Схема БД после миграций.
- **Проверки (каждая — отдельный код FAIL):**
  1. Поднять БД (как в check-migrations).
  2. psql -At: список индексов fiscal.unp_allocations содержит UNIQUE на (tenant_id, fiscal_device_number, unp_sequence) и UNIQUE(tenant_id, allocation_id) (DBS-001).
  3. RLS включён на таблицах из scripts/rls-tables.txt (DBS-002).
- **Красный фикстур (самотест):** DROP INDEX → DBS-001.
- **Приёмка:** `bash scripts/tests/test_check-db-schema.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-db-schema.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-23 · P1 · `scripts/check-mqtt-tls.sh`
- **Язык:** bash · **Нужен для:** ARCH_02
- **Назначение:** MQTT только по TLS в prod.
- **Проверки (каждая — отдельный код FAIL):**
  1. rg "tcp://|mqtt://" в конфигурациях prod (compose.prod.yml, deploy/) (TLS-001).
  2. Конфиг EMQX требует client cert (TLS-002).
- **Красный фикстур (самотест):** tcp://emqx:1883 в compose.prod.yml → TLS-001.
- **Приёмка:** `bash scripts/tests/test_check-mqtt-tls.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-mqtt-tls.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-24 · P1 · `scripts/check-rabbit-topology.sh`
- **Язык:** bash · **Нужен для:** ARCH_01, ARCH_05
- **Назначение:** Топология RabbitMQ соответствует коду.
- **Проверки (каждая — отдельный код FAIL):**
  1. Извлечь имена exchange/queue/routing key из POS/pos-gateway (rg) и из deploy/ definitions (RBT-001).
  2. Каждая очередь результата имеет DLQ (RBT-002).
  3. Расхождение имён → FAIL.
- **Красный фикстур (самотест):** переименовать очередь в definitions → RBT-001.
- **Приёмка:** `bash scripts/tests/test_check-rabbit-topology.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-rabbit-topology.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-25 · P1 · `scripts/check-data-location.sh`
- **Язык:** bash · **Нужен для:** ARCH_08
- **Назначение:** Данные ФУ/чеков не покидают ЕС.
- **Проверки (каждая — отдельный код FAIL):**
  1. Список регионов/доменов в deploy/ и compose*.yml (DLC-001).
  2. Все внешние хосты из scripts/data-location-allowlist.txt (DLC-002).
- **Красный фикстур (самотест):** внешний хост вне allowlist → DLC-002.
- **Приёмка:** `bash scripts/tests/test_check-data-location.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-data-location.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-26 · P1 · `scripts/check-version-bump.sh`
- **Язык:** bash · **Нужен для:** ARCH_08, NAP-22
- **Назначение:** Версия СУПТО меняется осознанно.
- **Проверки (каждая — отдельный код FAIL):**
  1. Файл с версией (fiscal-backend/internal/version/version.go) сравнить с base ветки (VER-001).
  2. Если изменились файлы из scripts/supto-sensitive-paths.txt, версия обязана измениться или в коммите trailer `Supto-Version-Unchanged: <причина>` (VER-002).
- **Красный фикстур (самотест):** изменение exports.go без bump → VER-002.
- **Приёмка:** `bash scripts/tests/test_check-version-bump.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "check-version-bump.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-27 · P1 · `scripts/e2e-trace-check.sh`
- **Язык:** bash · **Нужен для:** ARCH_05
- **Назначение:** Сквозная трассировка идентификаторов POS→gateway→backend→ФУ.
- **Проверки (каждая — отдельный код FAIL):**
  1. Запустить compose-e2e с correlation_id=trace-$RANDOM.
  2. Лог каждого сервиса содержит correlation_id (TRC-001).
  3. Порядок событий: pos.sale.created < gateway.published < backend.reserved < device.executed < backend.finalized (TRC-002).
- **Красный фикстур (самотест):** потеря correlation_id в gateway → TRC-001.
- **Приёмка:** `bash scripts/tests/test_e2e-trace-check.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "e2e-trace-check.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-28 · P1 · `scripts/gen-annex29-columns.py`
- **Язык:** python3 · **Нужен для:** ARCH_06
- **Назначение:** Генерация таблицы колонок Приложения 29.
- **Проверки (каждая — отдельный код FAIL):**
  1. Вход: docs/SUPTO/annex29_columns.csv; выход: Go-константы fiscal-backend/internal/domain/annex29_columns_gen.go.
  2. Детерминированный вывод; --check сравнивает с существующим файлом (exit 1 при расхождении).
- **Красный фикстур (самотест):** изменить CSV без регенерации → --check exit 1.
- **Приёмка:** `bash scripts/tests/test_gen-annex29-columns.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "gen-annex29-columns.py" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-29 · P1 · `scripts/verify_trace_tests_exist.rb`
- **Язык:** ruby · **Нужен для:** ARCH_06, ARCH_08
- **Назначение:** Трассировка требований НАП → тесты.
- **Проверки (каждая — отдельный код FAIL):**
  1. docs/SUPTO/traceability.csv: requirement, test_symbol, file.
  2. Каждый test_symbol найден rg в file (TRT-001).
  3. Каждое требование Приложения 29 из annex29_columns имеет ≥ 1 строку (TRT-002).
- **Красный фикстур (самотест):** удалить тест → TRT-001.
- **Приёмка:** `bash scripts/tests/test_verify_trace_tests_exist.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "verify_trace_tests_exist.rb" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-30 · P0 · `scripts/hil/run.sh`
- **Язык:** bash · **Нужен для:** ARCH_02, ARCH_11, MASTER_PLAN
- **Назначение:** Единая точка запуска HIL-сценариев.
- **Проверки (каждая — отдельный код FAIL):**
  1. Аргументы: --device bc50|daisy|tremol --scenario HIL-BC-01..N --out dir.
  2. Предусловия: env HIL_DEVICE_HOST, HIL_CONFIRM=yes (иначе exit 3), проверка NAP-N0 свежести.
  3. Каждый сценарий — файл scripts/hil/scenarios/<id>.sh с контрактом: печатает JSON-строки {step,expect,actual,ok}; run.sh собирает out/evidence.json и вызывает scripts/verify_hil_evidence (существующий) (HIL-001).
  4. Сырые кадры сохраняются в out/frames.hex (HIL-002).
- **Красный фикстур (самотест):** отсутствие HIL_CONFIRM → exit 3.
- **Приёмка:** `bash scripts/tests/test_run.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "run.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-31 · P1 · `scripts/loadtest/run.sh`
- **Язык:** bash · **Нужен для:** ARCH_02 TG-016
- **Назначение:** Нагрузочный тест.
- **Проверки (каждая — отдельный код FAIL):**
  1. Параметры --rps, --duration, --tenants; инструмент vegeta (или go test -bench).
  2. Сценарии: sale create→pay→finalize.
  3. Выход: out/summary.json {p50,p95,p99,errors,conflicts}; пороги из scripts/loadtest/thresholds.json (LDT-001).
- **Красный фикстур (самотест):** порог p95 нарушен → exit 1.
- **Приёмка:** `bash scripts/tests/test_run.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "run.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-32 · P1 · `scripts/mutate-vector.sh`
- **Язык:** bash · **Нужен для:** ARCH_03 TST-VENDOR
- **Назначение:** Мутационная проверка golden-векторов.
- **Проверки (каждая — отдельный код FAIL):**
  1. Для каждого вектора мутировать 1 байт (3 позиции: LEN, DATA, BCC) и запустить декодер; ожидается отказ (MUT-001).
  2. Сводка: «mutants killed N/N».
- **Красный фикстур (самотест):** декодер принимает мутант → MUT-001.
- **Приёмка:** `bash scripts/tests/test_mutate-vector.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "mutate-vector.sh" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-33 · P1 · `make lint`
- **Язык:** make · **Нужен для:** ARCH_04, ARCH_11
- **Назначение:** Единый линт.
- **Проверки (каждая — отдельный код FAIL):**
  1. Цель lint: golangci-lint run ./... (fiscal-backend, minipos/beeminipos-backend, POS/pos-gateway), ktlint для bluecash-app, tsc --noEmit + eslint для TS-приложений, scripts/check-lint-rules.sh, scripts/check-swallowed-errors.sh.
  2. Каждый инструмент вызывается через ${TOOL:-default}; отсутствие инструмента = FAIL (не SKIP).
- **Красный фикстур (самотест):** добавить panic( → lint падает.
- **Приёмка:** `bash scripts/tests/test_make_lint.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "make lint" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-34 · P0 · `make plan-check`
- **Язык:** make · **Нужен для:** ARCH_07, ARCH_09, ARCH_11
- **Назначение:** Агрегатор проверок документов.
- **Проверки (каждая — отдельный код FAIL):**
  1. Цель plan-check: check_plan_coverage.rb, check_plan_acceptance.rb, check-test-matrix.sh, check-vendor-docs.sh, check-golden-vectors.sh, check-decisions.sh, check-norm-decisions.sh, check-doc-invariants.sh, check-adr-links.sh, check-doc-drift.sh.
  2. Останавливается на первой ошибке; печатает итоговую строку «PLAN-CHECK OK».
- **Красный фикстур (самотест):** любая красная под-проверка → make exit 2.
- **Приёмка:** `bash scripts/tests/test_make_plan-check.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "make plan-check" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-35 · P1 · `make fuzz-short`
- **Язык:** make · **Нужен для:** ARCH_03
- **Назначение:** Короткий фаззинг кодека.
- **Проверки (каждая — отдельный код FAIL):**
  1. go test -fuzz=FuzzDecodeFrame -fuzztime=30s ./internal/... (Go-декодеры); Kotlin: jqwik/property-тест 10k итераций (FZZ-001).
  2. Корпус — golden-векторы + мутанты; падение = сохранить crasher в testdata/.
- **Красный фикстур (самотест):** паника декодера → exit 1.
- **Приёмка:** `bash scripts/tests/test_make_fuzz-short.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "make fuzz-short" Makefile scripts/README.md`); задача отмечена DONE в status.json.

### SCR-36 · P1 · `make contract-lock`
- **Язык:** make · **Нужен для:** ARCH_09
- **Назначение:** Алиас на существующий verify-contract-lock.
- **Проверки (каждая — отдельный код FAIL):**
  1. Цель contract-lock: ./scripts/verify-contract-lock.sh (уже существует). Задача сводится к добавлению цели в .PHONY.
- **Красный фикстур (самотест):** make contract-lock без цели → "No rule".
- **Приёмка:** `bash scripts/tests/test_make_contract-lock.sh` → exit 0.
- **DoD:** файл создан; самотест зелёный; цель/скрипт подключены (`rg -n "make contract-lock" Makefile scripts/README.md`); задача отмечена DONE в status.json.

