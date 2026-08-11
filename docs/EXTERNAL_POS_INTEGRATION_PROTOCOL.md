# Протокол подключения внешнего POS к BeeFiscal

Версия документа: `1.0`  
Версия API: `2026-08-07`  
Профиль: `BG_SUPTO_FULL / BG_UNP_V1`  
Валюта операций: `EUR`

Документ определяет единый протокол интеграции внешних POS-систем, включая BeeMiniPOS, с BeeFiscal через публичный REST API, подписанные WebHooks и локальный BLE transport. POS не подключается к MQTT устройств, внутренним БД, vendor SDK или приватным API Fiscal Platform.

Машиночитаемые источники истины:

- [публичный OpenAPI](../../BeeloyBackend/docs/Fiscal/api/openapi-public-v1.yaml);
- [runtime OpenAPI](../contracts/openapi-runtime-v1.yaml);
- [сгенерированные TypeScript-типы](../contracts/generated/openapi-runtime-v1.d.ts);
- [BLE GATT v1](../../BeeloyBackend/docs/Fiscal/ble/ble-gatt-v1.md).

При расхождении примера в этом документе и OpenAPI клиент обязан следовать reviewed OpenAPI соответствующей версии.

## 1. Архитектурная граница

```text
External POS UI
      |
      | собственный API/БД POS
      v
External POS backend
      |
      +--- HTTPS REST commands/status ---> Fiscal Caddy ---> BeeFiscal
      |
      <--- HTTPS signed WebHooks ---------- Fiscal outbox
      |
      +--- optional BLE ----------> Local Compliance Gateway ---> ФУ
```

Для BeeMiniPOS внешний клиент — `beeminipos-backend`, а не Expo UI. BeeMiniPOS хранит товары, сотрудников, смены UI и заказы в собственной БД, но вызывает BeeFiscal через те же публичные контракты, что и сторонний POS.

POS отвечает за UI и бизнес-намерение. BeeFiscal отвечает за:

- tenant, location, register, workstation и fiscal-device binding;
- готовность конечного ФУ;
- профиль требований и налоговые правила;
- выдачу УНП и regulatory identifiers;
- authoritative sale projection и totals;
- маршрутизацию к ФУ/платёжному терминалу;
- фискальный результат, reconciliation, storno, отчёты и audit.

POS запрещено:

- формировать УНП, номер фискального бона или fiscal reference;
- отправлять raw/vendor-команды кассовому аппарату;
- выбирать конкретный cloud/BLE/device route после принятия операции;
- считать timeout неуспешной операцией и автоматически повторять её с новым ID;
- подключаться к MQTT broker SmartDevices;
- использовать BLE как независимую фискальную реализацию;
- хранить credentials, PAN/PIN/CVV или regulatory signing authority.

## 2. Транспортные режимы

| Режим | Назначение | Источник истины |
|---|---|---|
| REST | команды, bootstrap, чтение состояния, recovery | BeeFiscal backend |
| WebHook | асинхронное уведомление об изменении server state | событие; состояние подтверждается REST при необходимости |
| BLE | локальная доставка того же `ComplianceIntent` при недоступности cloud route | Local Compliance Gateway с последующей синхронизацией |

Выбор маршрута выполняется до отправки intent по server/gateway readiness. После первой подтверждённой отправки одного `intent_id` запрещено отправлять его по другому маршруту. При неопределённом результате разрешены только status lookup и reconciliation.

Если недоступен cloud, но доказан маршрут `POS → BLE → Gateway → конечное ФУ`, локальная операция допустима в пределах действующей BLE authority. Если потеряно конечное ФУ, продажа блокируется независимо от доступности BLE.

## 3. REST

### 3.1 Endpoint и версия

Production base URL публикуется через Fiscal Caddy:

```text
https://<fiscal-domain>/public/v1
```

Запрещены прямые обращения к container/service port, `/internal/v1`, `/local/v1` через Internet и автоматическое переключение на неизвестный base URL.

Каждый запрос передаёт:

```http
Authorization: Bearer <OIDC access token>
X-Api-Version: 2026-08-07
Accept: application/json
```

Все mutation дополнительно требуют:

```http
Idempotency-Key: <16..255 characters, unique per logical command>
Content-Type: application/json
```

Изменение существующего aggregate также требует актуальную версию:

```http
If-Match: <integer version returned by server>
```

Tenant определяется только проверенным access token. `tenant_id`/`organization_id` из request body не могут переопределить token context.

### 3.2 Аутентификация и identity

Рекомендуемый внешний клиент использует OIDC/OAuth 2.0:

- пользовательский POS — Authorization Code + PKCE;
- server-to-server POS backend — отдельный confidential client с минимальными scopes, если это разрешено deployment policy;
- каждая кассовая операция всё равно содержит персональную operator session; shared cashier identity запрещена.

Access token не хранится в POS database/SQLite/log. Refresh authority хранится только по требованиям выбранного OIDC SDK и должна поддерживать rotation/revocation.

### 3.3 Idempotency

`Idempotency-Key` идентифицирует одно логическое намерение, а не один HTTP attempt.

Правила:

1. При timeout POS повторяет тот же method/path/query/body, identity context, `If-Match` и idempotency key.
2. Новый key для той же операции до reconciliation запрещён.
3. Тот же key с другим request fingerprint должен получить conflict/contract error.
4. POS сохраняет key вместе со своей operation link до terminal server state.
5. HTTP `202` означает durable acceptance, но не обязательно фискальный успех.

### 3.4 Ошибки

Ошибки возвращаются как `application/problem+json`. POS принимает решения по стабильному `code`, а не по локализованному `title`.

Минимальные реакции:

| HTTP/code | Реакция POS |
|---|---|
| `401/403` | остановить команду, восстановить identity/permissions |
| `409` | получить authoritative state; не обходить conflict |
| `428 IF_MATCH_REQUIRED` | перечитать ресурс и повторить только после решения пользователя/policy |
| `422` | исправить request; автоматический retry запрещён |
| `429` | соблюдать `Retry-After` |
| `5xx` до известного acceptance | повторить тот же idempotent request |
| `FISCAL_RESULT_UNKNOWN` | заблокировать повтор side effect, открыть reconciliation |
| неизвестный error code | fail-close и показать correlation/trace ID |

## 4. Bootstrap POS-сессии

Перед продажей POS выполняет:

1. Получает reference data и effective country/tax policy.
2. Создаёт или восстанавливает персональную workstation/operator session:

```http
POST /public/v1/workstations/{workstation_id}/sessions
```

3. Открывает или получает смену через `/shifts`.
4. Выполняет clock sync один раз на business date и после ошибки/смены ФУ:

```http
POST /public/v1/workstations/{workstation_id}/clock-sync
```

5. Обновляет readiness lease:

```http
POST /public/v1/workstations/{workstation_id}/readiness:refresh
GET  /public/v1/workstations/{workstation_id}/readiness
```

6. Проверяет, что server разрешает начало продажи. Отсутствие готового конечного ФУ блокирует продажу.

Binding IDs задаются администратором BeeFiscal и не редактируются кассиром. POS хранит их как server references.

## 5. REST lifecycle продажи

### 5.1 Первая позиция

В профиле `BG_SUPTO_FULL` продажа и первая строка создаются атомарно первым товарным действием:

```http
POST /public/v1/sales:open-with-line
Authorization: Bearer <token>
X-Api-Version: 2026-08-07
Idempotency-Key: 018f-pos-open-00000001
Content-Type: application/json

{
  "client_sale_surrogate_id": "550e8400-e29b-41d4-a716-446655440000",
  "workstation_id": "550e8400-e29b-41d4-a716-446655440010",
  "operator_session_id": "550e8400-e29b-41d4-a716-446655440020",
  "line": {
    "line_id": "550e8400-e29b-41d4-a716-446655440001",
    "product_code": "COFFEE-1",
    "name": "Кафе",
    "quantity": "1.000",
    "unit_price": {"amount":"2.50","currency":"EUR"},
    "tax_group": "B"
  }
}
```

Успех — `201` с `ETag`, server sale ID, `version`, `allowed_actions`, authoritative totals, immutable device snapshot, УНП и `regulatory_identifiers[]`. Regulatory identifiers являются opaque strings: POS хранит и отображает их без parsing/reformatting.

Товар нельзя показывать как принятую продажу до ответа сервера. Допустима только визуальная метка `PENDING_SERVER_ACCEPTANCE`.

### 5.2 Изменение продажи

| Действие | Endpoint |
|---|---|
| восстановить продажу | `GET /sales/{sale_id}` либо `GET /sales?...` |
| добавить строку | `POST /sales/{sale_id}/lines` |
| изменить строку | `PATCH /sales/{sale_id}/lines/{line_id}` |
| отменить строку | `POST /sales/{sale_id}/lines/{line_id}:cancel` |
| отменить продажу | `POST /sales/{sale_id}:cancel` |
| создать payment intent | `POST /sales/{sale_id}/payment-intents` |
| выполнить storno | `POST /sales/{sale_id}:reverse` |

Удаление строк или completed sale отсутствует: изменения создают компенсирующее событие. POS показывает только server `allowed_actions`.

### 5.3 Оплата

POS передаёт намерение CASH/CARD и сумму в EUR. BeeFiscal:

- повторно проверяет readiness непосредственно перед side effect;
- рассчитывает authoritative remaining amount;
- при CARD и активном payment terminal автоматически инициирует оплату картой;
- связывает payment и fiscal operation;
- возвращает durable operation со state.

POS не вызывает pinpad напрямую и не переводит CARD в CASH после timeout. Изменение способа оплаты — новое явное действие оператора только после определённого результата/reconciliation.

### 5.4 Асинхронный результат

`202` сохраняется POS как `PENDING`. Результат приходит WebHook либо читается:

```http
GET /public/v1/operations/{operation_id}
```

Если state `UNKNOWN`:

```http
POST /public/v1/operations/{operation_id}:reconcile
```

Reconcile выполняет lookup/recovery и не должен слепо повторять device side effect.

## 6. WebHooks

### 6.1 Регистрация endpoint

POS backend регистрирует публичный HTTPS URL:

```http
POST /public/v1/webhook-endpoints
Authorization: Bearer <admin/service token>
X-Api-Version: 2026-08-07
Idempotency-Key: webhook-create-00000001
Content-Type: application/json

{
  "url": "https://pos.example.com/integrations/beefiscal/webhooks",
  "events": [
    "fiscal.operation.updated",
    "fiscal.operation.succeeded",
    "fiscal.operation.failed",
    "fiscal.operation.reconciliation_required",
    "payment.updated",
    "device.readiness.changed"
  ]
}
```

Endpoint обязан быть публичным абсолютным HTTPS URL без credentials/fragment и не может указывать на loopback/private/link-local address. Secret содержит 32 random bytes в base64url и возвращается только при создании/rotation; GET/PATCH его не возвращают.

Управление:

- `GET /webhook-endpoints/{id}`;
- `PATCH /webhook-endpoints/{id}` с `If-Match`;
- `POST /webhook-endpoints/{id}/rotate-secret`;
- `DELETE /webhook-endpoints/{id}`.

При rotation старый secret действует ещё 24 часа. Receiver должен временно принимать current и previous secret по `kid`.

### 6.2 События

Поддерживаемые типы:

- `fiscal.operation.updated`;
- `fiscal.operation.succeeded`;
- `fiscal.operation.failed`;
- `fiscal.operation.reconciliation_required`;
- `payment.updated`;
- `device.readiness.changed`;
- `device.connectivity.changed`;
- `edge.connectivity.changed`;
- `register.report.completed`;
- `ble.session.revoked`.

Envelope:

```json
{
  "event_id": "event-uuid",
  "event_type": "fiscal.operation.succeeded",
  "api_version": "2026-08-07",
  "tenant_id": "tenant-from-server",
  "resource_id": "operation-id",
  "resource_version": 3,
  "occurred_at": "2026-08-11T10:30:00Z",
  "data": {
    "state": "FISCALIZED",
    "operation_id": "operation-id",
    "operation_type": "FISCAL_SALE",
    "sale_id": "sale-id",
    "external_id": "pos-reference",
    "fiscal_reference": "opaque-reference",
    "error_code": null
  }
}
```

### 6.3 Подпись

Нормативный заголовок фактического dispatcher:

```http
BeeFiscal-Event-Id: <event_id>
BeeFiscal-Signature: t=<unix-seconds>,kid=<webhook-endpoint-id>,v1=<hex-hmac-sha256>
```

Подписываемая последовательность байтов:

```text
UTF8(decimal_unix_timestamp) || "." || raw_HTTP_body_bytes
```

`v1 = lowercase_hex(HMAC-SHA256(endpoint_secret, signed_bytes))`.

Receiver обязан:

1. прочитать raw body до JSON parsing;
2. разобрать ровно поддерживаемую версию `v1`;
3. найти secret по `kid`;
4. отклонить timestamp вне допустимого окна, рекомендуемо ±5 минут;
5. вычислить HMAC по raw bytes и сравнить constant-time;
6. проверить `event_id` header = body, API version и tenant;
7. атомарно сохранить event в durable inbox с unique `(endpoint_id,event_id)`;
8. применить projection только если `resource_version` новее сохранённой;
9. вернуть любой `2xx` только после durable acceptance.

Не следует подписывать повторно сериализованный JSON: whitespace/order могут отличаться.

Формат `t,kid,v1`, подписываемая последовательность и pattern заголовка закреплены в runtime OpenAPI. Сторонний receiver не должен поддерживать прежний простой 64-символьный hex-формат.

### 6.4 Delivery и recovery

Доставка — at-least-once. Retry delays реализации: 10 секунд, 30 секунд, 2 минуты, 10 минут, 1 час, 6 часов, затем 24 часа. Поэтому receiver обязан быть идемпотентным.

- Любой `2xx` — принято.
- Network error/non-2xx — повтор.
- `410 Gone` — endpoint автоматически отключается.
- Порядок разных resources не гарантируется; применяйте `resource_version`.
- WebHook не является единственным recovery channel. После gap/restart POS читает REST operation/sale state.

## 7. BLE Local Compliance Protocol

### 7.1 Назначение

BLE используется только если cloud route недоступен, но рядом доступен доверенный Local Compliance Gateway, связанный с тем же tenant/location/register/конечным ФУ. Он принимает тот же business intent и владеет compliance policy, durable journal и device driver.

BLE не предоставляет raw fiscal API и не даёт POS offline UNP range/signing key.

### 7.2 Создание сессии через REST

POS сначала генерирует ephemeral X25519 key pair/installation public key и вызывает:

```http
POST /public/v1/registers/{register_id}/ble-sessions
Authorization: Bearer <operator-token>
X-Api-Version: 2026-08-07
Idempotency-Key: ble-session-00000001
Content-Type: application/json

{
  "app_instance_id": "app-installation-uuid",
  "operator_id": "operator-id",
  "public_key": "base64url-client-public-key"
}
```

Ответ содержит:

- `ble_session_id`;
- tenant/location/register binding;
- `edge_id` и конечный `device_id`;
- `binding_version`;
- protocol/service/characteristic UUID;
- advertising identity и edge public key;
- signed session ticket;
- device-bound client key envelope;
- expiry.

Сессия короткоживущая, привязана к actor subject, app instance, operator, register, Edge и конечному ФУ. Refresh/revoke выполняются только REST:

```http
POST /public/v1/ble-sessions/{id}/refresh
POST /public/v1/ble-sessions/{id}/revoke
```

Rebind/deactivation operator/register/device или увеличение binding version делает старую BLE authority недействительной.

### 7.3 Discovery и handshake

Gateway работает BLE peripheral. POS выбирает advertisement только по `advertising_identity` из server session, а не по имени/MAC.

Handshake v1:

1. POS подключается к server-provided service UUID.
2. POS отправляет canonical CBOR `HELLO` с protocol version, session ID, signed ticket, 16-byte client nonce и X25519 ephemeral public key.
3. Gateway проверяет signature, expiry, scope, tenant/register/device/binding и revoke state.
4. Gateway отвечает `CHALLENGE`: 16-byte edge nonce, edge ephemeral public key, `max_chunk`, `window`.
5. Стороны получают shared secret X25519 и выводят независимые directional keys/nonces через HKDF-SHA-256 с ticket digest, обоими nonces и context.
6. POS отправляет encrypted `AUTH_PROOF`.
7. Gateway отвечает encrypted `READY` с next expected counter и flow-control parameters.

Crypto suite: `X25519 + HKDF-SHA-256 + AES-256-GCM`. BLE pairing/proximity сами по себе не являются авторизацией.

### 7.4 Frames

Binary frame header:

```text
version(1) | type(1) | flags(2) | message_id(16)
counter(8) | chunk_index(2) | chunk_count(2) | ciphertext_length(2)
```

- Payload — canonical CBOR deterministic encoding.
- Полный header используется как AES-GCM AAD.
- Nonce — 4-byte directional prefix + 8-byte counter.
- Counter строго возрастает отдельно в каждом направлении и сохраняется до принятия fiscal intent.
- Counter reuse/rollback, invalid tag, conflicting chunk или expired ticket немедленно закрывает session.
- `message_id`/`intent_id` обеспечивают durable deduplication; chunking только транспортный.
- MTU/chunk/window берутся из handshake, а не hardcode POS.

### 7.5 ComplianceIntent

BLE передаёт canonical CBOR представление OpenAPI-схемы `ComplianceIntent`:

```json
{
  "intent_id": "550e8400-e29b-41d4-a716-446655440101",
  "action": "OPEN_WITH_LINE",
  "client_sale_surrogate_id": "550e8400-e29b-41d4-a716-446655440000",
  "operator_code": "A001",
  "app_instance_id": "550e8400-e29b-41d4-a716-446655440102",
  "expected_version": 0,
  "line": {
    "line_id": "550e8400-e29b-41d4-a716-446655440001",
    "product_code": "COFFEE-1",
    "name": "Кафе",
    "quantity": "1.000",
    "unit_price": "2.50",
    "tax_group": "B"
  }
}
```

Actions: `OPEN_WITH_LINE`, `ADD_LINE`, `CHANGE_LINE`, `CANCEL_LINE`, `CANCEL_SALE`, `PAYMENT`, `REVERSE`.

Ответ `ComplianceIntentResult`:

- `operation_id`;
- `state`: `FISCALIZED`, `FAILED`, `FISCAL_RESULT_UNKNOWN`, `BLOCKED`;
- `server_sale_id`, `version`;
- opaque `regulatory_identifiers`;
- `fiscal_reference` либо `error_code`.

Повтор того же `intent_id` возвращает durable сохранённый результат без повторного вызова ФУ. Другой payload с тем же ID отклоняется.

### 7.6 Offline journal и sync

Gateway до side effect атомарно сохраняет intent, binding и counters в SQLite/SD journal. После восстановления cloud он отправляет append-only batches:

```http
POST /public/v1/edge-sync/batches
Content-Type: application/cbor
```

Локальные записи хранятся минимум три месяца и удаляются только после durable business acknowledgement backend. Transport ack/MQTT/WebHook не заменяет business acknowledgement.

Если disconnect произошёл после возможного выполнения ФУ, результат — `FISCAL_RESULT_UNKNOWN`. POS не переключается на REST и не повторяет PAYMENT/SALE. Оно блокирует новую конфликтующую операцию до reconciliation.

## 8. Route selection

```text
Получить readiness/recommended transport
             |
       +-----+-----+
       |           |
     REST         BLE
       |           |
 send intent   validate live session + final-device probe
       |           |
       +-----+-----+
             |
     Accepted operation_id
             |
     WebHook and/or REST status
```

Правила:

1. `REST` — использовать public REST.
2. `BLE` — только действующая session и подтверждённый конечный ФУ.
3. `BLOCK` — продажа не отправляется.
4. Route фиксируется в operation journal перед отправкой.
5. Timeout не разрешает второй route.
6. После restart POS восстанавливает pending operations до новой продажи/оплаты.

## 9. WebHook + polling consistency

POS поддерживает inbox/outbox/read-model:

- `fiscal_operation_links`: POS aggregate ↔ BeeFiscal operation/sale;
- `webhook_inbox`: raw body digest, event ID, version, received/applied timestamps;
- `pending_commands`: idempotency key, request digest, route, operation ID, state;
- `sync_cursor`/last resource version для recovery.

WebHook ускоряет UI, но authoritative state остаётся REST projection. При следующих условиях выполняется polling:

- WebHook gap/out-of-order;
- restart receiver;
- pending operation дольше SLA;
- signature failure/security alert;
- reconciliation required;
- endpoint был disabled/rotated.

## 10. Security requirements

- Только TLS 1.2+; production URLs — HTTPS/WSS.
- OIDC tenant/subject/scopes проверяются backend.
- Webhook secret уникален для endpoint и хранится в secret manager.
- Логи содержат IDs/digests/trace ID, но не token, webhook secret, BLE keys или payment data.
- BLE private keys находятся в platform keystore и не экспортируются.
- CORS origin list явный; CORS не заменяет authentication.
- POS не принимает event другого tenant даже при валидной подписи неправильного endpoint.
- Все request/response/event/BLE business payload schemas фиксируются OpenAPI; transport framing не дублирует business model.
- Неизвестная версия API/protocol/event/schema обрабатывается fail-close.

## 11. Минимальный onboarding внешнего POS

1. Получить sandbox tenant, OIDC client и Fiscal Caddy base URL.
2. Сгенерировать client из locked OpenAPI.
3. Реализовать token lifecycle без persistent plaintext secrets.
4. Настроить tenant-owned location/register/workstation/operator bindings.
5. Реализовать REST bootstrap, sale lifecycle, idempotency и reconciliation.
6. Зарегистрировать HTTPS WebHook, сохранить one-time secret и проверить structured signature.
7. Реализовать durable WebHook inbox и polling recovery.
8. При необходимости BLE — реализовать GATT v1, canonical CBOR, crypto/counters и Local Compliance Intent; иначе при потере REST POS работает fail-close.
9. Пройти contract tests и sandbox E2E для CASH/CARD/split/cancel/storno/timeout/UNKNOWN.
10. BLE/физическое ФУ/платёжный терминал проходят отдельный HIL; STUB не является production evidence.

## 12. Критерии совместимости

Интеграция считается совместимой, если:

- использует только публичный OpenAPI REST surface;
- первый товар создаёт sale/line/УНП одной atomic mutation;
- поддерживает `Idempotency-Key`, `If-Match`, `202` и `UNKNOWN` без двойного side effect;
- проверяет structured WebHook HMAC по raw body, дедуплицирует event и восстанавливается polling;
- не доверяет порядку WebHooks без `resource_version`;
- BLE использует server-issued binding ticket, X25519/HKDF/AES-GCM, counters и canonical CBOR;
- не переключает route после принятия intent;
- блокирует продажу при недоступности конечного ФУ;
- не содержит vendor fiscal protocol и не подключается к MQTT SmartDevices;
- хранит opaque regulatory identifiers без изменения;
- проходит OpenAPI drift, REST/WebHook contract и end-to-end tests.
