# MVP1 — исполнимая спецификация фискального контура

Статус: требования для кодогенерации и приёмки controlled non-production MVP.
Дата среза реализации: 2026-08-13.

## Цель

MVP1 доказывает два сквозных маршрута для одного tenant, одной торговой точки,
одного кассового места и одного активного BlueCash-50:

```text
MiniPOS → Fiscal REST → MQTT → BlueCash → ФУ/pinpad → MQTT sync → Fiscal/WebHook
MiniPOS → backend-issued BLE session → BlueCash → ФУ/pinpad → MQTT sync → Fiscal/WebHook
```

Альтернативный track заменяет BlueCash приложением `edge-agent-s3`, сохраняя те
же business contracts, идентификаторы, journal и результат.

## Источники истины

1. [`../../contracts/openapi-runtime-v1.yaml`](../../contracts/openapi-runtime-v1.yaml)
   и canonical public OpenAPI.
2. [`../../contracts/bg-requirements-trace.json`](../../contracts/bg-requirements-trace.json).
3. Документы этого каталога.
4. Vendor protocols и as-built traceability.

При конфликте примера и OpenAPI исправляется OpenAPI и generated contracts;
нельзя внедрять недокументированный transport payload.

## Документы

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

## Не входит в MVP1

- Secure Boot V2, Flash/NVS Encryption, anti-rollback eFuse и закрытие JTAG;
- production PKI ceremony и массовая manufacturing line;
- OTA rollout rings;
- несколько устройств одной роли на кассовом месте;
- Web Bluetooth, если native Android MiniPOS является единственным BLE клиентом
  демонстрации;
- invoice, refund-to-card без исходной операции, cashback, tips, preauthorization;
- все vendor/model combinations одновременно.

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
two-route E2E on one real BlueCash-50
power/network interruption matrix
duplicate-delivery test proving one physical fiscal/card side effect
```

Simulator подтверждает только software semantics и не заменяет HIL.

## Текущий вердикт

На срезе 2026-08-13 документация описывает целевую архитектуру, но текущая
реализация ещё не является рабочим hardware MVP. Решение можно принимать как
MVP только после выполнения всех `MVP1-P0-*` из документа 06 и прохождения его
acceptance matrix. Статусы SUPTO `PARTIAL` не должны интерпретироваться как
регуляторная или production-готовность.
