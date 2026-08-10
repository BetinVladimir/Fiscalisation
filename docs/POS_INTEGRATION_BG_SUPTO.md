# Интеграция нового POS с BeeFiscal (BG SUPTO)

Версия контракта: `2026-08-07`; профиль идентификаторов: `BG_SUPTO_FULL / BG_UNP_V1 / 2026-08-10.1`; валюта: EUR. Машиночитаемый контракт — [`../contracts/openapi-runtime-v1.yaml`](../contracts/openapi-runtime-v1.yaml), типы TypeScript — [`../contracts/generated/openapi-runtime-v1.d.ts`](../contracts/generated/openapi-runtime-v1.d.ts).

## 1. Граница ответственности

POS — UI-клиент намерений. Он может создавать UUID только для `client_sale_surrogate_id`, `line_id`, `payment_id` и correlation/idempotency. POS не формирует УНП, номер чека, storno reference, fiscal command, offline fiscal envelope, маршрут REST/BLE или authoritative totals. Значения из `regulatory_identifiers[]` непрозрачны: POS хранит и показывает их без parsing/formatting.

Все online/offline пути заканчиваются в Compliance Gateway. BLE является transport к локальному Gateway, а не альтернативной фискальной реализацией POS. После отправки команды запрещено переключать маршрут; неоднозначный результат переводится в `UNKNOWN` и разрешает только status lookup/reconciliation.

## 2. Обязательный bootstrap

1. Получить OIDC token пользователя и создать/восстановить персональную operator session. Общие кассирские учётные записи запрещены.
2. Выбрать выданный сервером `workstation_id`; не использовать локальный alias вместо UUID.
3. Открыть смену публичным API.
4. Вызвать `POST /workstations/{id}/clock-sync` один раз на business date и после сообщения `CLOCK_SYNC_FAILED`/замены ФУ.
5. Вызвать `POST /workstations/{id}/readiness:refresh`. Lease не может быть длиннее двух часов, подписан backend и связан с workstation, ФУ, FMIN и profile version.
6. Показывать готовность только по ответу `GET /workstations/{id}/readiness`. При `409` продажа блокируется до успешного refresh.

Все mutations требуют `Authorization: Bearer`, `X-Api-Version` и уникальный `Idempotency-Key` длиной 16–255. Mutation существующего aggregate дополнительно требует `If-Match` с текущей версией/ETag.

## 3. Первая строка — создание продажи

Первый tap товара выполняет ровно один запрос:

```http
POST /public/v1/sales:open-with-line
Authorization: Bearer <access-token>
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

Только после `201` строка становится принятой. Response содержит authoritative sale projection, `unp`, `regulatory_identifiers[]`, immutable snapshot ФУ, `version` и `allowed_actions`. При timeout повторяется тот же body с тем же idempotency key; новый key/UUID до status lookup запрещён.

```ts
const opened = await client.POST('/sales:open-with-line', {
  headers: {'X-Api-Version': API_VERSION, 'Idempotency-Key': key},
  body: intent,
});
if (opened.error) throw opened.error;
renderOpaque(opened.data.regulatory_identifiers);
renderActions(opened.data.allowed_actions);
```

## 4. Последующие действия

- `GET /sales/{sale_id}` — recovery после restart/re-authentication.
- `POST /sales/{sale_id}/lines` — добавить строку; `If-Match` обязателен.
- `PATCH /sales/{sale_id}/lines/{line_id}` — изменить строку компенсирующим событием.
- `POST /sales/{sale_id}/lines/{line_id}:cancel` — отменить строку, не удаляя evidence.
- `POST /sales/{sale_id}:cancel` — отменить неоплаченную продажу с сохранением УНП.
- `POST /sales/{sale_id}/payment-intents` — передать намерение CASH/CARD. Backend повторно проверяет ФУ непосредственно перед side effect, рассчитывает остаток и выбирает терминал/route.
- `POST /sales/{sale_id}:reverse` — storno завершённой продажи по server policy с исходными ссылками и тем же regulatory binding.
- `GET /operations/{id}` и `POST /operations/{id}:reconcile` — единственный допустимый путь после `UNKNOWN`; не повторять оплату.

POS отображает только кнопки из `allowed_actions`. Локальная optimistic projection может иметь только `PENDING_SERVER_ACCEPTANCE`; её нельзя считать продажей, включать в отчёты или восстанавливать как authoritative cart.

## 5. BLE/offline

REST создаёт короткоживущую BLE session, привязанную к tenant/location/workstation/operator/app instance/конечному ФУ и client public key. После handshake POS посылает `ComplianceIntent` из [`openapi-runtime-v1.yaml`](../contracts/openapi-runtime-v1.yaml) локальному Compliance Gateway. Для co-located/native bridge тот же контракт доступен как защищённый loopback `POST /local/v1/intents`; BLE передаёт canonical CBOR той же схемы в зашифрованном frame. Это не публичный internet endpoint и не raw device-command API. Gateway владеет country policy bundle, readiness, FMIN, fenced UNP range, SQLite/SD append-only journal и sync. POS никогда не получает диапазон УНП или signing authority.

Минимальный offline intent:

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
    "name": "Кафе",
    "quantity": "1.000",
    "unit_price": "2.50",
    "tax_group": "B"
  }
}
```

Ответ содержит `operation_id`, `state`, `version` и opaque `regulatory_identifiers`. Gateway атомарно добавляет связку surrogate → regulatory identifier в durable journal до вызова ФУ; все последующие line/payment/cancel/reverse intents обязаны найти и повторно использовать эту связку. Повтор `intent_id` возвращает сохранённый результат без второго device call.

Если cloud недоступен, но `POS → BLE → Gateway → ФУ` подтверждён, Gateway принимает intent. Если потеряно конечное ФУ, операция блокируется. Если side effect мог произойти, состояние `UNKNOWN`; автоматический переход на другой маршрут запрещён. Отправленные backend события удаляются с SD не ранее трёх месяцев и только после durable acknowledgement.

## 6. Обработка ошибок

POS должен иметь болгарский словарь canonical Problem `code`, логировать correlation ID без token/PII и fail-close неизвестный code. Минимальные блокирующие состояния: `WORKSTATION_NOT_READY`, `CLOCK_SYNC_FAILED`, `OPERATOR_SESSION_INVALID`, `SALE_OPEN_REJECTED`, `IF_MATCH_REQUIRED`, `FISCAL_RESULT_UNKNOWN`, `IDEMPOTENCY_OUTCOME_UNKNOWN`. В последних двух случаях показывается отдельный reconciliation screen и блокируется новая оплата.

## 7. Приёмка нового POS

- первый tap создаёт server sale/first line/УНП одной mutation;
- failure не оставляет товар как открытую продажу;
- reload восстанавливает server projection и тот же opaque identifier;
- повтор key/body возвращает тот же ресурс, иной body с тем же key отклоняется;
- в bundle отсутствуют BG_UNP formatter, vendor protocol, fiscal route selector и offline envelope builder;
- storage не содержит access/refresh token, private key, authoritative sale или regulatory number;
- tenant, operator, stale version, expired lease, rebind ФУ и exhausted offline authority fail-close;
- CASH/CARD/split/UNKNOWN/cancel/storno проверены contract + E2E; physical adapter отдельно проходит HIL.

Команды проверки репозитория: `make contract-test`, `make supto-trace-test`, `make regression`; перед production дополнительно `make full-regression` и подписанный внешний HIL/legal evidence pack.

## 8. Текущее ограничение rollout

BeeMiniPOS переведён на server projection: первый tap вызывает atomic Fiscal API, line mutations и payment intents идут непосредственно в Fiscal Core, а старые checkout-first/local-cart authority модули удалены. Local Compliance Gateway, fenced offline authority, real PostgreSQL concurrent allocator и REST/BLE intent contracts реализованы и покрыты software tests. Профиль всё равно нельзя активировать до selected-device physical HIL, доверенного release/security evidence, production IdP/residency evidence и внешней регуляторной/сервисной приёмки; эти факты нельзя заменить STUB или локальной подписью.
