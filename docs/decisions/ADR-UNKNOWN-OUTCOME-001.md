# ADR-UNKNOWN-OUTCOME-001 — Handling Unknown Fiscal Outcomes

## Контекст
Таймаут записи в ФУ или потеря ответа MQTT оставляет операцию в неизвестном состоянии. Нельзя определить: была ли операция зафиксирована в ФУ. Повторная попытка может создать двойной документ. Автоматический возврат может вернуть деньги клиенту без фискального документа сторно.

## Решение
- Операция переходит в статус `UNKNOWN` при таймауте записи.
- В состоянии `UNKNOWN` запрещены: повторная запись (без lookup), автоматический возврат карты.
- Разрешены только: `lookup` (чтение последнего документа ФУ), `reconciler` (через `FindStuckOperations` → `ReconcileOperation`).
- После успешного lookup: если документ найден → `FISCALIZED`; если не найден → `FAILED`.
- Ошибка хранилища останавливает денежный побочный эффект (инвариант 5).

## Последствия
- Все пути, выполняющие операции с ФУ, должны обрабатывать UNKNOWN явно, а не как FAILED.
- Dashboard UNKNOWN-операций должен отображаться оператору для ручной верификации.
- Тесты должны включать scenario с timeout before write, timeout after write (возможно FISCALIZED).

## Тесты, обеспечивающие решение
- `TestPersistentRepositoryRecoversOrphanReplayAsUnknownWithoutReclaim` — fiscal-backend
- `TestExpiredDurableCommandBecomesUnknownWithWebhook` — fiscal-backend
- `TestOfflineFinalizeV2UnknownFinalAndLaterDuplicate` — fiscal-backend
- `TestDurableQueueIsAtomicAndSurvivesRestartWithoutUnknown` — fiscal-backend
