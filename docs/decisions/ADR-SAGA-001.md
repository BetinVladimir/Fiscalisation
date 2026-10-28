# ADR-SAGA-001 — Fiscal Operation Saga: Storno-First, UNKNOWN Idempotency

## Контекст
Фискальная операция охватывает несколько систем: POS, fiscal-backend, MQTT/BLE, ФУ, карточный терминал. Сбой на любом шаге может привести к частичному завершению. Неправильное восстановление (повторная операция, преждевременный возврат) нарушает фискальную отчётность и инварианты ФЗ.

## Решение
1. **Storno-first**: возврат по карте выполняется только после получения подтверждённого фискального сторно. Карточная операция без документа недопустима.
2. **UNKNOWN ≠ FAILED**: после таймаута записи нет повторной записи и нет автоматического возврата. Состояние UNKNOWN требует ручного или reconciler-based lookup.
3. **Idempotency key**: повтор запроса с тем же ключом возвращает исходный результат без создания новой операции.
4. **No synthetic document numbers**: номер документа берётся только из ответа ФУ.

## Последствия
- `finalizeSaleForTenant` и `reverseForTenant` должны проверять результат storno перед инициированием card refund.
- Reconciler (`ReconcileOperation`) обрабатывает UNKNOWN-операции только lookup-командами, не повторными денежными командами.
- Все новые saga-шаги требуют явного теста для UNKNOWN-ветки.

## Тесты, обеспечивающие решение
- `TestUnknownOutcomeRequiresReconciliationAndNoBlindRetry` — fiscal-backend
- `TestReconcileUnknownOperationOnly` — fiscal-backend
- `TestReconcilerResolvesExecutingOperations` — fiscal-backend
- `TestReportFinalPublicationIsAtomicAndRestartRecoversUnknown` — fiscal-backend
