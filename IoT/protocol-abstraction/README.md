# Universal Protocol Abstraction Layer

Слой унификации протоколов фискальных устройств.

Расположение в монорепозитории: `IoT/protocol-abstraction`.

## Цель
Преобразование canonical BeeFiscal команд в vendor-specific Daisy 93PC и
Datecs Bulgaria v2.11.4 протоколы для двух разрешённых IoT tracks MVP.

## Функции
- Compile-time выбор подтверждённого device profile; неподтверждённый profile не автоактивируется.
- Маппинг команд и форматов данных.
- Нормализация ответов и ошибок.
- Версионирование адаптеров.

## Модель сборки

- Это C++/Arduino модуль, который собирается вместе с firmware из `IoT/firmware`.
- Целевой протокол выбирается на этапе компиляции через конфигурацию препроцессора.
- Для разных устройств формируются разные firmware-артефакты с разными флагами сборки.

Пример идеи конфигурации (PlatformIO `build_flags`):

```ini
build_flags =
	-DPROTOCOL_EPSON
```

```ini
build_flags =
	-DPROTOCOL_DATECS
```

## Планируемая структура
- `adapters/daisy/`
- `adapters/datecs/`
- `contracts/` — канонические команды/события.

Текущий registry классифицирует все 88 Daisy и 73 Datecs command IDs. Любой
неизвестный код отклоняется, а conditional payment commands 194/55 остаются
`Excluded` до закрытия vendor/acquirer P0. Frame codec реализует wire envelope,
length/BCC/status extraction. Execution-critical commands now include validated
Daisy/Datecs payload builders and golden parsers for open receipt, item,
payment, close/cancel mapping, X/Z report, EUR cash movement and receipt status.
Datecs coverage additionally validates storno opening with original-document,
invoice and УНП binding (43), PC connectivity/print control (45), programmed
PLU sales (58), typed NRA connection/delivery state (71), fiscal-memory
date/Z-range reports (94/95), operator reports (105), all PLU report modes
(111), VAT-rate reads (50), subtotal/discount
(51), clock reads (62), last-fiscal-entry selection/results (64), daily taxation
(65), remaining FM Z-report capacity (68), last fiscal-record time (86), item
groups/departments and fiscal-memory self-test (87/88/89), device diagnostics
(90), tax-number reads (99), negative error-code lookup (100),
current-receipt state (103), all daily/operator totals variants (110/112),
BC-50-only currency conversion (115), bounded binary fiscal-memory reads (116)
all device-information options (123), typed receipt-period search (124) and EJ
document/text/base64/CSV transport modes (125) and structured fiscal-memory
capacity/Z/identity/tax/VAT/NRA/KLEN records (126), plus separated modem
identity/signal diagnostics (135), including
all eight tax groups and the documented ranges, identity, invoice, reversal and
date/time constraints. The executable gate currently proves 8 Daisy and 40
Datecs semantic commands and covers every Datecs command classified as
`Supported`; further expansion concerns only `Optional`, `Privileged` or
`Excluded` branches and cannot be silently activated.

Datecs 135 is deliberately canonicalized as `GetDiagnosticInfo`; modem signal
and identity are not evidence that the device transmitted data to NRA. That
regulatory state remains a distinct command/metric and is parsed from the
documented 13-field command 71 response.
Payment codes are accepted only from an explicitly configured device profile;
the abstraction does not guess terminal mappings. Remaining command-specific
semantics and all HIL evidence continue to be tracked separately and are not
production-approved.
