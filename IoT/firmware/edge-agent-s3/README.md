# BeeFiscal edge-agent-s3

PlatformIO firmware skeleton for an ESP32-S3 camera-class module acting as a
local fiscal edge adapter. It creates fiscal/payment drivers through
`IoT/protocol-abstraction` and maintains a durable SQLite transaction journal on
the built-in SD card.

## Build

```bash
cd IoT/firmware/edge-agent-s3
pio run
pio run -t upload
pio device monitor
```

Pinned dependencies:

- PlatformIO `espressif32@6.10.0`;
- Arduino framework supplied by that platform;
- `siara-cc/Sqlite3Esp32@2.5` from the
  [PlatformIO registry](https://registry.platformio.org/libraries/siara-cc/Sqlite3Esp32).

## Board and pins

The default compile target is `esp32-s3-devkitc-1` with 8 MB flash and OPI
PSRAM. “ESP32-S3 Camera Module” is not one unique board/pinout. Before hardware
deployment verify the exact module schematic and override:

```ini
-DEDGE_SD_CLK=<gpio>
-DEDGE_SD_CMD=<gpio>
-DEDGE_SD_D0=<gpio>
-DEDGE_FISCAL_UART_RX=<gpio>
-DEDGE_FISCAL_UART_TX=<gpio>
```

When all `EDGE_SD_*` values are `-1`, Arduino `SD_MMC` board defaults are used.
The storage runs in 1-bit SD mode to reserve pins for camera, UART
and other peripherals. GPIO 17/18 in the example are placeholders and
must also be checked against camera/PSRAM wiring.

## Подключение устройств: GPIO, UART/RS-232, USB и BLE

В текущей версии ключи хранятся во внутренней NVS ESP32-S3. Внешний модуль ATECC608A/ATECC608 не используется: подключение по I²C, Qwiic/STEMMA QT и выделение GPIO для него не требуются.

Основная прошивка текущего стенда — **ESP-IDF в `idf/`**. Команды для неё:

```bash
pio run -d IoT/firmware/edge-agent-s3/idf
pio run -d IoT/firmware/edge-agent-s3/idf -t upload
```

Команды из раздела Build выше запускают соседний Arduino skeleton. Номера ниже — **GPIO ESP32-S3**, а не порядковые номера контактов гребёнки. Для конкретной camera-платы сначала проверить её схему: камера, SD и PSRAM могут занимать эти выводы.

### Карта подключений стенда

| Устройство / сигнал | Вывод ESP32-S3 | Подключение и настройка |
|---|---|---|
| Datecs DP-150 MX, передача UART1 | GPIO17, TX | На TTL-вход передатчика RS-232-модуля (`T1IN`); `fiscal_endpoint.uart.tx_pin=17` |
| Datecs DP-150 MX, приём UART1 | GPIO18, RX | С TTL-выхода приёмника RS-232-модуля (`R1OUT`); `fiscal_endpoint.uart.rx_pin=18` |
| RS-232-модуль, питание | 3V3 и GND | Модуль должен поддерживать питание и логические уровни 3,3 В |
| Daisy Compact S 01, USB D− | GPIO19 | Native USB OTG, линия D− |
| Daisy Compact S 01, USB D+ | GPIO20 | Native USB OTG, линия D+ |
| Daisy, USB VBUS / GND | Питание USB host 5 В / GND | Через предусмотренный схемой host power switch/ограничитель тока; VBUS не является GPIO |
| BluePad-50 Plus / согласованный BLE POS | GPIO не требуются | Радиосвязь BLE; терминал питается отдельно, его identity/UUID задаются binding |
| Android/iOS MiniPOS, BLE-настройка и команды | GPIO не требуются | Радиосвязь BLE с S3 |
| SD_MMC CLK / CMD / D0 | GPIO39 / GPIO38 / GPIO40 | Текущие build flags **IDF**: `EDGE_SD_CLK=39`, `EDGE_SD_CMD=38`, `EDGE_SD_D0=40`; режим 1-bit |

GPIO17/18 — выбранная схема стенда и значения Arduino-примера; **IDF не назначает их автоматически**. IDF берёт UART-пины из подписанного composite binding. Пример его фрагмента:

```json
{
  "uart": {
    "tx_pin": 17,
    "rx_pin": 18,
    "baud": 115200,
    "data_bits": 8,
    "parity": "N",
    "stop_bits": 1
  }
}
```

Это поле `fiscal_endpoint.uart`. Скорость `115200`, 8N1 — пример: настройки COM на кассе и в binding должны совпадать. В текущем `uart_runtime.cpp` аппаратный RTS/CTS выключен; RTS/CTS на GPIO не подключаются. Если установленная конфигурация кассы требует handshake, её сначала согласовать с поддерживаемым профилем.

### Datecs через «TTL To RS232 Female/Male Level Module Serial DB9»

Нужен преобразователь **UART TTL 3,3 В ↔ RS-232**, например модуль на MAX3232 с поддержкой 3,3 В. Само название «TTL To RS232» и пол DB9 не гарантируют совместимость: проверить микросхему, питание, направление сигналов и разводку конкретного модуля. MAX232/модуль с 5-вольтовым TTL-выходом нельзя напрямую соединять с RX ESP32-S3. RS-232 со стороны DB9 также нельзя подключать непосредственно к GPIO.

```text
ESP32-S3                      Преобразователь                    Касса Datecs
3V3 ------------------------> VCC (допускающий 3,3 В)
GND ------------------------> GND -----------------------------> signal GND
GPIO17 / UART1 TX ----------> T1IN  -> T1OUT (RS-232 TX) --------> RS-232 RX
GPIO18 / UART1 RX <---------- R1OUT <- R1IN  (RS-232 RX) <-------- RS-232 TX
```

На TTL-гребёнке продавец может написать `RXD/TXD` с точки зрения микроконтроллера либо самого модуля. Определять соединение по **направлению**: TX S3 идёт во вход преобразователя, RX S3 — с его выхода. Обозначения `T1IN/R1OUT` выше относятся к функциям MAX3232 и устраняют эту неоднозначность.

**DB9: сначала определить DTE/DCE.** Для стандартного DTE-порта DB9: контакт 2 — RXD, 3 — TXD, 5 — signal GND. Если модуль действительно разведен как DTE, его RS-232 TX выведен на 3, RX — на 2. Для DTE↔DTE нужен null-modem: `2↔3`, `3↔2`, `5↔5`; для DTE↔DCE обычно прямой кабель. Это условная стандартная схема, **не подтверждённая распиновка разъёма конкретного Datecs**.

Male/female определяет механическое соединение, а не необходимость перекрёста. У кассы может быть собственный COM-разъём и фирменный кабель, включая разъём, отличный от DB9. До подключения сверить руководство именно своей модели и прозвонить кабель/модуль; ориентироваться по номерам контактов на разъёме — вид со стороны пайки зеркален виду со стороны подключения. Не объединять неизвестные контакты питания с DB9 signal GND и не питать кассу от 3V3 S3. Касса использует штатный блок питания.

### Daisy по USB

S3 работает как **USB host**, Daisy — как USB device. Использовать native USB OTG, а не разъём USB↔UART-моста платы. Подключить `D−→GPIO19`, `D+→GPIO20`, общий GND и корректное host-питание VBUS 5 В. Предпочтителен готовый USB host-разъём/адаптер по схеме платы, а не длинные отдельные провода D+/D−. Не подавать 5 В на GPIO и исключить встречное питание от другого USB host/ПК.

Касса питается штатно; бюджет питания USB отдельно проверяется по её интерфейсу. Текущий драйвер S3 — **CDC ACM host**: в binding нужны реальные `usb_vid`, `usb_pid`, `usb_interface` и проверка USB descriptors экземпляра. Произвольный USB↔RS-232 адаптер не считается совместимым с этим драйвером. Native USB с внутренним PHY занят host-соединением; для одновременного мониторинга/прошивки использовать предусмотренный платой отдельный UART-мост/отладочный путь.

### Проверка перед включением

1. Сверить модель/ревизию платы S3, отсутствие конфликтов GPIO17/18/19/20/38/39/40 с камерой и памятью, а также реальные пины SD-слота.
2. Проверить 3,3 В на TTL-стороне RS-232-модуля, общую землю и направления TX/RX; определить тип DB9 и схему кабеля.
3. Внести фактические UART-пины/параметры или USB identity в подписанный binding. Соединение кабеля само по себе не является READY — нужен ответ устройства на диагностическую команду.

Источники и код:

- [Espressif: USB/GPIO и ограничения платы](https://docs.espressif.com/projects/esp-hardware-design-guidelines/en/latest/esp32s3/schematic-checklist.html): GPIO19=D−, GPIO20=D+.
- [TI MAX3232 datasheet](https://www.ti.com/lit/ds/symlink/max3232.pdf): питание микросхемы 3–5,5 В; характеристики готового модуля проверяются отдельно.
- [Datecs DP-150 MX — документы модели](https://www.datecs.bg/en/products/323): распиновку конкретного кабеля/разъёма подтвердить по применимому руководству.
- [IDF build flags SD](idf/platformio.ini), [UART1 runtime](idf/main/uart_runtime.cpp), [USB CDC host](idf/main/usb_cdc_runtime.cpp), [binding parser](idf/main/provisioned_binding.cpp).


## Protocol integration

`DeviceProtocolProvider` is a thin firmware-facing entry point for:

- `IFiscalDevice`: Daisy, Datecs or Tremol over serial-derived channels;
- `IPaymentTerminal`: DatecsPay over a BLE GATT `Stream` such as
  `DatecsPayBleStream`.

The two objects are intentionally independent. `main.cpp` creates a Daisy UART
profile only as a non-operating development default. It never sends fiscal
commands during boot. Production must load vendor, channel and payment code from
signed provisioning data.

## SD and SQLite

`EdgeStorage` mounts SD_MMC at `/sdcard`, creates
`/sdcard/beefiscal/edge-agent.db`, applies idempotent schema migration and
provides:

- idempotent enqueue by unique `transaction_id`;
- ordered iteration over unsent transactions;
- attempt/error accounting;
- acknowledgement (`markSynced`);
- deletion only for acknowledged records and only after at least 90 days;
- explicit optimization/checkpoint operation.

SQLite uses `journal_mode=DELETE` and `synchronous=FULL`. WAL is intentionally
not selected for removable FAT media. Payloads are expected to be canonical JSON
and signatures are persisted separately; signing and verification belong to the
secure identity/orchestration layer.

The database is not encrypted by SQLite. Do not place tokens or private keys in
it. Each device generates its own P-256 identity keypair on ESP32-S3 during
first initialization. The private key is stored in NVS and is never exposed by
the firmware API. MVP permits development NVS without Flash Encryption; this is
acceptable only on controlled non-production hardware with non-production
credentials. Production requires Flash Encryption and NVS Encryption to protect
the at-rest representation. No ATECC608A or external crypto provider is part of
this architecture.

## MVP and production security boundary

Secure Boot V2, Flash Encryption, NVS Encryption, anti-rollback eFuse, disabled
JTAG/download mode and the production recovery ceremony are **not MVP acceptance
requirements**. They remain mandatory before a production device or production
credential is issued.

The MVP must still fail closed and may not replace transport/application
security with a trusted-lab assumption. Device identity, backend trust,
authenticated and encrypted BLE/MQTT commands, durable pre-I/O journaling,
anti-replay, final-device reachability checks and asynchronous acknowledged sync
remain MVP requirements.

## Remaining hardware decisions

- exact S3 camera board name, revision and pin map;
- SD interface voltage/pull-ups and whether it supports SD_MMC or only SPI;
- fiscal UART electrical layer (native TTL versus MAX3232 RS-232);
- BLE transport instance and BluePad pairing policy;
- production-only secure boot, flash/NVS encryption, anti-rollback and recovery
  provisioning; these do not block the controlled MVP.
