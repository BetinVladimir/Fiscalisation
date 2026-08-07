# BeeMiniPOS — эталонная подключаемая POS-система для MVP

**Статус:** продуктовая и техническая спецификация  
**Версия:** 0.1  
**Дата:** 2026-08-07  
**Frontend:** `/Fiscalisation/BeeMiniPOS` — Expo / React Native  
**Backend:** `/Fiscalisation/beeminipos-backend` — Go  

## 1. Назначение

BeeMiniPOS — минимальная автономная POS-система и одновременно эталон интеграции стороннего POS/ERP с BeeFiscal Platform.

Она должна доказать три свойства MVP:

1. Независимая POS-система может работать со своими пользователями, каталогом, сменами и операционной БД.
2. Для фискализации ей достаточно документированного публичного API BeeFiscal.
3. POS-клиент не содержит compliance-логики и не обращается к кассе, MQTT, edge-журналу или внутренним БД Fiscal Platform напрямую.

BeeMiniPOS не является частью внутреннего cloud-core и не получает привилегированных интеграционных путей. Все возможности, используемые BeeMiniPOS, должны быть доступны будущему внешнему партнеру через те же OpenAPI, webhook и sandbox.

## 2. Архитектурная граница

```text
BeeMiniPOS Expo
   |
   | HTTPS, собственный MiniPOS API
   v
beeminipos-backend
   |             |
   |             +--> BeeMiniPOS PostgreSQL
   |                  products, employees,
   |                  settings, shifts,
   |                  order projection
   |
   | public HTTPS API + signed webhooks only
   v
BeeFiscal Public API
   |
   +--> Fiscal Core / Compliance / Audit
   +--> MQTT Router --> Edge --> Fiscal Device
```

Запрещенные зависимости:

- прямое подключение `beeminipos-backend` к EMQX/MQTT;
- чтение/запись PostgreSQL, Redis или RabbitMQ Fiscal Platform;
- импорт внутренних Go packages Fiscal Platform;
- vendor-specific команды кассы из Expo/backend;
- общий database schema или общие миграции;
- использование внутренних, не опубликованных endpoints;
- копирование compliance rules в MiniPOS.

Текущий MQTT-клиент в каркасе `beeminipos-backend` должен быть удален из production path. События результата поступают через подписанные webhooks; потерянный webhook восстанавливается через публичный status API.

## 3. Владение данными

### 3.1 BeeMiniPOS является System of Record для

- карточек товаров и категорий;
- локальных цен продажи;
- сотрудников MiniPOS и назначения ролей UI;
- конфигурации одной торговой точки;
- конфигурации одного кассового места;
- бизнес-смен MiniPOS;
- корзин и нефискального состояния заказа;
- UI preferences и layout быстрых кнопок;
- ссылок на фискальные операции и локальной read-model их статусов.

### 3.2 BeeFiscal является System of Record для

- решения о необходимости фискального документа;
- device readiness и блокировок Н-18;
- фискальной sale session с момента первой зафиксированной позиции, если активен `BG_SUPTO_FULL`;
- УНП и его счетчика;
- payment-to-fiscal workflow;
- команды ФУ, номера ФБ, фискального результата и reconciliation;
- сторно-ФБ;
- compliance audit, обязательных выгрузок и истории правил;
- binding legal entity/location/register/workstation ↔ fiscal device.

MiniPOS может хранить immutable mirror полученного результата, но не может менять authoritative fiscal state.

## 4. Автономная база данных

`beeminipos-backend` получает отдельную PostgreSQL DB с отдельным database/user/migrations/backup policy. Она не размещается в schema Fiscal Platform.

Минимальные таблицы:

```text
organizations
locations
registers
employees
employee_roles
categories
products
product_prices
shifts
orders
order_lines
payments
fiscal_operation_links
webhook_inbox
outbox_events
sync_cursors
audit_events
app_settings
```

Ограничения MVP:

- ровно одна активная organization/legal entity;
- ровно одна активная location;
- ровно один active register/workstation;
- несколько сотрудников;
- одна открытая смена на register;
- валюта новых операций — EUR;
- одна локаль `bg-BG`, архитектура допускает расширение.

Ограничения enforce-ятся backend constraints, а не только скрытием кнопки в Expo.

### 4.1 Локальная БД Expo

Expo-приложение может использовать SQLite для encrypted/session-safe cache:

- каталог и категории;
- текущая корзина до подтверждения backend;
- read-only данные сотрудника/смены;
- очередь UI-intents, которым еще не присвоен server acknowledgement.

SQLite не является authoritative sales database. Клиент не генерирует УНП, не помечает оплату успешной и не исполняет fiscal command offline. После reconnect клиент сверяется с MiniPOS backend.

## 5. Минимальные редакторы

Все редакторы реализуются в Expo как touch-first forms и вызывают MiniPOS API.

### 5.1 Продукты

Поля:

- `code` — уникальный внутренний код;
- `barcode` — необязательный;
- название на болгарском;
- категория;
- цена с точностью decimal;
- налоговая группа/ставка, выбранная из разрешенного BeeFiscal reference API;
- единица измерения;
- active/inactive;
- цвет/позиция quick tile.

Изменение цены/налоговой группы effective-dated. Уже созданные продажи содержат snapshot и не меняются задним числом.

### 5.2 Сотрудники

Поля:

- уникальный ID;
- минимум два имени;
- email;
- четырехсимвольный operator code, согласованный с Fiscal Platform;
- роль `CASHIER` или `MANAGER`;
- `active_from` / `active_to`;
- active/inactive.

MVP использует passwordless login, но cashier session всегда однозначно связана с сотрудником. Общая учетная запись кассира запрещена.

### 5.3 Точка продаж

Одна редактируемая запись:

- название и адрес объекта;
- legal entity reference;
- timezone `Europe/Sofia`;
- Fiscal Platform `location_id`;
- контакты;
- статус binding.

### 5.4 Кассовое место

Одна редактируемая запись:

- название/код рабочего места;
- Fiscal Platform `workstation_id`, `register_id`, `device_id`;
- device model/serial/ИН ФУ как read-only данные из public API;
- readiness/last seen;
- receipt delivery capabilities;
- printer/fiscal status.

Нельзя вручную подменить ИН ФУ. Binding выполняется разрешенной процедурой Fiscal Platform.

## 6. Кассовые смены

Смена MiniPOS — бизнес-объект, связанный с register и сотрудником. Открытие/закрытие координируется с публичным Fiscal API и возможностями ФУ.

Состояния:

```text
CLOSED -> OPENING -> OPEN -> CLOSING -> CLOSED
                    |
                    +-> BLOCKED_RECONCILIATION
```

Открытие:

1. Аутентифицировать сотрудника.
2. Проверить отсутствие открытой смены.
3. Вызвать Fiscal Platform readiness.
4. При необходимости зарегистрировать начальную наличность через fiscal cash movement API.
5. Зафиксировать `fiscal_shift_reference`.
6. Разрешить экран продаж.

Закрытие:

1. Запретить новые корзины.
2. Дождаться/разрешить все pending/unknown операции.
3. Выполнить сверку cash expected/actual.
4. Запросить требуемый Z-report через public API.
5. Сохранить Z reference и итоги.
6. Закрыть смену только после подтвержденного результата или создать reconciliation case.

Функции:

- открыть/закрыть смену;
- внести/изъять наличность через `служебно въведени/изведени`;
- cash count;
- текущие итоги;
- список операций смены;
- manager note для расхождения;
- запрет параллельной смены на одном register.

## 7. Touch-first интерфейс продаж

### 7.1 Основной экран

Landscape-first tablet layout:

```text
+----------------------+----------------------+
| Search / categories  | Current cart         |
|                      | line, qty, price     |
| [large product tiles]| discount/remove     |
|                      |                      |
|                      | TOTAL                |
|                      | [PAY - large CTA]    |
+----------------------+----------------------+
```

Portrait поддерживается для handheld, но tablet landscape — эталон MVP.

Touch requirements:

- интерактивная зона минимум 48x48 dp, основные действия 56–64 dp;
- отсутствие hover-only действий;
- большие numeric keypad и payment buttons;
- не более двух taps для добавления популярного товара;
- постоянное отображение сотрудника, смены, connectivity и readiness;
- явная блокировка checkout с понятной причиной;
- защита от double tap/payment duplication;
- подтверждение destructive actions;
- доступность: contrast, scalable text, screen reader labels.

### 7.2 Продажа

1. Открыта смена и подтверждена readiness.
2. Кассир добавляет первую позицию.
3. MiniPOS backend создает/обновляет локальный order и вызывает публичный Fiscal Sales Session API.
4. Fiscal Platform возвращает fiscal sale ID/УНП/allowed actions.
5. Каждое изменение корзины синхронизируется с authoritative fiscal session либо отправляется окончательным immutable payload согласно утвержденному public contract. Для `BG_SUPTO_FULL` используется первый вариант, чтобы УНП возник в нормативный момент.
6. На `PAY` backend отправляет payment intent с idempotency key.
7. UI показывает `PROCESSING` до результата.
8. Успех отображается только после `FISCALIZED`; timeout показывает `Проверка результата`, не повторяет печать.
9. Корзина очищается после сохранения результата и ссылки на ФБ.

### 7.3 Операции MVP

- обычная продажа;
- количество/удаление строки до оплаты;
- процентная или суммовая скидка с правами manager;
- cash payment;
- card payment как тип фискального платежа, без реализации acquiring в первой итерации;
- split cash/card, если выбранное ФУ подтверждено capability tests;
- отмена незавершенной продажи;
- сторно завершенной продажи с manager authorization;
- повторное отображение результата без повторной фискализации.

## 8. Отчеты MiniPOS

### Операционные

- текущая смена;
- продажи по смене;
- оплаты по типу;
- продажи по сотруднику;
- продажи по товару/категории;
- отмены и сторно;
- cash movements;
- расхождения кассы;
- pending/failed/unknown fiscal operations.

### Фискальные через public API

- X-report;
- Z-report;
- status/history report references;
- receipt lookup;
- reconciliation status.

MiniPOS не строит «фискальный отчет» из собственной БД. Он запрашивает его у Fiscal Platform/ФУ и хранит ссылку/копию результата. Собственные отчеты маркируются как операционные и не имитируют фискальный документ.

## 9. Публичная интеграция

### 9.1 Authentication

- OAuth 2.1 client credentials или эквивалентный partner credential для backend-to-backend;
- scopes: `fiscal.sales`, `fiscal.reversals`, `fiscal.shifts`, `fiscal.reports`, `fiscal.devices.read`;
- отдельные credentials sandbox/production;
- secret никогда не попадает в Expo bundle;
- tenant/location scope закреплен server-side.

### 9.2 Используемые API

```text
GET  /public/v1/devices/{id}/readiness
POST /public/v1/fiscal-sales
PATCH /public/v1/fiscal-sales/{id}
POST /public/v1/fiscal-sales/{id}/payments
POST /public/v1/fiscal-sales/{id}/cancel
POST /public/v1/fiscal-sales/{id}/reversals
GET  /public/v1/fiscal-operations/{id}
POST /public/v1/registers/{id}/cash-movements
POST /public/v1/registers/{id}/reports
GET  /public/v1/reference/bg/tax-groups
GET  /public/v1/reference/bg/payment-types
```

Точный контракт фиксируется OpenAPI. BeeMiniPOS CI генерирует client SDK из опубликованной спецификации; hand-written calls допускаются только через один adapter package.

### 9.3 Webhooks

`beeminipos-backend` принимает:

- `fiscal.operation.updated`;
- `fiscal.operation.succeeded`;
- `fiscal.operation.failed`;
- `fiscal.operation.reconciliation_required`;
- `device.readiness.changed`;
- `register.report.completed`.

Webhook inbox сохраняет raw body до обработки, проверяет signature/timestamp, дедуплицирует event ID и обрабатывает at-least-once delivery. Polling status API остается обязательным fallback.

## 10. Надежность и автономность

Автономность означает независимость данных и жизненного цикла MiniPOS, а не возможность фискализировать без Fiscal Platform/Edge.

- MiniPOS DB продолжает обслуживать каталог, сотрудников и историю при временной недоступности Fiscal API.
- Новая фискальная продажа разрешена offline только если доступен локальный BeeFiscal Edge Core через тот же публичный контракт.
- Если ни cloud, ни authorized local API недоступны, UI не открывает новую фискальную продажу.
- Outbox применяется к MiniPOS→Fiscal API intents, но не разрешает blind retry после возможного исполнения кассой.
- Каждая команда имеет `external_id`, `idempotency_key`, `order_id`, `shift_id`, `operator_id`.
- Для `UNKNOWN` UI и backend запускают status/reconciliation, а не повтор payment.

## 11. Безопасность

- Expo вызывает только MiniPOS backend.
- Все бизнес-проверки дублируются/enforce-ятся backend.
- RBAC: `CASHIER`, `MANAGER`, `ADMIN`.
- Manager нужен для скидки выше лимита, сторно, reopen/reconciliation и editor access.
- Sensitive local storage использует platform secure storage; SQLite не хранит backend secrets.
- Audit фиксирует login/logout, editor changes, shift actions, discount, cancel, reversal и manual reconciliation.
- PII минимизируется; card data/PAN не хранится.
- Public API/webhook TLS, rotation, rate limits и replay protection обязательны.

## 12. Модули Expo

```text
src/
  app/                 navigation, providers
  auth/                login, session, role guards
  pos/                 product grid, cart, payment flow
  shifts/              open, cash count, close
  products/            list/editor
  employees/           list/editor
  setup/               location/register editors
  reports/             operational/fiscal views
  fiscal-status/       readiness, pending, reconciliation
  api/                 generated MiniPOS backend client
  storage/             cache only
  design-system/       touch components/tokens
```

Expo не содержит public Fiscal API client: только backend имеет partner credentials и вызывает Fiscal Platform.

## 13. Модули Go backend

Рекомендуется модульный монолит:

```text
internal/
  auth/
  catalog/
  employees/
  organization/
  shifts/
  orders/
  payments/
  reports/
  fiscalclient/        generated public API client + adapter
  webhooks/
  persistence/
  outbox/
  audit/
```

Текущий `internal/mqttclient` не входит в целевую архитектуру и должен быть удален после появления public API adapter.

## 14. Что не входит в MVP

- multi-location и multi-register UI;
- склад, закупки и поставщики;
- loyalty/CRM;
- рецептуры/ресторанные столы/KDS;
- полноценный acquiring/card SDK;
- сложные промо;
- e-commerce;
- бухгалтерские проводки;
- произвольный report builder;
- прямой BLE/MQTT/device protocol;
- собственная реализация Н-18 вместо Fiscal Platform.

## 15. Acceptance criteria

1. MiniPOS запускается с собственной чистой PostgreSQL без БД Fiscal Platform.
2. В production config отсутствуют MQTT credentials и broker dependency.
3. Все fiscal operations проходят опубликованный OpenAPI endpoint.
4. Можно создать товары, сотрудников, одну точку и одно кассовое место.
5. Backend запрещает вторую активную точку/кассовое место в MVP.
6. Сотрудник открывает смену только при готовом ФУ.
7. Первая позиция приводит к созданию fiscal sale/УНП через public API.
8. Double tap `PAY` создает один фискальный документ.
9. Timeout после отправки не вызывает повторной печати.
10. Закрытие смены невозможно при unresolved `UNKNOWN` без manager flow.
11. Z-report получен через public API и связан со сменой.
12. Собственные отчеты явно маркированы как операционные.
13. Webhook duplicate не меняет результат повторно.
14. Потерянный webhook восстанавливается polling/reconciliation job.
15. Tenant isolation проверена integration tests.
16. Основной sales flow проходит на tablet без mouse/keyboard.
17. Все ключевые touch targets ≥48 dp.
18. POS не содержит vendor-specific Datecs code.
19. Sandbox и production используют одинаковый public contract.
20. Внешний demo client способен повторить интеграцию без доступа к monorepo internals.

## 16. План MVP

### M0 — contracts и persistence

- MiniPOS PostgreSQL и migrations;
- organization/location/register constraints;
- products/employees;
- public Fiscal OpenAPI client;
- удалить MQTT из target dependency graph.

### M1 — vertical sale

- auth/operator session;
- shift open;
- touch product grid/cart;
- public fiscal sale/payment;
- webhook inbox + status polling;
- success/failed/unknown UI.

### M2 — operations

- cancel/storno;
- cash movements;
- shift close + X/Z;
- operational reports;
- reconciliation queue.

### M3 — reference quality

- sandbox/simulator;
- generated SDK example;
- fault injection tests;
- touch/accessibility QA;
- deployment/runbook;
- partner integration guide на основе BeeMiniPOS.

## 17. Итоговое решение

BeeMiniPOS — не «встроенный POS платформы», а первая внешняя система, использующая платформу на тех же условиях, что будущий партнер. Ее ценность для MVP — одновременно рабочая минимальная касса, integration test harness и живая документация публичного API.
