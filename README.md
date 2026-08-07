# Fiscalisation Monorepo

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
