# ADR-TIME-BUSINESS-DATE-001 — Business Date, Clock Drift, Europe/Sofia

## Контекст
Фискальный день ФУ определяется внутренними часами устройства в часовом поясе `Europe/Sofia`. Расхождение часов системы и ФУ более 2 часов может привести к неверной дате X/Z-отчёта. НАП требует совпадения даты операции и даты отчётности.

## Решение
- `business_date` всегда вычисляется от текущего времени ФУ (cmd62 для BC-50) в часовом поясе `Europe/Sofia`.
- Пороги: drift > 5 мин → WARN-лог; drift > 2 ч → BLOCK новых операций до синхронизации.
- `SyncWorkstationClock` выполняет синхронизацию. Без синхронизации за последние 24 часа — BLOCK.
- Симулятор и тесты используют контролируемые `time.Time`, не `time.Now()`.

## Последствия
- `SyncWorkstationClock` должна быть вызвана при старте смены и ежедневно.
- Все тесты с временными метками используют фиксированное время; нет `time.Sleep` в логике.
- Z-отчёт содержит `business_date` из `Europe/Sofia`, а не UTC.

## Тесты, обеспечивающие решение
- `TestClockDriftConstants` — fiscal-backend
- `TestBusinessDateUsesEuropeSofia` — fiscal-backend
- `TestClockDriftOver2hBlocksSales` — fiscal-backend
- `TestClockDrift5minWarnsOnly` — fiscal-backend
