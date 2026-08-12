# Активация и привязка SmartDevice к организации и точке продаж

> **DEPRECATED — не источник истины.** Этот ранний проект заменён
> [`BLUECASH_POS_INTEGRATION.md`](BLUECASH_POS_INTEGRATION.md), runtime OpenAPI и
> текущим кодом activation. Сохранён временно только как кандидат на удаление.

Статус: исторический проект алгоритма. Дата анализа: 2026-08-11.

## 1. Назначение и границы

Приложения из `SmartDevices` запускаются непосредственно на фискальном регистраторе. Для общей системы каждое такое приложение является единым smart-device adapter, который снаружи предоставляет стандартные возможности `FISCAL_DEVICE` и, если терминал это поддерживает и активирован эквайером, `PAYMENT_TERMINAL`. Протоколы производителя, соединение со встроенным ФУ и pinpad, повтор команд и разбор ответов остаются внутри приложения и не участвуют в активации.

`BeeFiscalApp` запускается на телефоне уполномоченного сотрудника либо в браузере на компьютере. Приложение уже активировано в контексте одной организации (tenant): `organization_id` находится в проверенном access token/session BeeFiscalApp и не выбирается во время настройки устройства. BeeFiscalApp показывает только торговые точки и кассовые места активного tenant, подтверждает привязку и доставляет credential на регистратор. Сотрудник не вводит `organization_id`, `location_id` или `device_id` на самом регистраторе и не копирует JWT вручную.

Этот документ описывает только первичную активацию, повторную привязку и отзыв. Выполнение продаж, фискальных команд и платежей не рассматривается.

## 2. Выбранный механизм

Рекомендуется QR-assisted, proof-of-possession activation:

1. Регистратор сам создаёт hardware-backed ключ устройства и одноразовую activation session.
2. Он показывает QR и короткий код сверки.
3. BeeFiscalApp сканирует QR либо получает session по короткому коду. Backend берёт организацию из активного tenant BeeFiscalApp, а сотрудник выбирает только торговую точку, кассовое место и роли адаптера.
4. Backend проверяет полномочия и выпускает не обычный переносимый bearer token, а credential, криптографически привязанный к публичному ключу регистратора.
5. BeeFiscalApp доставляет на регистратор одноразовый зашифрованный activation envelope по BLE. Если регистратор уже имеет доверенный HTTPS-доступ, он может забрать тот же envelope непосредственно с backend; это не меняет алгоритм и состояние привязки.
6. Регистратор доказывает владение приватным ключом backend и только после этого становится `ACTIVE`.

QR используется для выбора правильного физического устройства и защиты от подключения к соседней кассе; BLE — только для локальной доставки. Для компьютера без BLE доступны два безопасных пути: сотрудник подтверждает короткий код в браузере, а регистратор забирает envelope через HTTPS, либо BeeFiscalApp в Chrome использует Web Bluetooth. Передача credential через изображение QR с экрана компьютера не рекомендуется: QR может попасть в историю экрана или снимок камеры и имеет неудобный сценарий обновления.

Подход следует модели OAuth Device Authorization: устройство показывает короткий user code/verification URI, а пользователь подтверждает действие на другом аутентифицированном устройстве. Стандарт отдельно рекомендует показывать код на обоих устройствах для защиты от remote phishing. Источник: [RFC 8628 — OAuth 2.0 Device Authorization Grant](https://www.rfc-editor.org/rfc/rfc8628.html). Привязка credential к ключу следует принципу sender-constrained tokens: украденный token бесполезен без приватного ключа. Источник: [RFC 9449 — OAuth 2.0 DPoP](https://www.rfc-editor.org/rfc/rfc9449.html).

## 3. Участники и идентификаторы

- `SmartDevice` — приложение DaisySMART/BlueCash на регистраторе.
- `BeeFiscalApp` — доверенный интерфейс сотрудника, но не хранилище device secret.
- `Fiscal Backend` — единственный источник истины о tenant, location, register и bindings.
- `device_key` — неэкспортируемая асимметричная пара ключей в Android Keystore; предпочтительно StrongBox, иначе TEE с зафиксированным уровнем assurance.
- `device_instance_id` — серверный UUID инсталляции приложения, не серийный номер ФУ.
- `hardware_identity` — нормализованные vendor/model/serial/FMIN и capability digest, прочитанные внутренним vendor adapter.
- `activation_session_id` — случайный одноразовый идентификатор с TTL 10 минут.
- `user_code` — удобный код сверки, например четыре группы по четыре символа; он не является секретом или credential.
- `binding_version` — монотонный fencing counter. Любая повторная привязка или отзыв увеличивает его и делает старое разрешение недействительным.

Android Keystore позволяет хранить приватный ключ вне процесса приложения; hardware-backed attestation может подтвердить, что ключ неэкспортируем и находится в TEE/StrongBox. Challenge должен приходить от backend и проверяться сервером вместе с цепочкой и revocation status. Источник: [Android — Verify hardware-backed key pairs with key attestation](https://developer.android.com/privacy-and-security/security-key-attestation).

## 4. Алгоритм первичной активации

### Шаг 1. Локальная подготовка регистратора

SmartDevice запускает self-check и не открывает BLE activation service автоматически при каждом старте. Пользователь физически нажимает «Начать настройку» и подтверждает локальное системное действие.

Приложение:

1. проверяет, что это production-signed build и разрешённый vendor adapter;
2. считывает `vendor`, `model`, `serial`, `FMIN`, firmware и наличие встроенного payment terminal, не раскрывая чувствительные платежные данные;
3. создаёт в Android Keystore ключ `ES256`/P-256 с назначением sign, `device_key_id` и, где доступно, hardware attestation;
4. создаёт отдельный ephemeral ECDH key для шифрования только этой activation session;
5. получает от backend challenge и регистрирует неподтверждённую session либо, если сети нет, создаёт локальный nonce, который BeeFiscalApp зарегистрирует при сканировании;
6. включает BLE advertising только на 10 минут и только для activation service.

Приватные ключи, PIN, PAN, vendor credentials и будущий device credential не входят в QR или BLE advertisement.

### Шаг 2. Данные QR и визуальная сверка

Регистратор показывает QR с deep link вида `beefiscal://activate?...`. Payload содержит только:

```json
{
  "v": 1,
  "activation_session_id": "uuid",
  "user_code": "ABCD-EFGH-IJKL-MNPQ",
  "device_instance_id": "uuid",
  "vendor": "DATECS",
  "model": "BLUECASH_50",
  "serial_suffix": "1234",
  "device_key_thumbprint": "base64url-sha256",
  "ephemeral_encryption_public_key": "JWK",
  "expires_at": "RFC3339",
  "local_transport": {
    "type": "BLE",
    "service_uuid": "uuid"
  }
}
```

Payload подписывается `device_key`; при первом вводе эта подпись доказывает согласованность QR, но не доверенность неизвестного устройства. Backend устанавливает доверие только после policy/attestation/vendor checks. Полный serial/FMIN BeeFiscalApp получает от backend после проверки прав, а на обоих экранах показывает одинаковые model, последние четыре символа serial и `user_code`. Сотрудник обязан визуально подтвердить совпадение.

### Шаг 3. Аутентификация сотрудника

BeeFiscalApp использует OIDC Authorization Code + PKCE и MFA согласно политике компании. Browser/mobile access token относится к сотруднику и никогда не передаётся SmartDevice.

Backend проверяет:

- роль `ADMIN` либо отдельное право `FISCAL_DEVICE_ACTIVATE`;
- принадлежность сотрудника активному tenant из проверенного токена BeeFiscalApp;
- право на выбранную торговую точку;
- отсутствие уже активной несовместимой привязки serial/FMIN/device key;
- допустимость vendor/model/firmware и заявленных capabilities;
- TTL, одноразовость session, rate limits и отсутствие revoke/deny записи.

Организация всегда берётся из проверенного tenant context BeeFiscalApp. `organization_id` обязателен в подписанном activation credential, но отсутствует среди выбираемых полей и не принимается от клиента в теле команды. Backend отклоняет несовпадение tenant в access token, session устройства и создаваемой binding.

### Шаг 4. Выбор бизнес-привязки

После сканирования BeeFiscalApp показывает организацию активного tenant как неизменяемый контекст, полученные от backend данные устройства и позволяет выбрать:

1. торговую точку (`location_id`) только внутри активного tenant;
2. кассовое место (`register_id`) внутри этой точки;
3. роли:
   - обязательная `FISCAL_DEVICE`;
   - опциональная `PAYMENT_TERMINAL`, только если capability подтверждена adapter/firmware и есть отдельный действующий acquirer provisioning status.

Одна операция подтверждения создаёт draft binding. Backend сам вычисляет `organization_id` и проверяет цепочку `organization → location → register`. В UI явно показывается итог: «BlueCash-50 …1234 будет фискальным устройством и платежным терминалом кассы R01, магазин Sofia Center». Сотрудник подтверждает MFA/re-authentication для high-risk action.

### Шаг 5. Выпуск activation envelope

Backend атомарно резервирует binding и выпускает одноразовый envelope с TTL не более 5 минут:

```json
{
  "activation_session_id": "uuid",
  "device_instance_id": "uuid",
  "organization_id": "server-owned-id",
  "location_id": "uuid",
  "register_id": "uuid",
  "roles": ["FISCAL_DEVICE", "PAYMENT_TERMINAL"],
  "binding_version": 1,
  "device_key_thumbprint": "base64url-sha256",
  "credential_id": "uuid",
  "aud": "beefiscal-smart-device-bootstrap",
  "scope": "device.activate",
  "nbf": 0,
  "exp": 0,
  "jti": "uuid"
}
```

Envelope подписывается asymmetric backend signing key и шифруется на ephemeral public key из QR. Рекомендуемый формат — compact JWE с внутренним JWS; private backend signing key находится в KMS/HSM. Не следует передавать текущий HS256 bearer JWT: SmartDevice не может проверить его без общего симметричного секрета, а утечший bearer можно воспроизвести. Если API сохраняет название `activation_token`, его значение должно стать encrypted, one-time, key-bound envelope.

BeeFiscalApp получает только ciphertext и его SHA-256 fingerprint. Оно не может расшифровать или использовать device credential и не сохраняет envelope после завершения операции.

### Шаг 6. Безопасная доставка

При мобильном сценарии BeeFiscalApp подключается к BLE service, объявленному в QR. На Android предпочтителен Companion Device Manager: он даёт системный pairing UX и позволяет не запрашивать location permission для поиска companion device. Источники: [Android Bluetooth permissions](https://developer.android.com/develop/connectivity/bluetooth/bt-permissions) и [Companion device pairing](https://developer.android.com/develop/connectivity/bluetooth/companion-device-pairing).

Перед отправкой стороны выполняют challenge-response:

1. BeeFiscalApp отправляет backend nonce и свой session ID;
2. SmartDevice подписывает nonce `device_key`;
3. BeeFiscalApp передаёт proof backend;
4. backend сверяет key thumbprint/session и разрешает доставку;
5. ciphertext передаётся фрагментами с sequence, total, hash и write-with-response;
6. SmartDevice собирает пакет, сверяет общий hash, расшифровывает ephemeral private key, проверяет backend JWS, `aud`, `exp`, `jti`, session ID, key thumbprint и все binding IDs.

Обычного BLE pairing или знания MAC-адреса недостаточно: BLE является недоверенным транспортом, а целостность и конфиденциальность обеспечивает application-layer envelope. Незавершённый или конфликтующий набор fragments уничтожается по timeout.

При browser-сценарии приоритет такой:

1. регистратор получает envelope напрямую по HTTPS после подтверждения user code в BeeFiscalApp;
2. если cloud route отсутствует — Chrome Web Bluetooth в secure context;
3. если ни один путь недоступен — activation не выполняется; ручное копирование token запрещено.

### Шаг 7. Proof of possession и commit

После расшифрования SmartDevice не считает себя активированным. Оно вызывает backend activation commit по HTTPS и отправляет:

- `credential_id`, `activation_session_id`, `binding_version`;
- DPoP-style proof, подписанный постоянным `device_key`;
- server nonce, HTTP method/URI, `iat`, уникальный `jti`;
- digest hardware identity/capabilities и application build identity.

Backend проверяет подпись, nonce, replay cache, envelope state и неизменность draft binding. Затем одной транзакцией:

1. помечает activation session `CONSUMED`;
2. активирует device и binding с `active_from`;
3. записывает key thumbprint, attestation verdict, firmware/app versions и capabilities snapshot;
4. устанавливает роли адаптера для конкретного `register_id`;
5. выпускает краткоживущий sender-constrained operational access token и ротационный credential, привязанные к `device_key`;
6. увеличивает `binding_version`;
7. пишет неизменяемый audit event с actor subject, tenant/location/register/device, временем и fingerprints без секретов.

Успех показывается на обоих экранах только после commit. BeeFiscalApp читает server state и отображает organization, location, register, adapter roles, connectivity и credential expiry — не токены.

## 5. Повторная привязка и отзыв

Устройство нельзя молча перенести между организациями или точками. Перепривязка — отдельная high-risk операция. Перенос в другую организацию нельзя выполнить выбором tenant внутри activation form: администратор должен завершить отзыв в исходном tenant, активировать BeeFiscalApp в целевом tenant и начать новую регистрацию.

1. сотрудник с правом на текущую и целевую область либо central administrator инициирует `REASSIGN`;
2. backend немедленно увеличивает `binding_version`, отзывает operational credentials и блокирует новые операции;
3. открытые/unknown fiscal operations должны быть reconciled по отдельному процессу до переноса;
4. выполняется новый физический QR/user-code flow и MFA approval;
5. создаётся новый binding; старый остаётся в audit history с `inactive_from`.

Для компрометации/утраты доступны `REVOKE` и `QUARANTINE`. Backend прекращает принимать proof старого key, рассылает webhook/monitoring event и требует factory reset/re-enrollment. Удаление приложения или потеря Android Keystore автоматически означает новую регистрацию: восстановление private key из backup запрещено.

## 6. Состояния и fail-closed правила

Серверная машина состояний:

```text
UNREGISTERED → SESSION_CREATED → USER_CONFIRMED → ENVELOPE_ISSUED
→ PROOF_VERIFIED → ACTIVE

Из любого промежуточного состояния: EXPIRED | DENIED | CANCELLED
Из ACTIVE: QUARANTINED | REVOKED | REASSIGN_PENDING
```

Обязательные правила:

- session/user code/envelope/jti одноразовые;
- повтор запроса с тем же idempotency key возвращает тот же результат, но не создаёт вторую привязку;
- `ACTIVE` достигается только после proof-of-possession commit;
- payment role не активируется только по заявлению приложения — требуется capability и acquirer state;
- device не принимает operational команды при несовпадении organization/location/register/binding version;
- истечение session, BLE disconnect, неизвестный backend signature, invalid attestation или смена key приводит к отказу, а не к локальному «успеху»;
- никакой shared vendor/admin password, статический QR, серийный номер или MAC-адрес не является device identity;
- секреты не попадают в QR, логи, screenshots, analytics, crash reports или browser storage.

## 7. Итоговое решение для текущих приложений

Текущая схема BlueCash с ручным вводом organization/location/device на регистраторе и локальным разбором HS256 JWT подходит только как временный DEV bootstrap. Целевая схема должна заменить её следующим образом:

- SmartDevice генерирует key и QR/session, но не знает организацию до server-approved envelope;
- BeeFiscalApp получает server-owned organization из своего активного tenant, выбирает только location/register и подтверждает роли;
- backend выпускает encrypted key-bound one-time envelope;
- BeeFiscalApp только доставляет ciphertext по BLE или подтверждает выдачу через HTTPS;
- SmartDevice завершает activation прямым proof-of-possession commit;
- backend становится единственным источником истины о привязке, а terminal app инкапсулирует детали Daisy/Datecs Fiscal/Pinpad коммуникации за единым adapter capability surface.

Это одновременно проще для настройщика — scan, выбрать точку, подтвердить — и безопаснее текущей передачи bearer JWT: credential нельзя перенести на другую кассу, сотрудник его не видит, а одна и та же процедура работает для DaisySMART, BlueCash и последующих smart-регистраторов.
