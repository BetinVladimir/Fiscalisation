# ARCH_06 — Готовность к СУПТО / НАП (Наредба Н-18, Приложение № 29) и эксплуатация

Источники в репозитории: `docs/SUPTO/Приложение № 29.md` (24 пункта), `docs/SUPTO/Naredba N-18 2006*.md`, `contracts/supto-annex29-trace.json` (статусы по пунктам), `contracts/supto-feature-scope.json`, `MVP_GATES.md`, `MVP_BLOCKERS.md`, `docs/remediation/2026-10-17/{09_SUPTO_NAP_PREPARATION,16_NAP_OFFICIAL_PROCEDURE_RECHECK,49_NAP_PRIMARY_REQUIREMENTS_RECHECK}.md`.
**Подробное постатейное исследование по официальному тексту Н-18 — в [ARCH_08](ARCH_08_SUPTO_NAP_COMPLIANCE_STUDY.md) (приоритет над этим файлом при расхождениях).** **Это инженерная трассировка, не юридическое заключение.** Все нормативные толкования, помеченные TO VERIFY, должен подтвердить юрист/представитель НАП. CDOD — [ARCH_00_INDEX.md](ARCH_00_INDEX.md).

## 1. Текущий статус по `supto-annex29-trace.json` (на момент анализа)
Итого 24 пункта: **PASS 2** (п.11, п.14), **NOT_APPLICABLE 1** (п.3), **FAIL 1** (п.24 электронные чеки), **PARTIAL 20**. Во всех PARTIAL `production_blocked=true`.

Привязка пунктов к найденным дефектам и задачам (то, что нужно закрыть до заявления о готовности):

| Пункт Приложения 29 | Статус trace | Что мешает (наш анализ) | Наши ID | Автоматическая приёмка |
|---|---|---|---|---|
| 1 Интерфейс на болгарском | PARTIAL | Внешняя проверка; в POS-приложении тексты ошибок/статусов — проверить на языковую полноту | TST-E2E-*, ARCH_03 §7 | `node scripts/check-i18n-bg.mjs` (создать): все ключи UI имеют `bg`, нет латинских служебных строк кроме разрешённых |
| 2 Полнота и целостность данных | PARTIAL | снимок состояния без транзакций; парсинг денег | ARCH-001, TG-001, BUG-001/002 | `go test ./internal/domain -run RepositoryContract -race` |
| 4 Защита от модификации | PARTIAL | секреты в git, небезопасные дефолты, нет подписи релиза | BUG-009/011/060, TG-012 | `scripts/check-no-secrets.sh`, `go test ./internal/config` |
| 5 Время и синхронизация с ФУ | PARTIAL | синхронизация фиктивна при MQTT; UTC вместо Europe/Sofia; S3 время 0 | BUG-005, BUG-041, TG-003 | `go test -run 'ClockSync|DailyClock'`, `make iot-test` |
| 6 Данные операторов | PARTIAL | — | существующие тесты `admin_test.go` | `go test ./internal/domain -run Operator` |
| 7 Аутентификация | PARTIAL | пароль оператора ФУ в EditText; HMAC рядом с OIDC | BUG-011, BUG-032 | `go test ./internal/auth`; Robolectric тест |
| 8 Связь и готовность ФУ | PARTIAL | `/readyz` без проверок; транспорт без таймаутов | BUG-010, BUG-029, TG-004 | `go test ./internal/api -run Readyz`; Kotlin Datecs тесты |
| 9, 9.1 УНП при первом товаре | PARTIAL | дыры в нумерации; поток шлюза финализирует в конце | TG-002, BUG-040, POS-11 | `go test -run UNP`; `POS_SALE_FLOWS.md` без пустых ячеек |
| 10 Платёж и команда ФУ | PARTIAL | карта сторнируется при UNKNOWN; платежи теряются в шлюзе | BUG-021, BUG-055 | gradle-тест `saleDoesNotReverseCardWhenFiscalResultUnknown`; Go-тест шлюза |
| 12 Аннулирование незавершённой продажи | PARTIAL | чек остаётся открытым на ФУ | BUG-020, TG-005 | gradle-тест `saleCancelsOpenReceipt*` |
| 13 Защита завершённых продаж и сторно | PARTIAL | время исходного документа; политика сторно | BUG-008, TG-009 | `go test -run ReversalPolicy` |
| 15 Журнал | PARTIAL | ACK без проверки хэша; журнал/агрегат | BUG-030, ARCH-008 | gradle-тест `acknowledgeWithWrongHashRejected` |
| 16, 17 Просмотр, доступ к архиву | PARTIAL | персистентность; нет метрик/алертов | TG-001, TG-011, TG-015 | `scripts/check-migrations.sh` |
| 18 (18.1–18.9) Экспорт | PARTIAL | нет автоматической сверки колонок с Приложением | TST-NAP-01 ниже | `go test -run TestExportColumnsMatchAnnex29` |
| 19 Профиль аудитора | PARTIAL | роли/HMAC | BUG-011 | `go test ./internal/auth -run Auditor` |
| 20 Запрет тест/обучающего режима | PARTIAL | симулятор и stub-адаптеры включены по умолчанию; `Simulated:true`; `AUTHORIZED_TEST` fail-open | BUG-009, BUG-003, BUG-058, POS-07 | `go test ./internal/config -run 'Defaults|ProdChecks'`; `scripts/check-prod-artifact.sh` |
| 21 Интегрированная система | PARTIAL | нет единой схемы контрактов | POS-01, TG-013 | `make contract-test` |
| 22 Импорт продаж | PARTIAL | edge-sync; шлюз | POS-03, TST-INT | E2E TST-E2E-07 |
| 23 Экспорт в другие системы | PARTIAL | webhook/outbox | TG-011 | `go test ./internal/webhook` |
| 24 Электронные фискальные чеки | **FAIL** | нет сквозной реализации | TG-008 | `go test -run ElectronicReceipt`; `TST-E2E-16` |

> Пункт 20 особенно важен: симулятор, `Simulated:true`, режим `AUTHORIZED_TEST` и stub-адаптеры по умолчанию — это *возможность работы в тестовом режиме* в поставляемом продукте. Требуется артефакт PROD-сборки без этих путей.

### TST-NAP-01 · Сверка колонок экспорта с Приложением 29 (создать)
- **Файлы:** `fiscal-backend/internal/domain/exports_annex29_test.go`, `fiscal-backend/internal/domain/testdata/annex29_columns.json` (генерируется скриптом `scripts/gen-annex29-columns.py` из `docs/SUPTO/Приложение № 29.md`, таблицы 18.1–18.5, 18.9).
- **Шаги:** (1) скрипт извлекает порядок и названия колонок, типы, обязательность; (2) тест создаёт экспорт на тестовых данных и сравнивает заголовки/порядок/форматы; (3) разница — падение теста.
- **Приёмка:** `python3 scripts/gen-annex29-columns.py --check` (вывод совпадает с committed JSON) и `go test ./internal/domain -run TestExportColumnsMatchAnnex29 -count=1` → ok.
- **DoD:** CDOD; `supto-annex29-trace.json` п.18 получает ссылку на тест.

### TST-NAP-02 · Артефакт PROD без тест-режима (создать)
- **Файлы:** `scripts/check-prod-artifact.sh`.
- **Шаги:** собрать PROD-образ/бинарь; проверить: `APP_ENV=prod` по умолчанию в образе; в бинаре нет флага `ALLOW_STUB_ADAPTERS=true` по умолчанию; в APK/AAB BlueCash и pos-app нет `AUTHORIZED_TEST`, `training`, `simulator` (grep по строкам, список исключений в файле с обоснованиями); `go test ./internal/config -run TestProdRejectsStubs`.
- **Приёмка:** скрипт exit 0; при добавлении строки `simulator` в тестовую сборку — exit ≠ 0 (проверка мутацией в CI).

## 2. Устройства и формальные исключения (`MVP_GATES.md`)
| Устройство | Дисположение | Следствие для MVP-теста |
|---|---|---|
| Daisy SMART S | `STUB_ONLY`, отключено в PROD | вне MVP |
| Daisy Compact S 01 USB | `DOCUMENTED_NOT_APPROVED` | вне MVP-HIL до электрической проверки |
| DP-150 MX | `DOCUMENTED_NOT_APPROVED` | HIL допустим только после проверки уровней/питания и подтверждения прошивки EUR |
| BlueCash-50 фискальная/платёжная интеграция | **`EXCLUDED_FROM_PROD`** | противоречит цели «тест на реальных устройствах через BlueCash» — требуется явное решение: тест допускается как *лабораторный* без заявления готовности к PROD |
| BluePad-50 Plus | optional/UNSUPPORTED | вне MVP |
| НАП/юридическая сертификация | release blocker | не доказывается автотестами |
- **Задача GATE-01 (решение владельца):** зафиксировать в `MVP_GATES.md` режим первого HIL («лабораторный, не боевой») и условия перехода `EXCLUDED_FROM_PROD → SUPPORTED` (письменное разрешение вендора/эквайера, тестовые учётные данные, сертификат).
- **Приёмка:** `ruby scripts/verify_readiness_documentation.rb` → exit 0; в `MVP_GATES.md` есть строка «First HIL mode» с датой решения (`grep -c 'First HIL mode' MVP_GATES.md` ≥ 1).

## 3. Нормативные вопросы (TO VERIFY — юрист / НАП)
Для каждого вопроса — владелец и срок; без ответа соответствующая задача остаётся `BLOCKED_SPEC` (как в плане 70, §18).
| ID | Вопрос | Затронутые задачи |
|---|---|---|
| NORM-01 | Применимость электронных чеков (п.24) для MVP и формат QR/УНП | TG-008 |
| NORM-02 | Момент «ввода информации о продаже» для POS-сценариев (п.9.1) — розница, стол, онлайн | POS-11, TG-002 |
| NORM-03 | Допустимость дыр в нумерации УНП и их документирование | TG-002 |
| NORM-04 | Правила сторно: сроки, основания, частичное, способ возврата денег при оплате картой | TG-009, POS-13 |
| NORM-05 | Требования к ежедневной синхронизации времени (способ и допуск дрейфа) | TG-003 |
| NORM-06 | Хранение журнала/архива: срок, место (резидентность), неизменяемость | TG-001, TG-015 |
| NORM-07 | Процедура регистрации СУПТО/декларации производителя и перечень документов | ARCH_06 §5 |
| NORM-08 | Режим «лабораторного» тестирования на реальных ФУ с точки зрения требований к кассовым операциям | GATE-01 |
- **Приёмка:** `docs/MVP1/NORM_DECISIONS.md` (создать) содержит таблицу с колонками `ID | вопрос | ответ | источник | дата | владелец`; скрипт `scripts/check-norm-decisions.sh` падает, если у блокирующей задачи (список в скрипте) нет строки с непустым `источник`.

## 4. Эксплуатационные процедуры (нужны до пилота)
Создать документы (все — `docs/MVP1/ops/…`), каждый с проверяемым содержимым:
| Документ | Содержание (обязательные разделы) | Автопроверка |
|---|---|---|
| `OPS_UNKNOWN_OUTCOME.md` | что видит кассир при UNKNOWN; запрет повторной оплаты; шаги сверки (статус ФУ, журнал, EJ), кто решает; когда допустим возврат карты | разделы «Симптом/Действия кассира/Действия поддержки/Эскалация» (`grep -c '^## '` = 4) |
| `OPS_START_OF_DAY.md` | синхронизация часов, проверка ФУ, X-отчёт, готовность связи с НАП | чек-лист ≥ 8 пунктов (`grep -c '^- \[ \]'`) |
| `OPS_END_OF_DAY.md` | Z-отчёт, отправка/проверка данных, архив журнала | то же |
| `OPS_POWER_LOSS.md` | действия при потере питания/связи в середине продажи (BUG-020) | разделы по сценариям |
| `OPS_ROLLBACK.md` | возврат на предыдущую версию без потери журнала (см. `docs/rollback-plan.md`) | ссылки на команды существуют |
| `OPS_MONITORING.md` | метрики и алерты (TG-011): UNKNOWN > N, нет sync часов, задержка MQTT, очередь outbox | каждая метрика есть в `/metrics` (скрипт сверяет имена) |
| `PILOT_PLAN.md` | 1–2 точки, период, критерии остановки, ответственные, обучение кассиров | поля заполнены (нет `TBD`) |
- **Приёмка:** `scripts/check-ops-docs.sh` (создать) проверяет наличие файлов, разделов и отсутствие строк `TBD`/`TODO` → exit 0.

## 5. Процедура регистрации и доказательства (после исправления P0)
1. Заполнить `docs/EXTERNAL_LEGAL_ACTIVATION_EVIDENCE.md` по фактам (не по ожиданиям); отдельные строки для: декларация/сертификаты (если применимо — NORM-07), регистрация ФУ, договор с эквайером, тестовая среда НАП.
2. HIL-протоколы (`artifacts/hil/…/result.json`) подписываются ответственным и прикладываются к `HIL_EVIDENCE_VERIFICATION.md` (проверить существующий формат: `docs/HIL_EVIDENCE_VERIFICATION.md`).
3. Обновить `contracts/supto-annex29-trace.json` статусами с ссылками на реальные тесты; только затем менять `production_blocked`.
- **Приёмка:** `ruby scripts/verify_supto_trace.rb` → exit 0; дополнительная проверка (создать `scripts/verify_trace_tests_exist.rb`): каждый путь теста из trace существует, а имя теста находится `grep -r`; пункты со статусом `PASS`/`SOFTWARE_VERIFIED_*` имеют ≥ 1 тест.

## 6. Критерии «готово к тестированию с реальными устройствами» (сводка)
- [ ] Закрыты все P0 из ARCH_01 (BUG-001, 003, 005, 009, 020, 021, 022, 026, 028, 029, 032, 041, 050, 051, 053, 055, 060) — `scripts/check-test-matrix.sh --severity P0` → exit 0.
- [ ] TG-001, TG-002, TG-003, TG-004, TG-005, TG-006, TG-007 закрыты.
- [ ] Контракт-lock зелёный (DOC-003), `make contract-test` ok.
- [ ] E2E TST-E2E-01…06, 08, 12, 17, 20 зелёные в CI на симуляторе.
- [ ] GATE-01 принят; NORM-01…05 имеют ответы либо задачи помечены `BLOCKED_SPEC` и вынесены из MVP.
- [ ] HIL_RUNBOOK и ops-документы готовы (TG-014, §4).
- [ ] `ruby scripts/check_plan_coverage.rb` → `unmapped: 0` (ARCH_07 R-01).

> Уточнение (ARCH_08): Приложение № 43 (процедура чтения КЛЕН) — требование к документации производителя ФУ (чл. 8(6) т.5), а не к СУПТО; типы экспорта `KLEN`/`FISCAL_MEMORY` в нашем продукте — сервисные отчёты, а не нормативный экспорт.
