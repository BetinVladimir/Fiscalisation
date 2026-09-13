# INFRA

> Сгенерировано по фактически читаемым настройкам и обращениям в исходниках. Значения секретов намеренно не приводятся. При расхождении с кодом источником истины является код.

## Назначение и запуск

- Go-модуль: `fiscalisation/beeminipos-backend`.
- Единица развертывания: каталог этого файла; build context следует направлять сюда, если соседний compose/Dockerfile не задаёт иной context.
- Для readiness нужны только отмеченные ниже реальные зависимости; отсутствующие интеграции не следует добавлять в compose.

## Переменные окружения

| Переменная | Тип | Обязательность | Значение по умолчанию | Назначение | Источник |
|---|---|---|---|---|---|
| `AUTH_HMAC_KEY` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `DATABASE_URL` | URL/string | условно/нет | — | DSN основной PostgreSQL. | `internal/config/config.go` |
| `EMQX_BROKER` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `EMQX_SUB_TOPICS` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `EMQX_TOKEN` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `EMQX_USERNAME` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `FISCAL_AUTH_TOKEN` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `FISCAL_CREDENTIAL_ENCRYPTION_KEY_BASE64` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `FISCAL_DATABASE_URL` | URL/string | условно/нет | — | URL внешнего или внутреннего сервиса, обозначенного префиксом имени. | `internal/config/config.go` |
| `FISCAL_OAUTH_AUDIENCE` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `FISCAL_OAUTH_CLIENT_ID` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `FISCAL_OAUTH_CLIENT_SECRET` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `FISCAL_OAUTH_TOKEN_URL` | URL/string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `FISCAL_SYSTEM_TOKEN` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `LOCAL_FISCAL_TOKEN_SIGNING_KEY_PEM` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `RLS_DATABASE_URL` | URL/string | условно/нет | — | URL внешнего или внутреннего сервиса, обозначенного префиксом имени. | `internal/config/config.go` |
| `SMTP_ALLOW_PLAINTEXT` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `SMTP_FROM` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `SMTP_HOST` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `SMTP_PASSWORD` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |
| `SMTP_USER` | string | условно/нет | — | Настройка соответствующего компонента; точка чтения указана в колонке «Источник». | `internal/config/config.go` |
| `WEBHOOK_VERIFICATION_KEY` | string | условно/нет | — | Секрет/учётные данные; передавать через secret, не хранить в image или Git. | `internal/config/config.go` |

## Базы данных

- Подключение: PostgreSQL через `DATABASE_URL` (если переменная присутствует выше). Оно хранит доменные данные сервиса и/или обеспечивает transactional outbox.
- Обнаруженные таблицы/представления (имена без схемы означают зависимость от `search_path`):
- `s.smtpPlaintext`
- `excluded.payload`
- `minipos_email_outbox`
- `company_fiscal_bindings`
- `company_fiscal_credentials`
- `company_fiscal_resource_links`
- `minipos_fiscal_enrollment_sessions`
- `organizations`
- `minipos_auth_challenges`
- `minipos_auth_onboarding`
- `minipos_auth_accounts`
- `minipos_auth_refresh_tokens`
- `company_fiscal_binding_history`
- Обнаруженные вызываемые функции:
Не обнаружено в коде проекта.
- Собственные миграции: да; применить до старта приложения в порядке имён файлов.
- `internal/migrations/sql/016_email_outbox.sql`
- `internal/migrations/sql/015_fiscal_integration.sql`
- `internal/migrations/sql/014_email_auth.sql`
- `internal/migrations/sql/017_fiscal_binding_hardening.sql`

## RabbitMQ / AMQP

Не используется.

## Redis

Не используется.

## MinIO / S3 / объектное хранилище

Не используется.

## Другие сервисы и сетевые зависимости

- `http://localhost:8080/public/v1`

Переменные `*_URL`, `*_ENDPOINT`, `*_HOST` выше являются полным перечнем настраиваемых сетевых целей, найденных статическим анализом. В compose используйте DNS-имя сервиса, внутренний порт и healthcheck; `localhost` внутри контейнера означает сам контейнер.

## Минимальный checklist для Compose

- Передать все обязательные переменные и secrets; не использовать небезопасные defaults в production.
- Добавить healthchecks зависимостей и запускать приложение после их готовности.
- Подключить приложение и зависимости к общей внутренней сети; публиковать наружу только HTTP/gRPC listener, который действительно нужен.
- Применить собственные миграции до старта новой версии. Если миграций нет, закрепить совместимую версию внешней схемы.
- Настроить persistent volumes для БД/RabbitMQ/Redis/MinIO там, где потеря данных недопустима.
- Проверить TLS/CA/client certificate переменные и не монтировать ключи с правами шире read-only.
