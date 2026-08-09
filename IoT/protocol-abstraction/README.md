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
Daisy coverage additionally implements the documented 2026 tax-rate period
query (50), subtotal/adjustment command and eight tax totals (51), exact device
clock result (62), net/total selectors with last/current eight-group sale and
refund totals (64/65), fiscalization/BGN-to-all report counters (66), and the
logical/physical free fiscal-record invariant (68). All fields fail closed on
bad cardinality, date shape, decimal shape, counter order or inconsistent FM
capacity.
Daisy device/readiness coverage also implements diagnostic identity and checksum
(90), current VAT rates (97), fiscalized EIK state (99), current receipt and
invoice state (103), last document (113), first receipt not sent to NRA with
explicit EJT error/all-sent states (117), e-shop and BIM/NRA firmware certificate
information (118), correlated status-byte error lookup (174), and all/single
BGN-to-EUR transition-date queries (201). Command 201 validates zone 0..4,
daily-Z requirement, four DDMMYY/unset dates, exact five-decimal exchange rate
and the last FM date.
Daisy sale/report semantics additionally cover sale-and-display (52),
programmed PLU/barcode sale and correction (58), detailed/brief FM reports by
number and date with optional payment totals (73/79/94/95), operator report
(105), every documented daily/PLU/department report mode (108/111/165), cancel
receipt (130) and system-parameter printing (166). P/F acknowledgements,
closure/tax/refund totals and command-specific empty payloads are validated.
Daisy coverage now also includes every command classified as `Supported`:
current-day payment distribution/counters (110), operator sales/adjustment and
refund-reason totals (112), FM sum/net/tax/rate queries by record and date with
explicit `P/F/E` states (114/146), issued-document QR and full document/SHA1
evidence (116/119), all 26 reported device constants (128), department
sale/correction grammar (138), and the acknowledged text-report line stream
(153). Document flags, UNP/invoice/refund links, grouped SHA1, Bulgarian eight
tax groups, line sequence/font/framing and all single/range selectors fail
closed on malformed data.
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
date/time constraints. The executable gate currently proves 45 Daisy core plus
all 16 Daisy `Optional` commands. Optional semantics include display clear/text/
row/clock/configuration (33/35/46/47/63/133), paper feed/cut and drawer
(44/45/106), invoice customer data (57), printed diagnostics/tax rates
(71/176), all documented barcode and customer-QR modes (84/85), the full
customer program/delete/read/iteration directory (152), and model-conditional
Compact battery telemetry (173). Empty responses, P/F results, barcode-specific
alphabets/lengths, QR template identifiers, display command hex bodies and
battery ranges are enforced while disposition remains `Optional`.
The same gate proves 40 Datecs core semantic commands and covers every Datecs command classified as
`Supported`. All 12 `Optional` Datecs commands have typed semantic coverage,
including display, printer, sound, barcode and drawer builders, invoice fields
(57) and every client-directory option (140), plus golden/negative vectors
while retaining `Optional` disposition.
They and all `Privileged`/`Excluded` branches remain separately gated and
cannot be silently activated.

Datecs 135 is deliberately canonicalized as `GetDiagnosticInfo`; modem signal
and identity are not evidence that the device transmitted data to NRA. That
regulatory state remains a distinct command/metric and is parsed from the
documented 13-field command 71 response.
Payment codes are accepted only from an explicitly configured device profile;
the abstraction does not guess terminal mappings. Remaining command-specific
semantics and all HIL evidence continue to be tracked separately and are not
production-approved.
