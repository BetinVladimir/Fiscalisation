# DECISIONS — решения владельца D-1…D-11 и статус внешних документов

> **2026-10-23 владелец подтвердил/изменил решения — см. §5 (приоритетнее колонки DEFAULT).**
> Статус документа: **DEFAULT = рекомендация архитектора, действует, пока владелец не подтвердил или не изменил.**
> Агент исправлений НЕ блокируется на DEFAULT: берёт значение из колонки «DEFAULT» и помечает коммит `decision: D-n DEFAULT`.
> Подтверждение владельца — запись в колонке «Подтверждено» (дата, кто). Если владелец меняет решение — пересчитать «Затрагивает».
> Отчёты юриста NORM-01…16 сюда не входят (см. §3): по ним принимаются только консервативные временные правила.

Дата: 2026-10-22 · Автор: IT-архитектор (аудит MVP1) · Проверка: `scripts/check-decisions.sh` (см. SCR-14 в ARCH_12)

## 1. Решения D-1…D-11

| ID | Вопрос | Источник в репозитории / вне | DEFAULT (действует сейчас) | Затрагивает | Подтверждено |
|---|---|---|---|---|---|
| D-1 | Режим первого HIL | `MVP_GATES.md`: BC-50 = `EXCLUDED_FROM_PROD`, Daisy = `STUB_ONLY`, остальные `DOCUMENTED_NOT_APPROVED`; полный протокол есть только для Datecs BC-50 (ARCH_09) | **Лабораторный HIL только на Datecs BlueCash-50**, режим `LAB`; PROD-флаг остаётся `EXCLUDED_FROM_PROD`; Daisy/Tremol — после DOC-V4/V5 | GATE-01, W9, HIL-BC-* | — |
| D-2 | Линия отсечения MVP | ARCH_07 R-05 | **До лабораторного HIL (M0–M3):** все P0 + BUG-001/003/009/060, NAP-N0, VG-01…VG-13, TG-001a(re-scope, см. §2), TG-003, TG-004, TG-005, POS-03/04 (контракт и маппинг), TST-E2E-01…12. **После HIL (M4–M5):** Daisy/Tremol, встроенный пинпад, e-receipt, AdminApp/BeeFiscalApp UX | состав волн 6–8 | — |
| D-3 | Персистентность до лаб. HIL или до пилота | Код: `fiscal-backend/cmd/fiscal-backend/main.go:28-45` — Postgres-путь уже подключён; `fiscal-backend/internal/persistence/postgres.go` — `SaveDeltaVersioned` (generation CAS), типизированные проекции, `AllocateUNP` (advisory lock + UNIQUE) | **К лаб. HIL — НЕ требуется переписывание.** Требуется только TG-001a (проверка, что рестарт/конфликт поколения не теряют операцию) и TG-001b (замер/ограничение O(N) сериализации). Горизонтальное масштабирование (multi-replica) — до пилота (TG-001c) | волна 6 | — |
| D-4 | Округление (ADR-MONEY-ROUNDING-001) | Нормативного текста в репозитории нет; поведение ФУ — Datecs PM §cmd 49 (цена×кол-во округляется ФУ) | **half-up на строку в целых центах** (`round(unit_cents*qty_milli/1000)`), затем сумма строк; документ-уровневая скидка распределяется остатком на последнюю строку; расхождение с чеком ФУ > 0 центов = ошибка `AMOUNT_MISMATCH` (не молчаливая правка). Сверить на HIL-BC-03 | BUG-xxx округления, POS-xx | — |
| D-5 | Судьба `receipt_saga.go` | Проверено `rg`: в не-тестовом коде вне самого файла **не используются**: `ReceiptSaga`, `ReceiptSagaState` (+константы), `ReceiptSagaStore`, `MemoryReceiptSagaStore`, `ReserveReceiptSaga`, `AdvanceReceiptSaga`, `allowedReceiptTransition`, `ReceiptPlanHash`. **Используются** (service.go:983,990; delegated_sale_prepare.go:28,37; edge_sync*.go): `ReceiptFinalizePlan`, `ValidateReceiptPlan`, `receiptMoneyCents`, `supportedPaymentType` | **Разделить файл:** живое (`ReceiptFinalizePlan`, `ValidateReceiptPlan`, `supportedPaymentType`, `receiptMoneyCents`) → новый `receipt_plan.go`; мёртвое (саговый конечный автомат и `MemoryReceiptSagaStore`, `ReceiptPlanHash`) удалить вместе с его тестами. Единственная сага — карточная в BlueCashEngine + `payment_reconciliation.go`. BUG-001 правится в `receipt_plan.go` | BUG-006, BUG-001, CODE-006 | — |
| D-6 | Ответы юриста NORM-01…16 | не найдены | **Не закрывается мной.** Временные консервативные правила — §3 | TG-008, TG-009, NAP-* | — |
| D-7 | Ротация keystore/HMAC | нет политики в `docs/` | **Двухключевое окно:** `kid` в заголовке, активный + предыдущий ключ принимаются 24 ч, ротация ≤ 90 дней, старый ключ удаляется только после `max(exp)` выданных токенов; тест `TestVerifyAcceptsPreviousKidWithinWindow` | BUG-060, BUG-011 | — |
| D-8 | Оплата картой | ARCH_09 VG-14; текущий код: Datecs Pay `purchaseEur` + сага с компенсацией | **Внешний Datecs Pay + сага (как сейчас).** Встроенный пинпад ФУ (cmd 53 Type=1 / cmd 55) — только после DOC-V6, отдельные карточки `BLOCKED_SPEC` | VG-14, HIL карты | — |
| D-9 | Профиль слотов оплаты ФУ | Datecs PM §4.15 cmd 53 + [*13]: PaidMode 0 наличные, 1 кредитная карта, 2 дебитная карта (с пинпадом), 3/4/5 «другие» — заводские имена `ЧЕК`/`ВАУЧЕР`/`КУПОН` (программируются). Код: `BlueCashEngine.kt:397-406` сейчас CHEQUE→2 (это ДЕБ.КАРТА!), NHIF→4, DEFERRED/INTERNAL→5 | **Профиль `BC50_DEFAULT` по спецификации:** CASH→0; CARD (внешний терминал)→1; CHEQUE→3; VOUCHER→4; COUPON→5; **NHIF, DEFERRED, INTERNAL_CONSUMPTION → отказ `PAYMENT_TYPE_UNSUPPORTED`** до чтения программирования слотов (PRB-02). Слот 2 не использовать без пинпада ФУ | VG-05, VG-23, BUG-042 | — |
| D-10 | Режим валюты ФУ | ARCH_10 E1: режимы 0/1/2/3, меняется однократно производителем | **Допустимы 2 и 3** (основная — евро); `0` и `1` → `READY=false`, причина `CURRENCY_MODE_NOT_EURO`. Режим читается при активации и перед каждой сменой | VG-09, NAP-16 | — |
| D-11 | Объём MVP1 по `minipos/`, `AdminApp/`, `BeeFiscalApp/` | см. ARCH_12/13 (статический анализ) | **В MVP1 — `minipos/` (кассовый клиент) и `BeeFiscalApp/` (конфигурация/активация устройства) только в части вызовов fiscal-backend; `AdminApp/` — вне HIL-пути, но входит в статическую проверку безопасности** | SCOPE-01 | — |

## 2. Переопределение TG-001/ARCH-001 (следствие D-3)

Факты кода (все проверяются `rg`/`sed`, ссылки на файлы):
- `fiscal-backend/cmd/fiscal-backend/main.go:28-45`: `repo := NewMemoryRepository()`, но при `cfg.DatabaseURL != ""` выполняются миграции и подставляется `NewPersistentRepository(store)`.
- `internal/config/config.go:51-60`: в `AppEnv=prod` пустой `DatabaseURL`/`RLSDatabaseURL` → ошибка конфигурации. В `dev` MemoryRepository допускается.
- `fiscal-backend/internal/persistence/postgres.go:248-340`: запись = diff снимка → upsert/delete строк + типизированные проекции в одной SERIALIZABLE-транзакции, оптимистичная блокировка по `fiscal_state_meta.generation`; при расхождении `ErrConcurrentState`.
- `fiscal-backend/internal/persistence/postgres.go:59-86`: `AllocateUNP` — `pg_advisory_xact_lock` + `max(unp_sequence)+1` + INSERT с UNIQUE.
- `internal/domain/repository.go:392-440`: `persistLocked` сериализует **весь** `repositorySnapshot` на каждую мутацию, при ошибке `restoreLocked` перечитывает состояние.

Вывод: исходная формулировка TG-001 («снимок вместо транзакций, написать PostgresRepository») **устарела**. Новые задачи (подробно — `execution/decomp/backend-persistence.md`):
- **TG-001a (P0):** доказать рестарт-устойчивость и CAS-конфликт: тесты на реальном Postgres (двойная финализация, kill между `reserve` и `finalize`, конфликт поколения двух реплик).
- **TG-001b (P1):** измерить стоимость `json.Marshal` всего состояния + diff при N=10k/100k продаж; порог — p95 мутации ≤ 50 мс при 10k операций, иначе ограничить диф по затронутым коллекциям.
- **TG-001c (P2, до пилота):** multi-replica: поколение глобальное → любой коммит другой реплики инвалидирует запись; решение — per-tenant generation либо один писатель (leader). Документ ADR-PERSIST-001.
- **TG-001d (P1):** `dev` с пустым `DATABASE_URL` не должен запускаться при `PUBLIC_BASE_URL` не localhost (защита от случайного прод-старта на памяти).

## 3. Ответы юриста NORM-01…16 — временные консервативные правила

Применяются, пока нет письменного ответа; каждое правило помечено в коде константой/флагом `NORM_PENDING_<id>` и перечислено в `scripts/check-nap-baseline.sh`.

| NORM | Предмет | Временное правило (fail-closed) |
|---|---|---|
| NORM-01 | Режим регистрации POS-продаж (TG-008/NAP-13) | Beeloy POS = «тонкий клиент» (ADR-SUPTO-001): фискализацию выполняет только ФУ, Beeloy не формирует собственный фискальный документ |
| NORM-04/10 | Классификация Beeloy POS (СУПТО или нет) | Считать СУПТО; требования Н-18 Прил. 29 выполнять как для СУПТО |
| NORM-13 | Версия СУПТО при евро | Версию не менять без решения; фиксировать в `release-evidence.md` |
| NORM-14 | Чл.118 ЗДДС и e-чек | e-чек не включать в MVP |
| NORM-15 | ДВ 82/2026 | Следовать ARCH_10 E1…E15 |
| NORM-16 | Сторно лв-документов в евро-период | Сторно лв-чека в евро-период — **запрещено автоматом**, только вручную с кодом причины |
| остальные | — | Включают `BLOCKED_NORM` в status.json, агент берёт другие задачи |

## 4. Внешние документы DOC-V* — что закрыто мной

| ID | Статус | Что сделано / что осталось |
|---|---|---|
| DOC-V1 | **ЧАСТИЧНО**, требует владельца | Релиз-ноты BC-50 в публичном доступе не найдены; параметр валюты — есть в Прил.1 Н-18 т.16–20. Нужно письмо Datecs: версия прошивки и «допълнение към свидетелството» БИМ. Запрос оформлен в `vendor/REQUESTS.md` (SCR-15) |
| DOC-V2 | **ЗАКРЫТО ЧАСТИЧНО** | `Datecs PM v2.11.4 §cmd 255` содержит имена `UNP`, `StornoUNP`, `FMnumber`, `nLastFiscalDoc`, `NapBlockDateTime`, `DFR_needed`, `CurrNameLocal/Foreign`, `ExchangeRate`. Применимость к BC-50 **не подтверждена** → задача HIL-probe **PRB-01** (чтение каждого имени на реальном BC-50, запись ответа в `vendor/probe_results.json`) |
| DOC-V3 | **получен 2026-10-23** | DatecspayLowLevel v3 (2025) + Pinpad Commands v1.9 в `docs/vendor/`; разбор `DATECSPAY_PROTOCOL.md`; открыто Q-B1…Q-B3 |
| DOC-V4 | **получен частично 2026-10-23** | Protocol_Smart_Pro_v2_2025.pdf и Protocol_Daisy_Smart_Payment_v1_2026.pdf (`docs/vendor/`); SDK не нужен — протокол JSON/HTTP; открыты вопросы DOC-V4B |
| DOC-V5 | не получен | Свидетельства типа — запросить у Datecs/Tremol |
| DOC-V6 | не получен | Банк/эквайринг — только при выборе встроенного пинпада (D-8) |

## 5. HIL-пробы, вытекающие из DEFAULT

| ID | Что проверить на реальном BC-50 | Артефакт | Критерий |
|---|---|---|---|
| PRB-01 | cmd 255 read для имён из DOC-V2 | `vendor/probe_results.json` | для каждого имени: `supported`/`unsupported` + сырой кадр |
| PRB-02 | Имена платёжных слотов ФУ (программирование) | то же | решение по D-9 |
| PRB-03 | Округление цена×кол-во (3 набора из ARCH_15 §ROUND) | то же | ΔЦентов = 0 |
| PRB-04 | Режим валюты (чтение), D-10 | то же | значение ∈ {0,1,2,3} |

## Дополнение: D-12 (новое)
| D-12 | Порядок при возврате по карте (BUG-071) | DEFAULT: **storno-first** — фискальное сторно подтверждено → возврат карты; Unknown → без возврата, `RECOVERY_REQUIRED`; сбой возврата → повторяемый `REFUND_PENDING`. Альтернатива (refund-first) отвергнута: деньги уходят без фискального документа. |

## Пересмотр по `MVP_GOAL.md` (2026-10-23)
D-1, D-2 и D-11 **переопределены** целью владельца: MVP = шесть путей (P-A BlueCash-50 прямое; P-S Daisy Smart S Android прямое; P-B Datecs через S3+COM; P-C BluePay через S3+BLE; P-D Daisy Compact S 01 через S3+USB; P-E СУПТО), клиенты POS и MiniPOS, транспорт MQTT/BLE. «Только BlueCash-50» и «Daisy после HIL» больше не действуют; MiniPOS — полноценный клиент.

| ID | Вопрос | DEFAULT |
|---|---|---|
| D-13 | BLE для POS (Expo; BLE-кода в приложении нет)? | нет: POS → MQTT через backend; BLE — MiniPOS |
| D-14 | Типы оплаты на пути Daisy | наличные и типы протокола; карта — `PAYMENT_TYPE_UNSUPPORTED` (у Compact S 01 нет терминала) |
| D-15 | BluePad как отдельная привязка, а не часть `DatecsDp150BluePad50` | да (BP-03) |
| D-16 | Две Daisy-интеграции | **Подтверждено владельцем 2026-10-23:** Smart S = Android, прямое подключение (путь P-S, по образцу BC-50); Compact S 01 = ESP32-S3 + USB (путь P-D) |
| D-17 | Транспорт Smart S | **Подтверждено владельцем 2026-10-23:** Android-приложение, команды POS/MiniPOS по MQTT и BLE как у BC-50; к устройству — JSON по HTTP через localhost (фискальный протокол Smart Pro v2.2025 + Payment Bridge v1.2026). URL/порт/авторизация не в PDF → `baseUrl` в настройках, вопросы Q-S1/Q-P1 |
| D-18 | Деньги в JSON Smart S | JSON-число строится из целых центов через `BigDecimal` (`13.67`), без `double`-арифметики; qty — из `qty_milli` |
| D-19 | Карта на Smart S | Через Payment Bridge (`dsk`/`viva`, валюта только `EUR`); UNKNOWN разрешается `check_status` по `reference_id`; автоматического возврата нет, пока нет ответа Q-P4 (возврат вручную после фискального сторно). D-14 («карта unsupported») относится только к Compact S 01 |


## 5. Решения владельца 2026-10-23 (приоритетнее колонки DEFAULT)

| ID | Решение | Было (DEFAULT) | Последствия |
|---|---|---|---|
| D-1 | **Волнами:** 1) BC-50 → 2) Datecs через S3+COM → 3) Daisy Compact S 01 через S3+USB → 4) BluePay → 5) Smart S; каждая волна закрывается строками матрицы ARCH_17 §5 | только BC-50 | HIL-план по волнам; PROD-флаги не меняются |
| D-2 | **Как ARCH_07 R-05** (+ TG-001, ARCH-001, POS-BLE-01/02 из D-3/D-13) | без изменений | `mvp-cut.v1.json` (SCR-39.2) |
| D-3 | **Переписать персистентность на транзакции ДО HIL** (TG-001/ARCH-001 → P0, волна 2); TG-001a/b — тесты приёмки | не переписывать | срок +; ADR-PERSIST-001 |
| D-4 | принято | — | — |
| D-5 | принято: разделить файл, удалить мёртвое | — | BUG-006, CODE-006 |
| D-6 | принято: консервативные правила NORM_PENDING | — | вопросы юристу остаются открытыми |
| D-7 | принято: окно 24 ч, ротация ≤ 90 дней; вторичный пароль из вывода инструмента сменить | — | BUG-009/011 |
| D-8 | принято: внешний терминал + сага (BluePay, Payment Bridge Smart S); встроенный пинпад — после DOC-V6 | — | — |
| D-9, D-10, D-12 | приняты | — | — |
| D-11 | **POS + MiniPOS + BeeFiscalApp**; AdminApp вне HIL-пути, только статическая проверка | MiniPOS и BeeFiscalApp | SCOPE-01 уточнить |
| D-13 | **ДА: нативный BLE в POS** | нет | POS-BLE-01…03 (P0), +40 ключей HIL-матрицы (`pos.ble`) |
| D-14, D-15, D-18, D-19 | приняты | — | — |
| D-16, D-17 | подтверждены ранее | — | — |
| D-20 | **Smart S, F-1:** только LAB; в PROD результат продажи остаётся `UNKNOWN`, пока вендор не ответил на Q-S2 | — | SMS-04, SMS-06 |
