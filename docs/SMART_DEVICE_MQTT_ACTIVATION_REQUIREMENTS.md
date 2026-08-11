# Требования к активации SmartDevice и транспортному каналу

Статус: реализованный software baseline; production PKI/broker/HIL acceptance остаются внешними gates. Дата: 2026-08-11. Область: приложения `SmartDevices`, `BeeFiscalApp`, `fiscal-backend` и MQTT ingress.

As-built руководство и точные endpoints: [`BLUECASH_POS_INTEGRATION.md`](BLUECASH_POS_INTEGRATION.md). OpenAPI является нормативным контрактом: [`../contracts/openapi-runtime-v1.yaml`](../contracts/openapi-runtime-v1.yaml).

## 1. Результат анализа текущей реализации

Предложенный упрощённый алгоритм корректен и предпочтительнее ранее описанной передачи activation JWT через BeeFiscalApp:

1. регистратор создаёт собственную криптографическую identity;
2. backend хранит неподтверждённую заявку без tenant;
3. сотрудник подтверждает физическое устройство через QR;
4. tenant определяется access token BeeFiscalApp;
5. сотрудник выбирает только location, register и роли;
6. backend атомарно связывает устройство;
7. устройство самостоятельно получает подтверждение и начинает работу.

Это устраняет ручной ввод organization/location/device ID на регистраторе и исключает BeeFiscalApp из постоянного доверенного канала устройства.

Текущее состояние кода не соответствует этому алгоритму:

- `SmartDevices` имеет HTTPS proof-of-possession bootstrap, MQTT mTLS activation/command/sync clients и direct BLE transaction GATT;
- BlueCash принимает по BLE переносимый HS256 activation JWT и локально вводимые organization/location/device ID;
- `fiscal-backend/internal/mqttclient` — только заготовка: один backend client со статическими credentials подписывается на список тем и логирует topic/размер payload;
- MQTT message не проходит authentication/authorization на уровне device identity, schema validation, idempotency или domain handler;
- tenantless challenge/request хранятся отдельно миграцией `012_smart_device_activation.sql`; plaintext secret/code не сохраняется;
- MQTT broker/listeners/ACL/mTLS не входят в текущий Fiscal Compose; присутствующие EMQX env-переменные сами по себе transport не создают;
- отдельного WebSocket endpoint и протокола нет.

Legacy `activation-tokens` BLE-flow сохранён только для совместимости старых DEV-клиентов и формально исключён из production activation path. BeeFiscalApp его больше не вызывает.

## 2. Выбор MQTT вместо собственного WebSocket

Для постоянной связи фискального регистратора с backend выбирается MQTT 5.0:

- persistent session переживает временный обрыв сети;
- QoS 1 даёт доставку «как минимум один раз»;
- Message Expiry не позволяет выполнить просроченную команду;
- Response Topic и Correlation Data стандартизируют request/response;
- Last Will даёт backend быстрый connectivity signal;
- broker применяет device-level authentication и topic ACL независимо от application handler;
- один device client может получать команды и публиковать результаты/health без собственного WebSocket session protocol.

Эти возможности определены стандартом [OASIS MQTT 5.0](https://docs.oasis-open.org/mqtt/mqtt/v5.0/mqtt-v5.0.html). MQTT QoS не означает exactly-once business operation: QoS 1 допускает повтор, поэтому каждая команда всё равно обязана иметь `operation_id`, а SmartDevice — durable inbox/deduplication journal.

Основной transport — MQTT 5 over TLS на `8883` с mTLS. Для сетей, пропускающих только HTTPS, допускается MQTT 5 over secure WebSocket (`wss`) через Caddy/EMQX. Это тот же MQTT protocol, topics, payloads и ACL, а не второй прикладной WebSocket API.

EMQX поддерживает принудительную проверку клиентского X.509 certificate и привязку certificate identity к MQTT ClientID; topic authorization должна запрещать доступ к чужим устройствам. Источники: [EMQX TLS/mTLS](https://docs.emqx.com/en/emqx/latest/network/emqx-mqtt-tls.html), [EMQX X.509 authentication](https://docs.emqx.com/en/emqx/latest/access-control/authn/x509.html), [EMQX authorization](https://docs.emqx.com/en/emqx/latest/access-control/authz/authz.html).

Собственный WebSocket следует исключить: для него пришлось бы заново реализовать session persistence, reconnect, offline queue, ack, expiry, request correlation, presence и ACL. WebSocket имеет смысл только как transport MQTT over WSS.

## 3. Нужен ли activation envelope

Для основного online-сценария зашифрованный activation envelope не нужен.

Регистратор уже напрямую обращается к backend и доказывает владение неэкспортируемым private key. После подтверждения сотрудником backend возвращает регистратору:

- X.509 device certificate на ранее присланный public key/CSR;
- CA chain и адреса MQTT TLS/WSS listeners;
- подписанную binding configuration с `organization_id`, `location_id`, `register_id`, ролями и `binding_version`.

Certificate и binding configuration не содержат private key. Перехват certificate бесполезен без hardware-backed private key, поэтому дополнительное шифрование на ephemeral key усложняет lifecycle без дополнительной защиты. HTTPS TLS плюс proof-of-possession защищают bootstrap response.

Зашифрованный envelope требуется только как отдельно разрешённый fallback, если регистратор после создания заявки полностью потерял Internet, но BeeFiscalApp имеет связь с backend и может доставить ответ по BLE. Тогда envelope шифруется на public key из activation request, а устройство всё равно обязано выполнить online proof-of-possession commit до перехода в `ACTIVE`. Offline activation без server commit запрещена.

## 4. Отдельная tenantless таблица заявок

Backend должен создать отдельную таблицу, например `fiscal_device_activation_requests`. Она не является tenant business table и не должна использовать обычные tenant RLS/API repositories.

Минимальные поля:

- `activation_request_id UUID PRIMARY KEY`;
- `user_code_hash` — только hash короткого кода, не plaintext;
- `device_instance_id UUID`;
- `device_public_key_jwk` и `device_key_thumbprint` с unique constraint для незавершённых/активных identity;
- `attestation_chain` либо ссылка на зашифрованный evidence object;
- `attestation_verdict`, `attestation_security_level`;
- `vendor`, `model`, `serial`, `fmin`, firmware/app versions;
- `capability_digest` и заявленные adapter roles;
- `challenge_hash`, `challenge_expires_at`;
- `state`;
- nullable `claimed_tenant_id`, `claimed_location_id`, `claimed_register_id` — заполняются только server-side при подтверждении;
- `claimed_roles`, `binding_version`, `claimed_by_subject`, `claimed_at`;
- `expires_at`, `consumed_at`, `cancelled_at`, `created_at`, `updated_at`;
- attempt/rate-limit counters и audit correlation ID.

Разрешённые состояния:

```text
PENDING → CONFIRMED → CREDENTIAL_ISSUED → ACTIVE
PENDING → EXPIRED | CANCELLED | REJECTED
CONFIRMED → EXPIRED | CANCELLED | REJECTED
ACTIVE → QUARANTINED | REVOKED
```

До перехода в `CONFIRMED` все `claimed_*` обязаны быть `NULL`. Публичный bootstrap API может получить только запись по непрогнозируемому request secret и proof of possession; перечисление или поиск заявок запрещены. Просроченные незавершённые записи очищаются по retention policy, но audit факта активации сохраняется отдельно.

## 5. Целевой алгоритм активации

### Шаг 1. Создание hardware identity

При первом запуске SmartDevice:

1. создаёт в Android Keystore неэкспортируемый P-256 signing key; StrongBox предпочтителен, TEE допустим по policy;
2. получает backend nonce для key attestation;
3. внутренний vendor adapter читает vendor/model/serial/FMIN, firmware и capability set фискального регистратора/встроенного pinpad;
4. формирует CSR/public JWK, attestation chain и подписывает bootstrap request;
5. private key никогда не покидает Android Keystore и не резервируется в backup.

### Шаг 2. Создание непривязанной заявки

SmartDevice вызывает отдельный unauthenticated-by-tenant, но proof-of-possession bootstrap endpoint через Caddy HTTPS:

```text
POST /device-bootstrap/v1/activation-requests
```

Endpoint не использует employee OIDC token. Backend проверяет TLS, rate limit, attestation challenge, signature, uniqueness key/serial/FMIN, supported vendor/model/app signature и создаёт tenantless `PENDING` запись с TTL 10 минут.

Ответ содержит:

- `activation_request_id`;
- случайный высокоэнтропийный `request_secret`, возвращаемый только один раз;
- удобный `user_code`;
- `verification_uri`/QR payload;
- `expires_at` и polling interval.

В БД сохраняются hash `request_secret` и `user_code`; plaintext не сохраняется. `request_secret` остаётся только в memory/secure local storage SmartDevice и никогда не показывается в QR.

### Шаг 3. QR и сверка физического устройства

Регистратор показывает QR с:

- version;
- `activation_request_id`;
- `user_code`;
- verification URI;
- vendor/model и последние четыре символа serial/FMIN;
- expiry.

QR не содержит private key, request secret, будущий MQTT credential, organization/location/register или bearer access token.

BeeFiscalApp сканирует QR. При работе на компьютере сотрудник вводит короткий `user_code` на verification page. На регистраторе и BeeFiscalApp одновременно отображаются одинаковые code, model и serial/FMIN suffix; сотрудник подтверждает совпадение.

### Шаг 4. Tenant и бизнес-привязка

BeeFiscalApp уже работает в одном активном tenant. Backend получает `organization_id` только из проверенного employee OIDC token/session; компания не показывается как выбираемое поле и не принимается из request body.

Сотрудник с правом `FISCAL_DEVICE_ACTIVATE` выбирает:

1. `location_id` активного tenant;
2. `register_id`, принадлежащий location;
3. роль `FISCAL_DEVICE`;
4. опциональную роль `PAYMENT_TERMINAL`, только при подтверждённой capability и действующем acquirer provisioning.

BeeFiscalApp отправляет confirmation command с `activation_request_id`, location/register/roles и MFA/re-auth proof. Backend проверяет employee/tenant permissions, state/TTL, всю hierarchy и конфликты bindings.

### Шаг 5. Атомарное подтверждение

Backend одной транзакцией:

1. блокирует activation row;
2. повторно проверяет `PENDING`, TTL и отсутствие claim;
3. записывает `claimed_tenant_id` из token context, location/register/roles и actor subject;
4. создаёт tenant-owned `device` и draft register bindings либо связывает с заранее разрешённой записью;
5. устанавливает новый `binding_version`;
6. переводит request в `CONFIRMED`;
7. записывает immutable audit event.

Idempotency key возвращает тот же результат. Попытка второго tenant подтвердить ту же заявку получает generic conflict без раскрытия первого tenant.

### Шаг 6. Получение credential устройством

SmartDevice poll-ит bootstrap endpoint по HTTPS с `activation_request_id`, `request_secret` и свежей подписью server nonce:

```text
POST /device-bootstrap/v1/activation-requests/{id}/credential
```

До подтверждения backend отвечает `authorization_pending`; после expiry — `expired`; после отказа — `access_denied`. После `CONFIRMED` backend:

1. проверяет request secret и proof of possession первоначального key;
2. выдаёт X.509 client certificate на этот public key, но не private key;
3. возвращает MQTT endpoints/CA, server-signed binding configuration и минимальный topic policy identifier;
4. переводит request в `CREDENTIAL_ISSUED`.

Device certificate содержит стабильный `device_instance_id`/credential ID, но tenant/location/register authorization берётся из backend/broker policy, а не считается доверенной только из certificate text.

### Шаг 7. MQTT activation commit

SmartDevice подключается к MQTT 5 по mTLS:

- `ClientID = device_instance_id`;
- `Clean Start = true` для первого соединения;
- Last Will публикует offline state;
- broker сопоставляет certificate identity с ClientID и применяет deny-by-default ACL.

После CONNECT устройство публикует activation proof с QoS 1. Backend сверяет certificate/key, `binding_version`, hardware/capability digest и подтверждённую заявку. Только затем:

1. переводит activation request и device в `ACTIVE`;
2. активирует register bindings с `active_from`;
3. публикует подтверждение и effective configuration;
4. BeeFiscalApp получает server state и показывает «Устройство активно».

До этого commit устройство не принимает фискальные или платёжные команды.

## 6. MQTT identity и темы

ACL строятся по certificate-authenticated `device_instance_id`; wildcard tenant subscriptions устройства запрещены.

Рекомендуемый namespace:

```text
beefiscal/v1/devices/{device_instance_id}/commands
beefiscal/v1/devices/{device_instance_id}/results
beefiscal/v1/devices/{device_instance_id}/state
beefiscal/v1/devices/{device_instance_id}/events
beefiscal/v1/devices/{device_instance_id}/config
beefiscal/v1/devices/{device_instance_id}/activation
```

Device может:

- subscribe только на собственные `commands` и `config`;
- publish только в собственные `results`, `state`, `events`, `activation`.

Backend service identity может publish/subscribe по необходимым namespace. Anonymous access, shared device password, client-selected tenant topic и `#` ACL запрещены. Connectivity state не является юридическим доказательством выполнения команды.

## 7. Требования на переработку

### SmartDevices

- удалить ввод organization/location/device ID и локальный activation PIN;
- добавить Android Keystore identity, attestation и proof-of-possession;
- добавить HTTPS bootstrap client;
- добавить MQTT 5 client с mTLS, persistent reconnect, local durable inbox/outbox и certificate rotation;
- сохранить vendor Fiscal/Pinpad детали внутри adapter и публиковать только canonical capabilities/state;
- не активировать adapter до MQTT activation commit.

### fiscal-backend

- добавить отдельную tenantless activation table и repository/service с минимальными отдельными privileges;
- описать bootstrap и confirmation endpoints в OpenAPI;
- заменить logging-only MQTT callback на schema-validated router к domain services;
- добавить MQTT 5 response correlation, expiry, operation deduplication и durable processing;
- реализовать device CA/certificate issuance, rotation, revocation и broker authorization integration;
- не хранить/выдавать device private keys;
- связывать tenant только из employee token context.

### BeeFiscalApp

- QR scan/short-code lookup;
- неизменяемый tenant context из текущей session;
- выбор только location/register/roles;
- визуальная сверка model/serial/FMIN suffix и MFA confirmation;
- monitoring server state без отображения credentials;
- удалить обязательность BLE для online activation. BLE delivery оставить только как явно обозначенный fallback.

#### Страница активации SmartDevice

BeeFiscalApp должна иметь отдельную страницу `Smart Devices → Активировать устройство`, реализующую пошаговый мастер:

1. `Поиск устройства` — сканирование QR камерой либо ввод короткого `user_code`. Ручной ввод `activation_request_id`, organization ID, public key или credential запрещён.
2. `Сверка` — отображение одинакового short code, vendor/model, serial/FMIN suffix, firmware/app version, attestation status и времени истечения заявки. Сотрудник явно подтверждает, что данные совпадают с экраном физического регистратора.
3. `Точка подключения` — организация отображается неизменяемым значением из активного tenant; сотрудник выбирает только доступные ему `location_id` и принадлежащий ей `register_id`.
4. `Роли` — обязательная `FISCAL_DEVICE`; `PAYMENT_TERMINAL` доступна только при подтверждённой capability и отображает состояние acquirer provisioning. Недоступную роль нельзя включить только через изменение client request.
5. `Подтверждение` — итоговая карточка «устройство → tenant/location/register/roles», re-authentication/MFA и idempotent confirmation command.
6. `Ожидание устройства` — после подтверждения страница не показывает ложный успех, а наблюдает server state `CONFIRMED → CREDENTIAL_ISSUED → ACTIVE`. Показываются срок ожидания, последняя попытка устройства и безопасные действия `Повторить проверку`/`Отменить заявку`.
7. `Готово` — только после MQTT activation commit показываются effective binding, connection state, certificate expiry и ссылка на карточку устройства.

Мастер должен корректно отображать `authorization_pending`, expired QR, cancelled/rejected request, duplicate claim, чужой tenant, invalid attestation, неподдерживаемую firmware, потерю MQTT и уже активированное устройство. Возврат назад не должен создавать вторую заявку или binding. Resume после перезапуска BeeFiscalApp выполняется по server state, без хранения request secret или device credential.

#### Страница активных SmartDevices

BeeFiscalApp должна иметь tenant-scoped страницу `Smart Devices` со всеми устройствами, связанными с активным tenant. Сервер применяет tenant authorization; client-side filter не является механизмом изоляции.

Для каждого устройства отображаются:

- vendor/model, masked serial и FMIN;
- location и register;
- активные роли `FISCAL_DEVICE`/`PAYMENT_TERMINAL`;
- lifecycle state: `ACTIVE`, `OFFLINE`, `QUARANTINED`, `REVOKE_PENDING`, `REVOKED`, `REASSIGN_PENDING`;
- MQTT connectivity и `last_seen_at`, но с пояснением, что connectivity не подтверждает готовность конечного ФУ;
- readiness встроенного ФУ и payment terminal отдельными индикаторами;
- app/adapter/firmware versions и capability snapshot;
- certificate expiry/rotation status без certificate/private data;
- binding version, дата активации и активировавший сотрудник;
- незавершённые/unknown operations и требуемое reconciliation действие;
- последние безопасные audit/security события.

Страница поддерживает server-side pagination, поиск по разрешённым masked identifiers и фильтры location/register/role/state/connectivity. Детальная карточка устройства должна показывать историю bindings и действий, но никогда credentials, полный attestation chain, PAN/PIN или vendor secrets.

#### Отключение устройства от сети компании

Отключение является серверной security-операцией, а не удалением строки или локальным logout. В UI должны быть два разных действия:

1. `Приостановить (QUARANTINE)` — обратимая немедленная блокировка новых команд и MQTT authorizations с сохранением binding. Используется при диагностике, подозрении на компрометацию или временном выводе кассы из работы.
2. `Отвязать и отозвать (REVOKE)` — необратимый отзыв device certificate/credential и завершение binding. Повторное подключение возможно только через новую физическую активацию и новый key/certificate по policy.

Оба действия требуют права `FISCAL_DEVICE_DISABLE`, повторной аутентификации/MFA, причины из контролируемого списка и необязательного комментария. Перед подтверждением BeeFiscalApp показывает влияние: location/register/roles, открытая смена, незавершённые/unknown операции, активный payment terminal и время последней связи.

Backend выполняет отключение атомарно и идемпотентно:

1. блокирует device/binding record и повторно проверяет tenant/permission/current version;
2. увеличивает `binding_version`, чтобы все ранее выданные authority стали stale;
3. переводит device в `QUARANTINED` либо `REVOKE_PENDING`;
4. запрещает broker authentication и новые subscriptions/publishes для device identity;
5. закрывает активную MQTT session через broker administrative API;
6. при `REVOKE` отзывает certificate/credential и устанавливает `inactive_from` у bindings;
7. публикует command отключения только как best-effort notification — успех операции не зависит от того, находится ли устройство online;
8. сохраняет immutable audit event с actor, tenant, device, прежним/новым state, reason и correlation ID;
9. создаёт monitoring/security event для заинтересованных сотрудников.

Если имеются открытая смена либо fiscal/payment operations в `UNKNOWN`, обычное плановое отключение блокируется до reconciliation/закрытия по регламенту. Экстренный security revoke разрешён с отдельным правом и обязательным incident reason; незавершённые операции сохраняются и помечаются для ручного восстановления, а не удаляются.

SmartDevice при следующем connect/heartbeat получает отказ или signed revoked state, прекращает принимать команды, очищает operational tokens и показывает `Устройство отключено организацией`. Private hardware key можно удалить только после подтверждённого revoke/factory reset; само удаление ключа не заменяет server-side revoke.

Возврат из `QUARANTINED` выполняется отдельным действием `Восстановить доступ`: MFA, проверка binding version, новый certificate при необходимости и новый MQTT activation commit. `REVOKED` восстановлению не подлежит.

Все используемые страницами операции должны быть описаны в OpenAPI: lookup/confirm/cancel activation request, list/get SmartDevices, quarantine, restore, revoke и получение operation status. Командные endpoints требуют `Idempotency-Key` и optimistic concurrency (`If-Match`/expected binding version); UI не должен обращаться к broker administrative API напрямую.

### Deployment

- добавить EMQX в Fiscal Compose либо использовать управляемый broker;
- отдельные DEV/PROD TLS/WSS listeners, Caddy route для MQTT over WSS;
- mandatory mTLS для active devices, deny-by-default ACL и закрытый dashboard;
- certificate CA/KMS, revocation, backup и rotation runbooks;
- метрики connect/disconnect/auth failures/ACL denial/queue age/duplicate operations;
- HIL tests для offline reconnect, duplicate QoS 1 delivery, credential revoke/rotate и чужого topic access.

## 8. Критерии приёмки алгоритма

- неподтверждённая заявка не содержит tenant и не видна через tenant APIs;
- организация отсутствует среди выбираемых полей и всегда совпадает с tenant BeeFiscalApp token;
- один request нельзя подтвердить двумя tenant;
- QR/user code не позволяют получить credential без request secret и proof of private key;
- backend никогда не получает и не генерирует private device key;
- украденный certificate или binding document не работает без hardware key;
- MQTT connection без действующего certificate, с чужим ClientID или topic отклоняется;
- после подтверждения в BeeFiscalApp устройство не становится `ACTIVE`, пока не выполнит MQTT proof commit;
- повторные QoS 1 сообщения не создают повторную business operation;
- revoke немедленно закрывает новые MQTT sessions и блокирует команды;
- потеря cloud/MQTT связи отражается в monitoring, но не меняет binding самопроизвольно.
- мастер BeeFiscalApp не предлагает выбрать компанию и не показывает успех до MQTT activation commit;
- список SmartDevices содержит только устройства активного tenant и раздельно показывает MQTT connectivity, готовность ФУ и payment terminal;
- `QUARANTINE` блокирует устройство без удаления binding и может быть отменён только отдельной авторизованной операцией;
- `REVOKE` увеличивает binding version, отзывает credential, закрывает MQTT session и оставляет неизменяемый audit trail;
- offline-устройство отключается server-side немедленно: действие не ожидает доставки MQTT notification;
- плановое отключение при открытой смене/unknown operation блокируется, а emergency revoke требует отдельного права и incident reason.

## 9. Итоговая рекомендация

Упрощённый алгоритм следует принять. Ключевое уточнение: BeeFiscalApp не генерирует и не переносит рабочий token устройства. Оно подтверждает tenant-owned binding. Сам регистратор получает certificate и конфигурацию напрямую от backend, доказывая владение hardware key.

Целевой постоянный канал — MQTT 5 over mTLS, с MQTT over WSS через Caddy как совместимым transport fallback. Зашифрованный activation envelope нужен только для редкого BLE fallback и не должен быть обязательной частью online flow.
