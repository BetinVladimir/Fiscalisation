# Проблемы масштабируемости

**Дата:** 2026-08-21

---

## Backend (Go)

---

### SC-01 — [HIGH] Все listing-эндпоинты загружают весь датасет тенанта в память

**Файлы:** `fiscal-backend/internal/api/handler.go`, `fiscal-backend/internal/persistence/postgres.go`

**Описание:**  
Следующие эндпоинты загружают все записи тенанта в Go-слайсы, затем фильтруют, сортируют и пагинируют в памяти приложения:
- `GET /sales` — `auditEvents()`, `sales()`, `operations()`
- `GET /shifts`
- `GET /reports`

При 10,000 продаж в месяц полный годовой экспорт загружает 120,000+ строк. Это создаёт:
1. O(n) потребление памяти пропорционально истории тенанта
2. O(n) время чтения из БД вне зависимости от запрошенной страницы
3. Невозможность горизонтального масштабирования — каждый pod держит полную копию

**Рекомендация:**  
Перенести фильтрацию, сортировку и пагинацию на уровень БД. Добавить индексы на `created_at`, `shift_id`, `register_id`, `status`. Использовать SQL `WHERE`, `ORDER BY`, `LIMIT/OFFSET` или keyset-пагинацию с курсором `(created_at, id)`.

---

### SC-02 — [HIGH] N+1 запрос в `operations()` при фильтрации по `register_id`

**Файл:** `fiscal-backend/internal/api/handler.go`

**Описание:**  
При наличии query-параметра `register_id`, `operations()` итерирует все операции тенанта и вызывает `GetSaleForTenant()` для каждой по отдельности:
```go
for _, op := range ops {
    sale, err := h.db.GetSaleForTenant(ctx, tenantID, op.SaleID)
    // фильтр по sale.RegisterID
}
```
При 5,000 операциях — до 5,000 последовательных round-trip к БД в рамках одного HTTP запроса.

**Рекомендация:**  
Добавить `register_id` как фильтруемое поле в SQL запрос операций, либо включить `register_id` напрямую в строку операции для устранения join.

---

### SC-03 — [MEDIUM] Rate limiter map растёт неограниченно (утечка памяти)

**Файл:** `fiscal-backend/internal/api/rate_limit.go`

**Описание:**  
Sliding-window rate limiter хранит `map[string]rateWindow` по tenant ID или IP без eviction. Каждый уникальный IP, когда-либо сделавший запрос, остаётся в map навсегда.

**Рекомендация:**  
Добавить eviction записей по истечении window duration. Или заменить на Redis/Valkey для распределённого rate limiting.

---

### SC-04 — [MEDIUM] In-memory event-sourcing aggregate — не масштабируется горизонтально

**Файл:** `fiscal-backend/internal/domain/service.go`

**Описание:**  
`Coordinator` хранит полное состояние aggregate для всех тенантов в одной in-process структуре. При нескольких репликах каждый pod поддерживает независимое состояние, которое расходится по мере записи на разные поды. PostgreSQL обеспечивает durability, но не cross-pod когерентность.

**Рекомендация:**  
Задокументировать ограничение. Для горизонтального масштабирования рассмотреть:
- Sticky routing по тенанту (consistent hash на load balancer)
- Вынести состояние aggregate в Redis с advisory locks per-tenant
- При long-term event sourcing — использовать EventStoreDB или Kafka с compacted topics

---

### SC-05 — [LOW] Малые размеры connection pool к БД

**Файлы:**
- `fiscal-backend/internal/persistence/postgres.go` — `SetMaxOpenConns(20)`
- `minipos/beeminipos-backend/internal/persistence/postgres.go` — `SetMaxOpenConns(10)`

**Описание:**  
Serializable-транзакции и advisory locks могут удерживать соединения десятки миллисекунд. При 20 соединениях и p99 latency 50ms — пропускная способность ограничена ~400 req/s до появления connection starvation. `SetMaxIdleConns` не задан (дефолт Go = 2).

**Рекомендация:**  
Вынести размеры pool в переменные окружения. Задать `SetMaxIdleConns(5)` и `SetConnMaxIdleTime(5 * time.Minute)`. Ориентир размера pool: `num_cores × 2 + effective_spindle_count`.

---

## Frontend / Web (TypeScript)

---

### SC-06 — [HIGH] IndexedDB открывается и закрывается при каждой операции

**Файлы:**
- `minipos/miniposweb/src/storage/outbox.ts` (строки 17–58)
- `minipos/miniposweb/src/storage/referenceCache.ts` (строки 3–37)

**Описание:**  
Каждый вызов `putOutbox()`, `getOutbox()`, `listOutbox()`, `cachePut()`, `cacheGet()` вызывает `indexedDB.open(DB, 1)` и завершается `db.close()`. При фискальном checkout `putOutbox` вызывается 3–4 раза. Открытие IndexedDB-соединения — асинхронная операция с дисковым I/O, добавляющая 50–200мс на каждую запись на мобильных.

**Рекомендация:**  
Использовать постоянный singleton `IDBDatabase` на уровне модуля. Переоткрывать только при событиях `versionchange` или `close`.

---

### SC-07 — [MEDIUM] Sync loop работает каждые 15 секунд вне зависимости от состояния сети

**Файл:** `minipos/miniposweb/src/App.tsx` (строки 116–124)

**Описание:**  
`syncOfflineOrders()` вызывается каждые 15 секунд вне зависимости от `navigator.onLine`. В offline режиме все HTTP-запросы таймаутятся или отклоняются, тратя ресурсы. Каждая итерация открывает и закрывает IndexedDB.

**Рекомендация:**  
Проверять `navigator.onLine` перед sync. Использовать события `online`/`offline` для паузы/возобновления таймера. Early exit если `listOutbox()` возвращает пустой список.

---

### SC-08 — [MEDIUM] Token refresh timer игнорирует `expires_in` — hardcoded 12 минут

**Файл:** `minipos/miniposweb/src/App.tsx` (строки 84–103)

**Описание:**  
Refresh `useEffect` устанавливает фиксированный 12-минутный интервал, игнорируя поле `expires_in` из ответа сервера. При изменении lifetime токена на сервере клиент не синхронизируется. В `BeeMiniPOS/src/emailAuth.ts` корректный вариант уже реализован через `nextOidcTokenAction`.

**Рекомендация:**  
Использовать `expires_in` для вычисления задержки refresh, как в BeeMiniPOS.

---

### SC-09 — [MEDIUM] Route probe повторяется при каждом checkout без кэширования

**Файл:** `minipos/miniposweb/src/fiscal/routeController.ts` (строки 79–86)

**Описание:**  
`execute()` зондирует каждый маршрут (cloud, local HTTP, BLE) последовательно с 1.5с timeout перед каждым submit. В happy-path добавляет ~1.5с задержки на каждую продажу. При трёх маршрутах — worst case 4.5с до начала 120с operation timeout.

**Рекомендация:**  
Кэшировать результаты probe с коротким TTL (5–10с). `ConnectivityController` в `BeeMiniPOS/src/connectivity.ts` уже реализует подходящий state machine — применить тот же паттерн в web.

---

## IoT / Embedded (C++)

---

### SC-10 — [LOW] `DaisyProtocol._writeAll()` использует `uint8_t` для offset — переполнение при >255 байт

**Файл:** `IoT/common-modules/daisy/DaisyProtocol.cpp` (строки 83–92)

**Описание:**  
`offset` объявлен как `uint8_t`. При записи payload >255 байт `offset + written` переполняется и обнуляется. Текущий `DAISY_TX_BUF = 210` помещается в `uint8_t`, но это хрупко при любом расширении буфера.

**Рекомендация:**  
Изменить тип `offset` на `uint16_t` в `_writeAll()`.
