# Логические ошибки и баги

**Дата:** 2026-08-21

---

## Backend (Go)

---

### B-01 — [HIGH] `newID()` на основе nanosecond timestamp + atomic counter — коллизии между инстансами

**Файл:** `fiscal-backend/internal/domain/service.go`

**Описание:**
```go
func newID(prefix string) string {
    return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), atomic.AddInt64(&seq, 1))
}
```
Atomic counter защищает от коллизий внутри одного процесса, но при двух подах, стартующих в одну наносекунду (возможно на виртуализированной инфраструктуре с общим гипервизорным источником времени), оба счётчика начинаются с 0. Коллизия первичного ключа = потерянная запись или паника.

Дополнительно: timestamp в ID раскрывает время создания и позволяет оценить объём продаж по разнице ID.

**Рекомендация:**
```go
import "github.com/google/uuid"

func newID(prefix string) string {
    return prefix + "-" + uuid.NewString()
}
```

---

### B-02 — [MEDIUM] `reversalAllowed()` — падение `time.LoadLocation` молча блокирует все reверсалы

**Файл:** `fiscal-backend/internal/domain/reversal_policy.go` (строка 34)

**Описание:**  
Если `time.LoadLocation("Europe/Sofia")` завершается с ошибкой (отсутствие tzdata в образе), функция возвращает `false` и блокирует все `OPERATOR_ERROR`-реверсалы молча. `_ "time/tzdata"` уже импортирован (строка 6), что защищает от этой проблемы — однако нет явной проверки при старте приложения.

**Рекомендация:**  
Добавить startup check:
```go
if _, err := time.LoadLocation("Europe/Sofia"); err != nil {
    panic("fiscal: tzdata отсутствует — реверсалы будут заблокированы: " + err.Error())
}
```

---

### B-03 — [MEDIUM] Учётные данные интеграции имеют 1-год TTL без механизма продления

**Файл:** `fiscal-backend/internal/integration/service.go`

**Описание:**  
Интеграционные credentials выдаются с 1-летним TTL без уведомлений об истечении, автоматической ротации или флага в admin dashboard. При истечении интеграция молча прекращает работу — торговцы обнаруживают проблему когда фискальные чеки перестают отправляться.

**Рекомендация:**
1. Webhook-событие `integration.credential.expiring_soon` за 30 и 7 дней до истечения
2. GET-эндпоинт возвращающий дату истечения credentials
3. Механизм ротации: выдавать новый credential до отзыва старого (overlap window)

---

### B-04 — [LOW] `UploadOnce` в edge-agent не различает retryable и fatal ошибки

**Файл:** `edge-agent/sync/uploader.go` (строка 71)

**Описание:**  
При любом не-200 статусе возвращается `false, errors.New("Edge sync upload rejected")`. HTTP 429 (Too Many Requests) или 503 (Service Unavailable) трактуется так же как 400 (Bad Request) или 409 (Conflict). Retry loop не может корректно реагировать на transient vs. permanent ошибки.

**Рекомендация:**
```go
if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
    return false, ErrRetryable
}
if resp.StatusCode != http.StatusOK {
    return false, ErrFatal
}
```

---

## Frontend / Web (TypeScript)

---

### B-05 — [HIGH] Плавающая точка в денежных суммах — ошибки округления в фискальных чеках

**Файл:** `minipos/miniposweb/src/App.tsx` (строки 76–80, 296–316)

**Описание:**
```js
sum + Number(line.product.price.amount) * line.quantity - line.discount
```
`price.amount` — строка, конвертируется в `Number` для арифметики. JavaScript использует IEEE 754 double precision: `0.1 + 0.2 = 0.30000000000000004`. Хелпер `money()` вызывает `.toFixed(2)` который truncates, а не rounds (0.005 → 0.00 вместо 0.01). Для POS-системы, передающей суммы в налоговые органы, даже расхождение в 0.01 EUR инвалидирует чек.

**Рекомендация:**  
Использовать целочисленную арифметику (суммы в центах) или библиотеку `decimal.js` / `big.js`. Тип `Money` уже использует строку `amount` — арифметику производить через decimal библиотеку.

---

### B-06 — [HIGH] Матчинг позиций корзины по имени товара вместо ID — неверный `product_id`

**Файл:** `minipos/miniposweb/src/App.tsx` (строки 268–270)

**Описание:**
```js
product_id: cart.find((line) => line.product.name === item.name)?.product.id,
```
Поиск позиции корзины по `item.name`. При двух товарах с одинаковым именем (например, "Специальное предложение") всегда возвращается первый match. В фискальном intent окажется неверный `product_id` — несоответствие между облачным заказом и фискальной записью.

**Рекомендация:**  
Матчить по `item.item_id` (уже присутствует в `SaleItem`) или использовать индекс массива напрямую, поскольку items строятся из cart lines по порядку (строка 288): `cart[index]?.product.id`.

---

### B-07 — [HIGH] `shifts.items[0]` без проверки — undefined смена устанавливается молча

**Файл:** `minipos/miniposweb/src/App.tsx` (строки 138–139)

**Описание:**
```js
const shifts = await api.shifts(active.employee.id);
setShift(shifts.items[0]);
```
`shifts.items[0]` используется без проверки что `items` не пустой. Если открытой смены нет — `setShift(undefined)` молча переводит приложение в состояние "нет смены" без сообщения об ошибке, при этом access token сохраняется в `localStorage`. Если API вернёт смену на другом кассовом регистре — checkout будет отправлять intent с неверным `register_id`.

**Рекомендация:**  
Проверять `shifts.items.length > 0` и валидировать `shifts.items[0].register_id === configuration.fiscal_register_id`. При отсутствии валидной открытой смены — показывать явную ошибку.

---

### B-08 — [MEDIUM] Race condition — возможны параллельные вызовы `adapterToken()`

**Файл:** `minipos/miniposweb/src/App.tsx` (строки 207–226, 362, 442)

**Описание:**  
`adapterToken()` вызывается через `void adapterToken().catch(() => undefined)` в `useEffect`, и синхронно внутри `checkout()` если `localToken` пуст. При параллельном выполнении — два запроса к `/fiscal-local-tokens` с разными `crypto.randomUUID()` idempotency keys, что потенциально выдаёт два local токена. Второй вызов в `checkout()` обновляет `localToken` state, но параллельный useEffect может перезаписать его после начала checkout.

**Рекомендация:**  
Дедуплицировать запросы через module-level `Promise`. Использовать единый idempotency key на основе `(session.id + shift.id)` вместо случайного UUID.

---

### B-09 — [MEDIUM] Двойное увеличение `cloudSuccesses` в RouteController при восстановлении после сбоя

**Файл:** `minipos/miniposweb/src/fiscal/routeController.ts` (строки 65–84)

**Описание:**  
В пути восстановления, когда `preferred` уже установлен обратно в `"CLOUD"` после `recoveryThreshold` успехов, pre-flight probe пропускается (строка 65). Но cloud route зондируется в основном цикле (строка 82). Результат probe применяется через `this.observe()` дважды — в pre-flight (если выполняется) и в per-route (всегда). `cloudSuccesses` может инкрементироваться дважды, ускоряя восстановление некорректно.

**Рекомендация:**  
Унифицировать стратегию probe: либо всегда делать pre-flight и пропускать probe в цикле, либо наоборот. Убрать дублирующий pre-flight блок.

---

### B-10 — [LOW] Скидка на позицию может превышать сумму позиции — отрицательные суммы платежей

**Файл:** `minipos/miniposweb/src/App.tsx` (строка 283, 296–316)

**Описание:**  
`update()` ограничивает discount снизу (`Math.max(0, value)`), но не сверху. Скидка больше `product.price.amount * quantity` делает сумму позиции отрицательной. Если суммарный `total` остаётся положительным из-за других позиций, guard `total <= 0` не срабатывает, и `payments` может содержать отрицательную сумму CASH платежа.

**Рекомендация:**  
Ограничить скидку в `update()`: `Math.min(discount, product.price.amount * quantity)`.

---

## IoT / Embedded (C++)

---

### B-11 — [MEDIUM] Daisy: off-by-one в receive buffer — один байт записывается за пределы массива

**Файл:** `IoT/common-modules/daisy/DaisyProtocol.cpp`

**Описание:**  
В `_readUntilETX()`: `_rxBuf[_rxLen++] = b;` выполняется до проверки `if (_rxLen >= DAISY_RX_BUF)`. При `_rxLen == DAISY_RX_BUF - 1 = 219` post-increment делает `_rxLen = 220` и записывает байт в `_rxBuf[219]` — это последний валидный элемент. При `_rxLen == 220` запись идёт в `_rxBuf[220]` — один байт за пределами массива (single-byte buffer overwrite).

**Рекомендация:**  
Проверять границу до записи:
```cpp
if (_rxLen >= DAISY_RX_BUF) { return false; }
_rxBuf[_rxLen++] = b;
```

---

### B-12 — [MEDIUM] Tremol: `sendPacket()` молча обрезает данные при превышении буфера

**Файл:** `IoT/common-modules/termol/TremolProtocol.cpp` (строки 74–76)

**Описание:**
```cpp
uint8_t dataLen = (data && *data) ? (uint8_t)strlen(data) : 0;
if (dataLen > TR_TX_BUF - 7) dataLen = TR_TX_BUF - 7;
```
Слишком длинная строка молча обрезается вместо возврата ошибки. Название товара или параметр чека, передаваемый принтеру, окажется неполным — потенциально некорректный VAT, неверная цена, искажённое наименование на чеке. Нарушение фискального соответствия.

**Рекомендация:**  
Возвращать 0 (ошибку) вместо молчаливого truncation. Вызывающий код должен знать, что данные слишком длинные.

---

### B-13 — [MEDIUM] Datecs: результат `snprintf` не проверяется в `registerSale()`

**Файл:** `IoT/common-modules/datecs/DatecsPrinter.cpp` (строки 143–154)

**Описание:**  
Оба ветки `if/else` в `registerSale()` не проверяют возвращаемое значение `snprintf`. При truncation (строка длиннее `DATECS_MAX_DATA_TX`) пакет отправляется с неполными данными без сигнала об ошибке. Паттерн проверки `n >= (int)sizeof(buf)` уже используется в `displayLine1()` и других методах — но пропущен здесь.

**Рекомендация:**
```cpp
int n = snprintf(buf, sizeof(buf), ...);
if (n < 0 || n >= (int)sizeof(buf)) return false;
```

---

### B-14 — [MEDIUM] BLE Handshake: упорядочивание `secret.fill(0)` относительно async — latent race

**Файл:** `minipos/BeeMiniPOS/src/bleHandshake.ts` (строки 178–193)

**Описание:**  
`this.frames = await BleFrameSession.client(secret, ...)` (строка 179) вызывается до `secret.fill(0)` (строка 193). В текущей реализации `BleFrameSession.client()` вызывает `hmac256` синхронно в том же microtask, поэтому работает корректно. Но если реализация будет изменена на истинно async (например, Web Crypto), обнуление может произойти до завершения деривации ключа.

**Рекомендация:**  
Комментарий, фиксирующий ordering constraint. Убедиться, что `secret.fill(0)` выполняется только после `await` завершения.

---

### B-15 — [LOW] webBleRoute: Assembler не отклоняет `total > MAX` — возможна большая аллокация

**Файл:** `minipos/miniposweb/src/fiscal/webBleRoute.ts` (строка 57)

**Описание:**  
`Assembler.accept()` читает `total = view.getUint16(20)` (0–65535) и выделяет `new Uint8Array(total)`. При `MAX = 8192` — искажённый фрейм с `total = 65535` вызывает 64KB аллокацию. В `FrameAssembler.accept()` в `bleFrames.ts` (строка 56) проверка `if (!total || total > MAX ...)` уже есть — здесь пропущена.

**Рекомендация:**  
Добавить проверку `if (total > MAX) return null;` в начале `Assembler.accept()`.

---

### B-16 — [LOW] Datecs: `deviceInfo()` принимает `bufLen` как `uint8_t` — максимум 255 байт при ответе до 480

**Файл:** `IoT/common-modules/datecs/DatecsPrinter.cpp` (строка 288)

**Описание:**  
`deviceInfo(char* buf, uint8_t bufLen)` ограничивает буфер 255 байтами, тогда как `DATECS_MAX_DATA_RX = 480`. `strncpy(buf, _lastResp.data, bufLen - 1)` молча усечёт данные об устройстве. Проблема не memory safety, но диагностические данные будут неполными.

**Рекомендация:**  
Изменить тип `bufLen` на `uint16_t` в объявлении и реализации.
