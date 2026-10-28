# BluePad, локальное извлечение и отзыв маршрутов

Дата: 2026-10-18; база4b32636. [Evidence](evidence/bluepad-sqlite-2026-10-18/README.md).

BluePad central выполняет documented reference purchase → final01 → GET_RECEIPT_TAGS с точным DF03/amount/type/STAN/result → TX_END. Readiness ждёт CCCD/POWER/WAKE; отправка ждёт completion каждого19-byte chunk; disconnect/reset и stale callbacks ограничены generation/token. Unknown не приводит к автоматическому повтору оплаты. [Vendor mapping и ограничения](BLUEPAD_CENTRAL_HANDOVER.md). 38 actual-source host scenarios PASS, полный IoT PASS, S3 build PASS. Query/reversal не включены без доказанного протокола.

Добавлены [S3 SQLite read-only extractor](17_S3_SQLITE_EXTRACTION.md), actual-schema dictionary и11тестов. Это техническая часть N5/N6; не полное hybrid извлечение и не подтверждение юридической достаточности.

Отзыв активированного устройства теперь отзывает ACTIVE/PENDING composite bindings этого tenant, очищает зависимые adapter/physical/payment route links и отзывает относящиеся BLE sessions. Immutable issued binding evidence, физический кассовый ресурс, независимые маршруты и другие tenants сохраняются. Regression проверяет adapter, pending/stale routes, session scope, immutable evidence и idempotent repeat; domain/API/MQTT PASS.

**OPEN:** уже закэшированная offline authority и физический отзыв требуют отдельного fence/lease контракта. Отзыв backend routes не доказывает немедленную остановку отключённого S3, удаления NVS keys или отзыва сертификата на broker. Persistent terminal quarantine/reconciliation, полный trusted MiniPOS bridge, cloud/Android/web extraction, HIL Android/iOS USB/COM/BLE и внешняя применимость НАП остаются открытыми.
