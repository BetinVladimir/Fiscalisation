# ADR-PAYMENT-CAPABILITY-001 — Payment Type Capability Constraints

## Контекст
Не все ФУ поддерживают оплату картой (CARD) или разделённую оплату (SPLIT). Daisy Compact — только CASH до подтверждения I/O. Smart S — только LAB без УНП. Попытка CARD/SPLIT на неподдерживающем устройстве может привести к частичной фискализации.

## Решение
- `ValidatePaymentTypeForDevice` проверяет совместимость типа оплаты с возможностями устройства из `device_capabilities`.
- Daisy Compact: CARD и SPLIT запрещены до подтверждения I/O. Нарушение → `PAYMENT_TYPE_NOT_SUPPORTED`.
- Smart S: LAB-режим без УНП; в PROD режиме — UNKNOWN, не синтетический номер.
- BC-50: CARD через внешний терминал; saldo через cmd43; валютный режим 2/3 (BGN/EUR).
- E-чек: выключен; проверять отказ, не реализовывать отправку.

## Последствия
- Все пути оплаты проверяют `device_capabilities` перед созданием операции.
- Добавление нового типа оплаты требует обновления capability-матрицы ARCH_17.
- Тесты мокируют capability без обращения к реальному ФУ.

## Тесты, обеспечивающие решение
- `TestPaymentTypeCapabilityMatrix` — fiscal-backend
- `TestNHIFPaymentWithoutCapabilityRejectedBeforeIO` — fiscal-backend
- `TestNHIFPaymentWithCapabilityAllowed` — fiscal-backend
- `TestEReceiptUnsupported` — fiscal-backend
