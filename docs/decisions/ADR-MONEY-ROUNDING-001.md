# ADR-MONEY-ROUNDING-001 — Integer Cent Arithmetic with Half-Up Rounding

## Контекст
Все денежные суммы в системе (стоимость позиции, итог чека, сдача, платёж картой) должны быть точными, воспроизводимыми и соответствовать требованиям НАП. Использование `float`/`double` приводит к недетерминированному округлению (например, 0.1 + 0.2 ≠ 0.3) и к расхождению итогов между клиентом, серверном и ФУ.

## Решение
- Все суммы хранятся и передаются как **целые центы** (int64).
- Умножение количества на цену единицы: `(unit_cents × qty_milli + 500) / 1000` (half-up).
- Запрещено: `float`, `double`, `parseFloat`, `Number(price)` в денежных вычислениях.
- Граничный случай: `qty_milli` хранится как тысячные доли единицы (1000 = одна единица).
- Золотой чек: 1200¢ + 167¢ = 1367¢; CASH 500¢ / CARD 867¢.

## Последствия
- Любой новый код, производящий денежные вычисления, должен использовать целочисленную арифметику.
- Ревью PR проверяет отсутствие `float`/`parseFloat`/`Number(` в денежных путях.
- При получении JSON-суммы с float из внешней системы — конвертировать в центы на границе входа, не внутри логики.

## Тесты, обеспечивающие решение
- `TestReceiptMoneyCentsRejectsSignedAndMalformed` — fiscal-backend
- `TestMoneyRoundingGolden` — fiscal-backend
- `TestValidateReceiptPlanRejectsNegativePaymentThatSumsCorrectly` — fiscal-backend
- `TestValidateReceiptPlanRejectsItemsTotalMismatch` — fiscal-backend
