# Документация Fiscalisation

Этот каталог содержит документацию реализованной системы. Источники истины имеют
следующий приоритет:

1. OpenAPI/AsyncAPI и машинные contracts;
2. нормативная SUPTO-матрица и machine traceability;
3. as-built integration и operations guides;
4. ADR, объясняющие принятые архитектурные решения.

Незакрытые production-ограничения ведутся в
[`../MVP_GATES.md`](../MVP_GATES.md) и
[`../MVP_BLOCKERS.md`](../MVP_BLOCKERS.md). Текстовый статус в старом ТЗ не
может переопределять machine traceability или evidence gate.

## Актуальная архитектура и интеграция

- [`MVP1/README.md`](MVP1/README.md) — только актуальный незакрытый MVP1 backlog.
- [`MVP1/BLUECASH_BLE_MVP_REMEDIATION.md`](MVP1/BLUECASH_BLE_MVP_REMEDIATION.md)
  — требования устранения несовместимости BlueCash direct BLE, aggregate
  `SALE_FINALIZE` и скидок.
- [`system-overview.md`](system-overview.md) — состав системы, границы продуктов
  и текущие production gates.
- [`EXTERNAL_POS_INTEGRATION_PROTOCOL.md`](EXTERNAL_POS_INTEGRATION_PROTOCOL.md) —
  единый публичный REST/WebHook/BLE протокол внешнего POS, сразу реализующий
  профиль `BG_SUPTO_FULL`.

Нормативные машинные контракты находятся в [`../contracts`](../contracts):

- [`openapi-runtime-v1.yaml`](../contracts/openapi-runtime-v1.yaml);
- generated TypeScript contracts в [`../contracts/generated`](../contracts/generated);
- Annex 29 traceability в
  [`../contracts/supto-annex29-trace.json`](../contracts/supto-annex29-trace.json);
- security и BG requirements matrices в
  [`../contracts/security-regression-matrix.json`](../contracts/security-regression-matrix.json)
  и [`../contracts/bg-requirements-trace.json`](../contracts/bg-requirements-trace.json).

## Smart devices и Edge

- [`BLUECASH_POS_INTEGRATION.md`](BLUECASH_POS_INTEGRATION.md) — актуальный
  activation/binding lifecycle BlueCash и BeeFiscalApp.
- [`BLUECASH_END_TO_END_TRACEABILITY.md`](BLUECASH_END_TO_END_TRACEABILITY.md) —
  as-built трасса REST → MQTT/BLE → Android/vendor protocol → journal/sync/WebHook.
- [`EDGE_DEVICE_REGISTRY_DATA_DICTIONARY.md`](EDGE_DEVICE_REGISTRY_DATA_DICTIONARY.md) —
  таблицы registry, bindings, capabilities и revocations.
- [`../IoT/firmware/edge-agent-s3/README.md`](../IoT/firmware/edge-agent-s3/README.md) —
  сборка ESP32-S3, device identity и SD/SQLite.

Device identity ESP32-S3 создаётся самим контроллером при первой инициализации:
ECDSA P-256, подпись IEEE P1363 `r || s`, public JWK/RFC 7638 thumbprint. Внешний
криптографический provider и ATECC608A не являются частью архитектуры. Закрытый
ключ не доступен через firmware API. Secure Boot, Flash/NVS Encryption,
anti-rollback eFuse и закрытие JTAG не блокируют controlled non-production MVP,
но обязательны до выдачи production credentials и установки у клиента.

## Болгарский SUPTO baseline

- [`SUPTO/index.md`](SUPTO/index.md) — нормативная conceptual matrix.
- [`SUPTO/SUPTO_BG014_REMEDIATION_IMPLEMENTATION_SPEC.md`](SUPTO/SUPTO_BG014_REMEDIATION_IMPLEMENTATION_SPEC.md) —
  acceptance specification, на которую ссылается machine trace.
- [`decisions/`](decisions) — действующие ADR по thin POS, Local Compliance
  Gateway, FMIN/УНП и offline ranges.

Спецификация remediation не является отчётом о текущем прогрессе. Фактические
статусы `PASS/PARTIAL`, gaps и evidence берутся из machine traceability и
[`../IMPLEMENTATION_STATUS.md`](../IMPLEMENTATION_STATUS.md).

## Эксплуатация и выпуск

- [`operations-runbook.md`](operations-runbook.md) — запуск DEV/PROD,
  эксплуатационные проверки, backup/restore и incidents.
- [`rollback-plan.md`](rollback-plan.md) — безопасный rollback без отката
  фискальной истории.
- [`support-guide.md`](support-guide.md) — severity, диагностика и escalation.
- [`release-evidence.md`](release-evidence.md) — SBOM, scan, подпись и release
  evidence gate.

## Удалённые legacy-документы

Исходные implementation tasks, промежуточные activation/MQTT proposals,
устаревшие readiness-аудиты и ранняя MiniPOS reference specification удалены
2026-08-14. Их заменяют OpenAPI/AsyncAPI, machine traces, as-built guides и
актуальный [`MVP1/README.md`](MVP1/README.md).
История решений сохраняется в Git и действующих ADR.

## Документационная дисциплина

- Не копировать схемы request/response из OpenAPI в новые документы без ссылки
  на canonical schema.
- После изменения API запускать `make generate-openapi` и `make contract-test`.
- Текущий факт реализации подтверждать тестом/evidence, а не текстовым статусом.
- Нереализованные или внешние ограничения фиксировать в machine trace,
  `MVP_GATES.md` или `MVP_BLOCKERS.md`.
- Устаревший проект решения удалять после появления актуального as-built guide;
  ADR сохранять, если решение продолжает определять архитектуру.
