# MVP1 — открытые требования

Дата актуализации: 2026-08-14

Каталог содержит только незакрытые программные требования MVP1. Завершённые
спецификации, roadmap и промежуточные аудиты удалены; их история остаётся в Git.

Канонические общие контракты:

- [`../EXTERNAL_POS_INTEGRATION_PROTOCOL.md`](../EXTERNAL_POS_INTEGRATION_PROTOCOL.md)
  — единый REST/WebHook/BLE протокол с профилем `BG_SUPTO_FULL`;
- [`../SUPTO/index.md`](../SUPTO/index.md) — нормативная conceptual matrix;
- [`../../contracts/openapi-runtime-v1.yaml`](../../contracts/openapi-runtime-v1.yaml)
  — runtime OpenAPI;
- [`../../contracts/supto-annex29-trace.json`](../../contracts/supto-annex29-trace.json)
  — machine-readable SUPTO trace.

## Текущий программный backlog

- [`BLUECASH_BLE_MVP_REMEDIATION.md`](BLUECASH_BLE_MVP_REMEDIATION.md) — два
  обязательных P0: привести BlueCash direct BLE к `OPEN_MVP` wire profile и
  провести aggregate `SALE_FINALIZE` со скидками через общий durable processor.
- [`CARD_STORNO_AND_PRINTER_REMEDIATION.md`](CARD_STORNO_AND_PRINTER_REMEDIATION.md)
  — per-payment card orchestration, корректный возврат при сторно для edge и
  BlueCash, каноническая нефискальная команда тестовой печати.
- [`EDGE_MQTT_CONFIGURATION_AND_HEALTH_REMEDIATION.md`](EDGE_MQTT_CONFIGURATION_AND_HEALTH_REMEDIATION.md)
  — явный driver/protocol configuration, endpoint heartbeat/probe и честная
  materialization MQTT→REST.
- [`DEVICE_CONFIGURATION_AND_REALTIME_UI_REMEDIATION.md`](DEVICE_CONFIGURATION_AND_REALTIME_UI_REMEDIATION.md)
  — типизированный мастер настройки, realtime activity и безопасные тесты в
  BeeFiscalApp.
- [`LOCAL_HTTP_POS_CHANNEL_AND_MINIPOSWEB_IMPLEMENTATION.md`](LOCAL_HTTP_POS_CHANNEL_AND_MINIPOSWEB_IMPLEMENTATION.md)
  — третий Local REST/HTTP канал, signed A/B cache Web SPA на ESP32/BlueCash,
  offline token validation и эталонный React `miniposweb` с MiniPOS backend/БД.

Пока все перечисленные P0 не закрыты тестами, статус полного MVP —
`SOFTWARE_INCOMPLETE_HIL_PENDING`. REST→MQTT BlueCash и edge-agent-s3 BLE не
отменяют обязательность direct BLE fallback на BlueCash-50.

## После закрытия software P0

Остаются только физические HIL, vendor/acquirer acceptance и production
hardening. Их критерии перечислены в едином POS-протоколе и machine traces; они
не дублируются в этом каталоге.
