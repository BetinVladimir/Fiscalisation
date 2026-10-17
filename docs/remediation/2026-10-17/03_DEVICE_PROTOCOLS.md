# Протоколы устройств: источники и фактические реализации

[Навигация](README.md) · [Разрывы](02_GAP_REGISTER.md)

Проверка интернет-источников: **2026-10-17**. Источник от производителя означает происхождение спецификации, а не подтверждение применимости ко всем аппаратным ревизиям. Перед реализацией сохранять URL, downloaded SHA-256, версию/язык/регион, supported model/firmware и допустимость использования SDK. «Самый новый найденный документ» не означает «подходит к нашему экземпляру».

## Реестр источников

| ID | Локальный источник / версия | Официальный источник в интернете | Вывод и ограничение |
|---|---|---|---|
| V-01 | `BeeloyBackend/docs/Fiscal/beefiscal_IoT/doc/Datecs/PM_XXXXXX-BUL_CommunicationProtocol_v2.11.4 (7).pdf`, 17-Nov-2022 | [Datecs PM v2.11.4](https://www.datecs.bg/en/downloads/pdf?id=PM_XXXXXX-BUL_CommunicationProtocol_v2.11.4.pdf) | Версия совпадает по титулу; применимость сверять по model list, включая BC-50 и DP-150X/MX. Полное побайтовое тождество локального/remote PDF не проверено |
| V-02 | `…/daisy/93PC_FD_CProtocol_ver_2_0-4_2026_EN.pdf`, v2.0-4 2026 | [Daisy communication protocol](https://daisy.bg/filemanager/files/downloads/Communication%20protocol/93PC_FD_CProtocol_ver_2_0-4_2026_EN.pdf) | Официальный PDF той же версии; для Compact S 01 сверить firmware/interface/model capability |
| V-03 | `…/Datecs/BluePad-50-Plus.pdf` | [Datecs BluePad-50 Plus](https://www.datecs.bg/en/products/321), [официальная спецификация](https://www.datecs.bg/en/downloads/pdf?id=BluePad-50-Plus.pdf) | Подтверждает BLE и OS compatibility. Это datasheet, не спецификация TLV, UUID, STAN/refund или банковского протокола |
| V-04 | `BeeloyBackend/docs/Fiscal/Android/DatecsBC/{Fiscal,PinpadSDK}/DEMO` и SDK packages | [Datecs BlueCash-50](https://www.datecs.bg/en/products/322) | В репозитории уже подключены Android fiscal/pinpad JAR. Публичная карточка сегодня указывает Android15; это не версия OS имеющегося устройства. Версии SDK/OS/firmware записать с него |
| V-05 | `…/Termol/protocol_description_BG_2507141400.pdf` | [официальный BG ZFPLAB2507141400](https://tremol.bg/bg/support/zfplab/bg/2507141400) | Нужен именно BG profile. Другие country/version страницы содержат отличающиеся команды, passwords и payment modes |
| V-06 | `…/Eltrade` EN/BG protocol1.3.2; `…/Fibank/ECR protocol Wiseasy 2.0.1.pdf` | В этом этапе не установлена новая официальная applicable revision | Самостоятельных физических runtime profiles Eltrade/Wiseasy в исследованных целевых compositions нет. Нельзя применять эти PDF к DatecsPay/BluePad |
| V-07 | ESP-IDF runtime / USB CDC component | [Espressif USB Host ESP32-S3](https://docs.espressif.com/projects/esp-idf/en/stable/esp32s3/api-reference/peripherals/usb_host.html) | Различать USB-OTG host и USB-Serial-JTAG; нужны соответствующие CDC/VCP driver и питание/защита VBUS на конкретной плате |
| V-08 | `react-native-ble-plx` native client | [Apple maximumWriteValueLength](https://developer.apple.com/documentation/corebluetooth/cbperipheral/maximumwritevaluelength(for:)), [write completion callback](https://developer.apple.com/documentation/corebluetooth/cbperipheraldelegate/1518823-peripheral) | iOS write limit и completion брать из runtime API/библиотеки, не из Android requestMTU constant |

Пути `BeeloyBackend/…` расположены в соседнем репозитории `/Users/freelancer/Documents/Beeloy/BeeloyBackend`; они не входят в standalone checkout Fiscalisation. Для CI нужна законная versioned fixture-поставка, а не абсолютный путь разработчика. SDK demos — источник требований для transport handshake, но не доказательство успешной оплаты на S3.

## Daisy: расхождения actual runtime

По V-02: CP1251 для текста; retransmit сохраняет SEQ; NAK=15h, SYN=16h; timeout минимум500ms, SYN каждые100ms. BCC — четыре nibble с добавлением30h, поэтому значения A–F передаются как `:;<=>?`. LEN включает себя, SEQ, CMD, DATA и postamble.

Расчёт TX без DATA: LEN=`0x24`; сумма `24+50+4A+05=00C3`, BCC=`30 30 3C 33`. Из раздела примера протокола, стр.7:

```text
TX status74: 01 24 50 4A 05 30 30 3C 33 03
RX:          01 31 50 4A 88 80 80 80 80 B8 04 88 80 80 80 80 B8 05 30 37 35 34 03
```

Независимый host probe компилировал **файл, включаемый IDF**, `IoT/protocol-abstraction/src/FrameCodec.cpp`, с C++17. Наблюдалось:

```text
Daisy TX: 01 23 50 4A 05 30 30 43 32 03
Daisy vendor RX accepted: 0 DAISY_LENGTH
```

Это не предположение о кабеле или устройстве: mismatch воспроизводится без железа. RX length formula также смещена на1. Common-modules driver и facade tests не покрывают этот runtime codec. Закрытие: GAP-009, T-05/T-06/T-07.

Дополнительно сверить vendor-specific storno reason, VAT bytes, text byte limits, payment slots и password length. Нельзя передавать общий числовой reason без vendor mapping. В Daisy причины refund/operator error имеют другое соответствие числам, чем Datecs.

## Datecs fiscal: независимая проверка формата

V-01, разделы3.3–3.4: LEN считает байты после PRE до PST включительно плюс20h; LEN/CMD/BCC используют nibble+30h. Для request это `N+0x2A`; response с8status bytes — `N+0x33`. Ограничения DATA:496/480bytes; SEQ20h…FFh. Повтор SEQ возвращает последний ответ без повторного исполнения. NAK требует тот же SEQ, SYN продлевает ожидание.

В C++ `DatecsCodec` и Kotlin `DatecsFrameCodec` request offset=`0x26`, response offset=`0x2F`. Probe actual C++ для empty DATA:

```text
Actual LEN bytes:   30 30 32 36
Expected LEN bytes: 30 30 32 3A
```

Также нужно анализировать status **по номеру байта и mask**, а не bit0 любого byte, и typed command ErrorCode. Command43 storno содержит original document/time/FM/UNP; reason0=operator error,1=refund,2=tax base reduction. Эти требования сверены с V-01; применимость новых валютных режимов2026 отдельно подтвердить по firmware экземпляра. Закрытие: GAP-010/011/012/015/021, T-05…T-07.

Кодировка Datecs string fields требует подтверждения для выбранной модели и bridge: текущий Kotlin явно UTF-8. Нельзя автоматически считать это правильным либо переносить CP1251 из Daisy без model-specific evidence. Зафиксировать тест на болгарском тексте и byte limits.

## BluePad: не смешивать физическую модель и payment application

Официальный [BluePad-50](https://www.datecs.bg/en/downloads/pdf?id=BluePad-50.pdf) описывает Bluetooth3.0 SPP/iAP, а V-03 для **BluePad-50 Plus** — BLE4.2. Поэтому «BluePay по BLE» в плане нормализовано как проверяемый кандидат **Datecs BluePad-50 Plus**, но точное название/прошивку экземпляра и acquirer app необходимо записать. Другой терминал/платёжная программа требуют собственного adapter profile.

Локальный `IoT/common-modules/datecspay/DatecsPayBleProfile.h` и README, ссылающиеся на vendor demo, задают:

- service prefix `d839fc3c-84dd-4c36-9126-` и четыре характеристики POWER/WAKE/READ/WRITE;
- подписки на READ и POWER, wake peer name при power30h и ожидание31h;
- fragments <=19bytes с ожиданием каждого write completion.

Actual IDF central имеет только configured service/TX/RX, без POWER/WAKE и completion queue. Это расхождение с **локальным SDK-derived profile**, а не claim, что публичный datasheet документирует эти UUID. Полный applicable payment transport/protocol manual не найден среди подтверждённых открытых официальных источников; требуется version/hash SDK/spec от производителя или эквайера. STAN/RRN, lookup, VOID/refund, duplicate receipt и card-approved/fiscal-failed компенсацию подтвердить им. GAP-013/014/019, T-08/T-09.

## USB Daisy и COM Datecs на S3

`usb_cdc_runtime.cpp` открывает устройство по VID/PID/interface через CDC ACM host; это не доказательство, что конкретный Daisy exposes совместимый USB class. Снять descriptors с реального экземпляра, закрепить VID/PID/interface/serial/line coding и reconnect. Проверить short packets, queue overflow и detach во время записи; unknown после частичного I/O не становится NOT_SENT.

`uart_runtime.cpp` конфигурирует UART, но `uart_runtime_ready()` означает установленный драйвер, а не ответ фискального устройства. COM/RS-232 требует согласованной распиновки и transceiver; TTL UART нельзя считать RS-232 интерфейсом. Перед денежной операцией нужен protocol-level probe и отдельные transport/device health states. USB VBUS/current budget, питание принтера и SD pinout задаются схемой конкретного стенда, не названием чипа S3. T-06/T-20/T-21.

## Tremol и неподключённые протоколы

Tremol driver использует STX02h/terminator0Ah, XOR checksum, NBL и ACK/message responses; эти paths независимы от Datecs/Daisy. BG2507141400 локального PDF совпадает с versioned страницей ZFPLAB, однако полнота всех commands не доказана. Проверить command payload variants, length/truncation, invalid checksum nibbles, NBL wrap/response matching, Retry/busy и STE errors (T-24). До включения physical profile — явно вне целевого HIL acceptance.

Daisy SMART Android и Daisy USB printer — разные интеграции. Официальная [страница Daisy Smart Pro](https://daisy.bg/product/smart-pro) не заменяет Android SDK/API SMART S. Eltrade/Wiseasy PDF не реализуют отсутствующий runtime adapter. Не объявлять эти продукты поддержанными по наличию документации.
