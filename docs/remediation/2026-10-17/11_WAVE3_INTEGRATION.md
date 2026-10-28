# Wave 3: интеграция и незакрытые условия приёмки

## Результат объединённой проверки

После интеграции GAP020/GAP021 и исправления устаревшего legacy recovery fixture полный `make iot-test` завершился с exit 0. Сборка S3 exit 0: RAM 39860/327680 (12.2%), Flash 1638864/3145728 (52.1%). [Логи и source hashes](evidence/wave3/README.md).

Теперь recovery разрешает доказанные terminal tender состояния и проверяет каждую неопределённую оплату отдельно; физические query/reversal capability остаются выключенными до подтверждённого протокола. Реальная фискальная ссылка читается из Datecs/Daisy, legacy recovery стал read-only. Сторно после рестарта без persisted watermark остаётся UNKNOWN. Пункты 1–3 предыдущего списка частично реализованы, но сохраняют указанные ограничения и offline missing-sale gap.

Go `edge-agent/` полностью удалён по запросу пользователя вместе с executable/generator dependencies. Контрактные проверки API после удаления прошли; отсутствующие S3 offline/security/soak доказательства отмечены OPEN.

Новый authoritative offline contract готовится в отдельных пакетах: exact UNP allocation, issuer-bound authorization+client proof, immutable accepted evidence v2 и backend create/merge. До их объединения нельзя заявлять полное offline выполнение.

Дата: 2026-10-17. Базовый коммит: `2fd8c4d`. Это промежуточное состояние, не допуск к эксплуатации или подтверждение НАП.

## Интегрировано

- Реальная совместимость Go backend ↔ ESP-IDF: canonical JSON/HMAC, типизированные event/batch/ACK, сохранение int64 без округления, блокировка несовместимого ранее замороженного batch без удаления данных. В общий `make iot-test` включены оба host runner, использующие фактический Go oracle.
- CLOUD lookup MiniPOS восстанавливает потерянный результат через GET operation/sale с проверками принадлежности и атомарной локальной проекцией. Браузерные тесты используют настоящий IndexedDB и HTTP interception; повторная отправка при неопределённости запрещена.
- Денежный preflight: фиксированная арифметика, EUR, скидки, точное равенство оплат сумме, явные настройки оператора и payment slots. Перед оплатой/закрытием фискального чека читается и проверяется subtotal устройства.
- Платёжная saga: состав CARD платежей замораживается в SQLite; intent сохраняется до внешнего вызова, outcome после него; CAS проверяет владельца, сумму и состояние. Неопределённый второй платёж не запускает автоматическую компенсацию первого. Reversal требует проверенного профиля и исходного RRN; новый tender UUID не используется как банковская ссылка.
- Проверка HIL/legal evidence требует независимых trust registry и expected plan, проверяет Ed25519 signer, точный candidate, артефакты, даты и роли. Локальные тестовые подписи не являются внешним одобрением.

## Подтверждённые результаты до финального объединённого прогона

| Проверка | Результат | Граница доказательства |
|---|---|---|
| Canonical Go corpus | 2055 случаев PASS | Сериализация; не HIL |
| Canonical integrated `make iot-test` | exit 0 | Снимок до money/saga |
| ESP-IDF S3 canonical build | exit 0, RAM 12.1%, Flash 51.2% | Снимок до money/saga; новая сборка обязательна |
| MiniPOS backend tests / web tests / web build | exit 0 | CLOUD follow-up |
| Chromium IndexedDB | 12/12 PASS | Реальный браузер; устройства не подключались |
| Money host runner | exit 0 | Vendor payload vectors, фактические production builders и SQLite; transport boundary заменён |
| Saga SQLite fault runner | exit 0 | 27 режимов сбоев/CAS; не полное восстановление платежей |
| External evidence verifier tests | 48 tests, 89 assertions, 0 failures/errors/skips | Проверяет верификатор; не создаёт внешнего согласования |

Текущий объединённый IoT прогон: `/private/tmp/fiscal-money-saga-integrated-iot-20261017.log`. Независимый saga прогон: `/private/tmp/fiscal-saga-integrated-20261017.log`. Временные логи не заменяют привязанный к итоговому source snapshot evidence bundle.

## Обязательные следующие доработки

1. GAP019/020: реализовать per-tender recovery. Сейчас наличие saga marker даёт `PAYMENT_SAGA_RECONCILIATION_REQUIRED`, включая уже доказанную `COMPENSATED`; это предотвращает дубль, но не завершает восстановление. Проверить рестарт после каждого checkpoint, восстановление по исходным банковским ссылкам и фискальному доказательству, проекцию завершённой компенсации без повторных внешних вызовов.
2. GAP021: заменить синтетические EDGE/RECOVERED ссылки реальным документом устройства с проверкой номера, UNP, времени и идентичности; Datecs/Daisy close responses имеют разную семантику. При недостаточном доказательстве сохранять UNKNOWN.
3. GAP024: проверить и включить end-to-end REVERSAL contract и отображение причин для обоих vendors. Отдельно закрыть materialization offline SALE_FINALIZE при отсутствии sale в backend; успешный reversal не доказывает этот путь.
4. GAP015/004: подключить аутентифицированный provisioning execution config и секрета. Локальный NVS API уже есть, но UI/factory caller и deployment evidence защиты хранения отсутствуют.
5. GAP003–008/013–014/026–027: завершить BLE onboarding/security/fragmentation, клиентский lookup и lifecycle, BluePad transport и доказанный банковский query/reversal contract.
6. GAP022: отдельно проверить Kotlin canonical wire. C++/Go oracle не доказывает совместимость Android JSONObject.
7. GAP034/НАП: production release contract остаётся открытым. Текущий release verifier признаёт только PROD_NO_GO; activation сознательно не имеет PASS пути. Требуются согласованный production policy, реальные HIL evidence и независимое юридическое подтверждение применимости актуальной нормативной базы.

## Предварительная приёмка этой волны

- Независимое ревью saga и reversal без неустранённых P0/P1.
- Финальный `make iot-test`, full backend/web checks и S3 build на одном неизменном source snapshot; отдельные старые зелёные логи не агрегировать в заявление о новой сборке.
- Все новые runner включены в обязательный gate, ошибки dependencies не превращаются в skip/PASS.
- Сохранены исходные банковские evidence и журнал после рестарта, отсутствуют повторные purchase/reversal при UNKNOWN.
- Обновлены source/build hashes и ограничения в evidence bundle. HIL/production/НАП остаются отдельными условиями приёмки.
