# MVP1 — исполнимая спецификация фискального контура

Статус: требования для кодогенерации и приёмки controlled non-production MVP.
Дата среза реализации: 2026-08-13.

## Каноническая точка входа

Обязательный physical scope, hardware profiles и критерий `MVP1_GO` находятся в
[`index.md`](index.md). Этот README служит навигацией. Формулировки ниже должны
читаться только вместе с `index.md`.

## Цель

MVP1 доказывает сквозные маршруты для BlueCash-50 и обязательных edge-agent-s3
profiles:

```text
MiniPOS → Fiscal REST → MQTT → BlueCash → ФУ/pinpad → MQTT sync → Fiscal/WebHook
MiniPOS → backend-issued BLE session → BlueCash → ФУ/pinpad → MQTT sync → Fiscal/WebHook
```

`edge-agent-s3` не заменяет BlueCash, а реализует ещё два обязательных кассовых
профиля: DP-150 MX/COM + optional BluePad-50 Plus/BLE и Daisy Compact S 01/USB.
Все profiles сохраняют одинаковые business contracts, identifiers и journal.

## Источники истины

1. [`../../contracts/openapi-runtime-v1.yaml`](../../contracts/openapi-runtime-v1.yaml)
   и canonical public OpenAPI.
2. [`../../contracts/bg-requirements-trace.json`](../../contracts/bg-requirements-trace.json).
3. Документы этого каталога.
4. Vendor protocols и as-built traceability.

При конфликте примера и OpenAPI исправляется OpenAPI и generated contracts;
нельзя внедрять недокументированный transport payload.

## Документы

- [`index.md`](index.md) — канонические требования physical MVP.
- [`08_PHYSICAL_MVP_IMPLEMENTATION_ROADMAP.md`](08_PHYSICAL_MVP_IMPLEMENTATION_ROADMAP.md)
  — полный последовательный план реализации, regression и HIL.
- [`09_DUAL_ROUTE_FAILOVER_PROTOCOL.md`](09_DUAL_ROUTE_FAILOVER_PROTOCOL.md) —
  обязательный REST/WebHook↔BLE failover, lightweight ping и перенос чека между
  transports.
- [`01_MVP_REQUIREMENTS_AND_CODEGEN.md`](01_MVP_REQUIREMENTS_AND_CODEGEN.md) —
  обязательный scope, инварианты, порядок реализации и критерии приёмки.
- [`02_BLUECASH_COMMAND_TRACEABILITY.md`](02_BLUECASH_COMMAND_TRACEABILITY.md) —
  command-by-command трассировка REST/MQTT и direct BLE.
- [`03_PROTOCOL_COMMAND_GAPS.md`](03_PROTOCOL_COMMAND_GAPS.md) — отсутствующие
  fiscal/card команды и дефекты текущих orchestration contracts.
- [`04_EDGE_AGENT_S3_EQUIVALENT_TRACK.md`](04_EDGE_AGENT_S3_EQUIVALENT_TRACK.md) —
  что реализовать для той же схемы на ESP32-S3.
- [`05_BACKEND_DEVICE_ROUTE_SELECTION.md`](05_BACKEND_DEVICE_ROUTE_SELECTION.md) —
  корректный выбор BlueCash/ESP32/smart-device route по binding кассы.
- [`06_IMPLEMENTATION_READINESS_AUDIT_AND_CLOSURE.md`](06_IMPLEMENTATION_READINESS_AUDIT_AND_CLOSURE.md) —
  проверка документации против текущего кода, перечень обязательных P0-разрывов,
  порядок закрытия и исполнимые критерии готовности рабочего MVP.
- [`07_BLOCKERS_AND_MVP_DECISION.md`](07_BLOCKERS_AND_MVP_DECISION.md) — явное
  разделение устранимых software gaps и внешних блокеров, условия снятия каждого
  блокера и однозначное GO/NO-GO решение для simulator и physical MVP.

## Не входит в MVP1

- Secure Boot V2, Flash/NVS Encryption, anti-rollback eFuse и закрытие JTAG;
- production PKI ceremony и массовая manufacturing line;
- OTA rollout rings;
- несколько fiscal устройств одной роли на кассовом месте; отдельные fiscal и
  payment endpoints на одном `edge-agent-s3` явно входят в MVP;
- Web Bluetooth, если native Android MiniPOS является единственным BLE клиентом
  демонстрации;
- invoice, refund-to-card без исходной операции, cashback, tips, preauthorization;
- vendor/model combinations вне трёх profiles, перечисленных в `index.md`.

Эти исключения допустимы только для controlled MVP с непроизводственными
credentials. Transport authentication, encryption, idempotency, journal-before-I/O
и блокировка при недоступном конечном ФУ не исключаются.

## MVP1 release gates

MVP1 принимается только после прохождения:

```text
contract tests
backend unit/integration tests
BlueCash JVM tests
Android instrumentation for GATT + vendor sockets
REST/MQTT и BLE E2E на реальном BlueCash-50
DP-150 MX COM fiscal HIL
BluePad-50 Plus BLE payment и combined DP-150+BluePad HIL
Daisy Compact S 01 USB fiscal HIL
power/network interruption matrix
duplicate-delivery test proving one physical fiscal/card side effect
```

Simulator подтверждает только software semantics и не заменяет HIL.

## Текущий вердикт

На повторном срезе 2026-08-13 документация описывает целевую архитектуру, но текущая
реализация ещё не является рабочим hardware MVP. Решение можно принимать как
MVP только после выполнения всех `MVP1-P0-*` из документа 06 и прохождения его
acceptance matrix. Статусы SUPTO `PARTIAL` не должны интерпретироваться как
регуляторная или production-готовность.
