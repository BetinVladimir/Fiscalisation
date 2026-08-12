# Обязательные MVP1 profiles на edge-agent-s3

## 1. Обязательный scope

`edge-agent-s3` не является заменой или optional track. Наряду с BlueCash app он
обязан реализовать два physical profiles:

```text
EDGE_AGENT_S3
├── DP-150 MX fiscal endpoint: COM/RS-232 over UART level converter
│   └── BluePad-50 Plus optional payment endpoint: BLE
└── Daisy Compact S 01 fiscal endpoint: USB host/native protocol
```

MiniPOS и fiscal-backend не меняют business commands/results. Один edge может
обслуживать fiscal и payment endpoints разных типов одного кассового места.

## 2. Что уже есть

- P-256 identity, public JWK/thumbprint и P1363 signature;
- SD_MMC + SQLite transaction/command/cache schemas;
- base journal CRUD и replay sequence;
- `DeviceProtocolProvider` для раздельного fiscal/payment facade;
- Daisy/Datecs fiscal и DatecsPay payment abstractions;
- fail-closed BLE session skeleton;
- `SignedCommandValidator`/router interfaces;
- отсутствие fiscal command при boot.

## 3. Обязательный delta для MVP1

### 3.1 Runtime configuration

Сохранить подписанный active binding:

```text
tenant/location/register/device/route IDs
binding version
fiscal vendor/model/channel/pins/payment code
optional payment terminal vendor/channel
MQTT endpoint/topics/credential
backend trust public key
```

Нельзя оставлять hardcoded Daisy/UART profile из `main.cpp`.

### 3.2 MQTT runtime

Добавить TLS client, QoS 1 subscription/publication, reconnect/backoff,
credential refresh, command expiry, contiguous signed batches и business ACK.
Topic/payload должны совпадать с BlueCash `DeviceCommandEnvelopeV2`.

### 3.3 BLE runtime

Заменить skeleton на GATT server, ECDH/HKDF/AES-GCM, ticket verification,
fragmentation/replay и deterministic CBOR mapping. MQTT и BLE вызывают один
`SignedCommandValidator` и `CommandRouter`.

### 3.4 Concrete authorization

Реализовать backend trust verifier, capability verifier, sender signature
verifier, canonical payload digest, trusted time anchor и persistent revocation.
Обновление replay window и reservation command выполнить одной SQLite transaction.

### 3.5 Fiscal/payment orchestration

Для `SALE_FINALIZE`:

```text
probe final FU
persist immutable receipt session + ordered plan
CARD 1..N: payment.purchase(amountMinor), durable RRN/auth/STAN after each result
fiscal.openReceipt
fiscal.addItem * N
fiscal.addPayment * N
fiscal.closeReceipt
persist COMMITTED signed result/outbox
```

При card decline fiscal receipt не закрывается как оплаченный. При неизвестном
результате запрещён автоматический повтор. При сбое ESP32 выполняет cancel/storno
ФУ по фактической commit point и reverse уже approved card операций. Только
доказанный полный откат даёт `COMPENSATED`; иначе касса блокируется в
`RECOVERY_REQUIRED`. Реализовать `SALE_REVERSE`, X, Z, CASH_IN, CASH_OUT и
lookup/reconciliation.

### 3.6 Storage corrections

- настоящий migration runner;
- atomic replay+command reservation;
- `EXECUTING/UNKNOWN` recovery;
- outbox API и immutable in-flight batch;
- ACK ID/hash/checkpoint;
- capability/revocation/trusted-time accessors;
- durable card RRN/auth/STAN;
- durable receipt session, step log и compensation links;
- purge только acknowledged records старше 90 дней;
- storage-unavailable/full блокирует новый side effect.

### 3.7 Main state machine

```text
BOOT
→ IDENTITY_READY
→ STORAGE_READY
→ CONFIGURED
→ NETWORK_READY
→ MQTT_ACTIVE / BLE_AVAILABLE
→ DEVICE_READY
→ DEGRADED_CLOUD_OFFLINE | BLOCKED_DEVICE_OFFLINE | RECOVERY_REQUIRED
```

Текущий `loop(){delay(1000);}` заменить bounded event loop/tasks с watchdog.

## 4. Допустимо после MVP1

- Secure Boot, Flash/NVS Encryption, anti-rollback eFuse, JTAG closure;
- signed OTA production rollout;
- несколько hardware revisions;
- browser BLE;
- batch manufacturing automation;
- расширенные diagnostics/reports кроме обязательных X/Z.

## 5. ESP32 acceptance

1. Реальная плата и точный pin/electrical profile зафиксированы.
2. MQTT cash sale печатает один чек и синхронизирует result.
3. MQTT card sale вызывает terminal, затем печатает один чек.
4. Direct BLE sale работает при отключённом cloud.
5. После восстановления MQTT BLE sale появляется в backend/WebHook.
6. Повтор MQTT/BLE одного operation не печатает второй чек.
7. Потеря конечного ФУ блокирует side effect.
8. Reboot на каждой journal boundary не приводит к слепому retry.
9. Card reversal использует durable original terminal reference.
10. X/Z и cash movement достигают реального ФУ.
