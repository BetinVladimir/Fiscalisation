# BeeFiscalApp: настройка и realtime контроль устройств

Статус: `P0 OPEN`  
Дата аудита: 2026-08-14

## 1. Фактическое состояние

BeeFiscalApp уже позволяет создать базовую запись устройства
(`kind/vendor/model/serial`), выбрать закрытый composite profile, adapter,
fiscal/payment endpoint, подготовить/отключить binding и вручную запросить
readiness/diagnostics. Этого недостаточно для настройки физического контура:

- нет редакторов UART/USB/BLE и явных driver/protocol/version;
- ручная readiness опирается на backend broker/global probe;
- нет realtime adapter/fiscal/POS activity и freshness;
- нет команды и результата тестовой печати;
- нет отображения apply rejection/rollback и полного сравнения desired/applied
  generation.

## 2. P0-UI-CFG-001 — мастер composite configuration

Добавить экран/мастер кассового места:

1. выбор поддерживаемого профиля;
2. adapter edge/smart device;
3. fiscal endpoint и условный payment endpoint;
4. типизированные поля RS-232, USB CDC или BLE GATT из OpenAPI schema;
5. read-only resolved vendor/model/driver/protocol/version/capabilities;
6. client и server validation совместимости;
7. preview generation, confirm/publish, состояние
   `PENDING/APPLIED/REJECTED/ROLLED_BACK`, error details;
8. disable/rebind с optimistic concurrency и audit confirmation.

Секреты и private keys UI не показывает и не принимает. Tenant берётся только
из access token.

## 3. P0-UI-HEALTH-001 — realtime страница устройств

UI не должен подключаться к tenant MQTT напрямую. Backend materializes MQTT, а
BeeFiscalApp использует REST polling (рекомендуемо 5 секунд с backoff и
visibility pause) либо авторизованный SSE, если он будет добавлен в OpenAPI.

Экран списка показывает adapter/fiscal/POS отдельно: state, last seen/age,
location/register, transport, desired/applied generation и последнюю ошибку.
Карточка устройства показывает endpoint identity, protocol/driver, printer and
paper state, POS readiness, last successful I/O и последние state transitions.
После TTL UI явно показывает `STALE/OFFLINE`, а не старый зелёный статус.

## 4. P0-UI-TEST-001 — безопасные действия диагностики

Добавить RBAC-controlled действия:

- `Проверить связь` — asynchronous device probe, с прогрессом и результатом
  adapter/fiscal/payment endpoint;
- `Тест принтера` — вызывает канонический `PRINTER_TEST`, показывает operation
  state, paper/printer error и audit reference;
- refresh/retry только с новым UUID по явному решению пользователя; обычное
  обновление страницы не повторяет physical command.

Тест оплаты реальной картой не входит в обычную кнопку диагностики. Если он
понадобится для HIL, это отдельный service-mode workflow с явной суммой,
подтверждением и обязательным автоматическим возвратом/reconciliation.

## 5. P1-UI-OBS-001 — эксплуатационная детализация

- фильтры по точке, кассе, profile/state/vendor;
- индикатор причин `DEGRADED` и timeline переходов;
- ссылка на binding generation, operation и audit evidence;
- WCAG/accessibility labels и touch targets для Android/iOS/Web;
- локализация BG как обязательная, без расхождения enum labels с API.

## 6. Acceptance

- component/E2E tests покрывают три MVP profile и forbidden combinations;
- оператор на одном экране создаёт, публикует, видит apply generation и проверяет
  оба endpoint без ручного редактирования JSON;
- unplug/status/LWT достигают UI в установленный SLA и отображаются отдельно;
- ADMIN/SERVICE может выполнить printer test, CASHIER/AUDITOR получает 403 и
  UI не показывает запрещённое действие;
- Android, iOS и Web builds используют один OpenAPI-generated client contract;
- browser refresh/retry не повторяет физическую тестовую печать.

