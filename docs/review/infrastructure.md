# Инфраструктура и DevOps

**Дата:** 2026-08-21  
**Охват:** Docker Compose (все окружения), Dockerfiles, Caddyfile, SQL миграции, `postgres.go`, `.env.example`, Makefile

---

## Безопасность

---

### I-SEC-01 — [HIGH] CORS `*` wildcard в production compose

**Файлы:**
- `compose.fiscalisation.prod.yaml`, строка 19: `CORS_ALLOWED_ORIGINS: "*"`
- `compose.minipos.prod.yaml`, строка 17: `CORS_ALLOWED_ORIGINS: "*"`

**Описание:**  
Оба production override захардкодят `*` для CORS. Это позволяет любому origin — включая контролируемые атакующим сайты — делать credentialed cross-origin запросы к фискальным и POS API. Для финансовой/налоговой системы это критическая мискonfiguration.

**Рекомендация:**  
Заменить на явный allowlist. `Makefile` в `compose-check` target уже использует `FISCAL_CORS_ALLOWED_ORIGINS=https://admin.example.test,https://pos.example.test`, доказывая что приложение поддерживает это. Задать в prod compose:
```yaml
CORS_ALLOWED_ORIGINS: "${FISCAL_CORS_ALLOWED_ORIGINS:?required}"
```

---

### I-SEC-02 — [HIGH] Device CA приватный ключ смонтирован в API-контейнер

**Файл:** `compose.fiscalisation.yaml`, строка 59; `compose.fiscalisation.prod.yaml`, строки 39–40

**Описание:**  
`device-ca.key` смонтирован в fiscal-backend приложение:
```yaml
volumes: ["${DEVICE_PKI_DIR:-./deploy/device-pki}:/run/beefiscal/device-pki:ro"]
DEVICE_CA_KEY_FILE: "/run/beefiscal/device-pki/device-ca.key"
```
Любая RCE, path-traversal или SSRF уязвимость в fiscal-backend даёт атакующему полномочия выдавать сертификаты для произвольных устройств и имитировать фискальное оборудование.

**Рекомендация:**  
Вынести CA-подпись в выделенный signing sidecar или использовать аппаратное хранилище секретов (Vault, AWS KMS, GCP KMS). Ключ никогда не должен находиться в том же контейнере/процессе что и API-сервер.

---

### I-SEC-03 — [HIGH] `sslmode=disable` на всех соединениях с БД

**Файлы:** `compose.fiscalisation.yaml` строка 57; `compose.minipos.yaml` строка 21

**Описание:**
```
DATABASE_URL: "postgres://fiscal:...@postgres:5432/fiscal?sslmode=disable"
```
Все соединения с БД — без TLS. При изменении топологии сети (внешняя managed БД, read replica), пароли и данные фискальных транзакций передаются в открытом виде. Production override не изменяет это.

**Рекомендация:**  
Изменить на `sslmode=require` как минимум, `sslmode=verify-full` в production. PostgreSQL 16 поддерживает TLS по умолчанию.

---

### I-SEC-04 — [HIGH] Dev-секреты с известными значениями активны если prod overlay не подключён

**Файл:** `compose.fiscalisation.yaml` строка 32

**Описание:**
```yaml
EMQX_AUTHENTICATION__1__SECRET: "${EMQX_JWT_SECRET:-dev-only-emqx-secret}"
```
Если `EMQX_JWT_SECRET` не задан, брокер принимает JWT подписанные публично известной строкой `dev-only-emqx-secret`. Prod override корректно делает это `{:?required}`, но только если prod файл явно подключён. Деплой только с `compose.fiscalisation.yaml` молча использует dev-секрет.

Аналогичная проблема для `AUTH_HMAC_KEY`, `BLE_SIGNING_KEY`, `INTEGRATION_ENCRYPTION_KEY_BASE64`, `RABBITMQ_DEFAULT_PASS`.

**Рекомендация:**  
Убрать все literal дефолты из base compose. Использовать пустые дефолты (`${EMQX_JWT_SECRET:-}`) чтобы приложение падало при запуске если секрет не задан и prod overlay не подключён.

---

### I-SEC-05 — [MEDIUM] Dev SMTP по умолчанию на plaintext порту 25

**Файл:** `compose.minipos.dev.yaml` строки 13–14

**Описание:**  
```yaml
SMTP_HOST: "${MINIPOS_SMTP_HOST:-host.docker.internal}"
SMTP_PORT: "${MINIPOS_SMTP_PORT:-25}"
```
Unencrypted, unauthenticated SMTP. Любой процесс на машине разработчика может перехватить или подделать письма. Если разработчик подключает реального тестового тенанта к dev-стеку, в этих письмах могут быть operator codes и регистрационные данные.

**Рекомендация:**  
Использовать local SMTP sink (например Mailpit на порту 1025) вместо хостового порта 25.

---

### I-SEC-06 — [MEDIUM] MQTT plaintext порт 1883 открыт во внутренней сети

**Файл:** `compose.fiscalisation.yaml` строки 36–40

**Описание:**  
EMQX container открывает plaintext MQTT (1883) и plaintext WebSocket (8083) в `private` network. Любой скомпрометированный контейнер в той же сети может подписаться на wildcard topics и перехватывать device telemetry без TLS.

**Рекомендация:**  
В production override отключить plaintext listeners:
```yaml
EMQX_LISTENERS__TCP__DEFAULT__ENABLE: "false"
EMQX_LISTENERS__WS__DEFAULT__ENABLE: "false"
```
Использовать только TLS (8883/8084).

---

### I-SEC-07 — [MEDIUM] EMQX dashboard доступен всем контейнерам в private network

**Файл:** `compose.fiscalisation.yaml` строка 40

**Описание:**  
Порт 18083 (EMQX web dashboard) привязан к `127.0.0.1` на хосте, но также доступен всем контейнерам в `private` сети (`emqx:18083`). Dashboard предоставляет полное администрирование брокером включая создание/удаление auth правил.

**Рекомендация:**  
В production отключить dashboard: `EMQX_DASHBOARD__ENABLE: "false"`. Или выделить EMQX в отдельную изолированную сеть.

---

### I-SEC-08 — [MEDIUM] RabbitMQ в ingress network без необходимости

**Файл:** `compose.fiscalisation.yaml` строка 53

**Описание:**  
RabbitMQ подключён к `private` и `ingress` сетям. Только fiscal-backend потребляет его — и он находится только в `private`. Подключение к `ingress` расширяет blast radius при компрометации Caddy.

**Рекомендация:**  
Убрать `ingress` из networks RabbitMQ. Redis аналогично подключён к обеим сетям без необходимости.

---

### I-SEC-09 — [MEDIUM] Go backend контейнеры запускаются как root

**Файлы:**
- `fiscal-backend/Dockerfile`
- `minipos/beeminipos-backend/Dockerfile`

**Описание:**  
Используется `gcr.io/distroless/static-debian12` без `nonroot` — запускается как uid 0 (root). `edge-agent/Dockerfile` корректно использует `gcr.io/distroless/static-debian12:nonroot`.

**Рекомендация:**  
Изменить на `FROM gcr.io/distroless/static-debian12:nonroot` в обоих Dockerfile. Для Caddy: использовать `caddy:2.10.0-unprivileged` или `USER caddy`.

---

### I-SEC-10 — [MEDIUM] CSP `connect-src http: https:` позволяет эксфильтрацию на любой origin

**Файл:** `minipos/miniposweb/Caddyfile` строка 10

**Описание:**
```
Content-Security-Policy "default-src 'self'; connect-src 'self' http: https:; ..."
```
`connect-src 'self' http: https:` позволяет SPA делать fetch/XHR к любому HTTP/HTTPS origin. При XSS-атаке CSP не обеспечивает containment — данные могут эксфильтрироваться на любой endpoint.

**Рекомендация:**  
Ограничить `connect-src` конкретными API origins: `'self' ${MINIPOS_API_ORIGIN} ${FISCAL_API_ORIGIN}`.

---

### I-SEC-11 — [MEDIUM] Fiscal Caddyfile отсутствуют HSTS и X-Frame-Options

**Файл:** `deploy/fiscalisation/Caddyfile`

**Описание:**  
Только `X-Content-Type-Options` и `Referrer-Policy`. Отсутствуют:
- `Strict-Transport-Security` — критично при TLS
- `X-Frame-Options` / `frame-ancestors` — защита от clickjacking
- `Permissions-Policy`

**Рекомендация:**
```
Strict-Transport-Security "max-age=63072000; includeSubDomains; preload"
X-Frame-Options DENY
```

---

### I-SEC-12 — [MEDIUM] Дефолтные credentials dbmigrate в `.env.example`

**Файл:** `database/dbmigrate/.env.example`
```
DATABASE_URL=postgres://postgres:postgres@localhost:5432/fiscal?sslmode=disable
```
Если разработчик не копирует `.env.example` → `.env`, миграции запускаются с superuser credentials.

**Рекомендация:**  
Использовать non-superuser migration role с правами только на DDL целевой БД. Не иметь дефолтных значений.

---

## Операционные проблемы

---

### I-OPS-01 — [HIGH] PostgreSQL контейнеры без `restart: unless-stopped`

**Файлы:** `compose.fiscalisation.yaml` строки 3–9; `compose.minipos.yaml` строки 3–8

**Описание:**  
`redis`, `emqx`, `rabbitmq` и все application контейнеры имеют `restart: unless-stopped`. Оба `postgres` контейнера — нет. При падении PostgreSQL (OOM kill, disk fault) платформа останавливается и не поднимается до ручного вмешательства.

**Рекомендация:**  
Добавить `restart: unless-stopped` в оба postgres service definition.

---

### I-OPS-02 — [MEDIUM] MinIO не имеет healthcheck

**Файл:** `compose.minipos.yaml` строки 9–17

**Описание:**  
`dap-minio` без `healthcheck`. `beeminipos-backend` не ждёт готовности MinIO. При медленном старте — backend принимает запросы до доступности object store, что вызывает молчаливые ошибки при загрузке документов.

**Рекомендация:**
```yaml
healthcheck:
  test: ["CMD", "mc", "ready", "local"]
  interval: 10s
  timeout: 5s
  retries: 5
```
Добавить в `beeminipos-backend.depends_on`: `dap-minio: {condition: service_healthy}`.

---

### I-OPS-03 — [MEDIUM] EMQX и RabbitMQ без healthcheck — `fiscal-backend` зависит от `service_started`

**Файл:** `compose.fiscalisation.yaml` строка 60

**Описание:**
```yaml
depends_on: {emqx: {condition: service_started}, rabbitmq: {condition: service_started}}
```
`service_started` ждёт только запуска контейнера, не готовности брокера. EMQX инициализирует JWT auth конфигурацию 10–30 секунд. `fiscal-backend` упадёт при подключении к MQTT если EMQX не готов.

**Рекомендация:**
```yaml
emqx:
  healthcheck:
    test: ["CMD", "/opt/emqx/bin/emqx", "ping"]
    interval: 10s; timeout: 5s; retries: 12
rabbitmq:
  healthcheck:
    test: ["CMD", "rabbitmq-diagnostics", "-q", "ping"]
    interval: 10s; timeout: 5s; retries: 10
```
Изменить `depends_on` на `condition: service_healthy`.

---

### I-OPS-04 — [MEDIUM] `fiscal-backend` и `beeminipos-backend` без healthcheck

**Файл:** `compose.fiscalisation.yaml` строки 54–60; `compose.minipos.yaml` строки 18–29

**Описание:**  
Caddy начинает проксировать трафик сразу после старта контейнера приложения без ожидания готовности. При старте выполняются DB-миграции — `502` ответы неизбежны.

**Рекомендация:**
```yaml
healthcheck:
  test: ["CMD-SHELL", "wget -qO- http://localhost:8080/healthz || exit 1"]
  interval: 10s; timeout: 5s; retries: 10
```
Изменить `caddy.depends_on` на `condition: service_healthy`.

---

### I-OPS-05 — [MEDIUM] Дублирующиеся числовые префиксы в init SQL файлах

**Файл:** `database/fiscal/`

**Описание:**  
`012_ble_authority_identity.sql` и `012_smart_device_activation.sql` имеют одинаковый префикс `012_`. Аналогично `004_ble_actor_subject.sql` и `004_runtime_login.sh`. `docker-entrypoint-initdb.d` запускает скрипты в лексикографическом порядке — при совпадении префиксов порядок непредсказуем и может вызвать ошибки зависимостей.

**Рекомендация:**  
Переименовать: `012a_ble_authority_identity.sql` / `012b_smart_device_activation.sql`, или перенумеровать все последующие файлы.

---

### I-OPS-06 — [MEDIUM] DDL-миграции запускаются inline при каждом старте приложения

**Файл:** `minipos/beeminipos-backend/internal/persistence/postgres.go` строки 47–56

**Описание:**
```go
for _, q := range []string{`create table if not exists minipos_state_rows...`, ...} {
    if _, e = db.ExecContext(ctx, q); e != nil { ... }
}
if _, e = db.ExecContext(ctx, `alter table minipos_state_meta add column if not exists ...`); e != nil { ... }
```
При rolling deployment нескольких реплик, стартующих одновременно, `ALTER TABLE` с `ACCESS EXCLUSIVE` lock вызывает starvation соединений. `IF NOT EXISTS` обеспечивает идемпотентность, но не предотвращает lock pile-up.

**Рекомендация:**  
Вынести DDL в отдельный migration step (существующий `dbmigrate` tool или Kubernetes init container), выполняемый единожды до старта реплик.

---

### I-OPS-07 — [MEDIUM] Нет resource limits на контейнеры

**Файлы:** Все compose файлы — нет `deploy.resources` или `mem_limit`/`cpus`.

**Описание:**  
Без лимитов, один неисправный сервис (например EMQX при storm сообщений, RabbitMQ leak) может поглотить все ресурсы хоста. PostgreSQL без ограничений памяти при большом fiscal report экспорте может OOM весь хост.

**Рекомендация:**
```yaml
deploy:
  resources:
    limits:
      cpus: "2"
      memory: 512M
    reservations:
      memory: 256M
```
Минимум — ограничить EMQX и RabbitMQ.

---

### I-OPS-08 — [LOW] `minipos_runtime_configuration_local_adapter` constraint `NOT VALID` и никогда не валидируется

**Файл:** `database/minipos/013_local_adapter_configuration.sql` строки 22–28

**Описание:**  
Constraint добавлен с `NOT VALID` — существующие строки не проверяются. Follow-up миграция с `VALIDATE CONSTRAINT` отсутствует. Некорректные конфигурации с отсутствующим `adapter_base_url` молча остаются в БД.

**Рекомендация:**  
Добавить follow-up миграцию:
```sql
ALTER TABLE minipos_runtime_configurations
  VALIDATE CONSTRAINT minipos_runtime_configuration_local_adapter;
```

---

### I-OPS-09 — [LOW] Makefile использует фиксированные `/tmp` пути для Go cache — коллизии в CI

**Файл:** `Makefile` строки 8–10

**Описание:**
```makefile
GOCACHE=/tmp/beefiscal-go-cache go test ./...
```
При параллельных CI job на одном хосте оба jobs используют один и тот же cache directory — возможная корrupция cache.

**Рекомендация:**  
Использовать `$(TMPDIR)` или workspace-relative path: `GOCACHE=/tmp/beefiscal-go-cache-$(BUILD_ID)`.

---

## Сводная таблица

| ID | Severity | Категория | Описание |
|----|----------|-----------|----------|
| I-SEC-01 | HIGH | Security | CORS `*` wildcard в production compose |
| I-SEC-02 | HIGH | Security | Device CA key в API-контейнере |
| I-SEC-03 | HIGH | Security | `sslmode=disable` на всех соединениях с БД |
| I-SEC-04 | HIGH | Security | Dev-секреты активны без prod overlay |
| I-SEC-05 | MEDIUM | Security | Dev SMTP plaintext порт 25 |
| I-SEC-06 | MEDIUM | Security | MQTT plaintext 1883 в private network |
| I-SEC-07 | MEDIUM | Security | EMQX dashboard доступен всем контейнерам |
| I-SEC-08 | MEDIUM | Security | RabbitMQ в ingress network |
| I-SEC-09 | MEDIUM | Security | Go backends запускаются как root |
| I-SEC-10 | MEDIUM | Security | CSP connect-src разрешает любой origin |
| I-SEC-11 | MEDIUM | Security | Fiscal Caddy без HSTS/X-Frame-Options |
| I-SEC-12 | MEDIUM | Security | dbmigrate default credentials в .env.example |
| I-OPS-01 | HIGH | Operational | PostgreSQL без `restart: unless-stopped` |
| I-OPS-02 | MEDIUM | Operational | MinIO без healthcheck |
| I-OPS-03 | MEDIUM | Operational | EMQX/RabbitMQ без healthcheck; `service_started` недостаточно |
| I-OPS-04 | MEDIUM | Operational | fiscal-backend без healthcheck; Caddy проксирует до готовности |
| I-OPS-05 | MEDIUM | Operational | Дублирующиеся числовые префиксы в init SQL |
| I-OPS-06 | MEDIUM | Operational | DDL inline при каждом старте — lock contention |
| I-OPS-07 | MEDIUM | Operational | Нет resource limits на контейнеры |
| I-OPS-08 | LOW | Operational | NOT VALID constraint без VALIDATE |
| I-OPS-09 | LOW | Operational | Makefile Go cache коллизии в CI |
