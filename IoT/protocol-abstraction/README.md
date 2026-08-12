# BeeFiscal protocol abstraction

Единый C++/Arduino-фасад над драйверами из `IoT/common-modules`. Он скрывает
vendor API, но не смешивает фискальное устройство и платёжный терминал: для них
создаются разные объекты с разными контрактами.

## Публичные контракты

- `IFiscalDevice` — чек, позиция, оплата, закрытие/отмена, X/Z, служебное
  внесение/выведение.
- `IPaymentTerminal` — ping, purchase, void, transaction end, end-of-day и
  обработка асинхронных событий.
- `ProtocolFactory::createFiscal(ConnectionSpec)` создаёт только фискальный
  адаптер.
- `ProtocolFactory::createPayment(ConnectionSpec)` создаёт только платёжный
  адаптер.

Публичный header: `include/ProtocolFacade.h`. Vendor headers подключаются только
в отдельных translation units. Это существенно: существующие библиотеки имеют
глобальные имена (`DatecsError`, `CMD_*`), которые нельзя безопасно включить в
один пользовательский translation unit.

## Матрица выбора

| Фабрика | Vendor | Допустимые каналы |
|---|---|---|
| Fiscal | `Daisy` | RS-232, UART TTL, USB Serial, Embedded |
| Fiscal | `Datecs` | RS-232, UART TTL, USB Serial, Embedded |
| Fiscal | `Tremol` | RS-232, UART TTL, USB Serial, Embedded |
| Payment | `DatecsPay` | BLE GATT, Embedded |

`Stream` передаётся уже настроенным. Поэтому слой транспорта отвечает за UART,
USB CDC либо BLE GATT (`DatecsPayBleStream`), а фасад — за выбор совместимого
протокола и делегирование команд. Неподдерживаемая пара vendor/channel
отклоняется до обращения к оборудованию.

## Пример

```cpp
#include "ProtocolFacade.h"
using namespace beefiscal;

ConnectionSpec fiscalSpec{
    DeviceVendor::Datecs,
    TransportChannel::Rs232,
    &fiscalSerial,
    500,
    3,
    1 // явно запрограммированный в ФУ код оплаты картой
};
auto fiscal = ProtocolFactory::createFiscal(fiscalSpec);
if (!fiscal) {
    // fiscal.error
}

ConnectionSpec pinpadSpec{
    DeviceVendor::DatecsPay,
    TransportChannel::BleGatt,
    &datecsPayBleStream
};
auto pinpad = ProtocolFactory::createPayment(pinpadSpec);
```

Фискальный и платёжный экземпляры имеют независимые lifecycle и transport
sessions. Для комбинированного сценария оркестратор сначала выполняет
`pinpad->purchase(amountMinor)`, после подтверждённого результата регистрирует
на ФУ `addPayment({PaymentMethod::Card, amount})`, затем закрывает чек.

## Инварианты

- Денежная сумма фискального API передаётся decimal-строкой; терминалу — в minor
  units (`uint32_t`). Конвертацию выполняет вызывающий слой без `float`.
- Tax group канонически задаётся как `1..8`, адаптер переводит её в формат
  вендора.
- Коды безналичных оплат программируются на конкретном ФУ. Для `Card`/`Other`
  требуется `ConnectionSpec.paymentCode`; фасад никогда не угадывает код.
  Для Datecs это `0..6`, для Daisy/Tremol — native ASCII code.
- Невалидные данные и неподдерживаемые операции отклоняются до записи в
  transport.
- `TremolPrinter` не предоставляет invoice-open helper, поэтому invoice через
  этот адаптер возвращает `UnsupportedOperation`, а не печатает обычный чек.
- `timeoutMs/retries` относятся к фискальным serial-драйверам. DatecsPay сейчас
  использует таймауты своей библиотеки.

## Сборка и тесты

```bash
IoT/protocol-abstraction/run-tests.sh
make iot-test
```

Host-тест компилирует фасад совместно с Daisy, Datecs, Tremol и DatecsPay,
проверяет матрицу factory routing, раздельность интерфейсов и fail-closed
валидацию. При интеграции с firmware необходимо добавить `include`, все файлы
`src` и соответствующие vendor modules в PlatformIO/Arduino build.

В ходе интеграции также исправлены два дефекта Tremol transport: variadic helper
больше не использует тип, подверженный default promotion в `va_start`, а длина
256-байтного RX-буфера хранится в `uint16_t`, поэтому overflow guard достижим.
