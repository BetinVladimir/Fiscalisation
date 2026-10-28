# Декомпозиция до функций: android

## BUG-028
Типизированная классификация. Файлы: `DatecsProtocol.kt`, `DatecsAndroidPorts.kt`, `BlueCashEngine.kt` (4 места `e.message in setOf` ~170,206,274,366).

1. **В `DatecsProtocol.kt` добавить `sealed class DatecsFailure(msg): Exception` с подклассами `NotSent` (до flush), `Rejected(val code:Int)` (разобран ответ, код≠0), `Unknown` (всё после flush без разобранного ответа); поле `val phase`**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt`
   - проверка: компилируется
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*DatecsProtocolTest*"`
2. **`DatecsSocketProtocol.execute`: исключения до `output.flush()` → `NotSent`; `IOException`/`SocketTimeoutException`/`DATECS_*` после flush → `Unknown`; ответ с кодом ошибки → `Rejected`**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt`
   - проверка: тест ниже
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*DatecsProtocolTest*"`
3. **`DatecsAndroidFiscalPort.execute`: ловит `Throwable`, закрывает сокет, **пробрасывает DatecsFailure как есть** (не оборачивает в RuntimeException)**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsAndroidPorts.kt`
   - проверка: тест ниже
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*DatecsAndroidPortsTest*"`
4. **`BlueCashEngine`: заменить 4 проверки `e.message in setOf(...)` на `when(e){is DatecsFailure.Unknown->...; is Rejected->...; is NotSent->...}`; хелпер `private fun classify(e:Throwable):Outcome`**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt`
   - проверка: grep пусто
   - приёмка: `! grep -n "e.message in setOf" SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt`
5. **Тесты `everyTransportErrorAfterWriteIsUnknown` (параметры: IOException, SocketTimeoutException, DATECS_LENGTH, DATECS_SEPARATOR, DATECS_FRAME_TOO_LARGE, DATECS_HEX, EOF), `errorBeforeWriteIsNotSent`, `parsedNonZeroErrorCodeIsRejected`**
   - файлы: `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocolTest.kt`
   - проверка: зелёные
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*DatecsProtocolTest*"`

## BUG-020
Закрытие открытого чека. Зависит от BUG-028 (Unknown). Номер команды отмены чека — из PM v2.11.4 (cmd 60 «Cancel receipt»: сверить с PDF в DOC-V1; значение записать в константу `CMD_CANCEL_RECEIPT` с комментарием раздела).

1. **`DatecsPayloads.cancelReceipt(): String` + константа `CMD_CANCEL_RECEIPT`; юнит-тест кадра**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt`, `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocolTest.kt`
   - проверка: зелёный
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*DatecsProtocolTest*"`
2. **В `sale()` (BlueCashEngine.kt:217-304), `reverse()` (:305-376), `printerTest()`: переменная `var receiptOpened=false`, true сразу после успешного cmd 48/43/38**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt`
   - проверка: компилируется
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashEngineTest*"`
3. **В `catch`: `if(receiptOpened && classify(e) !is Unknown) runCatching{fiscal.execute(cancelReceipt())}` → журнал `RECEIPT_CANCELLED`/`RECEIPT_CANCEL_FAILED`**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt`
   - проверка: тесты ниже
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashEngineTest*"`
4. **При Unknown — НЕ отменять; записать `RECOVERY_REQUIRED` (разрешает BUG-066/VG-04 через cmd 74/76)**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt`
   - проверка: тест ниже
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashEngineTest*"`
5. **Тесты с фейковым портом (последовательность команд): `saleCancelsOpenReceiptWhenLineRejected` = [48,49✗,CANCEL]; `…WhenPaymentRejected` = [48,49,53✗,CANCEL]; `saleDoesNotBlindCancelOnUnknown` = [48,49,53(EOF)] без CANCEL; `cancelFailureIsJournaled`**
   - файлы: `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngineTest.kt`
   - проверка: 4 теста зелёные
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashEngineTest*"`

## BUG-021
Не сторнировать карту при UNKNOWN. Зависит от BUG-028. Решение BUG-071 (D-12) определяет порядок для reverse.

1. **В catch `sale()` (:274-300): ветка `is Unknown` → журнал `RECOVERY_REQUIRED(card_state=APPROVED)`, `card.reverse` не вызывать; результат `FISCAL_RESULT_UNKNOWN`**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt`
   - проверка: тест ниже
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashEngineTest*"`
2. **Ветка `is Rejected` → `card.reverse` ровно один раз; журнал `CARD_COMPENSATED`**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt`
   - проверка: тест ниже
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashEngineTest*"`
3. **Тесты: `saleDoesNotReverseCardWhenFiscalResultUnknown` (reverseCalls==0), `saleReversesCardWhenFiscalDefinitelyRejected` (reverseCalls==1)**
   - файлы: `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngineTest.kt`
   - проверка: зелёные
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashEngineTest*"`
4. **Раздел «Восстановление после UNKNOWN» в `docs/BLUECASH_POS_INTEGRATION.md`**
   - файлы: `docs/BLUECASH_POS_INTEGRATION.md`
   - проверка: grep
   - приёмка: `grep -q "Восстановление после UNKNOWN" docs/BLUECASH_POS_INTEGRATION.md`

## BUG-022
Предвалидация всех payload до побочных эффектов. Лимиты: цена ≤ 99999.99?, количество — по PM; [*4] произведение ≤ 9999999.99 (VG-06).

1. **`DatecsPayloads.validateSale(lines,payments): List<String>` — чистая функция без I/O, возвращает коды `PAYLOAD_INVALID:<поле>` (name>72, price, quantity, tax A–H, отриц., Price×Qty>9999999.99)**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt`
   - проверка: юнит-тест
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*DatecsProtocolTest*"`
2. **`sale()`: вызвать `validateSale` ДО `journal.append(ACCEPTED)` и до `card.purchaseEur`; ошибка → `FAILED(PAYLOAD_INVALID:…)` без I/O**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt`
   - проверка: тест
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashEngineTest*"`
3. **`BlueCashComplianceIntent.line()`: quantity по регэкспу `^[0-9]+(\.[0-9]{1,3})?$` до `BigDecimal(...)`**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashComplianceIntent.kt`
   - проверка: тест
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashComplianceIntentTest*"`
4. **Тесты: `salePrevalidatesAllPayloadsBeforeCardPurchase` (имя 73 симв. → purchaseCalls==0), `quantityScientificNotationRejected` ("1e3"), `quantity120Accepted`, `RejectsPriceTimesQuantityOver9999999_99`**
   - файлы: `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngineTest.kt`, `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashComplianceIntentTest.kt`
   - проверка: зелёные
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCash*"`

## BUG-026
Убрать выдуманный `fiscal_reference="DATECS-<op>"`. Зависит от TG-006 (парсинг ответа cmd 56/74).

1. **`DatecsResponse.parseReceiptClose(fields): FiscalDocRef(docNumber, uuidOrSignature?)` — чистый парсер, формат полей по PM cmd 56 (VG-02)**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt`
   - проверка: тест
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*DatecsProtocolTest*"`
2. **`sale()`: `fiscal_reference` = номер документа из ответа cmd 56; при отсутствии — `RECOVERY_REQUIRED`, не строка-заглушка**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt`
   - проверка: rg пусто
   - приёмка: `! rg -n "\"DATECS-" SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/`
3. **Тест `saleResultCarriesRealDocumentNumber` + `emptyCloseResponseIsNotSuccess`**
   - файлы: `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngineTest.kt`
   - проверка: зелёные
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashEngineTest*"`

## BUG-029
Протокол: дедлайн, SYN, NAK, SEQ. Файл `DatecsProtocol.kt:177-197`. Вспомогательные требования VG-03.

1. **Таблица дедлайнов `DatecsTimeouts.forCommand(cmd:Int):Duration` (X/Z 120 с; чек 30 с; прочее 5 с) + тест значений**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt`
   - проверка: тест
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*DatecsProtocolTest*"`
2. **Чтение кадра через `FrameReader.read(deadline)`: чтение в отдельном executor, по таймауту закрыть сокет и бросить `DatecsFailure.Unknown`**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt`
   - проверка: тест silentDeviceTimesOutWithUnknown
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*DatecsProtocolTest*"`
3. **SYN 0x16: сбросить таймер, продолжить; мусор до SOH игнорировать; лимит размера кадра**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt`
   - проверка: тест synBytesDoNotCorruptFrame
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*DatecsProtocolTest*"`
4. **NAK 0x15: повтор того же кадра байт-в-байт (SEQ не меняется), максимум 3, затем Unknown**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt`
   - проверка: тест nakTriggersByteIdenticalRetransmit
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*DatecsProtocolTest*"`
5. **SEQ: 0x20..0x7f, инкремент только после успешного ответа; ответ со старым SEQ игнорируется (`lateOldSequenceIgnored`)**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt`
   - проверка: тест
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*DatecsProtocolTest*"`
6. **Сериализация вызовов `concurrentCallersSerialized` (две нити, порядок кадров не перемешивается)**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt`, `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocolTest.kt`
   - проверка: тест
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*DatecsProtocolTest*"`

## BUG-030
ACK журнала со сверкой hash. `AndroidJournal.kt:157-195` (`acknowledge`), `SignedMemoryJournal.acknowledge`.

1. **Вынести SQL-слой в JVM-тестируемый интерфейс `JournalStore` (если ещё нет); in-memory реализация для тестов**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/AndroidJournal.kt`
   - проверка: компилируется
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*Journal*"`
2. **`acknowledge(throughSequence, hash)`: `stored = SELECT event_hash WHERE journal_seq=?`; `require(stored==hash)` иначе `JournalAckMismatch`, чекпоинт не двигать**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/AndroidJournal.kt`
   - проверка: тесты
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*Journal*"`
3. **Проверка цепочки `prev_hash` до `throughSequence` при первом ACK после рестарта**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/AndroidJournal.kt`
   - проверка: тест chainBreakRejected
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*Journal*"`
4. **Тесты `acknowledgeWithWrongHashRejected`, `acknowledgeDoesNotAdvanceCheckpointOnMismatch`, `purgeBeforeNeverDeletesUnacknowledged`**
   - файлы: `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/AndroidJournalTest.kt`
   - проверка: зелёные
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*Journal*"`

## BUG-032
Жизненный цикл рантайма. `MainActivity.kt:174-300, 325-335`, `AndroidManifest.xml`.

1. **Манифест: разрешения FOREGROUND_SERVICE, FOREGROUND_SERVICE_CONNECTED_DEVICE, RECEIVE_BOOT_COMPLETED, POST_NOTIFICATIONS, WAKE_LOCK; `<service BlueCashRuntimeService foregroundServiceType=connectedDevice>`; `<receiver BootReceiver>`**
   - файлы: `SmartDevices/bluecash-app/src/main/AndroidManifest.xml`
   - проверка: aapt
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:assembleProdRelease && aapt dump permissions bluecash-app/build/outputs/apk/prod/release/*.apk | grep -c -E "FOREGROUND_SERVICE|RECEIVE_BOOT_COMPLETED" | grep -qv "^0$"`
2. **Класс `BlueCashRuntimeService : Service` — `onStartCommand` вызывает `startForeground` с уведомлением, затем `RuntimeHost.start()`; `onDestroy` не останавливает при пересоздании Activity**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashRuntimeService.kt`
   - проверка: тест
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*RuntimeService*"`
3. **Перенести `startManagedRuntime` из `MainActivity` в `RuntimeHost` (чистый класс, без Context-логики UI); Activity только запускает сервис и показывает статус**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/MainActivity.kt`, `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/RuntimeHost.kt`
   - проверка: тест
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*RuntimeService*"`
4. **`BootReceiver`: на BOOT_COMPLETED запускает сервис, если устройство активировано**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BootReceiver.kt`
   - проверка: тест
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BootReceiver*"`
5. **Пароль оператора: поле `inputType=textPassword`, хранение в Keystore-обёртке (`SecretStore`), не в SharedPreferences открытым текстом**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/MainActivity.kt`, `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/SecretStore.kt`
   - проверка: тест secretIsNotStoredPlain
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*SecretStore*"`
6. **Локальный HTTP 8088: bind только на разрешённый интерфейс + документ о риске (TLS — отдельный TG)**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/MainActivity.kt`, `docs/BLUECASH_POS_INTEGRATION.md`
   - проверка: grep
   - приёмка: `grep -q "8088" docs/BLUECASH_POS_INTEGRATION.md`

## BUG-071
P0 (новая): `reverse()` делает возврат по карте ДО фискального сторно (`BlueCashEngine.kt:305-376`). Решение D-12 (DEFAULT): storno-first — сначала фискальный сторно, при успехе — возврат карты; при Unknown/Failed возврат не делается.

1. **Тест-характеристика `reverseCardRefundNeverBeforeFiscalStorno` (порядок вызовов фейков) — красный**
   - файлы: `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngineTest.kt`
   - проверка: красный до правки
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*reverseCardRefundNeverBeforeFiscalStorno*"`
2. **Переставить в `reverse()`: открыть сторно-чек → строки → платёж → закрыть (cmd 56) → только затем `card.refund`**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt`
   - проверка: зелёный
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashEngineTest*"`
3. **Ветки: storno Unknown → `RECOVERY_REQUIRED`, refund не вызывается; storno Rejected → refund не вызывается, результат FAILED; refund упал после успешного storno → `REFUND_PENDING` (повторяемая операция, не FAILED)**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt`
   - проверка: тесты
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashEngineTest*"`
4. **Тесты `reverseStornoUnknownDoesNotRefund`, `reverseRefundFailureAfterStornoIsRetryable`**
   - файлы: `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngineTest.kt`
   - проверка: зелёные
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashEngineTest*"`

## BUG-072
P0 (новая): нет взаимного исключения между MQTT/BLE/HTTP транспортами, `journal.find`+`append` — check-then-act. `BlueCashCommandProcessor.sale/reverse/printerTest`.

1. **Тест `concurrentDuplicateCommandsExecuteOnce`: 8 потоков с одним `operation_id` → fiscal.execute(48) ровно один раз — красный**
   - файлы: `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashCommandProcessorTest.kt`
   - проверка: красный до правки
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*concurrentDuplicateCommandsExecuteOnce*"`
2. **В процессоре: единый `ReentrantLock` на весь `sale/reverse/printerTest` (одна ФУ — один исполнитель) + повторный `journal.find` под локом**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt`
   - проверка: зелёный
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashCommandProcessorTest*"`
3. **Тест `differentOperationsSerialized` — вторая операция ждёт первую (проверка по таймстемпам фейка)**
   - файлы: `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashCommandProcessorTest.kt`
   - проверка: зелёный
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashCommandProcessorTest*"`

## VG-23
Маппинг оплат на слоты ФУ (D-9, профиль BC50_DEFAULT): `BlueCashEngine.kt:397-406 datecsPaymentMode`.

1. **Таблица-эталон `docs/MVP1/fixtures/payment_slots_bc50.json` {CASH:0,CARD:1,CHEQUE:3,VOUCHER:4,COUPON:5; NHIF/DEFERRED/INTERNAL_CONSUMPTION → PAYMENT_TYPE_UNSUPPORTED}**
   - файлы: `docs/MVP1/fixtures/payment_slots_bc50.json`
   - проверка: jq
   - приёмка: `jq -e ".CASH==0" docs/MVP1/fixtures/payment_slots_bc50.json`
2. **`datecsPaymentMode` читает таблицу-константу (CHEQUE→3, не 2); неизвестный тип → исключение `PAYMENT_TYPE_UNSUPPORTED` на этапе предвалидации (BUG-022)**
   - файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt`
   - проверка: тест
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*PaymentMode*"`
3. **Тест `PaymentModeMappingTest` параметризован по JSON-фикстуре**
   - файлы: `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/PaymentModeMappingTest.kt`
   - проверка: зелёный
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*PaymentMode*"`
