# Beeloy Fiscal Platform
## Единый облачный API для фискальной интеграции в ЕС

> **Stripe для фискальных операций** — универсальная платформа, превращающая сложную интеграцию с кассовыми аппаратами в простой REST API.

---

## Оглавление

1. [Проблема](#1-проблема)
2. [Решение](#2-решение)
3. [Архитектура системы](#3-архитектура-системы)
4. [Ключевые компоненты](#4-ключевые-компоненты)
5. [Технические возможности](#5-технические-возможности)
6. [Безопасность и соответствие требованиям](#6-безопасность-и-соответствие-требованиям)
7. [Собственная POS-система BeeMiniPOS](#7-собственная-pos-система-beeminipos)
8. [Конкурентные преимущества](#8-конкурентные-преимущества)
9. [Бизнес-модель и рынок](#9-бизнес-модель-и-рынок)
10. [Рыночные перспективы в ЕС](#10-рыночные-перспективы-в-ес)
11. [Текущий статус разработки](#11-текущий-статус-разработки)
12. [Дорожная карта](#12-дорожная-карта)
13. [Технологический стек](#13-технологический-стек)

---

## 1. Проблема

### Фискальные устройства в ЕС — головная боль каждого бизнеса

Любое торговое предприятие в ЕС обязано использовать сертифицированное фискальное оборудование для регистрации продаж и автоматической передачи данных налоговым органам. Сегодня интеграция с этим оборудованием является одной из самых дорогих и болезненных задач в retail-автоматизации.

#### Текущие проблемы рынка

**1. Проприетарные протоколы и vendor lock-in**
- Каждый производитель (Datecs, Daisy, Tremol, Epson, Sam4s...) реализует собственный бинарный протокол
- USB/UART/RS-232 интерфейсы требуют локальной инфраструктуры
- Программный драйвер, написанный для одного устройства, полностью несовместим с другим
- Смена поставщика оборудования = полная переработка интеграционного слоя

**2. Высокая стоимость и долгий rollout**
- Средняя стоимость интеграции нового фискального устройства в POS/ERP систему: **€15 000 – €80 000**
- Срок интеграции: **3–9 месяцев** на каждый новый тип устройства
- Поддержка и сертификация требуют отдельных специалистов в каждой стране
- Каждое обновление регуляторных требований (НАП, НАС, FURS...) требует срочного выпуска патчей

**3. Нет облачного контроля**
- Фискальные операции живут только локально на устройстве
- Отсутствует централизованный audit trail
- Offline-сценарии не стандартизированы: потеря чека = нарушение закона
- Нет real-time мониторинга для операторов и аудиторов

**4. Фрагментированный рынок**
- В Болгарии: ~ 15 сертифицированных производителей кассовых аппаратов
- В ЕС: более 60 производителей с национальной спецификой
- Каждая страна ЕС имеет собственный формат электронной отчётности (NAP/НАП BG, FURS SI, NTCA HU, KSeF PL...)
- Компании, работающие в нескольких странах ЕС, вынуждены поддерживать N независимых интеграций

```
Сегодня:

ERP/POS A  ──[Datecs driver v1.3]──► Datecs FP-700
ERP/POS A  ──[Daisy driver v2.1]───► Daisy FX1200
ERP/POS B  ──[Datecs driver v1.1]──► Datecs BlueCash-50
ERP/POS C  ──[Custom UART code]────► Tremol M20

Каждое соединение: отдельный код, отдельная сертификация, отдельная поддержка.
```

**Итог:** компании переплачивают за интеграцию, тратят месяцы на сертификацию и оказываются заложниками конкретного производителя оборудования.

---

## 2. Решение

### Единый API — как Stripe для платежей, но для фискализации

**Beeloy Fiscal Platform** разрывает этот порочный круг, предоставляя единый облачный REST API, который абстрагирует все фискальные устройства за стандартным интерфейсом.

```
Сегодня с Beeloy:

ERP/POS A  ──┐
ERP/POS B  ──┤──[POST /public/v1/sales]──► Beeloy Fiscal Platform
ERP/POS C  ──┘        (единый REST API)           │
                                                   ├──► Datecs FP-700
                                                   ├──► Datecs BlueCash-50
                                                   ├──► Daisy FX1200
                                                   └──► любое сертифицированное ФУ
```

#### Что даёт платформа

| Проблема | Решение Beeloy |
|---|---|
| Проприетарные протоколы | Единый REST API v1 с OpenAPI-контрактом |
| Vendor lock-in | Любое сертифицированное устройство за одним API |
| Долгая интеграция | Подключение нового POS/ERP за **1 день** по документации |
| Нет облачного контроля | Real-time audit trail, webhooks, centralized dashboards |
| Offline риски | Гарантированная доставка, 3-месячный криптографический журнал |
| Multi-country | Country-policy API — одна кодовая база для всех стран ЕС |

#### Ценностное предложение по ролям

**Для разработчиков POS/ERP:**
```http
POST /public/v1/sales
Authorization: Bearer {token}
{
  "payment_type": "CARD",
  "amount": "45.99",
  "currency": "EUR",
  "line_items": [...]
}
→ { "fiscal_reference": "AB123456-0001-0000123", "state": "FISCALIZED" }
```
Один вызов — получен фискальный чек, УНП, налоговая ссылка. Никакого UART.

**Для бизнеса:**
- Подключить новый кассовый аппарат = занести в admin-панель, не вызывая разработчиков
- Видеть все операции всех касс в реальном времени
- Получать webhook-уведомления о каждой транзакции

**Для аудиторов и налоговых органов:**
- Криптографически подписанный immutable audit trail
- Экспорт в стандартных форматах (JSON, CSV, XLSX)
- Полная прослеживаемость от чека до физического устройства

---

## 3. Архитектура системы

### Многоуровневая облачная платформа с offline-first Edge

```
┌────────────────────────────────────────────────────────────────────────┐
│                        ВНЕШНИЕ КЛИЕНТЫ                                  │
│    BeeMiniPOS        Сторонний ERP       BeeFiscalApp (Admin)           │
│   (Web/iOS/Android)   (REST API)         (iOS/Android/Web)             │
└─────────────┬──────────────┬──────────────────┬───────────────────────┘
              │              │                  │
              ▼              ▼                  ▼
┌────────────────────────────────────────────────────────────────────────┐
│                     FISCAL PLATFORM (Cloud)                             │
│                                                                         │
│  ┌─────────────────┐   ┌──────────────┐   ┌──────────────────────┐   │
│  │   API Gateway   │   │  Fiscal Core │   │   MiniPOS Backend    │   │
│  │   (Caddy TLS)   │   │  (Go)        │   │   (Go + PostgreSQL)  │   │
│  └────────┬────────┘   └──────┬───────┘   └────────────┬─────────┘   │
│           │                   │                         │              │
│  ┌────────▼────────────────────▼─────────────────────────▼──────────┐ │
│  │              PostgreSQL (Fiscal DB, isolated)                      │ │
│  │  Sales · Operations · Shifts · Audit · UNP sequences · Reports   │ │
│  └─────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│  ┌──────────────┐  ┌───────────────┐  ┌───────────────────────────┐  │
│  │  Webhook     │  │  MQTT Broker  │  │  OTA Control Plane        │  │
│  │  Outbox      │  │  (EMQX)       │  │  (Ed25519-signed)         │  │
│  └──────┬───────┘  └───────┬───────┘  └───────────┬───────────────┘  │
└─────────┼────────────────── ┼──────────────────────┼───────────────────┘
          │  Webhooks          │  MQTT/TLS            │  OTA Updates
          ▼                   ▼                       ▼
┌────────────────────────────────────────────────────────────────────────┐
│                         EDGE AGENT (Local)                              │
│  ┌────────────────────────────────────────────────────────────────┐   │
│  │  authority · journal (SQLite WAL) · BLE · runtime · sync · OTA │   │
│  │  Encrypted BLE Gateway · GATT processor · flow control         │   │
│  └────────────────────────┬───────────────────────────────────────┘   │
└───────────────────────────┼─────────────────────────────────────────────┘
                            │  BLE / USB / UART
                            ▼
┌────────────────────────────────────────────────────────────────────────┐
│                       SMART DEVICES (Android apps)                      │
│   Datecs BlueCash-50          Daisy SMART S          Другие ФУ         │
│   (CARD + FISCAL)             (CASH + FISCAL)        (roadmap)         │
└────────────────────────────────────────────────────────────────────────┘
```

### Три транспортных режима (единый UUID на всех)

| Режим | Сценарий | Механизм |
|---|---|---|
| **REST (Cloud)** | Основной путь при наличии сети | HTTPS → Fiscal Backend → Edge MQTT |
| **BLE (Local)** | Offline/нестабильная сеть | Encrypted BLE handshake → Edge Agent → ФУ |
| **Local HTTP** | LAN-сеть без интернета | HTTP → Edge Agent loopback → ФУ |

Ключевой принцип: **один `client_operation_id` и один `payload_sha256`** на всех транспортах. Переключение REST → BLE → Local HTTP не создаёт дублирующихся транзакций. Устройство выполняет команду ровно один раз.

---

## 4. Ключевые компоненты

### 4.1 Fiscal Backend (Go)

Сердце платформы — высокопроизводительный backend на Go с изолированной PostgreSQL базой данных.

**Операционные возможности:**
- Продажи: CASH / CARD / mixed / split tender
- Компенсации (сторно) с проверкой Art. 31(4) болгарского НК
- X-отчёт (дневной), Z-отчёт (закрытие смены) с фискальной ссылкой
- Отчёты о движении денежных средств (cash-in / cash-out)
- Reconciliation: `UNKNOWN → RECONCILING` с lookup-only семантикой (повторной отправки команды нет)

**Управление регистрами:**
- Жизненный цикл устройства: `DRAFT → PENDING_SERVICE_ACTIVATION → ACTIVE`
- Привязка: register ↔ fiscal device ↔ payment terminal (оба должны быть ACTIVE)
- BLE сессии: X25519 key agreement, AES-256-GCM, revocation journal
- Provisioning с evidence references (не симулируется в PROD)

**УНП / Regulatory identifiers:**
- Алгоритм BG_UNP_V1: FMIN-based, неповторяющиеся диапазоны
- 64 параллельных выделений без дублирования (тест на PostgreSQL)
- Привязка к tenant + register — другой тенант получает независимую нумерацию
- Immutable: единожды выданный УНП не может быть отозван или переиспользован

### 4.2 Edge Agent (Go)

Локальный агент, работающий на ESP32-S3 (или другом embedded-контроллере) рядом с фискальным устройством.

**Хранение и надёжность:**
- SQLite WAL/FULL journal на SD-карте
- 3-месячное ACK-gated хранение с GC
- Durable-before-device: команда записана в журнал ДО отправки на устройство
- После рестарта прерванная команда становится `UNKNOWN/RECONCILE` — никакого повторного выполнения

**Синхронизация с облаком:**
- Signed batch + signed ACK (HMAC): целостность каждого пакета
- При потере ответа — отправляется тот же idempotency key, не новый батч
- Accelerated soak test: 72 часа симуляции потерь сети, 0 потерянных событий, 0 дублей

**BLE протокол:**
- X25519/HKDF-SHA-256 key agreement
- AES-256-GCM с directional counters
- Canonical RFC 8949 deterministic CBOR
- Фрагментация по MTU (185/247/517 байт)
- Flow control: one in-flight, ACK bitmap, BUSY/cancel/resume

**OTA обновления:**
- Ed25519-signed манифесты с monotonic counter
- A/B staging + rollback ниже vulnerability floor → `RECOVERY_REQUIRED`
- Безопасный antirollback до получения production credentials

### 4.3 SmartDevices (Android)

Два независимых Android-приложения — по одному для каждого типа фискального устройства.

**Datecs BlueCash-50 (`bluecash-app`)**
- BLE provisioning через key-bound QR/code activation
- Интеграция CARD (платёжный терминал) + FISCAL (кассовый аппарат)
- mTLS transport; vendor SDK pending (PROD gate)

**Daisy SMART S (`daisy-smart-app`)**
- Полный контракт-тест STUB: 45 `Supported` команд + 16 `Optional`
- Display/printer/drawer, invoice, barcode types 1–13, customer QR, FM queries
- Activated-terminal enforcement; PROD gate: real device HIL

### 4.4 IoT / Protocol Abstraction (C++)

Arduino/PlatformIO уровень унификации протоколов Daisy и Datecs.

- **Daisy:** 88 команд (45 Supported + 16 Optional + Excluded) — полный typed реестр
- **Datecs:** 73 команды (40 core + 12 Optional + Excluded) — полный typed реестр
- Frame encoder/parser с BCC validation
- Golden vectors для всех Supported команд (C++ unit tests)
- Выбор целевого протокола при компиляции через препроцессор
- ECDSA P-256 device identity генерируется контроллером при первой инициализации

---

## 5. Технические возможности

### 5.1 Единый API — 92 операции с генерируемым контрактом

```
73 canonical operations   (зафиксированы в OpenAPI, hash-locked)
19 runtime operations     (расширения без изменения canonical)
─────────────────────────
92 total endpoints
92 request contracts  (body + path/query/header parameters)
92 response contracts (validation до передачи клиенту)
```

**Гарантии контракта:**
- Runtime middleware валидирует запрос до бизнес-логики
- Недокументированный 2xx или некорректный 4xx/5xx = contract violation, запрос отклоняется
- Генерируемые TypeScript-клиенты из canonical OpenAPI
- Drift-test: `make contract-test` — byte-for-byte сравнение с locked canonical

### 5.2 Idempotency — защита от дублей в distribuited-среде

Каждая мутация в обоих продуктах проходит через дуплексный idempotency механизм:

1. `client_operation_id` (UUID, 16-255 символов) в header
2. PENDING claim сохраняется ПЕРЕД бизнес-логикой
3. При duplicate request — byte-for-byte replay ответа из хранилища
4. `5xx` ответы также персистируются: retry не вызывает второй fiscal side effect
5. Fingerprint включает: body + query + `If-Match` + `X-App-Instance-Id` + OIDC issuer/subject

Результат: при потере сети, рестарте сервера или race condition — транзакция выполняется **ровно один раз**.

### 5.3 Compliance Exports

- **JSON**: полные immutable snapshots линий, скидок, tax groups, платежей
- **CSV**: стандартный разделитель, поля для аудита
- **XLSX**: ZIP-архив, совместимый с бухгалтерским ПО
- **BGN/EUR**: раздельные манифесты для периодов до/после перехода (2025-12-31)
- SHA-256 manifest привязан к каждому артефакту
- Immutable: созданный артефакт не может быть изменён задним числом

### 5.4 Аудит-журнал

- Append-only: каждая мутация добавляет запись, существующие записи не изменяются
- Hash-chaining на Edge (SQLite) и в облаке (PostgreSQL)
- Поля: actor, action, object, timestamp, УНП, event hash
- Фильтрация: actor / УНП / RFC3339 period
- Роль AUDITOR: read-only доступ к журналу без доступа к коммерческим данным

### 5.5 Offline-first продажи

```
Статусная машина при потере связи:

POS добавляет товар → IndexedDB outbox
                       ↓
                   Cloud down? ──Yes──► BLE ready? ──Yes──► BLE route
                       ↓                                       ↓
                      No                              Edge сохраняет в SQLite
                       ↓                                       ↓
                   REST route ◄──────────────────── Sync при восстановлении
```

- BLE offline-команды несут bounded expiry и SHA-256 canonical CBOR
- После восстановления сети: `POST /orders:import-offline` с дедупликацией
- Fiscal webhook доставляет `fiscal_reference` в MiniPOS асинхронно
- Receipt reference персистируется отдельно от storno reference

---

## 6. Безопасность и соответствие требованиям

### 6.1 Аутентификация и авторизация

**Production аутентификация:**
- OIDC RS256/JWKS с пиннингом `alg=RS256` и `kid` ротацией
- Проверка issuer / audience / expiry / tenant / roles / scope
- HMAC JWT — только DEV/E2E, запрещён в PROD конфигурационным guard

**RBAC (fail-closed):**

| Роль | Права |
|---|---|
| `CASHIER` | Свои смены/заказы; нет employee directory, нет tenant-wide отчётов |
| `SUPERVISOR` | Фискальные отчёты, reversal, export |
| `ADMIN` | Полное управление: конфигурация, provisioning, webhooks, editors |
| `AUDITOR` | Read-only: audit journal, отчёты |
| `SERVICE` | Edge sync, diagnostics (не публичный POS API) |

**Force RLS на PostgreSQL:**
- Каждый backend работает под non-owner reader pool
- `FORCE ROW LEVEL SECURITY` на всех таблицах
- Межтенантное чтение физически невозможно на уровне БД

### 6.2 Вебхуки

- Подпись `t,kid,v1` по схеме `timestamp.raw_body`
- Constant-time HMAC сравнение (защита от timing attacks)
- 300-секундное replay window
- HTTP 410 → дурабельное отключение endpoint без retry
- Secrets disclosed once, GET — redacted
- 24-часовое overlap при ротации secret

### 6.3 BLE Security

- X25519 ephemeral key agreement (каждая сессия)
- HKDF-SHA-256 key derivation с bidirectional keying
- AES-256-GCM с monotonic directional counters
- Proof-of-possession: MiniPOS генерирует X25519 key ДО REST issuance; Fiscal подписывает его в ticket; Edge принимает HELLO только при совпадении
- Revocation: atomic в SQLite + outbox event → Edge кэширует отозванные tickets

### 6.4 Регуляторное соответствие Болгария (SUPTO BG-014)

**SUPTO Annex 29 trace — 24 пункта:**

| Статус | Количество | Примечание |
|---|---|---|
| PASS | 3 | Полностью software-verified |
| PARTIAL | 19 | Software done; external/hardware gate |
| NOT_APPLICABLE | 2 | Не применимо к профилю |

**Реализованные требования:**
- `BG_UNP_V1` — алгоритм выдачи фискальных номеров
- EUR currency policy с effective-date API
- Tax groups B/20% (seed проверен), A/C-H fail-closed до подписания policy update
- Operator authority с `active_from/active_to` временными границами
- Art. 31(4): сторно `OPERATOR_ERROR` запрещено с 00:00 8-го числа следующего месяца
- Document policy: 6 шаблонов, server-side enforcement, fiscal wording prohibition
- Signed release evidence: SBOM (CycloneDX 1.6), SLSA Provenance v1, Ed25519

**Production gates (намеренно not-simulated):**
- Реальный HIL на физических устройствах (DP-150, BlueCash-50, Daisy Compact)
- Подпись НАП/BIM/authorized-service
- Юридическое принятие
- Production signing и vulnerability scan

---

## 7. Собственная POS-система BeeMiniPOS

Параллельно с платформой разработан готовый к использованию POS для малого и среднего бизнеса.

### 7.1 BeeMiniPOS — Web (React/Vite)

**Возможности продаж:**
- Каталог товаров с поиском по штрих-коду / SKU / названию
- Добавление нескольких позиций, редактирование количества
- Скидки: процент или абсолютная сумма (SUPERVISOR / ADMIN)
- Типы оплаты: CASH / CARD / mixed split
- Split tender: оператор вводит сумму CARD → система считает CASH remainder
- Автоматический reversal (сторно) последнего заказа

**Смены и отчёты:**
- Open shift → sales → Z-report close (фискальная ссылка обязательна)
- Blocked Z? → `POST /shifts/{id}/reconcile` (lookup-only, без повторной Z-команды)
- Коммерческий отчёт продаж за период (RFC3339, half-open, EUR total, CASH/CARD breakdown)

**Offline:**
- IndexedDB outbox при потере cloud
- `syncOfflineOrders()` каждые 15 секунд после восстановления
- Fiscal webhook доставляет `fiscal_reference` асинхронно

### 7.2 BeeMiniPOS — Expo (iOS / Android / Web)

Нативное приложение на Expo React Native с одной кодовой базой для трёх платформ.

**Дополнительные возможности:**
- BLE подключение к Edge Agent (Chrome Web Bluetooth / react-native-ble-plx)
- Офлайн-продажи через зашифрованный BLE канал
- UNKNOWN outcome panel: freeze cart → GET-only polling → reconcile
- Shift recovery после перезагрузки: GET filtered by employee/register/state
- Scan штрих-кода (camera / scanner)

**Платформа:**

| Платформа | Статус |
|---|---|
| Web | ✅ Production bundle, Playwright E2E |
| Android | ✅ APK (debug + unsigned release) |
| iOS | ✅ Simulator bundle |
| Android native BLE | ✅ react-native-ble-plx |
| iOS native BLE | ✅ linked |

### 7.3 BeeFiscalApp — Административный интерфейс

Expo-приложение для администраторов торговых точек и операторов.

- **Устройства:** регистрация, DRAFT→ACTIVE lifecycle, capability read, diagnostics
- **Операции:** история с фильтрами, UNKNOWN → reconciliation (server-owned `allowed_actions`)
- **Отчёты:** X/Z/KLEN, фискальные reference, export
- **Аудит:** tab доступен только ADMIN/AUDITOR, полная immutable история
- **Администрация:** locations, registers, operators, device bindings
- **BLE preparation:** REST-issued proof-of-possession session с X25519 public key

---

## 8. Конкурентные преимущества

### 8.1 Технические рвы

**1. Единый UUID через все транспорты**
Конкуренты привязывают транзакцию к конкретному транспорту. Beeloy сохраняет один `client_operation_id` при переключении REST → BLE → Local HTTP. Физическое устройство выполняет команду ровно один раз, независимо от потерь сети.

**2. Durable-before-device**
Edge Agent записывает команду в SQLite ДО отправки на физическое устройство. При любом сбое (power loss, crash) — команда либо FISCALIZED, либо UNKNOWN/RECONCILE, но никогда не пропадает молча.

**3. Cryptographic audit trail**
Hash-chaining от физического устройства до облака. Каждое событие включает `event_hash`, привязанный к предыдущему. Подделка промежуточной записи математически обнаруживаема.

**4. Одна интеграция — все устройства**
C++ protocol abstraction layer: 88 команд Daisy + 73 команды Datecs с typed builders/parsers. Добавление нового производителя = реализация адаптера без изменения API.

**5. Country Policy API**
`GET /country-policy?effective_at=` возвращает действующие налоговые правила для конкретной даты. ERP/POS не кодирует НДС-ставки жёстко — они приходят из платформы. Смена законодательства = обновление платформы, а не всех клиентов.

### 8.2 Операционные рвы

**Зафиксированные OpenAPI-контракты**
73 canonical операции защищены hash-lock. Клиенты получают стабильные контракты. Обратная совместимость гарантируется machine-проверкой, а не процессами.

**Генерируемые клиенты**
TypeScript SDKs генерируются из canonical OpenAPI: `openapi-typescript` + `openapi-fetch`. Клиент всегда синхронизирован с сервером.

**Изолированные продукты**
Fiscal и MiniPOS — два полностью независимых PostgreSQL, сети, Caddy-экземпляра. MiniPOS не имеет доступа к internal Fiscal API. Падение одного не влияет на другой.

### 8.3 Сравнение с альтернативами

| Критерий | Beeloy | Традиционная интеграция | Локальный middleware |
|---|---|---|---|
| Подключение нового ФУ | Дни (адаптер) | Месяцы | Месяцы |
| Multi-country | Country Policy API | Отдельный проект per country | Отдельный проект |
| Offline | BLE + SQLite journal | Зависит от POS | Обычно нет |
| Audit trail | Cryptographic, cloud | Локальный, vendor-specific | Частичный |
| Vendor lock-in | Нет | Полный | Частичный |
| Стоимость интеграции | €0 API / pay-per-use | €15–80k per device type | €10–50k |

---

## 9. Бизнес-модель и рынок

### 9.1 Монетизация

**API-as-a-Service (pay-per-use)**
- Первые **1 000 транзакций/месяц бесплатно на клиента** (не на кассу) — поддержка микробизнеса, нулевой барьер входа
- Сверх лимита: **€0.01 за транзакцию** (фиксированная ставка, вне зависимости от суммы чека)
- Аналогия: Stripe берёт ~1.4% + €0.25; Beeloy — фиксированная цена за фискальную операцию
- При 5 000 транзакций/день = (5 000 × 30 − 1 000 free) × €0.01 = **≈ €1 490/месяц** с клиента

**SaaS подписка (для платформенных партнёров)**
- Базовый доступ к API: **€29 – €99/месяц** за локацию (кассу)
- Включает: мониторинг, audit trail, export, webhooks, поддержка

**Professional Services**
- Интеграция под ключ для крупных ERP/retail: **€5 000 – €30 000**
- Custom country profiles
- Белая этикетка (white-label) для национальных дистрибьюторов кассовых систем

**Hardware / Edge Agent**
- Продажа или аренда ESP32-S3 edge-агентов: **€80 – €150** единица
- Recurring: прошивка, мониторинг, OTA обновления — **€5/месяц**

### 9.2 Каналы продаж

1. **Direct (POS/ERP вендоры)** — B2B продажа API-доступа разработчикам
2. **National distributors** — white-label платформы для фискальных дистрибьюторов в странах ЕС
3. **Device manufacturers** — партнёрство с Datecs, Daisy и другими производителями
4. **Retail chains** — прямые Enterprise-контракты для сетей с несколькими локациями

### 9.3 Unit Economics (базовый сценарий)

```
Целевой клиент: retail-сеть, 50 локаций × 2 кассы
Транзакций в день: 200 per касса × 100 касс = 20 000/день

Free tier: 1 000 транз/мес на клиента (не на кассу)
Платные: (20 000/день × 30) − 1 000 = 599 000 транз/мес
Pay-per-use: 599 000 × €0.01 = €5 990/мес ≈ €71 880/год

SaaS (€49/касса/мес): 100 × €49 × 12 = €58 800/год

Клиент выбирает тариф. При pay-per-use:
Gross margin: ~70% = €50 316/год с клиента
Микробизнес (≤1 000 транз/мес): €0 — рост аудитории без CAC
CAC payback крупного клиента: 3–5 месяцев
```

---

## 10. Рыночные перспективы в ЕС

### 10.1 Размер рынка

**TAM (Total Addressable Market):**
- В ЕС зарегистрировано **~28 миллионов предприятий** (Eurostat 2024)
- Из них ~**6 миллионов** обязаны использовать фискальные кассы
- При ARPU €600/год (SaaS per локация) → TAM = **€3.6 млрд/год**

**SAM (Serviceable Addressable Market):**
- Рынки с обязательной фискализацией и цифровой отчётностью: BG, PL, IT, HU, SK, SI, HR, RO
- Совокупно ~**1.8 миллиона** активных фискальных устройств в этих странах
- SAM = **€1.08 млрд/год**

**SOM (Serviceable Obtainable Market, 3 года):**
- Целевой захват 2% рынка SAM через партнёрские каналы
- SOM = **€21.6 млн/год ARR** через 3 года

### 10.2 Регуляторные катализаторы

**Тренд ЕС на обязательную электронную фискализацию:**

| Страна | Статус | Особенность |
|---|---|---|
| 🇧🇬 Болгария | Обязательно с 2001, SUPTO 2024 | Первый MVP-рынок; API уже SUPTO-compliant |
| 🇵🇱 Польша | KSeF с 2025 (B2B), фискальные кассы обязательны | Крупнейший рынок CEE, 40M населения |
| 🇮🇹 Италия | Corrispettivi telematici с 2020 | 5M фискальных устройств; API-интеграция востребована |
| 🇭🇺 Венгрия | NTCA online с 2018 | Обязательна real-time передача данных |
| 🇸🇰 Словакия | Virtuálna registračná pokladnica | Государственная облачная касса |
| 🇸🇮 Словения | FURS с 2016 | API-based с самого начала |
| 🇷🇴 Румыния | e-Factura с 2024 | Быстро растущий рынок |
| 🇭🇷 Хорватия | Fiskalizacija | Переход на EUR в 2023 |

**Ключевой тренд:** каждые 2–3 года в очередной стране ЕС вводится обязательная электронная фискализация. Каждое такое событие — новый рынок для Beeloy.

**EUR унификация:**
- Присоединение к еврозоне (BG 2026, RO 2027 plan) создаёт дополнительный спрос: переход на EUR требует обновления всех фискальных систем.
- Beeloy работает в EUR natively — минимальные изменения при страновой экспансии.

### 10.3 Конкурентный ландшафт

| Игрок | Подход | Слабость |
|---|---|---|
| Локальные дистрибьюторы ФУ | Поставка оборудования + драйвер | Vendor lock-in, нет облака |
| Страновые IT-интеграторы | Custom per-country разработка | Не масштабируется |
| ERP-вендоры (SAP, 1C) | Встроенный модуль | Работает только с их ERP |
| Независимые POS (iiko, r_keeper) | Собственная интеграция | Не открыт как платформа |
| **Beeloy** | **API-first, multi-country, open** | **Первый в классе** |

**Прямых конкурентов в классе "фискальный API-first middleware для ЕС" на момент написания нет.** Ближайшая аналогия — Stripe в 2011 году: рынок платёжных интеграций был таким же фрагментированным.

### 10.4 Почему сейчас

1. **Регуляторное давление растёт:** ЕС движется к обязательной E-invoicing (eIDAS 2, VIDA) — фискальные API станут требованием для участия в B2G и B2B операциях.

2. **Cloud POS волна:** SaaS POS-системы (Lightspeed, Square, Shopify POS) активно входят в Европу, им нужен надёжный фискальный backend — они не хотят строить его самостоятельно.

3. **Post-COVID цифровизация SMB:** малый бизнес в CEE перешёл на cloud-инструменты; старая модель "локальный сервер + USB-кабель" теряет долю.

4. **EUR расширение еврозоны:** Болгария, Румыния, Польша — три крупных рынка на пороге перехода, требующего замены фискального ПО.

---

## 11. Текущий статус разработки

### Статус: `SOFTWARE_COMPLETE_HIL_PENDING`

Весь программный функционал MVP реализован и прошёл автоматизированную регрессию. Оставшиеся gates — исключительно аппаратные и внешние (не software).

### Что реализовано

**Backend (Go):**
- ✅ 92 OpenAPI операции с request/response enforcement
- ✅ PostgreSQL RLS, typed projections, delta CAS
- ✅ Idempotency: cross-replica, durable PENDING, byte-for-byte replay
- ✅ BG_UNP_V1 allocation, EUR policy, tax groups
- ✅ Fiscal sale / storno / shifts / reports / exports / audit
- ✅ OIDC RS256/JWKS, RBAC, CORS, rate limiting
- ✅ Signed webhooks, retry/backoff, endpoint lifecycle
- ✅ Split tender (CASH+CARD), discounts, receipt persistence

**Edge Agent (Go):**
- ✅ SQLite WAL journal, 3-month ACK-gated retention
- ✅ HMAC-signed sync batches/ACK
- ✅ X25519/AES-256-GCM BLE protocol
- ✅ OTA Ed25519-signed manifests, A/B rollback
- ✅ Accelerated 72h soak: zero loss/duplicates

**Mobile/Web:**
- ✅ BeeMiniPOS Web (React/Vite): все операции продаж
- ✅ BeeMiniPOS Expo: iOS/Android/Web, BLE native
- ✅ BeeFiscalApp Expo: iOS/Android/Web, administration
- ✅ Playwright E2E: 85 тестов (miniposweb + BeeMiniPOS)
- ✅ Cross-app pos2fiscal E2E: 25 тестов

**IoT/Protocol:**
- ✅ C++ Daisy: 45 Supported + 16 Optional команд, golden vectors
- ✅ C++ Datecs: 40 core + 12 Optional, golden vectors
- ✅ ESP32-S3: PlatformIO build, ECDSA P-256 identity

**Регрессия:**
- ✅ Go race/vet: все 3 модуля
- ✅ TypeScript строгая проверка
- ✅ 2-Compose E2E: cash/card/split/storno/Z-close/UNKNOWN/restart/backup-restore
- ✅ PostgreSQL интеграция: RLS, rollback, parallel reads
- ✅ 70 fault cases, 55 security cases, 41+ UI acceptance cases

### Что остаётся до PILOT

| Gate | Что нужно | Оценка |
|---|---|---|
| HIL Datecs BlueCash-50 | Физический сценарий CARD+FISCAL на реальном устройстве | 2–4 нед |
| HIL Daisy Compact S | USB/электрика/EUR firmware | 2–4 нед |
| HIL ESP32-S3 | SD power-loss/endurance, LAN soak | 1–2 нед |
| Acquirer | BluePad-50 Plus + acquirer credentials | Внешний |
| НАП/BIM | Подпись authorized-service | Внешний |
| Production signing | Vulnerability scan (zero Critical/High) + Ed25519 release key | 1–2 нед |
| Legal | Юридическое принятие MVP | Внешний |

---

## 12. Дорожная карта

### Q4 2026 — PILOT

- [ ] HIL на реальных Datecs + Daisy устройствах
- [ ] Первые 3–5 PILOT-клиентов (Болгария, SMB retail)
- [ ] NAP/BIM подпись, legal acceptance
- [ ] Production security scan

### Q1–Q2 2027 — Коммерческий запуск BG

- [ ] SaaS billing (pay-per-use + subscription)
- [ ] Partner portal для POS-вендоров
- [ ] White-label SDK
- [ ] 50+ платящих локаций

### Q3–Q4 2027 — Экспансия в ЕС

- [ ] Poland (KSeF profile)
- [ ] Slovenia (FURS profile)
- [ ] Romania (e-Factura profile)
- [ ] 500+ локаций

### 2028 — Масштабирование

- [ ] Italy (Corrispettivi telematici)
- [ ] Hungary (NTCA)
- [ ] Marketplace: готовые интеграции с SAP, 1C, Shopify, WooCommerce
- [ ] 5 000+ локаций, Series A

---

## 13. Технологический стек

### Backend

| Компонент | Технология | Версия |
|---|---|---|
| Fiscal Backend | Go | 1.23+ |
| MiniPOS Backend | Go | 1.23+ |
| Edge Agent | Go | 1.23+ |
| Database (Fiscal) | PostgreSQL | 16.10 |
| Database (MiniPOS) | PostgreSQL | 16.10 |
| Edge Storage | SQLite WAL/FULL | embedded |
| API Gateway | Caddy | 2.x |
| MQTT | EMQX | 5.x |

### Frontend / Mobile

| Компонент | Технология |
|---|---|
| BeeMiniPOS Web | React + Vite + TypeScript |
| BeeMiniPOS App | Expo 53 / React Native |
| BeeFiscalApp | Expo 53 / React Native |
| BLE (Web) | Chrome Web Bluetooth |
| BLE (Native) | react-native-ble-plx |
| Crypto | @noble/hashes + @noble/curves |

### Infrastructure / Security

| Компонент | Технология |
|---|---|
| Container | Docker Compose (2 isolated projects) |
| IoT Firmware | PlatformIO / ESP-IDF, C++ Arduino |
| Signing | Ed25519 (OTA, release), HMAC-SHA256 (webhooks, sync) |
| BLE Crypto | X25519 + HKDF-SHA-256 + AES-256-GCM |
| SBOM | CycloneDX 1.6 |
| Provenance | SLSA Provenance v1 / in-toto |
| Auth | OIDC RS256/JWKS (PROD), HS256 (DEV only) |

### Testing

| Инструмент | Назначение |
|---|---|
| Go testing + race detector | 3 Go-модуля |
| Playwright 1.62 | 110 E2E тестов (85 per-app + 25 cross-app) |
| PostgreSQL integration | RLS, rollback, parallel |
| 2-Compose E2E | Full-stack sale/restart/restore |
| C++ GoogleTest | Protocol driver vectors |
| Accelerated soak (72h) | Edge journal reliability |

---

## Итог для партнёров и инвесторов

**Beeloy Fiscal Platform — первый API-first unified middleware для фискальной интеграции в ЕС.**

Мы решаем задачу, которая стоила индустрии миллиарды euros в виде дорогих интеграций, vendor lock-in и compliance-рисков. Наш подход аналогичен Stripe для платёжного рынка: убрать сложность протоколов за стандартный REST API и зарабатывать на объёме транзакций.

**Ключевые факты:**
- 92 OpenAPI операции, 110 E2E тестов, `SOFTWARE_COMPLETE_HIL_PENDING`
- Единственная платформа с cross-transport idempotency (REST/BLE/Local HTTP)
- SUPTO BG-014 compliant, готов к расширению на 8+ стран ЕС
- Собственный POS (BeeMiniPOS) — демонстрация и референс-клиент одновременно
- TAM €3.6 млрд, SOM €21.6M ARR через 3 года при 2% рынка

**Следующий шаг:** HIL-тестирование на реальных устройствах + первые PILOT-клиенты в Болгарии.

---

*Документ подготовлен: август 2026*  
*Версия платформы: API 2026-08-07, Release profile: BG_MVP_FUNCTIONAL_NONPROD*  
*Контакт: beeloy.org@gmail.com*
