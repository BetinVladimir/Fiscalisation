# BeeFiscal Edge Agent

Запускаемый локальный контур между MiniPOS, фискальным устройством и Fiscal public API. Журнал SQLite работает в WAL/FULL и должен находиться на SD-карте либо другом постоянном носителе. DEV/HIL может использовать детерминированный simulator; неподтверждённый hardware track выбирает fail-closed `unsupported`. Контейнер не подменяет аппаратный GATT/USB/UART adapter.

## Запуск

```bash
EDGE_ID=edge-01 \
DEVICE_ID=device-01 \
EDGE_DATABASE_PATH=./data/edge.db \
FISCAL_EDGE_SYNC_URL=http://localhost:8080/public/v1/edge-sync/batches \
EDGE_SYNC_HMAC_KEY=dev-edge-sync-key \
WEBHOOK_VERIFICATION_KEY=dev-edge-webhook-key \
TENANT_ID=tenant-01 \
REGISTER_ID=register-01 \
DEVICE_ADAPTER=simulator \
EDGE_LOCAL_API_TOKEN=dev-local-api-token \
go run ./cmd/edge-agent
```

Переменные:

- `HTTP_ADDR` — control/health HTTP listener, по умолчанию `:8082`.
- `EDGE_DATABASE_PATH` — постоянная SQLite БД; каталог создаётся с правами `0750`.
- `EDGE_ID`, `DEVICE_ID` — стабильные идентификаторы, подписываемые в каждом sync batch.
- `TENANT_ID`, `REGISTER_ID`, `FENCING_TOKEN` — binding локальной authority; диапазоны задаются `OPERATION_FROM/TO` и `UNP_FROM/TO`.
- `DEVICE_ADAPTER` — `simulator` только в DEV/test либо `unsupported`; simulator в PROD запрещён при старте.
- `SIMULATOR_DEVICE_REACHABLE` — независимое состояние конечного ФУ для DEV fault scenarios.
- `EDGE_LOCAL_API_TOKEN` — минимум 16 байт; защищает loopback/HIL endpoints в DEV. В PROD endpoints вообще не монтируются.
- `FISCAL_EDGE_SYNC_URL` — полный `/public/v1/edge-sync/batches` URL.
- `EDGE_SYNC_HMAC_KEY` — ключ подписи batch/ACK, минимум 16 байт в DEV и 32 в PROD.
- `WEBHOOK_VERIFICATION_KEY` — secret созданного Fiscal webhook endpoint для события `ble.session.revoked`.
- `FISCAL_AUTH_TOKEN` — Bearer JWT; обязателен в PROD.
- `SYNC_INTERVAL_MS` — 100..300000, по умолчанию 2000.
- `EDGE_STORAGE_QUOTA_BYTES` — проверяемая ёмкость durable media, по умолчанию 1 GiB и минимум 1 MiB.

В PROD sync URL обязан быть HTTPS. Входящий endpoint `POST /control/v1/fiscal-webhooks` проверяет `BeeFiscal-Signature` до JSON parsing и подтверждает `204` только после записи revocation в SQLite. `GET /healthz` предназначен для локальной readiness-проверки. Повторная доставка того же revocation безопасна.

DEV/HIL executable smoke использует `GET /internal/v1/final-device`, `GET /internal/v1/storage` и `POST /internal/v1/commands` с `Authorization: Bearer ...`. Команда обязана точно совпасть с tenant/register/device/fencing binding. Она проходит тот же `Runtime`, что и GATT processor: durable commit предшествует device call, а replay после restart возвращает сохранённый результат без повторного исполнения. Storage telemetry показывает 70/85/95/100% states; начиная с 95% новая команда блокируется до любого fiscal side effect. Это внутренние adapter endpoints, не публичный POS API и не доступные в PROD.

## Реализованные слои

- `authority` — lease, fencing и восстановление sequence после рестарта;
- `journal` — hash-chain, ACK-gated хранение минимум три месяца, sync cursor, restart-durable frozen pending batch и BLE revocation cache;
- `ble` — ticket validation, X25519/HKDF/AES-GCM, framing, reassembly и flow control;
- `runtime` — durable-before-device execution и exactly-once/UNKNOWN semantics;
- `device` — детерминированный DEV simulator и fail-closed unsupported adapter с разделением known rejection/ambiguous outcome;
- `localapi` — защищённый DEV/HIL executable transport для process/restart tests;
- `gateway` — transport-neutral GATT command processor: decrypt/reassembly, strict envelope validation, runtime execution и encrypted correlated result;
- `sync` — подписанные batch/ACK, HTTP uploader и restart recovery; после первой отправки границы/hash pending batch фиксируются в SQLite до валидного ACK, поэтому новые события не меняют idempotency key после ambiguous response loss;
- `ota` — Ed25519-signed model/hardware/ring manifest, SHA-256 verification before staging and boot, monotonic anti-downgrade and SQLite-durable A/B health/rollback state. Rollback below the signed vulnerability floor becomes `RECOVERY_REQUIRED`; secure boot, physical flash/recovery partition and vendor compatibility remain hardware HIL gates.
- `control` — аутентифицированное получение BLE revocation events;
- `cmd/edge-agent` — production-guarded процесс, authority/device/runtime composition, sync loop, graceful shutdown и health/control HTTP.

Аппаратный GATT server должен только передавать bytes в `gateway.Processor` и публиковать возвращённые event frames. OS-specific GATT server и Daisy/Datecs USB/UART adapter остаются vendor/HIL-gated и не должны подменяться симулятором в PROD.

`make soak-regression-test` ускоренно моделирует 72 часа пяти-минутных network-flap слотов и семь суток десяти-минутных journal слотов. Gate проверяет zero loss/duplicate commit, replay того же batch после ambiguous loss, ежедневные SQLite restart, ACK cursor, hash chain и трёхмесячное retention. Это детерминированное software evidence; оно не заявляет физический SD-card/HIL или wall-clock endurance.
