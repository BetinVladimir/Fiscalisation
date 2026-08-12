# Задача: identity, выпуск, назначение и безопасное подключение Edge Agent S3

Статус: **техническое задание, реализация не начата**  
Дата: 2026-08-12  
Область: ESP32-S3 edge agent, manufacturing tooling, Fiscal Backend, BeeFiscalApp и AdminApp.

## 1. Результат задачи

Нужно реализовать единый жизненный цикл устройств `edge-agent-s3`:

1. одна общая production-прошивка устанавливается на партию устройств;
2. каждое устройство при первом доверенном запуске создаёт собственную
   неэкспортируемую асимметричную identity;
3. производственная станция регистрирует serial, public key и аппаратные данные
   в Fiscal Backend, не передавая backend приватный ключ устройства;
4. оператор платформы через AdminApp назначает произведённое устройство tenant;
5. администратор этого tenant через BeeFiscalApp привязывает устройство к
   торговой точке, кассовому месту и ролям;
6. Admin/POS работают с устройством по одному подписанному command protocol через
   MQTT или BLE;
7. BLE рассматривается как открытый недоверенный транспорт без pairing/bonding;
   авторизация обеспечивается backend-signed capability, подписью отправителя,
   ECDH и AEAD на application layer;
8. устройство хранит неподтверждённые транзакции в SQLite на SD и синхронизирует
   их через MQTT после восстановления связи.

Код в рамках этого документа менять нельзя. Настоящий документ является
декомпозированной задачей для последующей реализации.

## 2. Затрагиваемые проекты

| Контур | Абсолютный путь | Ответственность |
|---|---|---|
| Firmware | `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3` | Device identity, BLE GATT, secure session, MQTT, command validator, protocol facade, SD/SQLite |
| Protocol drivers | `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction` | Создание независимых fiscal/payment adapters по vendor и channel |
| Manufacturing tooling | `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/device-manufacturing` | Прошивка, назначение serial, регистрация public identity, evidence |
| Backend | `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend` | Device Registry, manufacturing registration, capabilities, assignment, activation, MQTT auth, revocation |
| Public contracts | `/Users/freelancer/Documents/Beeloy/Fiscalisation/contracts` | Полное OpenAPI-описание новых REST API и schemas |
| Tenant admin | `/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp` | Привязка уже назначенного tenant устройства к location/register, локальная BLE-настройка и мониторинг |
| Platform admin | `/Users/freelancer/Documents/Beeloy/Fiscalisation/AdminApp` | Все устройства платформы, manufacturing state, назначение/отвязка tenant, quarantine/retire |

`AdminApp` на момент подготовки задачи является пустым каталогом. Его создание —
часть задачи. Это отдельное platform-operator приложение, а не режим BeeFiscalApp.

## 3. Зафиксированные архитектурные решения

### 3.1 Корни доверия

- Backend/KMS владеет `BACKEND_PRIVATE_KEY`; он никогда не попадает в firmware,
  CI artifacts, frontend или manufacturing scripts.
- Firmware содержит trust store из `backend_root_key_id + BACKEND_PUBLIC_KEY`.
- Во время rotation устройство поддерживает текущий и следующий public key.
- Каждое устройство при первой инициализации самостоятельно генерирует
  уникальную пару `device_private_key/device_public_key` непосредственно на
  ESP32-S3. Внешний криптографический provider/secure element не используется.
- Backend хранит только public key и доказательства регистрации.
- Backend не создаёт индивидуальный server signing key для каждого устройства.
- Admin и POS создают собственные installation keypairs; backend знает только их
  public keys.
- TOFU запрещён: устройство не доверяет первому подключившемуся Admin/POS.

Для MVP алгоритмы необходимо зафиксировать однозначно:

- identity/signatures: ECDSA P-256 + SHA-256, выполняемые встроенным crypto stack
  ESP32-S3; canonical signature encoding IEEE
  P1363 (`r || s`, 64 bytes), public keys как uncompressed SEC1/JWK P-256;
- session agreement: ephemeral ECDH P-256;
- KDF: HKDF-SHA-256;
- AEAD: AES-256-GCM с 96-bit nonce и 128-bit tag;
- canonical payload: RFC 8785 JSON Canonicalization Scheme;
- opaque signed objects: CBOR/COSE не вводить в MVP, чтобы не иметь два
  canonical formats. Версия 2 может мигрировать на COSE отдельным protocol bump.

### 3.2 BLE

- BLE pairing, bonding, PIN и passkey отсутствуют.
- GATT advertising и connect доступны физически находящемуся рядом клиенту.
- Без авторизации разрешён только `device.info` и `session.challenge`.
- SSID, password, transaction contents и credentials передаются только внутри
  application-layer AEAD session.
- BLE MAC не является identity или фактором доверия.
- Один и тот же `SignedCommand` валидируется одинаково для MQTT и BLE.

### 3.3 Разделение административных полномочий

- `AdminApp`: platform scope, видит все tenant и все произведённые устройства;
  назначает `company_id`, приостанавливает и выводит устройство из эксплуатации.
- `BeeFiscalApp`: tenant scope из проверенного access token; компания не
  выбирается. Пользователь выбирает только `location_id`, `register_id` и роли.
- Перенос устройства между tenant выполняется только через AdminApp с audit и
  revoke текущего binding. BeeFiscalApp не имеет такого endpoint/UI.

## 4. Целевая модель состояний

### 4.1 Device lifecycle

```text
UNREGISTERED -> MANUFACTURED -> ASSIGNED -> DEPLOYED
                        |           |          |
                        +------> SUSPENDED <---+
                                     |
                                     +------> RETIRED
```

- `UNREGISTERED` существует только локально до manufacturing registration.
- `MANUFACTURED`: identity зарегистрирована, `tenant_id/location_id/register_id`
  отсутствуют.
- `ASSIGNED`: AdminApp назначил tenant, но tenant ещё не завершил deployment.
- `DEPLOYED`: BeeFiscalApp привязал location/register/roles, device подтвердил
  конфигурацию и вышел в MQTT.
- `SUSPENDED`: MQTT и privileged commands запрещены; доступны ограниченная
  диагностика и signed recovery.
- `RETIRED`: терминальное состояние; повторное назначение запрещено.

Нельзя делать произвольный `PATCH status`. Каждый переход реализуется отдельной
domain-командой с transition guard, idempotency и audit event.

### 4.2 Binding lifecycle

`PENDING -> ACTIVE -> REVOKED | SUPERSEDED`. `binding_version` — монотонный
fencing counter устройства. Любые reassign, suspend, key rotation или revoke
увеличивают его. Команда со старым `binding_version` всегда отклоняется.

### 4.3 Capability lifecycle

`ISSUED -> ACTIVE -> EXPIRED | REVOKED`. Admin capability живёт 5–15 минут; POS
capability — не более 24 часов. Capability не продлевается локально и содержит
`capability_id`, `kid`, `device_id`, tenant/store/register scope, subject key,
permissions, `nbf`, `exp`, `binding_version`.

## 5. Backend и база данных

### 5.1 Новые domain-модули

В `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain`:

- `device_registry.go` — lifecycle и immutable manufacturing identity;
- `manufacturing.go` — station authentication, registration, evidence;
- `device_assignment.go` — tenant assignment/reassignment;
- `device_capability.go` — issuance, validation, rotation, revocation;
- `device_auth.go` — challenge/proof и short-lived MQTT credential;
- `signed_command.go` — canonicalization contract и verification metadata;
- `trusted_time.go` — signed server time/monotonic rollback policy.

Не расширять generic `CreateResource("device", ...)` для security-critical
переходов. Использовать типизированные команды и repository methods.

### 5.2 Обязательные таблицы

Подготовить версионированную SQL migration и data dictionary для:

#### `fiscal_device_registry`

- `device_id uuid primary key`;
- `serial varchar(64) unique not null`;
- `device_public_key_jwk jsonb not null`;
- `device_key_thumbprint varchar(64) unique not null`;
- `hardware_revision`, `firmware_version`, `bootloader_version`;
- `manufacturing_batch`, `manufactured_at`, `manufacturing_station_id`;
- `state` с CHECK по lifecycle;
- nullable `tenant_id`;
- `binding_version bigint not null default 0`;
- `last_seen_at`, `suspended_at`, `retired_at`;
- `created_at`, `updated_at`, optimistic `version`;
- immutable hashes `firmware_sha256`, `registration_evidence_sha256`.

Public key, serial и manufacturing identity после `MANUFACTURED` меняются только
через отдельную audited key-recovery процедуру.

#### `fiscal_manufacturing_stations`

`station_id`, display name, credential/key thumbprint, allowed batches,
`status`, `last_used_at`, `created_at`, `revoked_at`.

#### `fiscal_device_bindings`

`binding_id`, `device_id`, `tenant_id`, nullable `location_id/register_id`,
roles array/normalized child rows, `binding_version`, lifecycle, actor,
activation/revoke timestamps. Partial unique indexes запрещают более одного
active fiscal device на register и более одного active binding устройства.

#### `fiscal_actor_installations`

Admin/POS installation identity: `installation_id`, tenant, subject, type,
public key/thumbprint, state, registered/revoked timestamps. Приватный ключ
никогда не хранится.

#### `fiscal_device_capabilities`

Хранить digest подписанного capability, scope, permissions, validity,
`binding_version`, state и revoke reason. Сам bearer secret отсутствует.

#### `fiscal_device_auth_challenges`

Одноразовый nonce hash, device, purpose, expiry, consumed timestamp. TTL cleanup.

#### `fiscal_device_revocations`

Device/capability/installation target, monotonically increasing revision,
reason, effective time, audit actor. MQTT retained snapshot строится из неё.

Все tenant-owned таблицы получают PostgreSQL RLS. Manufacturing registry до
назначения tenant доступен только platform role, не через tenant RLS fallback.

## 6. REST API — обязательный OpenAPI surface

Все endpoints сначала описываются в
`/Users/freelancer/Documents/Beeloy/Fiscalisation/contracts/openapi-runtime-v1.yaml`,
затем генерируются/валидируются существующими scripts. Нельзя добавить handler
без OpenAPI operation, error schema, security, idempotency и examples.

### 6.1 Manufacturing API

Отдельная audience/scope `beefiscal.manufacturing`; обычный tenant JWT запрещён.

- `POST /platform/v1/manufacturing/devices:register`
  - station authentication + optional factory network policy;
  - принимает serial, public JWK, hardware/firmware/batch, registration nonce,
    device proof и evidence digest;
  - возвращает `device_id`, state, backend root set/version и signed receipt;
  - идемпотентен по serial + key thumbprint; конфликт serial/key возвращает 409.
- `POST /platform/v1/manufacturing/devices/{device_id}:verify`
  - новый challenge-response после flash/read-back;
  - переводит запись в `MANUFACTURED`, если evidence совпадает.
- `POST /platform/v1/manufacturing/stations/{station_id}:revoke`.

### 6.2 Platform Admin API для AdminApp

Security scope `PLATFORM_DEVICE_ADMIN`; endpoints не должны принимать tenant из
обычного tenant token.

- `GET /platform/v1/devices` с cursor pagination и фильтрами state, serial,
  tenant, batch, firmware, last_seen;
- `GET /platform/v1/devices/{device_id}`;
- `GET /platform/v1/devices/{device_id}/history`;
- `POST /platform/v1/devices/{device_id}:assign-tenant` body `{tenant_id}`;
- `POST /platform/v1/devices/{device_id}:unassign-tenant` с обязательным reason;
- `POST /platform/v1/devices/{device_id}:suspend`;
- `POST /platform/v1/devices/{device_id}:resume`;
- `POST /platform/v1/devices/{device_id}:retire` с step-up authentication;
- `GET /platform/v1/manufacturing/batches` и batch detail.

Mutation endpoints требуют `Idempotency-Key`; state-sensitive operations также
`If-Match`. Ответ содержит новый ETag/version.

### 6.3 Tenant API для BeeFiscalApp

- `GET /public/v1/devices?assignment_state=ASSIGNED` возвращает только устройства
  tenant из JWT;
- `POST /public/v1/devices/{device_id}/deployment-sessions` создаёт короткую
  capability на временный public key BeeFiscalApp;
- `POST /public/v1/device-deployments/{id}:confirm` принимает location/register/
  roles, но **не принимает tenant/company_id**;
- `GET /public/v1/device-deployments/{id}`;
- `POST /public/v1/devices/{device_id}:disconnect` отзывает deployment внутри
  tenant, но не снимает platform tenant assignment;
- `POST /public/v1/devices/{device_id}/capabilities` выпускает Admin/POS
  capability с allow-listed permissions;
- `POST /public/v1/devices/{device_id}:rotate-binding`;
- `GET /public/v1/devices/{device_id}/connectivity` и `/diagnostics`.

Существующие `/device-activation-requests:*`, X.509 issuer и HMAC transport keys
нужно мигрировать. Нельзя одновременно считать старый JWT/HMAC и новый
capability protocol production-validными. Ввести protocol version gate:

- `v1-legacy` доступен только DEV до migration cutoff;
- `v2-capability` обязателен PROD;
- после cutoff удалить `CommandHMACKey`, `SyncAckHMACKey`, `BLETicketHMACKey` из
  выдаваемого credential и OpenAPI.

### 6.4 Device Auth/MQTT API

- `POST /device/v1/auth/challenges`;
- `POST /device/v1/auth/token` с `device_id`, nonce, device signature, firmware
  digest и binding version;
- ответ — short-lived MQTT token либо broker credential, TTL несколько часов,
  ACL только для конкретного device topic namespace;
- `GET /device/v1/trust-bundle` возвращает подписанный root rotation bundle;
- `GET /device/v1/time` возвращает signed server time envelope.

## 7. MQTT contracts

Целевые topics:

```text
beefiscal/v2/devices/{device_id}/commands
beefiscal/v2/devices/{device_id}/events
beefiscal/v2/devices/{device_id}/status
beefiscal/v2/devices/{device_id}/sync/batches/{batch_id}
beefiscal/v2/devices/{device_id}/sync/acks/{batch_id}
beefiscal/v2/devices/{device_id}/authorization
beefiscal/v2/revocations/{device_id}
```

Broker ACL выводится из authenticated `device_id`, а не из client-supplied
tenant. Device не может subscribe/publish в namespace другого устройства.
Commands используют QoS 1, persistent session и application idempotency.
Retained допустим для status policy/revocation snapshot, но не для transaction
commands.

## 8. Единый SignedCommand

Минимальный envelope:

```json
{
  "version": 2,
  "device_id": "uuid",
  "sender_id": "installation uuid",
  "command_id": "uuidv7",
  "sequence": 912,
  "binding_version": 7,
  "command": "fiscal.receipt.open",
  "issued_at": "RFC3339",
  "expires_at": "RFC3339",
  "payload": {},
  "capability": {},
  "signature": "base64url-p1363"
}
```

Порядок validator на firmware неизменяем:

1. parse с size/depth limits и reject unknown critical fields;
2. version/device/binding match;
3. backend root `kid` и capability signature;
4. capability validity, permission, subject key и scope;
5. sender signature над RFC 8785 canonical unsigned envelope;
6. trusted time/expiry;
7. atomic anti-replay reservation (`sender_id + sequence/command_id`);
8. journal `RECEIVED` до обращения к ФУ;
9. выполнение через `protocol-abstraction`;
10. journal result, подпись device key и доставка event/ack.

Один `command_id` с другим payload digest — security conflict, не retry.

## 9. BLE protocol v2

### 9.1 GATT surface

В firmware создать `include/ble` и `src/ble` со следующими характеристиками:

- `DEVICE_INFO` read: protocol/device/serial suffix/firmware/root key ids/nonce;
- `SESSION_CONTROL` write+indicate: challenge/auth/close;
- `SESSION_RX` write-with-response: encrypted fragments;
- `SESSION_TX` indicate: encrypted response fragments;
- `SESSION_STATUS` read/notify: state и generic error codes без secrets.

Advertising name содержит только короткий serial suffix. Полный public key,
tenant/store, Wi-Fi и transaction state не рекламируются.

### 9.2 Handshake

1. App получает backend-signed capability на свой temporary public key.
2. App соединяется без pairing и запрашивает challenge.
3. Device возвращает random 256-bit nonce, random 128-bit session ID и ephemeral
   ECDH public key, подписанные device identity.
4. App отправляет capability, nonce_app, ephemeral public key и подпись своим
   capability-bound private key.
5. Device выполняет полный capability validator.
6. Обе стороны вычисляют ECDH и HKDF:
   - salt: `nonce_device || nonce_app`;
   - info: `"beefiscal-ble-v2" || session_id || device_id`;
   - output: separate client-to-device key, device-to-client key и nonce bases.
7. После `AUTH_OK` разрешены только AEAD frames.

Frame содержит `version, session_id, direction, seq, fragment_index,
fragment_count, ciphertext, tag`. Header — AEAD associated data. Nonce никогда
не повторяется для одного ключа. При gap, duplicate, tag failure, timeout или
disconnect ключи/буферы обнуляются. Максимумы: одна privileged session,
ограниченное число unauthenticated connections, message <= 32 KiB, fragments <=
256, handshake <= 30 s, idle <= 60 s.

### 9.3 BLE permissions

- Admin: `wifi.set`, `store.bind`, `config.read/write`, `device.reboot`,
  `diagnostics.read`;
- POS: только `transaction.command`, `transaction.status`, `sync.request`;
- destructive/recovery permissions не выдаются POS capability.

Существующий `BFA1|...|JWT` в BeeFiscalApp должен быть полностью заменён после
включения v2. Простое фрагментирование JWT не удовлетворяет целевой модели.

## 10. Изменения edge-agent-s3

В `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3`:

```text
include/
  identity/DeviceIdentity.h
  security/BackendTrustStore.h
  security/CapabilityVerifier.h
  security/SignedCommandVerifier.h
  security/TrustedClock.h
  ble/SecureGattService.h
  mqtt/DeviceMqttClient.h
  commands/CommandRouter.h
  config/SecureConfiguration.h
  storage/EdgeStorage.h
src/ (соответствующие реализации)
test/embedded/ и test/native/
```

Обязательная логика:

- first-boot key generation непосредственно ESP32-S3, self-test и stable device ID;
- конкретный `Esp32DeviceIdentity` без абстракции внешнего crypto provider:
  создание P-256 keypair, получение public JWK/thumbprint и ECDSA signing;
- private key сохраняется в NVS только после успешной генерации и self-test;
  production использует NVS Encryption вместе с Flash Encryption. Публичный API
  класса не предоставляет export private key;
- ATECC608A, внешний secure element и сменный hardware crypto provider исключены
  из целевой архитектуры;
- verify backend root before accepting any capability/config;
- NVS Wi-Fi secrets отдельно от SD transaction DB;
- BLE v2 handshake/AEAD;
- MQTT token challenge/refresh и reconnect backoff with jitter;
- общий command router для MQTT/BLE;
- vendor/channel profile создаёт раздельные `IFiscalDevice` и
  `IPaymentTerminal` через `DeviceProtocolProvider`;
- watchdog, brownout-safe state transitions и no fiscal command on boot;
- signed status/evidence events;
- trusted time anchored to signed backend time + monotonic uptime; backward
  movement запрещён;
- persisted anti-replay window и capability/revocation cache;
- OTA только подписанного firmware с anti-rollback.

### 10.1 SQLite migration

Текущую `transaction_journal` расширить следующими logical entities:

- `commands`: command ID, sender, sequence, digest, capability ID, transport,
  received/started/completed timestamps, result and device signature;
- `outbox`: signed events, attempts, next retry, acknowledged time;
- `capability_cache`: signed capability, digest, expiry, binding version;
- `revocation_cache`: revision and signed snapshot;
- `trusted_time_anchor`: server time, monotonic counter, signature digest;
- `schema_version` с последовательными migrations, без destructive recreate.

Удалять можно только acknowledged transaction/outbox records старше минимум
трёх месяцев (нормативный проектный default 93 дня). Несинхронизированные записи
не удаляются по возрасту. При нехватке SD устройство блокирует новые продажи до
освобождения/синхронизации и не теряет journal silently.

## 11. Manufacturing script

Создать отдельный каталог:

`/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/device-manufacturing`

Предлагаемая структура:

```text
README.md
requirements.lock / pyproject.toml
config.example.yaml
src/beefiscal_factory/
  cli.py
  flash.py
  serial_allocator.py
  device_probe.py
  registration_client.py
  evidence.py
tests/
schemas/manufacturing-evidence.schema.json
```

CLI команды:

- `factory flash --port ... --firmware ... --serial ... --batch ...`;
- `factory probe --port ...` — получает device public key/challenge и build info;
- `factory register --evidence ...` — authenticated backend request;
- `factory verify --device-id ... --port ...` — backend challenge + device proof;
- `factory run-station` — последовательный guarded workflow для оператора.

Скрипт должен:

- проверять SHA-256 и подписанный release manifest до flash;
- запрещать случайный firmware path в production profile;
- получать serial из backend-reserved batch/range либо принимать barcode scan;
- не логировать Wi-Fi password, station credential или private data;
- хранить resumable evidence JSON с firmware hash, esptool output digest, serial,
  chip ID/MAC как diagnostic metadata, public key thumbprint и timestamps;
- использовать OIDC workload identity/mTLS station credential из OS key store;
- повторять только идемпотентные операции;
- после flash выполнять read-back identity/proof, а не доверять exit code;
- отправлять данные только в manufacturing endpoint;
- иметь DEV simulator и запрет DEV endpoint/credentials в PROD profile.

Factory Wi-Fi может быть в общей прошивке только как ограниченный bootstrap
credential: отдельный VLAN, доступ только к registration service, короткая
ротация и отсутствие доступа к MQTT/production APIs. Предпочтительнее передавать
его station-to-device во временной RAM-сессии после flash, чтобы общий firmware
не содержал долговечный пароль партии.

## 12. AdminApp

Создать приложение в
`/Users/freelancer/Documents/Beeloy/Fiscalisation/AdminApp` на текущем
утверждённом UI stack репозитория. Оно использует отдельную OIDC audience и роли
`PLATFORM_DEVICE_VIEWER`, `PLATFORM_DEVICE_ADMIN`, `PLATFORM_SECURITY_ADMIN`.

Обязательные страницы:

1. **Devices** — серверная pagination/filter/search, state badges, tenant, batch,
   firmware, last seen, security posture.
2. **Device detail** — immutable identity, public key thumbprint, manufacturing
   evidence, lifecycle timeline, tenant assignment, current binding, firmware,
   connectivity, revocations и audit.
3. **Assign tenant** — поиск tenant, preview последствия, optimistic version,
   confirmation. Location/register здесь не выбираются.
4. **Reassign/unassign** — reason обязателен; показывает, что deployment,
   capabilities и MQTT sessions будут revoked.
5. **Suspend/resume/retire** — step-up confirmation; retire необратим.
6. **Manufacturing batches/stations** — batch progress, conflicts, station revoke.

Нельзя показывать private key, Wi-Fi secrets, MQTT token или полный capability.
Все mutations отображают operation/audit ID. Cross-tenant platform access должен
быть явной platform permission, а не отключением backend RLS на клиенте.

## 13. BeeFiscalApp

Изменить tenant workflow в
`/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp`:

- список `ASSIGNED` устройств текущего tenant;
- wizard: выбрать устройство → проверить serial/thumbprint → location → register
  → roles → получить capability → BLE connect → encrypted setup → ждать MQTT
  proof/deployment commit;
- tenant/company всегда read-only из OIDC context и отсутствует в request body;
- role `PAYMENT_TERMINAL` предлагается только при подтверждённой capability;
- ввод Wi-Fi password скрыт, не сохраняется в state persistence/analytics/logs;
- экран активных devices с connectivity, firmware, store/register, roles,
  credential/capability expiry, last seen;
- disconnect отзывает deployment, но не platform tenant assignment;
- mobile использует native BLE; web — Web Bluetooth только в Chrome secure
  context, с понятным fallback на cloud flow;
- временный Admin keypair создаётся на installation/session и private key не
  отправляется backend;
- заменить `smartDeviceBle*.ts` BFA1/JWT flow на BLE v2 state machine;
- accessibility, touch-oriented UX, retry/cancel/resume и explicit success only
  после backend deployment state `DEPLOYED`.

## 14. Security и production configuration

Для production firmware обязательны отдельное PlatformIO environment и release
procedure:

- Secure Boot V2;
- Flash Encryption release mode;
- NVS encryption;
- signed OTA и anti-rollback eFuse;
- JTAG/download mode disabled согласно утверждённой recovery policy;
- unique ESP32-generated device identity self-test;
- no factory/debug secrets in logs or crash dumps;
- backend root public key rotation support;
- reproducible build manifest/SBOM и firmware hash registration.

Эти eFuse операции необратимы и не выполняются обычным `pio run -t upload`.
Manufacturing tool обязан иметь отдельные DEV/PROD profiles, dry-run, double
confirmation и hardware-revision allowlist.

## 15. Тестовое покрытие

### 15.1 Firmware native/unit

- canonical JSON golden vectors;
- ECDSA valid/invalid/high-S/malformed signatures;
- capability wrong device/tenant/register/permission/version/time;
- ECDH/HKDF/AES-GCM known-answer vectors;
- BLE fragmentation, reordering, duplicate, truncation, bad tag, disconnect;
- anti-replay across reboot and both transports;
- identical MQTT/BLE command produces one execution;
- SQLite power-loss/migration/outbox/retention/full-card behavior;
- trusted-time rollback/reboot/long-offline behavior;
- factory reset cannot turn RETIRED/SUSPENDED into trusted device.

### 15.2 Backend

- lifecycle transition table;
- serial/key uniqueness and idempotent factory retry;
- station revoke and wrong audience;
- tenant assignment authorization;
- tenant isolation/RLS and no tenant body override;
- capability issuance/revoke/expiry/binding fencing;
- device proof and MQTT topic ACL;
- root rotation overlap;
- audit completeness and secret redaction;
- OpenAPI request/response conformance.

### 15.3 UI

- AdminApp platform role matrix and assignment confirmation;
- BeeFiscalApp tenant cannot see/assign foreign device;
- tenant cannot be selected or altered in deployment request;
- BLE v2 happy path and every failure state;
- web unsupported-browser fallback;
- no secret in persisted state, console, analytics snapshots or screenshots;
- disconnect/revoke visibility and stale UI concurrency with ETag.

### 15.4 HIL/E2E

1. flash clean S3 → key generation → factory register/verify;
2. AdminApp assigns tenant;
3. BeeFiscalApp binds location/register over hostile/open BLE;
4. device obtains short-lived MQTT credential and becomes DEPLOYED;
5. signed POS command via MQTT reaches protocol abstraction exactly once;
6. MQTT outage → same command through BLE → one execution;
7. local result enters SQLite/outbox → MQTT restore → backend ack;
8. capability revoke/expiry and stale binding are rejected;
9. power loss at every journal boundary recovers without duplicate fiscal action;
10. suspend/reassign prevents old tenant/Admin/POS from controlling device.

HIL evidence сохраняет firmware hash, device serial, test vector IDs, backend
operation IDs и redacted logs.

## 16. Последовательность реализации

1. Утвердить protocol v2 wire schemas, algorithms и threat model.
2. Расширить OpenAPI на 100% новых endpoints/errors/examples.
3. Добавить DDL, data dictionary, RLS и typed repositories.
4. Реализовать Device Registry/manufacturing lifecycle и factory simulator.
5. Реализовать firmware identity/trust store/trusted time.
6. Реализовать capability signer/verifier и actor installation registration.
7. Реализовать BLE v2 handshake/AEAD и common command validator.
8. Реализовать MQTT device auth/ACL и единый command router.
9. Расширить SQLite journal/outbox/recovery.
10. Создать manufacturing CLI и DEV HIL workflow.
11. Создать AdminApp и platform assignment UI.
12. Переделать BeeFiscalApp tenant deployment/BLE UI.
13. Провести minimal MVP acceptance по happy path.
14. Провести полный security/regression/fault/HIL цикл и bugfix.
15. Выполнить controlled production security provisioning rehearsal.

Каждый следующий этап начинается только после contract tests предыдущего; UI не
должен временно использовать undocumented endpoint.

## 17. Критерии завершённости

Задача считается выполненной только если одновременно:

- одна прошивка партии создаёт уникальную non-exportable identity на каждом S3;
- backend никогда не получает device private key;
- factory station authentication является основным контролем, IP allowlist —
  только дополнительным;
- AdminApp видит все devices и безопасно назначает tenant;
- BeeFiscalApp не позволяет выбирать компанию и привязывает только assigned
  device текущего tenant к location/register;
- BLE работает без pairing, но privileged plaintext не принимается;
- capability, sender signature, permission, expiry, binding version и replay
  проверяются до выполнения команды;
- MQTT и BLE используют один SignedCommand/validator;
- short-lived MQTT credential ограничен topics одного устройства;
- revocation и trusted time работают online, а offline риск ограничен TTL;
- SQLite не удаляет unsynced записи и сохраняет минимум три месяца synced data;
- `protocol-abstraction` создаёт отдельные fiscal/payment instances и вызывается
  только после command authorization;
- OpenAPI покрывает 100% нового REST surface, generated contracts актуальны;
- unit, contract, integration, UI, HIL, power-loss и security tests зелёные;
- production build имеет Secure Boot/Flash/NVS encryption evidence и signed OTA;
- legacy BFA1/JWT/HMAC activation отключён в PROD;
- документация deployment, recovery, root rotation, station revoke и incident
  response обновлена.

## 18. Блокирующие решения до начала production-реализации

1. Точная модель/revision ESP32-S3 Camera Module, распиновка SD/camera/UART/I²C,
   объём flash/PSRAM и доступные eFuse security features.
2. Источник и формат заводского serial, правила batch/range reservation и
   физическая маркировка.
3. KMS/HSM provider для backend root и root rotation ceremony.
4. OIDC provider/audience/roles для platform AdminApp и manufacturing stations.
5. Broker support для proof-derived short-lived credentials и per-device ACL.
6. Production recovery policy после Secure Boot/Flash Encryption/JTAG disable.
7. Допустимый maximum offline POS capability TTL и процедура emergency revoke.
8. Подтвердить production eFuse/NVS Encryption/Flash Encryption provisioning,
   защищающий ESP32-generated private key от чтения внешней flash.
9. Точный список command permissions и vendor capabilities для каждой hardware
   revision.

Эти вопросы не блокируют разработку OpenAPI, DDL, симуляторов и криптографических
golden vectors, но блокируют production provisioning и выпуск hardware партии.
