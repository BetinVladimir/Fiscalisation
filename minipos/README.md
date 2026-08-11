# BeeMiniPOS — развёртывание и разработка

Этот каталог содержит оба компонента автономной POS-системы BeeMiniPOS:

| Каталог | Технология | Назначение |
|---|---|---|
| [`BeeMiniPOS/`](BeeMiniPOS/) | Expo / React Native | Touch POS — Android, iOS, Web |
| [`beeminipos-backend/`](beeminipos-backend/) | Go | Backend MiniPOS, автономная PostgreSQL |

Canonical документация, OpenAPI, roadmap и Н-18 трейсability — в
[`../BeeloyBackend/docs/Fiscal/beeminipos-mvp-reference.md`](../../BeeloyBackend/docs/Fiscal/beeminipos-mvp-reference.md).

---

## Архитектурная граница

MiniPOS — полностью автономная система. Запрещено:

- прямое подключение `beeminipos-backend` к EMQX/MQTT, Redis, RabbitMQ или PostgreSQL Fiscal Platform;
- импорт внутренних Go packages `fiscal-backend/`;
- обращение к любым endpoints кроме публичного `FISCAL_PUBLIC_BASE_URL/public/v1`.

---

## Переменные окружения

### Обязательные для всех окружений

| Переменная | Описание |
|---|---|
| `MINIPOS_DB_PASSWORD` | Пароль PostgreSQL (writer) |
| `MINIPOS_RLS_DB_PASSWORD` | Пароль PostgreSQL RLS reader |
| `FISCAL_PUBLIC_BASE_URL` | Базовый URL Fiscal API, точно `https://<host>/public/v1` |
| `WEBHOOK_VERIFICATION_KEY` | HMAC-ключ для проверки входящих webhook от Fiscal |

### Только для PROD

| Переменная | Описание |
|---|---|
| `OIDC_ISSUER` | Issuer OIDC (HTTPS) |
| `OIDC_AUDIENCE` | Audience токена |
| `OIDC_JWKS_URL` | URL JWKS endpoint (HTTPS) |
| `FISCAL_OAUTH_TOKEN_URL` | OAuth 2.0 token endpoint для MiniPOS→Fiscal (HTTPS) |
| `FISCAL_OAUTH_CLIENT_ID` | Client ID |
| `FISCAL_OAUTH_CLIENT_SECRET` | Client Secret |
| `FISCAL_OAUTH_SCOPE` | Scope (по умолчанию `fiscal.base`) |
| `MINIPOS_SITE` | Публичный HTTPS URL, через который работает Caddy |
| `MINIPOS_CORS_ALLOWED_ORIGINS` | Разрешённые CORS origins |

> **DEV-запрещено в PROD:** `AUTH_HMAC_KEY`, `FISCAL_AUTH_TOKEN`, `MINIPOS_SITE=:80`,
> wildcard CORS. Конфигурационные guards проверяются при старте.

---

## Запуск DEV

```bash
# из корня репозитория
cp .env.example .env
docker compose -p beeminipos-dev \
  -f compose.minipos.yaml \
  -f compose.minipos.dev.yaml \
  up --build
```

Сервисы после старта:

| Сервис | Адрес |
|---|---|
| MiniPOS API (через Caddy) | `http://localhost:8081/public/v1/minipos/` |
| Caddy HTTP (DEV) | `localhost:8081` |

---

## Запуск PROD

```bash
docker compose -p beeminipos-prod \
  -f compose.minipos.yaml \
  -f compose.minipos.prod.yaml \
  --env-file .env.prod \
  up -d --build
```

PROD требует HTTPS Caddy (`MINIPOS_SITE=https://pos.example.com`), внешний OIDC,
OAuth 2.0 client credentials и принудительно запрещает все DEV-значения.

---

## Разработка backend

```bash
cd beeminipos-backend

# запустить тесты
GOCACHE=/tmp/beeminipos-go-cache go test ./...

# тесты с race detector
GOCACHE=/tmp/beeminipos-go-cache go test -race ./...

# vet
GOCACHE=/tmp/beeminipos-go-vet-cache go vet ./...

# запустить локально
go run ./cmd/beeminipos-backend
```

Для PostgreSQL integration tests:
```bash
# из корня репозитория
make postgres-integration
```

---

## Разработка UI (BeeMiniPOS)

```bash
cd BeeMiniPOS

npm ci

# Web dev сервер
EXPO_PUBLIC_APP_ENV=dev \
EXPO_PUBLIC_MINIPOS_API_URL=http://localhost:8081/public/v1/minipos \
EXPO_PUBLIC_FISCAL_API_URL=http://localhost:8080/public/v1 \
npx expo start --web

# TypeScript check
npx tsc --noEmit

# Unit tests
npm test
```

### Сборка Android

```bash
cd BeeMiniPOS/android
./gradlew assembleDebug --no-daemon
# APK: BeeMiniPOS/android/app/build/outputs/apk/debug/app-debug.apk
```

### Сборка iOS (Simulator)

```bash
cd BeeMiniPOS/ios
pod install
xcodebuild \
  -workspace BeeMiniPOS.xcworkspace \
  -scheme BeeMiniPOS \
  -configuration Debug \
  -sdk iphonesimulator \
  -destination 'generic/platform=iOS Simulator' \
  CODE_SIGNING_ALLOWED=NO build
```

### Web export

```bash
cd BeeMiniPOS
EXPO_PUBLIC_APP_ENV=dev npx expo export --platform web --output-dir dist-web
```

---

## BLE (Bluetooth Low Energy)

BLE-сессия выпускается Fiscal Backend через REST. Клиент BeeMiniPOS получает
short-lived ticket, устанавливает зашифрованный GATT-канал через Edge и
выполняет фискальную команду локально при недоступности cloud.

Canonical контракт: [`../contracts/`](../contracts/) и
[BLE GATT spec](../../BeeloyBackend/docs/Fiscal/ble/ble-gatt-v1.md).

Платформенная поддержка:

| Платформа | Статус |
|---|---|
| Android (native) | Production path |
| iOS (native) | Foreground-only, production path |
| Web Chrome/Chromium | HTTPS + user gesture, REST fallback если BLE недоступен |
| Safari / iOS Web | Не поддерживается |

---

## Regression suite (из корня)

```bash
# Только MiniPOS-связанные gates
make typecheck       # TypeScript + unit tests
make boundary-test   # изоляция продуктов и сетей
make contract-test   # API surface + OpenAPI drift

# Полная регрессия включает MiniPOS
make regression
make full-regression  # + PostgreSQL integration + compose E2E
```

---

## Backup и восстановление (pilot drill)

```bash
# Создать backup из работающего контура
docker exec beeminipos-dev-postgres-1 \
  pg_dump -U minipos --format=custom minipos > minipos-backup.dump

# Восстановить в новой БД
docker exec -i beeminipos-dev-postgres-1 \
  pg_restore -U minipos -d minipos_restored minipos-backup.dump
```

Целевой RTO pilot: < 120 секунд (проверяется автоматически в `make compose-e2e`).

---

## Ограничения (MVP Gates)

| Gate | Статус |
|---|---|
| BlueCash-50 fiscal/payment integration | `EXCLUDED_FROM_PROD` — требует vendor/acquirer approval |
| BluePad-50 Plus pairing | `UNSUPPORTED` — до письменного разрешения vendor/acquirer |
| НАП/legal certification | Release blocker — только физическая сертификация |

Simulator и STUB **технически запрещены** в PROD-конфигурации.

Полный реестр ограничений: [`../MVP_GATES.md`](../MVP_GATES.md).
