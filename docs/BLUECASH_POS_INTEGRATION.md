# Интеграция и активация Datecs BlueCash-50

Версия: `2026-08-11`. Нормативный API-контракт: [`../contracts/openapi-runtime-v1.yaml`](../contracts/openapi-runtime-v1.yaml). Общий протокол внешнего POS: [`EXTERNAL_POS_INTEGRATION_PROTOCOL.md`](EXTERNAL_POS_INTEGRATION_PROTOCOL.md).

## Граница ответственности

BlueCash app работает на самом регистраторе и инкапсулирует Datecs fiscal API и BORICA pinpad API. BeeFiscalApp работает на телефоне сотрудника или в браузере. Access token сотрудника никогда не передаётся регистратору. Организация берётся исключительно из проверенного tenant claim BeeFiscalApp; на устройстве и в форме подтверждения organization ID отсутствует.

Legacy `POST /devices/{id}/activation-tokens` и доставка bearer JWT по BLE не являются production activation path. Новый путь основан на неэкспортируемом P-256 ключе, HTTPS bootstrap, mTLS и CA-подписанном MQTT commit.

## Последовательность

1. Подписанная DEV/PROD сборка BlueCash получает HTTPS URL fiscal-backend через `BuildConfig.FISCAL_BACKEND_URL`. Пользователь не может заменить endpoint в UI.
2. Приложение создаёт стабильный `device_instance_id`, запрашивает `POST /device-bootstrap/v1/challenges`, затем генерирует hardware-backed P-256 Android Keystore key с attestation challenge. Software-only key блокируется.
3. Приложение подписывает канонический activation proof и вызывает `POST /device-bootstrap/v1/activation-requests`. `request_secret` хранится AES-GCM ключом Android Keystore; QR содержит только verification URI, request ID и одноразовый human code.
4. ADMIN в BeeFiscalApp сканирует QR/вводит code. UI вызывает `GET /device-activation-requests:lookup`, показывает vendor/model/serial/FMIN/key thumbprint, затем сотрудник выбирает location, register и роли. Компания не выбирается.
5. `POST /device-activation-requests/{id}:confirm` связывает заявку с tenant из access token.
6. Регистратор доказывает владение ключом на `POST /device-bootstrap/v1/activation-requests/{id}/credential`. Ответ содержит key-bound 90-day client certificate, CA chain, MQTT URI, binding, три независимых per-device HMAC verifier key и fenced диапазон УНП. Ответ имеет `Cache-Control: no-store` и сохраняется только зашифрованно.
7. Регистратор подключается к MQTT по mTLS, подписывает commit тем же hardware key и публикует его в `beefiscal/v1/devices/{device_id}/activation`. Backend атомарно создаёт ACTIVE device, fiscal/payment bindings и audit event.
8. Устройство проверяет ACTIVE ack подписью Device CA и все binding fields. Только после этого запускаются command MQTT, async journal sync и transaction GATT.

## Рабочие ключи и темы

Credential содержит отдельные `command_hmac_key`, `sync_ack_hmac_key`, `ble_ticket_hmac_key`, детерминированно выведенные backend master key с привязкой к device и credential. Компрометация одного устройства не позволяет подписывать трафик другого. Private P-256 key не экспортируется.

- activation: `beefiscal/v1/devices/{device_id}/activation` → `.../activation/ack`;
- команды: `tenants/{tenant_id}/devices/{device_id}/commands`;
- journal upload: `tenants/{tenant_id}/devices/{device_id}/sync/batches/{batch_id}`;
- sync ack: `tenants/{tenant_id}/devices/{device_id}/sync/acks/{batch_id}`.

Все сообщения QoS 1 и бизнес-идемпотентны по `operation_id`. SQLite хранит подписанную hash-chain минимум три месяца; удаляются только подтверждённые backend записи. Direct BLE принимает только backend-issued session ticket и `ComplianceIntent`, а не raw Datecs команды. Диапазон УНП резервируется backend атомарно и не раскрывается внешнему POS.

## Отключение

ADMIN вызывает `POST /devices/{device_id}:disconnect` с `Idempotency-Key` и `{}`. Backend атомарно переводит activation/device в `REVOKED`, увеличивает binding version, очищает fiscal/payment binding кассового места, отзывает BLE sessions и пишет audit. Broker ACL/PKI revocation должны запретить повторное mTLS подключение; их deployment-настройка является обязательной частью production PKI runbook.

## Проверки

- Go: domain lifecycle, proof-of-possession, tenant isolation, CA certificate/key binding, MQTT topic binding, asymmetric ACTIVE ack, per-device keys, revoke.
- Android JVM: Datecs fiscal/payment codecs, sale/reversal, CBOR/BLE framing/handshake, sync wire, journal logic; `assembleDebug` подтверждает интеграцию Android Keystore, QR, mTLS и GATT.
- Обязательный HIL до production: реальный BlueCash-50, vendor fiscal service/JAR, BORICA provisioning, EMQX mTLS ACL, certificate revoke и power/network fault injection.
