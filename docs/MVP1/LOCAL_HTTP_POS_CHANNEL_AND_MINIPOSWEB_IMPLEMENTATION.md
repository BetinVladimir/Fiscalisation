# Локальный REST/HTTP канал и эталонный MiniPOS Web

Статус: `P0 OPEN — implementation specification`  
Дата: 2026-08-14

## 1. Цель MVP

Добавить третий, единообразный канал исполнения фискальных операций:

```text
Cloud REST/WebHook  |  Local REST/HTTP  |  Direct BLE
                              ↓
                 один canonical fiscal intent
                              ↓
                один durable journal/device saga
```

Local HTTP endpoint и проверенный кэш Web SPA должны одинаково работать в:

- `IoT/firmware/edge-agent-s3` — ESP-IDF, SPA на SD, HTTP в локальной сети;
- `SmartDevices/bluecash-app` — Android, SPA во внутреннем app storage, локальный
  HTTP server;
- `minipos/miniposweb` — новый эталонный React Web SPA;
- `minipos/beeminipos-backend` — единственный backend MiniPOS и PostgreSQL.

HTTP разрешён только в изолированной локальной сети MVP. Production hardening
обязан добавить HTTPS/mTLS либо иной защищённый transport. Bearer token по
открытой недоверенной сети запрещён.

## 2. Решение по технологии UI

`React` и `.NET MAUI` — разные UI/runtime платформы. Для Web SPA принимается:

- `minipos/miniposweb`: React 19 + TypeScript + Vite;
- общие canonical types/intent builders выносятся из `BeeMiniPOS` в
  `minipos/packages/pos-protocol`;
- существующий Expo/React Native `BeeMiniPOS` остаётся Android/iOS/Web клиентом;
- MAUI не вводится в MVP: он не требуется для SPA и создаст второй runtime и
  дублирование контрактов. При необходимости MAUI позднее может быть только
  WebView/native container над тем же подписанным bundle.

## 3. Границы ответственности

### POS backend

`minipos/beeminipos-backend` отвечает за login/OIDC, сотрудников, товары,
торговую точку, кассу, смены, продажи, токены локального исполнения и собственную
PostgreSQL. Он интегрируется с fiscal-backend только через public OpenAPI.

### POS Web App

`miniposweb` отвечает за локальный кэш справочников, токенов, смены, черновика
чека, route state и reconciliation UI. Адаптер не становится POS database.

### Fiscal adapter

Edge/BlueCash отвечает только за:

- доставку и проверку SPA bundle;
- локальный HTTP API фискализации;
- offline validation scoped token;
- durable reservation/idempotency;
- fiscal/payment device saga;
- MQTT sync после восстановления cloud.

Адаптер не хранит товары, сотрудников или POS credentials и не выдаёт POS login
token.

## 4. Единый local HTTP API

Добавить machine-readable OpenAPI:

`contracts/openapi-local-adapter-v1.yaml`

Base URL:

```text
http://<adapter-ip>/beeloy/local/v1
```

Обязательный surface:

| Method/path | Назначение |
|---|---|
| `GET /healthz` | дешёвая liveness без device I/O и БД |
| `GET /readyz` | binding, journal и mandatory endpoint readiness |
| `GET /device` | adapter/endpoints identity, capabilities, generation |
| `GET /deployment` | активная версия SPA и состояние обновления |
| `POST /intents` | canonical `SALE_FINALIZE`, `SALE_REVERSE`, reports, cash, `PRINTER_TEST` |
| `GET /operations/{uuid}` | authoritative persisted result/unknown state |
| `POST /operations/{uuid}:reconcile` | lookup-only recovery, без повторного side effect |

`POST /intents` принимает ровно тот же canonical payload, UUID, receipt session,
ordered items/payments, discounts и digest, что MQTT/BLE. Отдельные HTTP-only
бизнес-команды запрещены.

Обязательные headers:

```text
Authorization: Bearer <local-fiscal-token>
Idempotency-Key: <client_operation_id UUID>
X-Beeloy-API-Version: 2026-08-14
Content-Type: application/json
```

Ответы: `202` для принятой durable операции, `200` для сохранённого duplicate,
`409 IDEMPOTENCY_PAYLOAD_CONFLICT`, `401/403`, `422`, `429`, `503`. До ответа
`202` operation и canonical digest должны быть зафиксированы на SD/SQLite или в
Android Room/SQLite.

### Cross-channel invariant

Одна operation может начаться через Cloud, Local HTTP или BLE и быть проверена
через другой канал. Ключ дедупликации — `(tenant_id, register_id,
client_operation_id)` плюс immutable payload digest. Физический I/O выполняется
не более одного раза. HTTP handlers обязаны вызывать существующий общий intent
processor, а не драйвер напрямую.

## 5. Токен локального исполнения

Токен получает `miniposweb` у собственного backend:

```text
POST /public/v1/minipos/fiscal-local-tokens
```

Backend авторизует сотрудника/смену/кассу, сверяет active fiscal binding через
public Fiscal API и выдаёт короткоживущий JWT:

- `iss` — MiniPOS backend;
- `aud` — `beeloy-local-fiscal-adapter` и конкретный adapter/device ID;
- `sub`, `tenant_id`, `location_id`, `register_id`, `operator_id`, `shift_id`;
- scopes из закрытого списка;
- `jti`, `iat`, `nbf`, `exp`, `binding_generation`;
- не более 15 минут для MVP.

Edge/BlueCash валидируют JWT локально по pinned issuer key/JWKS snapshot,
доставленному подписанным composite binding. Проверяются алгоритм, `kid`, все
claims, clock skew, generation и scope. `alg=none`, symmetric key from browser,
неизвестный key и expired token отклоняются. При offline истечении токена POS
блокирует новые операции; уже durable operation можно только читать/reconcile.

Хранение/обновление token и справочников — ответственность POS App. Для Web
предпочтителен memory token; если offline persistence обязательно, использовать
IndexedDB с WebCrypto-wrapped key и документированным риском. Не помещать token
в URL, local logs, descriptor или static files.

## 6. Beeloy deployment descriptor

Канонический путь в source hosting:

`.well-known/beeloy-pos-deployment.json`

Schema: `contracts/beeloy-pos-deployment-v1.schema.json`. Canonical JSON
подписывается Ed25519 release key, публичный ключ pin-ится configuration plane.

Минимальная структура:

```json
{
  "schema_version": 1,
  "application_id": "com.beeloy.miniposweb",
  "version": "1.0.0",
  "build_id": "sha256:...",
  "created_at": "2026-08-14T00:00:00Z",
  "minimum_adapter_api": "2026-08-14",
  "entrypoint": "index.html",
  "files": [
    {"path":"index.html","size":1234,"sha256":"...","media_type":"text/html"},
    {"path":"assets/app.abcd.js","size":1234,"sha256":"...","media_type":"text/javascript"}
  ],
  "signature": {"kid":"minipos-release-1","alg":"Ed25519","value":"..."}
}
```

Требования:

- полный allow-list файлов, без `..`, absolute path, duplicate или Unicode path
  ambiguity;
- SHA-256 и точный размер каждого файла, общий лимит и лимит файла;
- только разрешённые media types; HTML/JS запрещено исполнять до проверки всего
  bundle;
- descriptor version/build не может откатиться без подписанного rollback permit;
- bundle содержит только content-hashed assets; никаких секретов/environment
  credentials;
- `index.html` с `Cache-Control: no-cache`, hashed assets —
  `public,max-age=31536000,immutable`, descriptor — `no-store`;
- CSP как минимум `default-src 'self'; connect-src 'self' http:; object-src
  'none'; base-uri 'none'; frame-ancestors 'none'`. Inline/eval запрещены.

## 7. Проверка, обновление и A/B cache

### Общий алгоритм

1. Получить descriptor через configured HTTPS source URL.
2. Проверить TLS, signature, application ID, version policy и API compatibility.
3. Проверить свободное место и скачать файлы в inactive slot.
4. Для каждого файла проверить size/SHA-256; затем проверить полноту bundle.
5. `fsync`, записать verified marker и атомарно переключить active slot.
6. Выполнить local smoke load `index.html` и assets.
7. Опубликовать MQTT state `AVAILABLE/DOWNLOADING/VERIFIED/ACTIVE/FAILED`.
8. При boot failure вернуть last-known-good slot.

Незавершённая загрузка никогда не видна HTTP clients. Обновление не прерывает
уже принятую fiscal operation. Проверка выполняется при boot, MQTT
`DEPLOYMENT_CHECK`, периодически с jitter и вручную из BeeFiscalApp.

### ESP32-S3

Новые пути:

- `IoT/firmware/edge-agent-s3/idf/main/local_http_server.{h,cpp}`;
- `.../spa_deployment_manager.{h,cpp}`;
- `.../local_token_validator.{h,cpp}`;
- `.../deployment_descriptor.{h,cpp}`.

Bundle хранится на SD: `/sdcard/beeloy/spa/slot-a|slot-b`; SQLite хранит
deployment state/history. Задать compile-time/request limits, bounded workers и
не загружать bundle целиком в RAM. HTTP server использует ESP-IDF `esp_http_server`.

### BlueCash Android

Новые packages:

- `.../localhttp/BlueCashLocalHttpServer.kt`;
- `.../deployment/SpaDeploymentWorker.kt`;
- `.../deployment/DeploymentStore.kt`;
- `.../security/LocalFiscalTokenVerifier.kt`.

Использовать Android internal app storage, atomic directory rename и WorkManager
для update/retry. Server стартует только после active device binding, слушает
на configurable LAN interface, не публикует Android management API.

## 8. Local network discovery и безопасность MVP

- mDNS service `_beeloy-fiscal._tcp`, TXT только protocol version, adapter ID
  suffix и active deployment version; никаких token/tenant/customer data;
- bind не на cellular/VPN/all interfaces, если политика устройства это позволяет;
- CORS allow-list генерируется из active local origins; wildcard с Authorization
  запрещён;
- Host/Origin validation, request/body limits, timeouts, rate limiting;
- local HTTP server не предоставляет directory listing, upload или arbitrary
  filesystem access;
- операции и ответы не кэшируются HTTP proxy/browser;
- журнал аудирует token `jti`/subject без самого token.

Обычный HTTP не защищает token от пассивного наблюдателя и MITM. MVP допускает
его только для доверенной изолированной LAN с зафиксированным risk acceptance.
Это production P0, но не отменяет функциональный MVP.

## 9. `minipos/miniposweb` — эталонная реализация

Создать:

```text
minipos/miniposweb/
  package.json
  vite.config.ts
  src/api/minipos.ts
  src/api/localFiscal.ts
  src/fiscal/routeController.ts
  src/fiscal/intentBuilder.ts
  src/storage/indexedDb.ts
  src/screens/Login.tsx
  src/screens/Shift.tsx
  src/screens/Sale.tsx
  src/screens/OperationRecovery.tsx
  src/screens/Settings.tsx
  public/.well-known/beeloy-pos-deployment.json  # build output, не source secret
  tests/
```

Функциональный минимум:

- login только через `beeminipos-backend`;
- одна точка/касса из user context, смена, товары, сотрудники, touch sale UI;
- CASH/CARD/split, optional line discount, aggregate `SALE_FINALIZE`, storno;
- immutable checkout plan и UUID до первой отправки;
- route state machine `CLOUD -> LOCAL_HTTP -> BLE` с hysteresis; Web SPA по HTTP
  не использует Web Bluetooth как обязательный путь;
- операция может продолжаться по другому каналу через lookup с тем же UUID;
- IndexedDB outbox/reference cache и reconciliation после restart;
- local adapter discovery может быть настроен вручную и через mDNS helper;
- диагностический экран показывает backend, adapter, ФУ, POS, deployment
  version и printer test при разрешённой роли.

`beeminipos-backend` расширить token endpoint, adapter association, public key
rotation/JWKS, audit и POS-owned reference sync. Прямой MQTT и доступ к fiscal
PostgreSQL остаются запрещены.

## 10. Route policy

1. Cloud считается доступным только после лёгкого fiscal ping и валидной route
   package; бизнес timeout не доказывает failure и требует lookup.
2. При cloud unavailable использовать Local HTTP, если `/readyz` свеж и identity,
   register, binding generation совпадают.
3. BLE — последний локальный transport для клиентов, где он поддержан.
4. Возвращаться на cloud после N успешных ping с hysteresis; смена transport не
   меняет UUID/digest.
5. `UNKNOWN` никогда не переисполняется; только lookup/reconcile.
6. Фактический результат Local HTTP синхронизируется адаптером через MQTT, затем
   fiscal-backend materializes operation/WebHook как для BLE.

## 11. Этапы кодогенерации

1. Добавить OpenAPI local adapter и JSON Schema descriptor; сгенерировать
   TypeScript/Kotlin/C++ contract fixtures.
2. Вынести общий canonical intent processor boundary в ESP-IDF и Android.
3. Реализовать token issuer в MiniPOS backend и offline verifiers.
4. Реализовать HTTP server edge-agent, затем BlueCash теми же conformance tests.
5. Реализовать A/B deployment managers и signed build publisher.
6. Создать `miniposweb`, IndexedDB journal и route controller.
7. Добавить build pipeline: Vite build → manifest inventory → digest → sign →
   descriptor → reproducibility verification.
8. Добавить BeeFiscalApp deployment/config/status controls согласно
   `DEVICE_CONFIGURATION_AND_REALTIME_UI_REMEDIATION.md`.
9. Выполнить fault regression, compose E2E и stub integration; только затем HIL.

Каждый этап отдельным reviewable change; generated files не правятся вручную.

## 12. Тестовая стратегия

### Contract/golden

- один intent JSON/digest/result для Cloud, Local HTTP, MQTT и BLE;
- OpenAPI request/response validation для C++, Kotlin, TypeScript, Go;
- descriptor canonicalization/signature/path traversal/missing-extra-file vectors;
- JWT valid/expired/wrong audience/register/generation/kid/algorithm vectors.

### Unit/component

- route hysteresis, cross-channel UUID/digest, IndexedDB restart/outbox;
- HTTP auth/CORS/Host/rate/body/time limits;
- A/B update power loss после каждого durable boundary;
- full SD/storage, corrupt download, rollback attempt, incompatible API;
- parallel request and duplicate handling;
- printer test не создаёт fiscal sale/payment.

### Integration/E2E со stub driver

```text
MiniPOS login/catalog/shift
→ miniposweb loads from adapter cache without Internet
→ token was issued by beeminipos-backend
→ cloud route fails
→ Local HTTP accepts SALE_FINALIZE
→ card + fiscal stub execute once
→ browser loses response and uses GET operation
→ adapter MQTT sync reaches fiscal-backend
→ WebHook reaches MiniPOS backend
→ UI shows one completed sale
```

Повторить для cash/card/split/discount/storno, decline, printer failure, unknown,
restart, Local HTTP→BLE и Local HTTP→Cloud continuation. Запустить одинаково
против ESP-IDF host harness и Android instrumentation server.

### Non-functional

- ESP heap/stack/leak and concurrent HTTP/MQTT/BLE/device I/O soak;
- bundle update при активных продажах;
- cold load и route switch SLA;
- 72-hour fault soak; power cut/restart matrix;
- security tests: traversal, malformed HTTP, token replay, cross-tenant, XSS/CSP,
  rogue descriptor/source and downgrade.

## 13. Критерии приёмки MVP

1. Один опубликованный signed bundle устанавливается и одинаково открывается с
   edge-agent и BlueCash по LAN HTTP без Internet.
2. Corrupt/unsigned/partial/downgrade bundle не становится active; last-known-good
   остаётся доступен после restart.
3. MiniPOS backend является единственным issuer POS identity/local token и
   единственным источником справочников.
4. Local HTTP принимает весь обязательный fiscal command surface, включая card,
   storno/refund и `PRINTER_TEST`, через общий processor/journal.
5. Дубликаты и смена Cloud/HTTP/BLE не создают второй чек или платёж.
6. Edge SD и Android journal переживают power/process loss и синхронизируются
   MQTT после восстановления.
7. Все контракты описаны OpenAPI/JSON Schema и проверены generated clients/golden
   tests; ручных divergent DTO нет.
8. `miniposweb`, `beeminipos-backend`, PostgreSQL и fiscal stub поднимаются
   compose profile DEV; production config fail-closed запрещает simulator и
   небезопасные defaults.
9. Unit, contract, component, Playwright, Android instrumentation, ESP host,
   compose E2E и regression PASS. После этого остаётся только physical HIL и
   formal risk acceptance локального HTTP.

До выполнения этих критериев статус — `SOFTWARE_INCOMPLETE_HIL_PENDING`.

