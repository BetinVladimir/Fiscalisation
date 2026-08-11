# BeeFiscal / BeeMiniPOS MVP

Исполняемая реализация двух изолированных продуктов: BeeFiscal и автономного BeeMiniPOS. Canonical документация, OpenAPI, roadmap и Н-18 traceability находятся в [`../BeeloyBackend/docs/Fiscal`](../BeeloyBackend/docs/Fiscal/README.md); их контрольные SHA-256 закреплены в [`contracts/CONTRACT_LOCK.md`](contracts/CONTRACT_LOCK.md).

Android-приложения для встроенных smart-касс, включая отдельные Daisy SMART и Datecs BlueCash tracks, описаны в [`SmartDevices/README.md`](SmartDevices/README.md). Интеграция нового POS с BlueCash activation: [`docs/BLUECASH_POS_INTEGRATION.md`](docs/BLUECASH_POS_INTEGRATION.md).

Машиночитаемое соответствие всех 25 этапов roadmap находится в [`contracts/roadmap-stage-acceptance.json`](contracts/roadmap-stage-acceptance.json) и проверяется командой `make roadmap-acceptance-test`. Реестр отличает реализованный software MVP от формально исключённых hardware/vendor/legal этапов и не позволяет выдать ожидающие внешние подписи за `MVP_ACCEPTED`.

Stage 1 governance закреплён в [`contracts/implementation-governance.json`](contracts/implementation-governance.json): два независимых продукта, владельцы всех обязательных модулей, единая нумерация requirements/tests/defects/releases и полный P0 decision register. `make governance-test` проверяет владельцев, evidence paths и обязательный production block для каждого внешнего P0.

Stage 15 product boundary исполняемо проверяется `make boundary-test`: два Compose-проекта сохраняют собственные БД/private networks/Caddy, MiniPOS не импортирует Fiscal private code и не вызывает internal API, а `FISCAL_PUBLIC_BASE_URL` обязан быть точным HTTP(S) base `/public/v1` без credentials, query или fragment. Gate входит в `make regression`.

Уточнённая SUPTO BG-014 remediation: исходная спецификация — [`docs/SUPTO/SUPTO_BG014_REMEDIATION_IMPLEMENTATION_SPEC.md`](docs/SUPTO/SUPTO_BG014_REMEDIATION_IMPLEMENTATION_SPEC.md), machine trace Annex 29 — [`contracts/supto-annex29-trace.json`](contracts/supto-annex29-trace.json), решения — [`docs/decisions`](docs/decisions), а руководство подключения нового POS — [`docs/POS_INTEGRATION_BG_SUPTO.md`](docs/POS_INTEGRATION_BG_SUPTO.md). `make supto-trace-test` проверяет полное покрытие 1–24 и не позволяет скрыть production-blocked gap.

SUPTO software gates доступны отдельными целями `make supto-unp-test`, `country-profile-test`, `regulatory-identifier-binding-test`, `supto-sale-lifecycle-test`, `supto-readiness-test`, `supto-time-test`, `supto-audit-test`, `supto-export-test`, `supto-offline-equivalence-test`, `supto-document-policy-test` и `supto-security-test`. `make supto-full-acceptance` намеренно fail-close, пока trace содержит PARTIAL/EXTERNAL_BLOCKED либо отсутствуют независимо подписанные physical HIL/release/legal artifacts; внешние пакеты проверяются `supto-hil-verify`, `supto-release-verify`, `supto-legal-verify` с `EVIDENCE_DIR`.

## Проверка baseline

```bash
make regression
make compose-e2e
make full-regression
./scripts/verify-contract-lock.sh
make evidence-test
```

`make regression` запускает Go race tests и `go vet` трёх модулей, TypeScript-проверку двух Expo UI, Playwright Web interaction E2E, C++ protocol tests, CycloneDX/release-evidence gate и render всех четырёх Compose-конфигураций. Для каждого модуля используется отдельный воспроизводимый cache в `/tmp`, поэтому gate не зависит от пользовательского Go build cache. `make compose-e2e` поднимает два независимых Compose-проекта и проверяет cash/card/split/reversal, сохранение исходной и storno fiscal references, restart recovery, backup/restore и автономность базы MiniPOS; `make full-regression` выполняет оба набора.

`make generate-openapi` пересоздаёт versioned TypeScript surface из locked canonical и runtime OpenAPI. Обычный `make contract-test` не изменяет tree: он генерирует контрольную копию во временном каталоге, проверяет byte-for-byte drift, наличие всех 106 `operationId`, 106 runtime request contracts (body + path/query/header parameters), 106 successful response contracts, canonical Problem Details и компиляцию типизированных client factories. Runtime middleware валидирует запрос до business handler, а недокументированный 2xx/3xx или некорректный 4xx/5xx fail-close как contract violation.

Release evidence по умолчанию намеренно создаётся как `UNSIGNED_NO_GO`. Для подписанного пакета передайте путь к закрытому Ed25519 PKCS#8 PEM через `RELEASE_SIGNING_PRIVATE_KEY`; проверяющая сторона должна независимо передать доверенный SPKI public key через `RELEASE_TRUSTED_PUBLIC_KEY`. Вложенный в пакет public key не считается источником доверия. Генератор также выпускает подписанно-привязанный in-toto/SLSA provenance и может принять точный SBOM-bound scan через `RELEASE_VULNERABILITY_REPORT`; формат и воспроизводимый двухшаговый процесс описаны в [`docs/release-evidence.md`](docs/release-evidence.md). Даже корректная подпись и zero-Critical/High scan не снимают hardware/vendor и legal `PROD_NO_GO`.

```bash
RELEASE_SIGNING_PRIVATE_KEY=/secure/release-private.pem ruby scripts/generate_release_evidence.rb artifacts/evidence/rc
RELEASE_TRUSTED_PUBLIC_KEY=/trusted/release-public.pem ruby scripts/verify_release_evidence.rb artifacts/evidence/rc
```

## Запуск DEV

```bash
cp .env.example .env
docker compose -p beefiscal-dev -f compose.fiscalisation.yaml -f compose.fiscalisation.dev.yaml up --build
docker compose -p beeminipos-dev -f compose.minipos.yaml -f compose.minipos.dev.yaml up --build
```

Fiscal и MiniPOS имеют отдельные PostgreSQL, сети, Caddy и lifecycle. MiniPOS обращается к Fiscal только через `FISCAL_PUBLIC_BASE_URL` и публичный API. `.env.example` является запускаемым DEV-профилем и проверяется `make compose-check`; его значения нельзя переносить в PROD. Для PROD обязательны `OIDC_ISSUER`, `OIDC_AUDIENCE`, `OIDC_JWKS_URL`, `FISCAL_OAUTH_TOKEN_URL`, client credentials, 32+ byte `BLE_SIGNING_KEY`/webhook keys, DB passwords, HTTPS public/Caddy URLs и явные HTTPS CORS origins; HMAC JWT, статический `FISCAL_AUTH_TOKEN`, wildcard CORS, HTTP upstream, simulator и STUB запрещены конфигурационными guards.

Текущие доказательства перечислены в [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md), строгий gap-аудит — в [`MVP_COMPLETION_AUDIT.md`](MVP_COMPLETION_AUDIT.md), unresolved contract P0 и запрещённые обходы — в [`MVP_BLOCKERS.md`](MVP_BLOCKERS.md), exact версии — в [`TOOLCHAIN.md`](TOOLCHAIN.md), hardware/vendor/legal ограничения — в [`MVP_GATES.md`](MVP_GATES.md).

Stage-25 handover: [`RELEASE_NOTES.md`](RELEASE_NOTES.md), [`contracts/mvp-acceptance-v1.json`](contracts/mvp-acceptance-v1.json), [`docs/operations-runbook.md`](docs/operations-runbook.md), [`docs/rollback-plan.md`](docs/rollback-plan.md) и [`docs/support-guide.md`](docs/support-guide.md). `make handover-test` подтверждает base software PASS, но намеренно запрещает локально подделывать human signatures или повышать `PILOT/PROD NO-GO` без внешнего evidence.

---

# Historical repository overview

Universal IoT Middleware Platform for Fiscal Devices.

European standard layer for fiscal device integration with cloud ERP.

## Vision

Миссия: превратить сложную интеграцию фискальных устройств в простой cloud API.

Позиционирование: Stripe-like infrastructure for fiscal transactions.

Ключевая ценность:
- единый REST API вместо проприетарных драйверов
- pay-per-use модель
- независимость от конкретного оборудования

## Структура

```text
.
├── IoT/
│   ├── common-modules/
│   ├── firmware/
│   └── protocol-abstraction/
├── SmartDevices/
│   ├── apps/
│   └── shared-sdk/
├── edge-agent/
├── cloud-core/
│   ├── mqtt-router/
│   ├── transaction-engine/
│   ├── audit-compliance/
│   ├── security-layer/
│   └── erp-api/
├── fiscal-backend/
├── database/
│   └── dbmigrate/
├── BeeMiniPOS/
├── BeeFiscalApp/
├── beeminipos-backend/
└── docs/
```

## Компоненты

- `IoT` — PlatformIO/Arduino C++-проекты: общие модули, конечные firmware и `IoT/protocol-abstraction`.
- `SmartDevices` — Android-приложения под разные кассовые аппараты.
- `protocol-abstraction` — часть `IoT` (`IoT/protocol-abstraction`): C++ Arduino-слой унификации протоколов (Epson/Datecs/Tremol...) с выбором целевого протокола при сборке прошивки через конфигурацию препроцессора.
- `edge-agent` — локальный слой подключения, буферизации и offline-first синхронизации.
- `cloud-core` — облачное event-driven ядро платформы.
- `fiscal-backend` — Go backend для фискального контура.
- `database/dbmigrate` — PostgreSQL-миграции.
- `BeeMiniPOS` — Expo React Native приложение.
- `BeeFiscalApp` — Expo React Native приложение для мониторинга и администрирования тенанта.
- `beeminipos-backend` — Go backend для BeeMiniPOS.

## Problem Statement

Текущие проблемы рынка (BG/ЕС):
- устройства зависят от USB/UART и proprietary протоколов
- требуется локальная инфраструктура и интеграторы
- высокая стоимость внедрения, долгий rollout, vendor lock-in

Эта платформа устраняет ограничения через универсальный middleware-слой и API-first модель.

## High-Level Architecture

```text
[Fiscal Device]
	|
[Edge Agent / Gateway]
	|
[IoT Middleware Platform]
	|
[REST API / Webhooks]
	|
[Cloud ERP]
```

## Core System Layers

1. Universal Protocol Abstraction Layer
- автоопределение устройства
- адаптация vendor protocol
- нормализация команд и ответов

2. Edge Agent
- USB/UART bridge
- offline queue
- secure tunnel to cloud

3. IoT Middleware Cloud Core
- MQTT communication
- routing and device orchestration
- retry and reconciliation

4. Security Layer
- TLS/mTLS
- device identity and auth policies

5. Transaction Engine
- idempotency keys
- logically exactly-once processing

6. Audit & Compliance Layer
- immutable logs
- hash chaining
- signed transaction records

7. ERP API Layer
- REST API
- webhooks
- tenant/device operations

## Transaction Flow

1. ERP отправляет транзакцию в API.
2. Middleware валидирует и обогащает запрос.
3. Команда маршрутизируется на Edge Agent.
4. Edge передает команду на устройство.
5. Устройство выполняет операцию.
6. Ответ возвращается в cloud core.
7. Событие фиксируется в audit/compliance журналах.
8. ERP получает результат и статус.

## Offline-first

- Транзакции буферизуются локально на edge при потере связи.
- Каждой транзакции присваивается уникальный ID.
- После восстановления сети выполняются replay и reconciliation.

## Compliance Focus (НАП и ЕС)

- append-only fiscal journal
- edge + cloud synchronization
- WORM/immutable storage patterns
- подготовка к этапам пилот -> pre-cert -> certification

## Target KPI

- latency: <= 800 ms
- uptime: >= 99.5%
- delivery success: >= 99.9%
- device coverage: >= 75%

## Быстрый старт

### Go backend сервисы

```bash
cd fiscal-backend && go run ./cmd/fiscal-backend
cd beeminipos-backend && go run ./cmd/beeminipos-backend
```

### Expo приложение

```bash
cd BeeMiniPOS
npm install
npm run start

cd BeeFiscalApp
npm install
npm run start
```

### Локальная инфраструктура

```bash
docker compose up --build
```

## EMQX JWT Токены

В `docker-compose.yml` EMQX настроен на JWT-аутентификацию:
- токен передается в MQTT `password`
- claim `acl` определяет доступные топики

Пример payload токена:

```json
{
	"username": "device-001",
	"exp": 1924992000,
	"acl": [
		{
			"permission": "allow",
			"action": "publish",
			"topic": "devices/device-001/telemetry"
		},
		{
			"permission": "allow",
			"action": "subscribe",
			"topic": "devices/device-001/commands/#"
		}
	]
}
```

Секрет подписи JWT сейчас задан как `change_me_super_secret` в [docker-compose.yml](docker-compose.yml) и должен быть заменен на безопасный секрет перед использованием.

### Переменные окружения

Скопируйте [.env.example](.env.example) в `.env` и задайте значения:

```bash
cp .env.example .env
```

Используются переменные:
- `EMQX_JWT_SECRET`
- `EMQX_DASHBOARD_USERNAME`
- `EMQX_DASHBOARD_PASSWORD`

### Генерация токена

В репозитории добавлен скрипт [scripts/emqx/generate_jwt.py](scripts/emqx/generate_jwt.py):

```bash
python scripts/emqx/generate_jwt.py \
	--secret "$EMQX_JWT_SECRET" \
	--username device-001 \
	--pub devices/device-001/telemetry \
	--sub devices/device-001/commands/#
```

### Пример MQTT-клиента

Через `mosquitto_pub`/`mosquitto_sub`:

```bash
# 1) Подписка (в одном терминале)
mosquitto_sub -h localhost -p 1883 \
	-u device-001 \
	-P "<JWT_TOKEN>" \
	-t devices/device-001/commands/# -d

# 2) Публикация (в другом терминале)
mosquitto_pub -h localhost -p 1883 \
	-u device-001 \
	-P "<JWT_TOKEN>" \
	-t devices/device-001/telemetry \
	-m '{"temp":23.4}' -d
```

Если токен не содержит разрешение на topic/action, EMQX отклонит операцию.
