# MVP1 — состояние реализации

Дата проверки: 2026-08-14
Статус: `SOFTWARE_COMPLETE_HIL_PENDING`

Открытых программных P0 в утверждённом функциональном профиле MVP1 нет.
Удалённые remediation-документы закрыты кодом и regression evidence; их история
остаётся в Git. Канонические действующие документы:

- [`../EXTERNAL_POS_INTEGRATION_PROTOCOL.md`](../EXTERNAL_POS_INTEGRATION_PROTOCOL.md)
  — REST/WebHook, Local HTTP и BLE, общий UUID/digest и SUPTO-профиль;
- [`../SUPTO/index.md`](../SUPTO/index.md) — regulatory conceptual matrix;
- [`../../contracts/openapi-runtime-v1.yaml`](../../contracts/openapi-runtime-v1.yaml)
  — cloud/runtime OpenAPI;
- [`../../contracts/openapi-local-adapter-v1.yaml`](../../contracts/openapi-local-adapter-v1.yaml)
  — Local HTTP adapter OpenAPI;
- [`../../contracts/beeloy-pos-deployment-v1.schema.json`](../../contracts/beeloy-pos-deployment-v1.schema.json)
  — signed SPA deployment descriptor.

## Закрытый software scope

- BlueCash-50: aggregate sale, ordered CASH/CARD/split, скидки, компенсация,
  сторно/refund, printer test, X/Z и cash-in/out через один durable processor для
  MQTT, Local HTTP и открытого MVP BLE.
- edge-agent-s3: DP-150/BluePad и Daisy Compact profiles, UART/BLE/USB,
  SQLite/SD reservation-before-I/O, recovery, cross-channel deduplication,
  MQTT TLS/QoS1 sync и Local HTTP.
- MiniPOS: одинаковый client UUID и canonical payload digest при Cloud ↔ Local
  HTTP ↔ BLE, hysteresis, lookup-only UNKNOWN, IndexedDB outbox и durable offline
  import в собственный PostgreSQL.
- MiniPOS Web: React/Vite touch POS, offline references, shifts, discounts,
  cash/card/split, storno, diagnostics и signed reproducible bundle descriptor.
- Fiscal backend/BeeFiscalApp: composite binding, typed endpoint configuration,
  generation fencing, MQTT health materialization/history, activity UI,
  printer test и role gates.
- Два независимых Compose-проекта с PostgreSQL и Caddy; расширенная MiniPOS
  adapter configuration сохраняется typed и без потери полей после restart.

## Проверенное evidence

- `make regression` — PASS;
- `make postgres-integration` — PASS;
- `make compose-e2e` — PASS;
- чистая PlatformIO ESP-IDF сборка — `firmware.elf` и `firmware.bin` созданы;
- Android BlueCash/Daisy debug и release APK — PASS;
- MiniPOS/BeeFiscalApp Android/iOS/Web bundles — PASS;
- signed SPA descriptor generation с Ed25519 self-verification — PASS.

Regression также включает Go race/vet, OpenAPI drift/coverage, SUPTO/BG trace,
fault/security/72-hour accelerated soak, UI interaction E2E, protocol drivers и
ESP-IDF native saga. Симуляторы и stubs доказывают software semantics, но не
подменяют физическое evidence.

## Что остаётся вне software-complete

Только внешние и аппаратные gates:

- реальные BlueCash-50 fiscal/card/BLE сценарии и acquirer acceptance;
- DP-150 MX RS-232 и BluePad-50 Plus BLE HIL;
- Daisy Compact S 01 USB/electrical/EUR firmware HIL;
- SD power-loss/endurance и LAN/радио soak на целевом ESP32-S3;
- NAP/BIM/authorized-service и юридическое подтверждение;
- production signing, vulnerability scan, Secure Boot/Flash Encryption и
  formal HTTP-in-trusted-LAN risk acceptance.

До появления этих артефактов `PILOT` и `PROD` остаются `NO_GO`; статус
`SOFTWARE_COMPLETE_HIL_PENDING` не является разрешением production deployment.
