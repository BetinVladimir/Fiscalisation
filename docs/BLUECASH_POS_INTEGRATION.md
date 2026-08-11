# Интеграция нового POS с Datecs BlueCash activation

Версия: `2026-08-11`. Контракт: [`../contracts/openapi-runtime-v1.yaml`](../contracts/openapi-runtime-v1.yaml), operation `createSmartDeviceActivationToken`.

## Предусловия

POS является публичным API-клиентом и не получает прямой доступ к БД или внутренним драйверам. Нужны ADMIN identity, зарегистрированные в одной организации `location` и BlueCash `device` со статусом `DRAFT` либо `PENDING_SERVICE_ACTIVATION`, а также физическое присутствие у устройства. В PROD вызов идёт только через Fiscal Caddy HTTPS ingress.

## REST

```http
POST /public/v1/devices/11111111-1111-4111-8111-111111111111/activation-tokens HTTP/1.1
Authorization: Bearer <admin-oidc-access-token>
X-Api-Version: 2026-08-07
Idempotency-Key: 8f6f27dc-8aae-4a76-b7dc-88b5a4160937
Content-Type: application/json

{
  "location_id": "22222222-2222-4222-8222-222222222222",
  "app_instance_id": "33333333-3333-4333-8333-333333333333"
}
```

`organization_id` намеренно отсутствует в request: backend берёт его из проверенного tenant. Ответ `201` содержит короткоживущий `activation_token`, явные `organization_id`, `location_id`, `device_id`, `app_instance_id`, `expires_at`. POS обязан проверить совпадение response с запросом до BLE write и не логировать токен.

Ошибки: `404` — device/location отсутствует либо принадлежит другому tenant; `409` — устройство не допускает activation или signing unavailable; `422` — неизвестное/неполное поле. Повтор с тем же idempotency key и тем же body возвращает первоначальный результат.

## BLE

После локального login BlueCash рекламирует service `7b6f1000-7c6d-4c7a-9e4f-424545464953`. POS разбивает ASCII JWT на фрагменты до 120 символов и последовательно пишет с response в characteristic `...1001...`:

```text
BFA1|<transfer-id>|<1-based-index>|<total>|<fragment>
```

`transfer-id` — 8–64 ASCII `[A-Za-z0-9-]`, `total` — 1–32. После последнего frame POS читает `...1002...`; единственный успех — `ACTIVATED`. `RECEIVING_TOKEN`, `LOGIN_REQUIRED`, binding/expiry error и disconnect считаются отказом. Native клиент использует `react-native-ble-plx`; Web-клиент — Chrome Web Bluetooth в secure context.

## Security и recovery

- JWT lifetime — 5 минут, audience и scope узкие; в claims явно присутствуют organization/location/device/app instance.
- BlueCash не получает HMAC key. Он проверяет структуру, expiry и привязку, но подпись проверяет backend при последующем managed connection. До такой проверки token нельзя считать серверной авторизацией.
- При обрыве POS создаёт новый BLE connection и новый activation token; старый token не переиспользуется после expiry.
- Токен нельзя помещать в AsyncStorage, логи, crash reports, telemetry или UI result.
- BLE activation не заменяет fiscal/card HIL и не разрешает продажу при недоступности конечного ФУ.

## Критерии приёмки нового POS

- Клиент сгенерирован/проверен по OpenAPI и передаёт обязательные headers.
- Cross-tenant location/device и client-supplied organization отклоняются.
- BLE недоступен до локального login; logout прекращает advertising и очищает token.
- JWT длиннее одного ATT write успешно передаётся фрагментами; конфликтующий frame отклоняется.
- UI показывает только IDs, expiry и device status, никогда JWT.
- Проверены Android physical BLE и Chrome secure-context Web Bluetooth; iOS требует Bluetooth usage description.
- Production включение запрещено, пока не закрыты vendor/acquirer SDK, firmware и HIL gates.
