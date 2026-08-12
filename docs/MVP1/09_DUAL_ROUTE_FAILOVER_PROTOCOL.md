# REST/WebHook ↔ BLE automatic failover protocol

Статус: обязательный transport protocol MVP1 для BlueCash и edge-agent-s3.  
Цель: продолжать одну продажу при потере cloud без двойного чека, списания или
расхождения backend/device state.

## 1. Границы маршрутов

`REST` и `BLE` — transports одной business session, а не два независимых
источника продажи. Immutable physical route содержит adapter и fiscal/payment
endpoints. При failover меняется только `transport_epoch.transport`:

```yaml
sale_id: uuid
client_sale_surrogate_id: uuid
receipt_session_id: uuid
unp: BG_UNP_V1
route_id: uuid
binding_generation: integer
adapter_device_id: uuid
transport_epoch:
  epoch: integer
  transport: CLOUD_REST | DIRECT_BLE
  switched_at: RFC3339Nano
  reason: CLOUD_UNREACHABLE | CLOUD_RESTORED | MANUAL_DIAGNOSTIC
```

Vendor/model/device нельзя менять посередине незавершённого чека. Rebind
разрешён только после commit/compensation/reconciliation либо контролируемого
service recovery.

## 2. Lightweight ping

### 2.1 Контракт

Добавить public unauthenticated endpoint:

```http
HEAD /connectivity/ping HTTP/1.1

HTTP/1.1 204 No Content
Cache-Control: no-store
X-BeeFiscal-Ping: 1
```

`GET` MAY возвращать тот же `204`; клиент обязан использовать `HEAD`. Endpoint:

- не читает БД/Redis/MQTT/device registry;
- не проверяет ФУ и не создаёт readiness lease;
- не пишет application audit/access body;
- имеет отдельный дешёвый ingress rate limit;
- отвечает не более чем фиксированными headers, без JSON и compression;
- доступен через тот же origin/Caddy/TLS/DNS path, что public Fiscal REST.

Рекомендуемая реализация: Caddy направляет ping на отдельный in-process
`/internal/live` handler backend, который выполняет только atomic process-state
read. Это проверяет ingress **и живой HTTP process**, но не нагружает business
middleware/БД. Статический `204` только на Caddy недостаточен: он был бы зелёным
при упавшем backend. `/internal/live` недоступен извне в обход Caddy.

Ping не является readiness, authorization или гарантией исполнения следующей
REST-команды. REST timeout/5xx всё равно участвует в circuit breaker.

### 2.2 Частота и нагрузка

По умолчанию:

```text
FOREGROUND_REST: every 10 s ±20% jitter
SUSPECT: 1 s, 2 s, 4 s retry
BLE_ACTIVE: every 5 s ±20% jitter
BACKGROUND/no active shift: every 30 s
request timeout: 1500 ms
```

На сервере endpoint должен выдерживать целевое число POS без DB connection.
HTTP keep-alive разрешён; ответы не должны попадать в webhook/outbox/audit.
Operational access metrics агрегируются по status/region, без tenant lookup.

## 3. Client connectivity state machine

```text
CLOUD_HEALTHY
  ├─ one ping/transport failure → CLOUD_SUSPECT
  └─ REST success             → CLOUD_HEALTHY

CLOUD_SUSPECT
  ├─ ping recovered           → CLOUD_HEALTHY
  └─ 3 consecutive failures
     or 5 s without success   → CLOUD_UNAVAILABLE

CLOUD_UNAVAILABLE
  ├─ BLE authority + radio + adapter ready → BLE_ACTIVE
  └─ no usable BLE                         → OFFLINE_BLOCKED

BLE_ACTIVE
  ├─ 3 consecutive successful pings spanning ≥10 s → CLOUD_RECOVERING
  └─ BLE/device lost                         → OFFLINE_BLOCKED

CLOUD_RECOVERING
  ├─ reconcile/sync succeeds → CLOUD_HEALTHY
  └─ ping/sync failure       → BLE_ACTIVE
```

Порог и интервалы конфигурируются signed policy, но клиент не может поставить
zero failures/zero hysteresis. Network callback используется как ранний signal,
но не заменяет ping того же public origin.

## 4. Authority для BLE

Backend выдаёт короткоживущую BLE authority при старте workstation session и
обновляет её пока cloud healthy, а не после обрыва. Ticket связан с tenant,
location, register, adapter device, fiscal/payment endpoints, operator/app
session, binding generation, scopes, expiry и fencing token.

Для продолжения уже открытой sale ticket должен разрешать её immutable
`sale_id/client_sale_surrogate_id/UNP` либо делегированный fenced offline range.
Истёкший ticket не расширяется локально. Если отсутствует действующая authority,
BLE route блокируется безопасно.

## 5. Продолжение чека между transports

Каждая POS mutation до передачи получает стабильные:

```text
client_operation_id UUID, созданный и durable сохранённый MiniPOS
sale_id / client_sale_surrogate_id
receipt_session_id, если checkout зарезервирован
step_id
expected_sale_version
canonical payload_sha256
```

`client_operation_id` создаётся заново для каждой новой business mutation, но
никогда не меняется для retry или transport failover этой mutation. Это основной
end-to-end idempotency key. Backend correlation/trace IDs остаются только
диагностическими и не могут заменить его в dedupe.

Adapter использует одну idempotency/journal table для MQTT и BLE. Вход через
другой transport с тем же ID и digest возвращает существующий result либо
продолжает recovery. Тот же ID с другим digest отклоняется.

### 5.1 REST не успел принять intent

Если client достоверно получил network error **до отправки bytes**, intent можно
отправить по BLE с тем же ID. Если момент send неизвестен, применяется 5.2.

### 5.2 REST мог принять или опубликовать MQTT command

BLE сначала отправляет
`OPERATION_LOOKUP(client_operation_id, canonical_payload_sha256)`:

- `NOT_FOUND` у adapter + доказано отсутствие durable cloud command на device —
  adapter атомарно резервирует intent и выполняет;
- `RESERVED/EXECUTING` — BLE подписывается на/запрашивает status, не повторяет I/O;
- terminal result — возвращает тот же result;
- `UNKNOWN` — запускает physical reconciliation;
- digest conflict — блокирует sale.

Backend outbox может позже доставить исходную MQTT-команду; общий device dedupe
возвращает существующий результат без side effect.

### 5.3 BLE начался, cloud восстановился

MiniPOS не отправляет активный BLE step повторно через REST. Device завершает или
reconciles step, публикует signed journal через MQTT, backend materializes его и
WebHook обновляет MiniPOS backend. После подтверждения sync/authority version
следующая mutation может идти по REST с теми же sale/session identifiers.

### 5.4 Что означает «чек продолжается»

Разрешены, например:

- первый товар REST, следующие строки BLE, finalize BLE;
- sale открыта BLE, строки/checkout продолжаются REST после sync;
- checkout reserved REST, фактическое выполнение/получение result через BLE
  после lookup.

Нельзя одновременно исполнять два physical steps одной session. Device-side
lease/fencing и journal являются последней защитой от гонки каналов.

## 6. Возврат на REST и WebHook

Успешный ping сам по себе не переключает открытую sale. Перед возвратом:

1. дождаться hysteresis `CLOUD_RECOVERING`;
2. adapter публикует pending contiguous batches;
3. backend возвращает signed business ACK;
4. MiniPOS получает authoritative projection polling/WebHook;
5. сравниваются sale version, receipt session state и last operation/step;
6. только затем transport epoch увеличивается и новые intents идут REST.

WebHook остаётся backend→MiniPOS уведомлением. Потерянный WebHook восстанавливается
polling по operation/sale ID; WebHook не является подтверждением physical commit.

## 7. Ошибки и UX

MiniPOS показывает Bulgarian localized состояния:

- cloud online;
- switching to local BLE;
- working locally, pending synchronization;
- cloud restored, reconciling;
- device unavailable — sale blocked;
- transaction outcome unknown — manager/service action required.

Переключение автоматическое и не требует выбора vendor/channel кассиром.
Ручная кнопка разрешена только для повторного ping/BLE diagnostics, но не для
обхода fencing/readiness/reconciliation.

## 8. Обязательные тесты

Для BlueCash, DP-150+BluePad и Daisy profiles на driver stubs, затем HIL:

1. ping healthy → только REST;
2. 1 lost ping → REST не переключается немедленно;
3. threshold reached → BLE automatically;
4. ping restored but hysteresis incomplete → BLE remains;
5. recovery + sync/ACK → new intents REST;
6. first line REST, add/finalize BLE;
7. first line BLE, sync, finalize REST;
8. REST timeout before send;
9. REST timeout after durable backend outbox/MQTT send;
10. MQTT arrives after same BLE intent;
11. BLE step active when cloud recovers;
12. duplicate across transports gives one device I/O;
13. payload conflict blocks;
14. ticket expired/rebound/fenced blocks BLE;
15. cloud lost and BLE lost blocks sale;
16. physical FU lost while ping/BLE alive blocks sale;
17. WebHook loss recovered by polling;
18. 1000 simulated clients ping test proves no DB/MQTT/business calls.
19. каждая операция чека имеет отдельный client UUID;
20. retry REST→BLE→MQTT сохраняет UUID и даёт один side effect;
21. тот же UUID с изменённым payload/type/sale ID отклоняется backend и device;
22. два одинаковых intent с разными UUID выполняются как две явные операции;
23. restart MiniPOS восстанавливает pending UUID из локальной БД.

HIL evidence должно доказать ровно один fiscal receipt и card side effect при
переключении на каждой journal boundary.
