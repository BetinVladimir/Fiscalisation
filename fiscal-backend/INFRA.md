# INFRA

> Сгенерировано по фактически читаемым настройкам и обращениям в исходниках. Значения секретов намеренно не приводятся. При расхождении с кодом источником истины является код.

## Назначение и запуск

- Go-модуль: `fiscalisation/fiscal-backend`.
- Единица развертывания: каталог этого файла; build context следует направлять сюда, если соседний compose/Dockerfile не задаёт иной context.
- Для readiness нужны только отмеченные ниже реальные зависимости; отсутствующие интеграции не следует добавлять в compose.

## Переменные окружения

| Переменная | Тип | Обязательность | Значение по умолчанию | Назначение | Источник |
|---|---|---|---|---|---|
| `ALLOW_STUB_ADAPTERS` | string | условно/нет | `true` | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `API_VERSION` | string | условно/нет | `2026-08-07` | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `APP_ENV` | string | условно/нет | `dev` | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `AUTH_HMAC_KEY` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `BLE_SIGNING_KEY` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `CORS_ALLOWED_ORIGINS` | string | условно/нет | `*` | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `DATABASE_URL` | URL/string | условно/нет | — | DSN основной PostgreSQL. | `internal/config/config.go` |
| `DEVICE_CA_CERT_FILE` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `DEVICE_CA_KEY_FILE` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `DEVICE_MQTT_TLS_URI` | URL/string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `DEVICE_MQTT_WSS_URI` | URL/string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `EMQX_BROKER` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `EMQX_CLIENT_ID` | string | условно/нет | `beefiscal-backend` | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `EMQX_SUB_TOPICS` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `EMQX_TOKEN` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `EMQX_USERNAME` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `HTTP_ADDR` | string | условно/нет | `:8080` | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `INTEGRATION_ENCRYPTION_KEY_BASE64` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `INTEGRATION_SECRET_PEPPER` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `LOCAL_FISCAL_TOKEN_ISSUER` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `LOCAL_FISCAL_TOKEN_PUBLIC_KEY_DER_BASE64` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `LOCAL_FISCAL_TOKEN_SIGNING_KID` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `OIDC_AUDIENCE` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `OIDC_ISSUER` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `OIDC_JWKS_URL` | URL/string | условно/нет | — | URL внешнего или внутреннего сервиса, обозначенного префиксом имени. | `internal/config/config.go` |
| `PUBLIC_BASE_URL` | URL/string | условно/нет | `http://localhost:8080/public/v1` | URL внешнего или внутреннего сервиса, обозначенного префиксом имени. | `internal/config/config.go` |
| `RABBITMQ_URL` | URL/string | условно/нет | — | AMQP(S)-адрес RabbitMQ. | `internal/config/config.go` |
| `RLS_DATABASE_URL` | URL/string | условно/нет | — | URL внешнего или внутреннего сервиса, обозначенного префиксом имени. | `internal/config/config.go` |
| `SIMULATOR_CARD_TERMINAL_AVAILABLE` | string | условно/нет | `false` | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `SMTP_FROM` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `SMTP_HOST` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `SMTP_MAILDOMAIN` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `SMTP_PASSWORD` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `SMTP_PORT` | int/string | условно/нет | `587` | Сетевой порт соответствующего listener/сервиса. | `internal/config/config.go` |
| `SMTP_USER` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `SPA_DEPLOYMENT_DESCRIPTOR_URL` | URL/string | условно/нет | — | URL внешнего или внутреннего сервиса, обозначенного префиксом имени. | `internal/config/config.go` |
| `SPA_DEPLOYMENT_PUBLIC_KEY_DER_BASE64` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `SPA_DEPLOYMENT_SIGNING_KID` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |

## Базы данных

- Подключение: PostgreSQL через `DATABASE_URL` (если переменная присутствует выше). Оно хранит доменные данные сервиса и/или обеспечивает transactional outbox.
- Обнаруженные таблицы/представления (имена без схемы означают зависимость от `search_path`):
- `excluded.payload`
- `excluded.last_sequence`
- `time.Time`
- `integration_resources`
- `fiscal_device_registry`
- `fiscal_manufacturing_stations`
- `fiscal_device_bindings_v2`
- `timestamptz`
- `fiscal_actor_installations`
- `fiscal_device_capabilities`
- `fiscal_device_auth_challenges`
- `fiscal_device_revocations`
- `integration_command_outbox_archive`
- `integration_commands_archive`
- `webhook_delivery_attempts_archive`
- `webhook_deliveries_archive`
- `integration_retention_runs`
- `integration_command_outbox`
- `integration_commands`
- `OF`
- `selected`
- `archived`
- `SKIP`
- `webhook_deliveries`
- `webhook_delivery_attempts`
- `integration_idempotency_replays`
- `fiscal_email_outbox`
- `enrollment_conflict_decisions`
- `external_enrollment_challenges`
- `integration_security_events`
- `external_systems`
- `external_system_credentials`
- `external_system_audit_log`
- `tenant_source_bindings`
- `tenant_integration_credentials`
- `integration_change_journal`
- `ON`
- `BEFORE`
- `OR`
- `tenant_user_memberships`
- `app_auth_challenges`
- `app_auth_sessions`
- `app_issued_tokens`
- Обнаруженные вызываемые функции:
Не обнаружено в коде проекта.
- Собственные миграции: да; применить до старта приложения в порядке имён файлов.
- `internal/persistence/migrations/20260817_integration_resources.sql`
- `internal/persistence/migrations/20260824_beeloy_external_system.sql`
- `internal/persistence/migrations/20260812_device_registry.sql`
- `internal/persistence/migrations/20260822_integration_retention.sql`
- `internal/persistence/migrations/20260823_retention_expiry_cutoff.sql`
- `internal/persistence/migrations/20260819_delivery_hardening.sql`
- `internal/persistence/migrations/20260820_conflict_review.sql`
- `internal/persistence/migrations/20260816_server2server.sql`
- `internal/persistence/migrations/20260821_production_readiness.sql`
- `internal/persistence/migrations/20260818_integration_hardening.sql`

## RabbitMQ / AMQP

- Роль: publisher и listener.
- Очереди/exchange/routing keys, найденные как литералы:
- `beefiscal.integration`
- `beefiscal.integration.commands`
- `beefiscal.integration.webhooks`
- `webhook.deliver`
- Смысл маршрутов следует из имени домена/события; `.in` — вход задания, `.out` — результат, `.dlq` — необработанные сообщения. Для compose нужны RabbitMQ, vhost/user/password из URL и healthcheck до запуска приложения.

## Redis

Не используется.

## MinIO / S3 / объектное хранилище

Не используется.

## Другие сервисы и сетевые зависимости

- `http://localhost:8080/public/v1`
- `http://json-schema.org/draft-07/schema#\`
- `spiffe://beefiscal/device/`
- `http://schemas.openxmlformats.org/package/2006/content-types`
- `http://schemas.openxmlformats.org/package/2006/relationships`
- `http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument`
- `http://schemas.openxmlformats.org/spreadsheetml/2006/main`
- `http://schemas.openxmlformats.org/officeDocument/2006/relationships`
- `http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet`
- `beefiscal://provision/`
- `https://fiscal.beeloy.com/activate`

Переменные `*_URL`, `*_ENDPOINT`, `*_HOST` выше являются полным перечнем настраиваемых сетевых целей, найденных статическим анализом. В compose используйте DNS-имя сервиса, внутренний порт и healthcheck; `localhost` внутри контейнера означает сам контейнер.

## Минимальный checklist для Compose

- Передать все обязательные переменные и secrets; не использовать небезопасные defaults в production.
- Добавить healthchecks зависимостей и запускать приложение после их готовности.
- Подключить приложение и зависимости к общей внутренней сети; публиковать наружу только HTTP/gRPC listener, который действительно нужен.
- Применить собственные миграции до старта новой версии. Если миграций нет, закрепить совместимую версию внешней схемы.
- Настроить persistent volumes для БД/RabbitMQ/Redis/MinIO там, где потеря данных недопустима.
- Проверить TLS/CA/client certificate переменные и не монтировать ключи с правами шире read-only.
