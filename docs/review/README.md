# BeeFiscal Platform — Комплексный технический аудит

**Дата аудита:** 2026-08-21  
**Дата обновления:** 2026-08-21 (после применения исправлений)  
**Охват:** Backend (Go), Frontend (TypeScript/React), IoT (C++), Инфраструктура (Docker Compose, БД)  
**Методология:** Статический анализ исходного кода всех компонентов платформы

---

## Структура отчёта

| Документ | Содержание |
|----------|------------|
| [security.md](security.md) | Уязвимости безопасности (auth, crypto, injection, transport) |
| [scalability.md](scalability.md) | Проблемы масштабируемости (БД, пагинация, память, соединения) |
| [bugs.md](bugs.md) | Логические ошибки и баги (бизнес-логика, гонки, IoT) |
| [infrastructure.md](infrastructure.md) | DevOps: Docker Compose, healthcheck, БД, CI/CD |

---

## Статус исправлений

| # | Компонент | Severity | Описание | Статус |
|---|-----------|----------|----------|--------|
| S-01 | fiscal-backend `auth/auth.go` | HIGH | Auth middleware bypass при отсутствии провайдера | ✅ Исправлено |
| S-03 | fiscal-backend `api/handler.go` | HIGH | `authorizeSale()` разрешал доступ без tenant claim | ✅ Исправлено |
| S-09 | fiscal-backend `config/config.go` | MEDIUM | CORS `*` разрешался в prod-валидации | ✅ Исправлено |
| S-10 | fiscal-backend `auth/oidc.go` | LOW | Минимальный RSA ключ 1024 бит | ✅ Исправлено → 2048 бит |
| SC-03 | fiscal-backend `api/rate_limit.go` | MEDIUM | Rate limiter map растёт без eviction | ✅ Исправлено |
| B-01 | fiscal-backend `domain/service.go` | HIGH | `newID()` коллизии между инстансами | ✅ Исправлено → UUID v4 |
| S-01 | beeminipos-backend `auth/auth.go` | HIGH | Auth bypass при пустом `AUTH_HMAC_KEY` | ✅ Исправлено |
| S-06 | beeminipos-backend `api/handler.go` | MEDIUM | Авторизация отключена в non-prod окружениях | ✅ Исправлено |
| S-08 | beeminipos-backend `emailauth/service.go` | MEDIUM | SMTP без принудительного TLS | ✅ Исправлено → `tls.Dial` |
| B-06 | miniposweb `src/App.tsx` | HIGH | Матчинг товаров в корзине по имени вместо ID | ✅ Исправлено |
| B-07 | miniposweb `src/App.tsx` | HIGH | `shifts.items[0]` без проверки | ✅ Исправлено |
| SC-06 | miniposweb `src/storage/outbox.ts` | HIGH | IndexedDB открывается на каждой операции | ✅ Исправлено → singleton |
| SC-06 | miniposweb `src/storage/referenceCache.ts` | HIGH | IndexedDB открывается на каждой операции | ✅ Исправлено → singleton |
| SC-07 | miniposweb `src/App.tsx` | MEDIUM | Sync loop без проверки `navigator.onLine` | ✅ Исправлено |
| SC-08 | miniposweb `src/App.tsx` | MEDIUM | Token refresh игнорирует `expires_in` | ✅ Исправлено |
| SC-10 | IoT `daisy/DaisyProtocol.cpp` | LOW | `_writeAll()` offset `uint8_t` переполнение | ✅ Исправлено → `uint16_t` |
| B-12 | IoT `termol/TremolProtocol.cpp` | MEDIUM | `sendPacket()` молча обрезал данные | ✅ Исправлено → возвращает 0 |
| B-13 | IoT `datecs/DatecsPrinter.cpp` | MEDIUM | `registerSale()` не проверял результат `snprintf` | ✅ Исправлено |
| B-15 | IoT `datecs/DatecsPrinter.cpp` + `.h` | LOW | `deviceInfo()` bufLen `uint8_t` — max 255 байт | ✅ Исправлено → `uint16_t` |
| B-14 | miniposweb `src/fiscal/webBleRoute.ts` | LOW | Assembler не отклонял `total > MAX` | ✅ Исправлено |
| I-OPS-01 | compose файлы | HIGH | PostgreSQL без `restart: unless-stopped` | ✅ Исправлено |
| I-SEC-02 | `compose.fiscalisation.prod.yaml` | HIGH | Device CA private key в API контейнере | ✅ Исправлено → Docker secret |
| I-SEC-03 | compose prod overlays + config.go | HIGH | `sslmode=disable` в prod | ✅ Исправлено → `sslmode=require` + валидация |
| I-SEC-04 | compose базовые файлы | HIGH | Dev-секреты без prod overlay | ✅ Исправлено → перенесены в dev overlay |
| S-02 | `fiscal-backend` + `beeminipos-backend` config.go | HIGH | Захардкоженные дефолтные AES/HMAC секреты | ✅ Исправлено → `os.Getenv` без дефолтов |

---

## Оставшиеся проблемы (требуют дополнительной работы)

### Оставлено намеренно

| # | Компонент | Файл | Описание |
|---|-----------|------|----------|
| I-SEC-01 | Инфраструктура | `compose.fiscalisation.prod.yaml` | CORS `*` в prod override — **оставлено по решению команды** |

### Архитектурные — требуют планирования

| # | Компонент | Описание |
|---|-----------|----------|
| SC-01 | fiscal-backend | Listing-эндпоинты загружают весь тенант в память (нет пагинации на уровне БД) |
| SC-02 | fiscal-backend | N+1 запрос при фильтрации `operations()` по `register_id` |
| SC-04 | fiscal-backend | In-memory aggregate не масштабируется горизонтально |
| S-12 | miniposweb | Токены в `localStorage` вместо `HttpOnly` cookies |
| S-14 | BeeMiniPOS | BLE канал без шифрования (`OPEN_MVP` — `X25519_AES_GCM` не активирован) |
| S-15 | BeeFiscalApp | BLE `authenticate()` — crypto provider не реализован |
| I-SEC-06 | Инфраструктура | MQTT plaintext на порту 1883 в internal network |
| I-OPS-03/04/05 | Инфраструктура | EMQX, RabbitMQ, fiscal-backend, beeminipos-backend без healthcheck |
| I-OPS-07 | Инфраструктура | Нет resource limits на контейнеры |

### Низкий приоритет

| # | Описание |
|---|----------|
| S-04 | Path-matching `strings.Contains()` в `Allowed()` — хрупкий |
| S-05 | Content-Disposition header injection (требует sanitization URL-сегмента) |
| S-07 | Внутренние ошибки утекают клиентам через `detail` поле |
| S-11 | DNS rebinding в SSRF-protection transport (disable keep-alives) |
| S-13 | JWT декодируется без подписи на клиенте |
| S-16 | `prepareSessionPublicKey()` возвращает random bytes вместо X25519 |
| S-17 | Детерминированный AES-GCM nonce в BLE AUTH_PROOF |
| B-02 | reversalAllowed() — нет startup check для timezone loading |
| B-03 | Integration credentials без уведомления об истечении |
| B-04 | edge-agent: retryable vs fatal errors не различаются |
| B-05 | BLE signing key не валидируется в non-prod |
| B-08 | Race condition в `adapterToken()` |
| B-09 | Cloud probe считается дважды в RouteController |
| B-11 | Daisy off-by-one в receive buffer (код уже корректен в текущей версии) |
| I-SEC-09 | Go backend контейнеры запускаются как root |
| I-SEC-10 | CSP `connect-src http: https:` разрешает любой origin |
| I-OPS-05 | Дублирующиеся числовые префиксы в init SQL |
| I-OPS-06 | DDL inline при каждом старте — lock contention |
| I-OPS-10 | NOT VALID constraint без VALIDATE |
