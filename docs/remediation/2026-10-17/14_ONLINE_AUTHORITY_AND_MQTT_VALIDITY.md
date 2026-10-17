# Online UNP, MQTT validity и serial-ledger

Дата: 2026-10-18. Статус: интегрировано; полный IoT gate и backend domain/MQTT/API завершились exit 0. Финальная целевая сборка S3 завершилась exit 0.

## Реализовано

- SERVER_ASSIGNED сохраняет существующий серверный UNP, расходует только operation sequence и не расходует offline sale range. Подписанная authority связывает source, server_sale_id, UNP, payload_sha256, operation_id, receipt_session_id; изменённые строки/оплаты отклоняются до I/O.
- Backend и MQTT используют общий BuildFinalizePayload. Синхронизация проверяет immutable подготовленную команду и исторический binding; UNP существующей продажи не заменяется. Исправлен пустой 204 ответ привязки identity оператора.
- SQLite запрещает повторное использование sale UNP/serial sequence под другой операцией, пересечение текущих и исторических диапазонов, включая конкурентные подключения. REVERSAL использует исходный документ.
- Подписанные MQTT issued_at/expires_at обязательны: строгий UTC RFC3339Nano, issued <= now < expires. Проверка выполняется после HMAC, внутри reservation transaction и перед первым device I/O. Просрочка после резервирования сохраняет terminal REJECTED/COMMAND_EXPIRED; recovery не исполняет команду.
- Общий атомарный serial-ledger связывает actual fiscal serial и FM, сохраняет highwater онлайн/офлайн, проверяет исторические коллизии и rollback. Наличие старых FM-leases не считается устранённым по одному status=REVOKED: миграция блокируется до доказанного отзыва физических офлайн-полномочий.

## Evidence и ограничения

После интеграции online пакетов: actual authority20 PASS; backend domain/MQTT/API PASS; S3 cross-build PASS, RAM 39932/327680, Flash 1658028/3145728. Эти результаты получены до последующего добавления MQTT validity/serial-ledger; они не доказывают последнюю сборку.

Первый общий IoT gate остановился на двух устаревших source-token тестах. Они удалены, поскольку соответствующее поведение проверяется actual runtime authority/storage suites. Остальные Python29 PASS. Повторный полный gate: `/private/tmp/fiscal-final-authority-iot-20261018.log`; backend: `/private/tmp/fiscal-final-authority-backend-20261018.log`. Оба прогона завершились exit 0. [Сохранённые логи и 349 source hashes](evidence/authority-2026-10-18/README.md). Последняя целевая сборка S3 завершилась exit 0: RAM 39940/327680, Flash 1659640/3145728; лог и хеши firmware сохранены в evidence.

Открыты: MQTT trusted UTC/rollback evidence на устройстве; безопасный legacy lease fence; typed activation/provisioning caller с actual fiscal serial (не S3 board serial); полный MiniPOS authorization/client proof flow; HIL USB/COM/BLE, Android/iOS и внешнее подтверждение применимости НАП. Ключи остаются во внутренней NVS ESP32-S3, ATECC608A не используется.
