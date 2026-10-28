# MiniPOS: аудит реализации и план испытаний с реальными устройствами

## Предварительный этап: программные разрывы

Этот hardware plan следует выполнять после gates G0–G3 из
[глубокого аудита и плана доработок](remediation/2026-10-17/README.md).
Аудит выявил дополнительные блокеры, включая actual runtime codecs, MQTT topic
mismatch и durability/recovery. Текущий статус — **SOFTWARE_GAPS_OPEN**.
Диагностический HIL допустим для уточнения vendor protocol; основную приёмку
не объявлять выполненной до закрытия перечисленных программных разрывов.


Дата: 2026-10-17
Базовый commit: `cfff038`
Объект: текущая рабочая копия Fiscalisation, включая незакоммиченные изменения.

## Вывод

Полностью реализованным контур считать нельзя. Значительная часть логики, драйверов и тестов существует, но обнаружены программные разрывы, мешающие аппаратным сценариям. Заявление `SOFTWARE_COMPLETE_HIL_PENDING` требует пересмотра до устранения выявленных проблем.

Изучены `Fiscalisation/docs`, связанные материалы `BeeloyBackend/docs/Fiscal`, backend, MiniPOS, Android BlueCash и ESP-IDF прошивка S3. В ходе аудита код не изменялся, устройства не прошивались. APK, iOS-приложение и прошивка не собирались; физическая связь и печать не проверялись.

Названия Datecs, Daisy и BlueCash приняты как подразумеваемые в запросе. Для отдельного BLE-терминала код предусматривает BluePad-50 Plus / DatecsPay. Если «Blue Pay» означает другую модель, совместимость следует проверить отдельно.

## 1. Архитектура и целевые профили

| Компонент | Назначение |
|---|---|
| `minipos/BeeMiniPOS` | Нативный MiniPOS Android/iOS, продажи, выбор маршрута, BLE |
| `minipos/miniposweb` | Отдельный веб-клиент |
| `minipos/beeminipos-backend` | Пользователи, товары, смены, заказы, интеграция с Fiscal |
| `fiscal-backend` | Фискальные операции, binding, MQTT, sync, webhook |
| `BeeFiscalApp` | Администрирование и настройка устройств |
| `IoT/firmware/edge-agent-s3/idf` | S3: UART, USB CDC, BLE, MQTT, журнал на SD |
| `SmartDevices/bluecash-app` | Android на BlueCash: fiscal/pinpad API, журнал, MQTT/BLE |
| `edge-agent` | Go-модуль удалён по запросу пользователя; строка сохранена только как историческое пояснение аудита |

Для аппаратного стенда использовать ESP-IDF-проект `IoT/firmware/edge-agent-s3/idf`. Соседний Arduino-проект описан как skeleton и не эквивалентен полной прошивке.

| Профиль | Фискальное подключение | Оплата |
|---|---|---|
| S3 + Datecs DP-150 | UART через соответствующий RS-232 преобразователь | BluePad по BLE |
| S3 + Daisy Compact S 01 | USB Host / CDC ACM | CARD в текущем профиле должен отклоняться |
| BlueCash-50 | Встроенный Android fiscal service | Встроенный pinpad service |

Подключение внешнего BluePad по BLE к BlueCash-приложению в изученном коде не найдено: выбран `PinpadManager` встроенного устройства.

Cloud path: MiniPOS → REST Fiscal API → MQTT → устройство → journal/sync → Fiscal → webhook/status → MiniPOS.

Local path: MiniPOS → BLE → устройство → fiscal/payment driver → journal → последующая синхронизация с Fiscal.

## 2. Обнаруженные программные разрывы

### 2.1. Незавершённая BLE-настройка S3

`BeeFiscalApp/src/smartDeviceBle.native.ts:71`: `authenticate()` всегда заканчивается `EDGE_BLE_CRYPTO_PROVIDER_NOT_AVAILABLE`. `setWifi()` и `bindStore()` не выполняют настройку. Приложение ищет service `7b6f2000…`, S3 публикует `7b6f0000…`. Протоколы настройки не согласованы.

### 2.2. Wi-Fi и первичная загрузка credentials

В ESP-IDF runtime не найдены вызовы запуска Wi-Fi, получения IP и синхронизации времени. MQTT-секреты читаются из `edge-secrets` NVS, но штатный путь их первичной записи в изученном runtime отсутствует. Подписанный binding содержит ссылки на секреты. После установки binding устройство может остановиться на `load_secrets()`.

Источники: `IoT/firmware/edge-agent-s3/idf/main/app_main.cpp:129`, `runtime_config.cpp`, `BINDING_PROVISIONING.md`.

### 2.3. Несогласованные MQTT ACK topics

Backend формирует `…/sync/acks`, прошивка подписывается на точную тему и сравнивает её целиком. Backend публикует ACK в `…/sync/acks/{batch_id}`. Требуется согласовать subscription и проверку темы. Иначе business ACK и продвижение checkpoint не замкнуты.

Источники: `fiscal-backend/internal/domain/admin.go:426`, `fiscal-backend/internal/mqttclient/client.go:334`, `IoT/firmware/edge-agent-s3/idf/main/mqtt_runtime.cpp:14`.

### 2.4. Readiness не подтверждает доступность ФУ

Datecs readiness означает инициализацию UART, Daisy readiness — открытие USB CDC. Отключённая касса может считаться готовой до первой команды. Карточная операция выполняется перед фискализацией. Нужен безопасный запрос статуса конечного ФУ перед оплатой.

Источники: `IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp:21`, `uart_runtime.cpp`.

### 2.5. Скидки и идентичность чека

S3 читает `discount` из intent, но `finish_receipt()` не задаёт `discountType/discountValue` в товарной строке. Возможны расхождение суммы и неполное закрытие чека. Успешный результат возвращает вычисленный `EDGE-…`, а не считанный номер реального чека.

Источник: `IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp:17`.

### 2.6. Несколько CARD tender

Если первая CARD-оплата одобрена, а следующая отклонена, `ProfileExecutor` возвращает отказ без компенсации предыдущей. Исправить сценарий либо запретить несколько CARD tender в одном intent.

Источник: `IoT/firmware/edge-agent-s3/idf/main/profile_orchestrator.cpp:25`.

### 2.7. BLE advertising и MTU

По статическому анализу имя `BeeFiscal-setup`, UUID и flags превышают вместимость legacy advertising. Ошибка установки advertising просто завершает функцию. Результат передаётся фиксированными фрагментами без учёта negotiated MTU. Требуются исправление и аппаратная проверка, особенно на iOS.

Источник: `IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp:91`.

### 2.8. BlueCash release pipeline

Vendor JAR включены и реальные fiscal/pinpad порты выбраны, но `assemble…Release`/`bundle…Release` намеренно завершаются `STORE_BLOCKED`. Makefile ожидает release APK по старым путям без flavors. Нужен согласованный debug/demo pipeline для лаборатории.

Источник: `SmartDevices/bluecash-app/build.gradle.kts:56`, `Makefile`.

### 2.9. Расхождения документации

README SmartDevices описывает старую активацию. MiniPOS README обещает зашифрованный BLE, фактический native client принимает `OPEN_MVP`. `docs/MVP1/README.md` заявляет отсутствие программных P0. Обновить документы по фактическому состоянию и разделить software verification, physical HIL и production approval.

## 3. Выполненные проверки

| Проверка | Результат |
|---|---|
| Python ESP-IDF unit tests | 33 PASS |
| Native C++ saga/profile tests | PASS |
| MiniPOS TypeScript tests | 47 PASS |
| Go edge-agent, все пакеты — дополнительная историческая проверка вне текущего контура | PASS, не evidence готовности S3 |
| Fiscal backend domain и mqttclient | PASS |
| BlueCash release composition check | PASS |
| Go MiniPOS backend | FAIL: три API-теста |

Команды:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s IoT/firmware/edge-agent-s3/idf/tests -p 'test_*.py'
IoT/firmware/edge-agent-s3/idf/tests/run_native_saga.sh
ruby scripts/verify_bluecash_release_composition.rb
(cd minipos/BeeMiniPOS && npm test)
(cd fiscal-backend && GOCACHE=/tmp/beefiscal-audit-go-cache go test ./internal/domain ./internal/mqttclient)
(cd minipos/beeminipos-backend && GOCACHE=/tmp/beefiscal-audit-go-cache go test ./...)
```

Падающие тесты:

- `TestEditorsShiftOrder`: 403, operator identity does not match employee.
- `TestShiftRecoveryFiltersByEmployeeRegisterAndState`: 403 с той же причиной.
- `TestPublicContractAliasAndOptimisticUpdate`: version must be mandatory.

Повторный запуск с разрешёнными localhost-серверами подтвердил эти три ошибки. Возможны устаревшие fixtures; до разбора нельзя объявлять всю регрессию зелёной. Существующие PASS не покрывают все обнаруженные разрывы и не заменяют physical HIL.

## 4. Этапы аппаратного тестирования

### Этап 0. Подготовка программного контура

1. Закрыть provisioning/Wi-Fi/credentials/MQTT ACK/readiness.
2. Исправить передачу скидок и получение реальной идентичности чека.
3. Исправить multi-CARD compensation либо ограничить профиль.
4. Согласовать BLE advertising, MTU и протокол настройки.
5. Исправить/актуализировать три падающих API-теста и build gate.
6. Зафиксировать commit, конфигурации и hashes сборок.

### Этап 1. Зафиксировать стенд

Записать:

- точную модель и ревизию S3, схему GPIO, питание и SD-карту;
- Datecs/Daisy model, serial, firmware, FMIN;
- UART baud/parity/stop bits и RS-232 преобразователь;
- USB VID/PID/interface, USB Host role и питание;
- BluePad model/firmware и тестовый merchant/acquirer profile;
- BlueCash model, Android, vendor services и firmware;
- модели и версии ОС Android-телефона, Android-планшета, iPhone и iPad.

COM Datecs подключать через согласованный электрический интерфейс RS-232. Daisy — через USB Host с проверенным питанием. Подготовить управляемое отключение питания, сетевые сбои, расходные материалы и сбор трасс.

### Этап 2. Проверить устройства отдельно от MiniPOS

1. Прочитать identity/status, время, бумагу и состояние открытого документа.
2. Выполнить тестовую печать.
3. Выполнить CASH-продажу и считать реальный номер чека.
4. Проверить налоговые группы, суммы, X/Z, cash-in/out и сторно.
5. Для терминала проверить approve, decline, cancel, timeout, lookup и reversal.
6. Зафиксировать применимость команд для каждой model/firmware.

### Этап 3. Доказать настройку с чистого S3

Стереть тестовую конфигурацию → найти S3 по BLE → идентифицировать → передать Wi-Fi и credentials → установить подписанный binding → перезапустить → получить MQTT connection и актуальный статус конечного ФУ.

Негативные сценарии: неверный пароль Wi-Fi, обрыв передачи, повтор установки, старая generation, недействительная подпись, отсутствующие credentials, недоступная SD. Процесс должен завершаться понятной ошибкой и безопасно возобновляться.

### Этап 4. Сборка и установка BlueCash demo APK

1. Согласовать flavor-specific debug-сборку для лаборатории.
2. Собрать APK, записать SHA-256 и установить через ADB на конкретное устройство.
3. Проверить доступность vendor fiscal/pinpad services.
4. Проверить Keystore/attestation и QR activation.
5. Проверить ACTIVE ACK, запуск MQTT/GATT и сохранение activation после restart.
6. Проверить отзыв binding/credentials и прекращение операций.

Release/production gates не обходить для заявления о готовности. Внешний BluePad на BlueCash считать отдельным неподтверждённым сценарием до реализации соответствующего транспорта.

### Этап 5. Матрица Android/iOS × MQTT/BLE

| Исполнитель | Android MiniPOS | iOS MiniPOS |
|---|---|---|
| S3 + Datecs | Cloud→MQTT и direct BLE | Cloud→MQTT и direct BLE |
| S3 + Daisy | Cloud→MQTT и direct BLE | Cloud→MQTT и direct BLE |
| BlueCash | Cloud→MQTT и direct BLE | Cloud→MQTT и direct BLE |

В cloud-сценарии MiniPOS обращается к REST API. MQTT связывает Fiscal backend и устройство.

Для каждой ячейки выполнить:

- CASH;
- CARD при поддержке выбранного профиля;
- CASH+CARD и разрешённые split tender;
- скидки, дробное количество, округления, налоговые группы;
- возврат/сторно со ссылкой на исходный чек и оплату;
- cash-in/out, X/Z, открытие и закрытие смены;
- повтор запроса с тем же UUID/digest;
- неподдерживаемый CARD с явным отказом без подмены на CASH.

### Этап 6. Сбои и восстановление

| Сбой | Ожидаемый результат |
|---|---|
| Потерян результат после печати | UNKNOWN → lookup, без повторной печати |
| Потерян ответ после списания | Lookup/reconcile, без повторного списания |
| Один UUID одновременно MQTT и BLE | Одно аппаратное исполнение |
| Тот же UUID с другим payload | Conflict до аппаратного действия |
| MQTT PUBACK есть, business ACK потерян | Журнал сохранён, sync безопасно повторяется |
| Отключён COM/USB или питание ФУ | Продажа блокируется |
| SD заполнена, извлечена или повреждена | Новые операции блокируются до side effect |
| Restart после оплаты, до закрытия чека | Recovery либо явная reconciliation |
| Потерян webhook | Результат восстанавливается через status pull |
| Broker/network reconnect | Нет потери журнала и повторных side effects |

Отключать питание в точках до/после durable reservation, оплаты, открытия/закрытия чека, записи результата, публикации sync и получения business ACK.

Для BLE дополнительно проверить отказ разрешений, выключение Bluetooth, background/foreground, блокировку экрана, reconnect, различные negotiated MTU и одновременную работу S3 как peripheral для MiniPOS и central для BluePad.

### Этап 7. Циклические испытания

Предлагаемые критерии стенда, а не уже полученные результаты:

- 10 smoke-операций на каждую поддерживаемую комбинацию;
- 100 повторений ключевых сценариев сбоев;
- 72 часа смешанного MQTT/BLE прогона;
- отдельные тесты SD power loss, заполнения, retention и ACK-gated удаления.

Банковские циклы проводить в тестовом профиле эквайера. Неподдерживаемые или недоступные аппаратные сценарии отмечать BLOCKED, не PASS.

## 5. Критерии приёмки и evidence

Для каждой операции должны сходиться MiniPOS order, Fiscal operation, локальный журнал, реальный фискальный чек и банковская транзакция.

Обязательные инварианты:

1. Нет повторных списаний и повторных чеков при retry, reconnect и смене маршрута.
2. Нет потерянных принятых операций.
3. UNKNOWN разрешается доказательствами либо остаётся видимой блокирующей задачей сверки.
4. Готовность адаптера не подменяет доступность конечного ФУ.
5. Журнал не удаляется до подтверждённого business ACK и срока хранения.
6. Неподдерживаемая операция не превращается в симулированный успех.

Сохранять для каждого прогона:

- commit, состояние рабочей копии, hashes APK/firmware и версии контрактов;
- model/serial/firmware устройств и параметры подключения;
- operation_id и payload digest;
- обезличенные MQTT/BLE/UART/USB request/response traces;
- фото/сканы чеков и считанные fiscal references;
- payment references, RRN/auth code в разрешённом обезличенном виде;
- journal/checkpoint до и после восстановления;
- итог PASS/FAIL/BLOCKED и известные отклонения.

Не сохранять токены, приватные ключи, PAN/PIN и другие платёжные секреты в тестовых артефактах.

## 6. Очерёдность работ

1. Provisioning, Wi-Fi, credentials, MQTT ACK, реальный readiness.
2. Корректность чека, скидок, идентификаторов и compensation.
3. Воспроизводимая BlueCash demo-сборка и установка на устройство.
4. Аппаратная матрица Android/iOS × MQTT/BLE.
5. Fault injection, циклы, soak и evidence.
6. Отдельная production-приёмка выбранных hardware profiles по vendor/acquirer/security/release gates.

Текущее состояние подходит для целенаправленной доработки и стендовых испытаний. Полная реализация запрошенных сценариев и готовность к production не подтверждены.
