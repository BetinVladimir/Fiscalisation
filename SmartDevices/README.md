# SmartDevices: Daisy SMART и Datecs BlueCash

## Актуальный статус — 2026-10-17

**SOFTWARE_GAPS_OPEN.** В `bluecash-app/libs` уже поставлены fiscal/pinpad JAR и
Gradle подключает их; наличие bridge не доказывает protocol correctness/HIL.
[Аудит и задания](../docs/remediation/2026-10-17/README.md) фиксируют MQTT topic
mismatch, ошибки Datecs LEN/stream parsing и неактуальный build gate.
`demo`/`prod` flavors имеют application IDs `org.beeloy.bluecash.demo` и
`org.beeloy.bluecash.prod`; `com.beeloy.fiscal.bluecash` — namespace.
Release tasks сейчас намеренно завершаются `STORE_BLOCKED`.
Для будущего debug-HIL использовать точные flavored tasks после проверки
`gradlew tasks`. `make smart-device-test` выполняет prod/demo debug unit tests и создаёт четыре debug APK; `make smart-device-host-test` отдельно проверяет actual Kotlin Datecs codec на vendor vectors. Нужны JDK17, Android SDK36 и cached Kotlin2.0.21 dependencies для host runner. Release gate по-прежнему блокирует выпуск до HIL; debug build не подтверждает работоспособность Daisy stub.
Daisy SMART остаётся STUB и отличается от Daisy USB через S3.


Здесь находятся два независимых Android application-модуля с разными package/application ID. Общим остаётся только transport-neutral интерфейс `:shared`.

| Модуль | Application ID | Назначение | Статус |
|---|---|---|---|
| `daisy-smart-app` | `com.beeloy.fiscal.daisy.smart` | Daisy SMART S | debug-only STUB; PROD запрещён до документации вендора |
| `bluecash-app` | `org.beeloy.bluecash.demo` / `org.beeloy.bluecash.prod` | Datecs BlueCash-50 | vendor JAR подключены; программные gaps открыты, HIL pending |

## Изученные материалы Datecs

Исходный комплект: `/Users/freelancer/Documents/Beeloy/BeeloyBackend/docs/Fiscal/Android/DatecsBC`.

- `Fiscal/PACKAGE/FiscalDeviceSDK_MultiPlatform.1.0.1.nupkg` и `Com.Android.Fiscal.dll` — .NET/MAUI binding для фискальных функций;
- `PinpadSDK/PACKAGE/DatecsPaySDK_MultiPlatform_Net8.1.0.4.nupkg` и `Com.Android.Pinpad.dll` — .NET 8/MAUI binding для карточных операций;
- demo-проекты показывают Bluetooth discovery/connect и особый случай встроенного `bluecash-50` hardware.

Исходный комплект содержит .NET bindings; текущий Android Gradle также подключает `libs/com.android.fiscal.jar` и `libs/com.android.pinpad.jar`. Реальная wire-интеграция нуждается в исправлениях и аппаратной проверке; прежнее описание только `MissingVendor*Adapter` устарело.

## BlueCash activation

1. На BlueCash оператор вводит organization ID, location ID, device ID и локальный activation PIN (не менее 8 знаков). Только после этого приложение публикует BLE service.
2. Администратор в BeeFiscalApp выбирает BlueCash и location, подключается по BLE.
3. BeeFiscalApp вызывает `POST /public/v1/devices/{device_id}/activation-tokens` через Caddy с OIDC bearer, `X-Api-Version` и `Idempotency-Key`.
4. Backend получает организацию только из аутентифицированного tenant, проверяет принадлежность device/location и выдаёт HS256 JWT на 5 минут. Claims: `organization_id`, `location_id`, `device_id`, `app_instance_id`, `aud=beefiscal-bluecash-activation`, `scope=smart-device.activate`, `iat/nbf/exp/jti`.
5. JWT передаётся фрагментами `BFA1|transfer-id|index|total|fragment` (до 120 ASCII-символов на frame), собирается приложением и сверяется с локально введёнными organization/location/device и сроком действия.
6. Секрет HMAC никогда не передаётся приложению. Локальный decode claims не является проверкой подписи: окончательное доверие возникает только при предъявлении JWT backend, который проверяет HS256. Токен хранится только в памяти и очищается при logout/process death.

BLE UUID:

- service `7b6f1000-7c6d-4c7a-9e4f-424545464953`;
- token write `7b6f1001-7c6d-4c7a-9e4f-424545464953`;
- status read `7b6f1002-7c6d-4c7a-9e4f-424545464953`.

Полный публичный API и инструкция для POS: [`../docs/BLUECASH_POS_INTEGRATION.md`](../docs/BLUECASH_POS_INTEGRATION.md). Машиночитаемый контракт: [`../contracts/openapi-runtime-v1.yaml`](../contracts/openapi-runtime-v1.yaml).

## Сборка и проверки

```sh
make smart-device-test
```

Текущий target требует исправления под flavors и release guard (GAP-029); не считать его подтверждённой успешной сборкой. Для BlueCash обязательны также программные доработки из аудита и production evidence: vendor SDK/license, acquirer credentials, сертифицированное EUR firmware, физический BlueCash-50 и HIL fiscal/card scenarios.
