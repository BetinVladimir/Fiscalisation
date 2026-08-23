# BeeFiscal Platform — Стратегия Production-развёртывания

**Дата:** 2026-08-23  
**Цель:** Быстрый старт на MVP-железе → масштабирование до сотен TPS без смены архитектуры  
**Принцип:** vendor-unlocked, hardware-portable, GitOps-first

---

## Содержание

1. [Архитектурные решения](#1-архитектурные-решения)
2. [Карта сервисов и их характеристики](#2-карта-сервисов)
3. [Тиры нагрузки и ресурсные требования](#3-тиры-нагрузки)
4. [Стратегия быстрой миграции между железом](#4-миграция-железа)
5. [База данных: масштабирование и архивирование](#5-база-данных)
6. [Edge-agent: развёртывание на POS-устройствах](#6-edge-agent)
7. [CI/CD и GitOps](#7-cicd-и-gitops)
8. [Disaster Recovery](#8-disaster-recovery)
9. [Checklist для production-запуска](#9-checklist)

---

## 1. Архитектурные решения

### 1.1 Оркестрация: Docker Compose → K3s → Kubernetes

| Этап | Инструмент | Когда переходить |
|------|-----------|-----------------|
| MVP (0–50 TPS) | Docker Compose + prod overlay | Сейчас |
| Рост (50–200 TPS) | K3s (lightweight K8s) | При выходе из одного сервера |
| Масштаб (200–1000 TPS) | Kubernetes (kubeadm / Talos) | При необходимости HA на уровне control plane |

**Почему K3s, а не сразу Kubernetes:**  
K3s работает на 1 GB RAM, содержит встроенный Traefik ingress и SQLite control-plane (опционально), не требует etcd кластера на малых инсталляциях. При росте — тривиально апгрейдить до полного K8s заменой бинарника.

**Почему не Docker Swarm:**  
Меньшая экосистема, хуже поддержка rolling updates, нет Helm-charts для EMQX/RabbitMQ кластеров. K3s совместим с тем же `kubectl` и всеми стандартными инструментами.

### 1.2 Хранилище: Longhorn (distributed block storage)

Longhorn — CSI-драйвер для Kubernetes, работает на голом железе, полностью vendor-unlocked:
- Replicated block volumes (настраиваемый replication factor: 1, 2, 3)
- Snapshot + backup в S3/MinIO
- Live volume migration между нодами без downtime
- Доступен через `helm install longhorn longhorn/longhorn`

**Альтернатива на одном сервере:** локальные bind-mounts (как сейчас в Compose) — достаточно для Tier 0–1.

### 1.3 Ingress/TLS: Caddy → Traefik (в K8s)

Caddy уже в проекте, продолжаем использовать его для Tier 0–1 (он и TLS получает и reverse proxy делает). При переходе на K3s Traefik уже встроен — Caddy можно убрать или оставить как sidecar.

### 1.4 Мониторинг

| Tier | Решение |
|------|---------|
| 0–1 | Docker stats + Caddy access logs |
| 2 | Prometheus + Grafana (docker-compose или Helm) |
| 3 | Victoria Metrics (более эффективно по RAM) + Grafana + AlertManager |

Метрики Go-приложений: добавить `prometheus/client_golang` endpoint `/metrics` (1-2 часа работы).

### 1.5 Registry контейнеров

- **Старт:** GitHub Container Registry (ghcr.io) — бесплатно, уже интегрирован с CI
- **Own hardware:** Gitea (self-hosted) со встроенным container registry — 150 MB RAM, простая установка
- **Продакшен образы:** тегировать по `git describe --tags`, не использовать `latest`

---

## 2. Карта сервисов

### 2.1 Fiscalisation Stack

| Сервис | Образ | Stateful? | Критичность | RAM (MVP) |
|--------|-------|-----------|-------------|-----------|
| `fiscal-backend` | distroless, ~22 MB | ❌ | Критический | 96 MB |
| `postgres` (fiscal) | postgres:16.10 | ✅ | Критический | 256 MB |
| `redis` | redis:7-alpine | ✅ (AOF) | Критический | 64 MB |
| `emqx` | emqx:5.9.3 | ✅ | Критический | 256 MB |
| `rabbitmq` | rabbitmq:3-management | ✅ | Критический | 128 MB |
| `caddy` | caddy:2.10.0 | ✅ (TLS certs) | Критический | 32 MB |

**EMQX — в критическом пути фискализации.** Edge-агенты отправляют синхронизированные батчи операций через MQTT-топики `tenants/+/devices/+/sync/batches/+`. Падение EMQX блокирует получение фискальных подтверждений с POS-устройств. Буферизация на edge-agent (SQLite) защищает от потери данных, но не от блокировки.

**RabbitMQ** — асинхронная доставка webhook'ов (exchange `beefiscal.integration`, queue `beefiscal.integration.webhooks`) и обработка integration commands с dead-letter retry до 5 попыток. При падении — операции ставятся в очередь, клиенты интеграций не получают уведомлений, но фискализация не останавливается.

### 2.2 MiniPOS Stack

| Сервис | Образ | Stateful? | Критичность | RAM (MVP) |
|--------|-------|-----------|-------------|-----------|
| `beeminipos-backend` | distroless, ~22 MB | ❌ | Критический | 64 MB |
| `postgres` (minipos) | postgres:16.10 | ✅ | Критический | 128 MB |
| `dap-minio` | minio:RELEASE.2025-09-07 | ✅ | Некритический | 128 MB |
| `miniposweb` | node/nginx SPA | ❌ | Важный | 32 MB |
| `caddy` | caddy:2.10.0 | ✅ (TLS certs) | Критический | 32 MB |

**MinIO** не в критическом пути — используется для отчётов и артефактов. При недоступности Z-отчёты не сохраняются, но продажи продолжаются.

### 2.3 Edge-Agent (per-device)

| Характеристика | Значение |
|---------------|---------|
| Бинарник | ~15–20 MB, distroless:nonroot |
| Storage | SQLite WAL, квота 1 GiB (настраивается) |
| Sync interval | 2000 мс (100–300 000 мс) |
| Shutdown timeout | 30 сек |
| Масштабируемость | 1 инстанс = 1 device/register |
| RAM | 32–64 MB per instance |

---

## 3. Тиры нагрузки

### Tier 0 — MVP (до 10 TPS, ~1 000 транзакций/день)

**Сервер:** 1 × VPS или bare metal, **2 GB RAM, 2 vCPU, 40 GB SSD**

**Развёртывание:** Docker Compose с prod overlay

```
docker compose \
  -f compose.fiscalisation.yaml \
  -f compose.fiscalisation.prod.yaml \
  -f compose.minipos.yaml \
  -f compose.minipos.prod.yaml \
  up -d
```

**Распределение RAM:**

| Сервис | RAM |
|--------|-----|
| fiscal-backend | 96 MB |
| beeminipos-backend | 64 MB |
| postgres (fiscal) | 256 MB |
| postgres (minipos) | 128 MB |
| redis | 64 MB |
| emqx | 256 MB |
| rabbitmq | 128 MB |
| minio | 128 MB |
| miniposweb | 32 MB |
| caddy × 2 | 48 MB |
| ОС + sshd + мониторинг | 256 MB |
| **Итого** | **~1.46 GB** ✅ |

**Ограничения Tier 0:**
- Нет HA — падение сервера = downtime
- PostgreSQL без репликации — backup только через `pg_dump`
- EMQX single-node — без отказоустойчивости

**Backup стратегия (cron):**
```bash
# pg_dump в MinIO каждые 6 часов
0 */6 * * * pg_dumpall -U fiscal | gzip | mc pipe minio/backups/fiscal-$(date +%Y%m%d-%H%M).sql.gz
```

---

### Tier 1 — Стабильный пилот (10–50 TPS, ~50K транзакций/день)

**Сервер:** 1 × **4–8 GB RAM, 4 vCPU, 200 GB NVMe SSD**

**Изменения относительно Tier 0:**
- Добавить resource limits в compose (предотвратить OOM kill одним сервисом)
- PostgreSQL: увеличить `shared_buffers` до 512 MB, `work_mem` до 16 MB
- Redis: включить `maxmemory 512mb` + `maxmemory-policy allkeys-lru`
- EMQX: настроить `vm_args` для 5000+ concurrent connections
- Добавить Prometheus + Grafana (дополнительные ~300 MB RAM)

**Пример resource limits в compose:**
```yaml
services:
  fiscal-backend:
    deploy:
      resources:
        limits:
          memory: 256M
          cpus: '1.0'
  postgres:
    deploy:
      resources:
        limits:
          memory: 1G
```

**Оценка ресурсов:**

| Компонент | RAM |
|-----------|-----|
| Все сервисы (расширенные лимиты) | 3.0 GB |
| Prometheus + Grafana | 512 MB |
| ОС + overhead | 512 MB |
| **Итого** | **~4.0 GB** ✅ |

**Включить PostgreSQL WAL archiving** (критично для быстрой миграции железа):
```sql
-- в postgresql.conf
wal_level = replica
archive_mode = on
archive_command = 'mc cp %p minio/wal-archive/%f'
```

---

### Tier 2 — Рост (50–200 TPS, ~500K транзакций/день)

**Кластер:** 3 сервера K3s, **каждый 8 GB RAM, 8 vCPU, 500 GB NVMe SSD**

**Архитектурные изменения:**
- Перевод на K3s (1 master + 2 worker)
- Stateful сервисы на выделенном worker'е с Longhorn volumes (replication factor: 2)
- Stateless сервисы (fiscal-backend, beeminipos-backend) — 2 реплики с rolling update
- PostgreSQL: primary + 1 streaming read replica (для reporting queries)
- EMQX: 3-node кластер (quorum-based)
- RabbitMQ: 3-node кластер с mirrored queues

**Database partitioning (CRITICAL на этом тире):**  
При 500K транзакций/день накапливается ~15M строк/месяц в `fiscal_runtime_sales` + `fiscal_operation_events`. Необходимо партиционирование:

```sql
-- Партиционирование sales по месяцам
ALTER TABLE fiscal_runtime_sales PARTITION BY RANGE (created_at);
CREATE TABLE fiscal_runtime_sales_2026_01 
  PARTITION OF fiscal_runtime_sales
  FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
```

**Kubernetes deployments:**

```yaml
# fiscal-backend deployment
replicas: 2
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxUnavailable: 0
    maxSurge: 1
resources:
  requests:
    memory: "128Mi"
    cpu: "250m"
  limits:
    memory: "512Mi"
    cpu: "2000m"
```

**Распределение по нодам:**

| Нода | Роль | Сервисы |
|------|------|---------|
| node-1 (master) | Control plane + stateless | fiscal-backend ×2, beeminipos-backend ×2, caddy |
| node-2 (worker) | Stateful tier 1 | postgres-primary, redis, rabbitmq |
| node-3 (worker) | Stateful tier 2 | postgres-replica, emqx, minio |

**RAM на ноду:**

| Нода | Использование |
|------|--------------|
| node-1 | 3 GB (сервисы) + 1 GB (K3s + система) = 4 GB из 8 GB |
| node-2 | 4 GB (БД + брокеры) + 1 GB (система) = 5 GB из 8 GB |
| node-3 | 3 GB (реплика + EMQX) + 1 GB (система) = 4 GB из 8 GB |

---

### Tier 3 — Масштаб (200–1000 TPS, ~5M транзакций/день)

**Кластер:** 6+ серверов K8s (kubeadm или Talos OS)

| Пул нод | Количество | Конфигурация | Назначение |
|---------|-----------|--------------|-----------|
| Control plane | 3 | 4 GB / 4 vCPU | etcd + API server |
| Stateless workers | 3+ | 16 GB / 8 vCPU | backend replicas (auto-scale) |
| Database nodes | 3 | 32 GB / 16 vCPU / 2 TB NVMe | PostgreSQL Patroni cluster |
| Broker nodes | 3 | 8 GB / 4 vCPU | EMQX 3-node + RabbitMQ 3-node |

**PostgreSQL Patroni (High Availability):**
- 1 primary + 2 replicas + HAProxy для connection routing
- Automatic failover <30 секунд
- `synchronous_commit = on` для фискальных операций

**Horizontal Pod Autoscaler:**
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
spec:
  scaleTargetRef:
    name: fiscal-backend
  minReplicas: 2
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        averageUtilization: 70
```

**EMQX кластер (5 nodes при 1000 TPS):**  
EMQX обрабатывает до 5M сообщений/сек на кластере из 5 нод. При 1000 POS-устройств и sync interval 2 сек — 500 messages/sec, один EMQX-узел справляется. Кластер нужен для HA, не для пропускной способности.

**Оценка стоимости на собственном железе (Tier 3):**

| Позиция | Количество | Цена (б/у Dell/HP серверы) |
|---------|-----------|--------------------------|
| Control plane (4C/8GB) | 3 | ~$200–400/шт |
| Worker nodes (8C/32GB) | 6 | ~$500–800/шт |
| NVMe SSD 2TB | 3 | ~$150–200/шт |
| 10Gb switch | 1 | ~$300–500 |
| **Итого железо** | — | **~$6 000–10 000** |

---

## 4. Миграция железа

### Принцип: image-first, state-last

Все сервисы — Docker образы. Migrate = поднять новый сервер + перенести state + переключить DNS.

### 4.1 Миграция между серверами (Tier 0→1, Compose→Compose)

**Downtime: <5 минут**

```bash
# 1. На старом сервере: включить streaming replication
echo "host replication replicator NEW_SERVER_IP/32 scram-sha-256" >> pg_hba.conf
psql -c "CREATE USER replicator REPLICATION LOGIN PASSWORD '...';"

# 2. На новом сервере: pg_basebackup
pg_basebackup -h OLD_SERVER -U replicator -D /var/lib/postgresql/data \
  --wal-method=stream -P

# 3. Запустить новый сервер (все сервисы кроме приложений)
docker compose -f compose.fiscalisation.yaml -f compose.fiscalisation.prod.yaml up -d postgres redis emqx rabbitmq minio

# 4. Дождаться синхронизации replication lag = 0
psql -c "SELECT pg_wal_lsn_diff(pg_current_wal_lsn(), sent_lsn) FROM pg_stat_replication;"

# 5. Остановить старый сервер
ssh old-server "docker compose stop fiscal-backend beeminipos-backend"

# 6. Promote на новом сервере
pg_ctl promote -D /var/lib/postgresql/data

# 7. Запустить приложения на новом сервере
docker compose up -d

# 8. Обновить DNS / IP edge-агентов
```

### 4.2 Миграция Compose → K3s (Tier 1→2)

**Downtime: 0 (rolling migration)**

```bash
# 1. Установить K3s на новых серверах
curl -sfL https://get.k3s.io | sh -

# 2. Добавить worker nodes
K3S_URL=https://MASTER_IP:6443 K3S_TOKEN=NODE_TOKEN \
  curl -sfL https://get.k3s.io | sh -

# 3. Развернуть Longhorn
helm install longhorn longhorn/longhorn -n longhorn-system

# 4. Мигрировать PostgreSQL через Patroni (поднять реплику в K8s, promote после sync)
# 5. Переключить DNS на K3s ingress
# 6. Снять старый Compose
```

### 4.3 Миграция edge-agent на новое POS-железо

```bash
# Сохранить состояние
sqlite3 /var/edge/edge.db ".backup /tmp/edge-backup.db"

# На новом устройстве
cp /tmp/edge-backup.db /var/edge/edge.db
docker run -d \
  -v /var/edge:/data \
  --env-file /etc/edge-agent.env \
  ghcr.io/beeloy/edge-agent:latest
```

Конфигурация edge-agent (`/etc/edge-agent.env`) — полностью переносима. Смена железа = 10 минут.

### 4.4 Обновление без downtime (rolling update)

**Stateless сервисы (fiscal-backend, beeminipos-backend):**
```bash
# Compose (Tier 0–1)
docker compose pull fiscal-backend
docker compose up -d --no-deps fiscal-backend
# Нулевой downtime: Caddy продолжает роутить на старый контейнер пока новый не стартовал

# K8s (Tier 2–3)
kubectl set image deployment/fiscal-backend fiscal-backend=ghcr.io/beeloy/fiscal-backend:v1.2.0
# RollingUpdate: maxUnavailable=0, maxSurge=1
```

**Stateful сервисы (PostgreSQL, EMQX):** всегда через репликацию + failover, не через перезапуск.

---

## 5. База данных

### 5.1 Расчёт роста данных

| Нагрузка | Транзакций/день | sales rows/мес | operation_events/мес | Размер БД/год |
|---------|----------------|---------------|---------------------|--------------|
| 10 TPS (пик) | ~10K | ~300K | ~600K | ~5 GB |
| 50 TPS (пик) | ~100K | ~3M | ~6M | ~50 GB |
| 200 TPS (пик) | ~500K | ~15M | ~30M | ~250 GB |
| 1000 TPS (пик) | ~2.5M | ~75M | ~150M | ~1.2 TB |

### 5.2 Партиционирование (обязательно с Tier 2)

Таблицы для партиционирования: `fiscal_runtime_sales`, `fiscal_runtime_operations`, `fiscal_operation_events`, `fiscal_audit_events`.

Стратегия: **RANGE по `created_at`** (ежемесячные партиции) + **pg_partman** для автоматического создания и архивирования.

```sql
-- Пример: partition maintenance
SELECT partman.create_parent(
  p_parent_table => 'public.fiscal_runtime_sales',
  p_control => 'created_at',
  p_type => 'range',
  p_interval => 'monthly'
);

-- Архивирование партиций старше 12 месяцев в cold storage
-- через pg_dump + загрузка в MinIO/S3
```

### 5.3 Connection pooling: PgBouncer

На Tier 1+ добавить PgBouncer перед PostgreSQL:
- Режим: `transaction` (совместим с RLS через `app.tenant_id` session var)
- Лимит: 100 соединений к PostgreSQL, 2000 клиентских соединений
- RAM: ~10 MB

**Важно:** RLS использует `SET LOCAL app.tenant_id = ...` — это работает в transaction-mode PgBouncer, но нужна проверка с конкретной реализацией в коде.

### 5.4 Read replica для отчётности

Reporting-запросы (Z-отчёты, export, аудит) отправлять на реплику через `RLS_DATABASE_URL`:
```bash
RLS_DATABASE_URL=postgres://beefiscal_tenant:...@postgres-replica:5432/fiscal?sslmode=require
```

Реплика не участвует в записи → не блокирует OLTP.

---

## 6. Edge-Agent

### 6.1 Топология развёртывания

```
Каждая точка продаж:
  POS-device (RPi / NUC / мини-ПК)
    └── edge-agent (Docker container / systemd)
          ├── SQLite WAL (local persistence)
          ├── ← BLE ← miniposweb / BeeMiniPOS app
          └── → HTTPS → fiscal-backend (sync batches)
                      → EMQX ← fiscal-backend (revocation events)
```

**Минимальные требования на POS-устройство:**
- ARM64 или x86_64 Linux
- 512 MB RAM (edge-agent потребляет 32–64 MB)
- 4 GB persistent storage (SQLite + OS)
- Сеть: HTTPS outbound к fiscal-backend (возможно через 4G/LTE)

### 6.2 Обновление edge-agent (OTA)

```bash
# systemd unit на POS-устройстве
# /etc/systemd/system/edge-agent.service
[Unit]
Description=BeeFiscal Edge Agent
After=docker.service

[Service]
Restart=always
RestartSec=5s
ExecStartPre=-/usr/bin/docker pull ghcr.io/beeloy/edge-agent:${EDGE_AGENT_VERSION}
ExecStart=/usr/bin/docker run --rm \
  --name edge-agent \
  -v /var/edge:/data \
  --env-file /etc/edge-agent.env \
  ghcr.io/beeloy/edge-agent:${EDGE_AGENT_VERSION}
```

Обновление на 100 устройствах: обновить `EDGE_AGENT_VERSION` в Ansible playbook → `ansible-playbook update-edge-agents.yml`.

### 6.3 Мониторинг edge-агентов

Endpoint `/healthz` на edge-agent (если открыт) + метрики через EMQX топик `beefiscal/v1/devices/+/status`. fiscal-backend уже подписан на этот топик.

---

## 7. CI/CD и GitOps

### 7.1 Текущее состояние

GitHub Actions уже настроен:
- `full-fiscal-e2e.yml` — E2E тесты (35 мин timeout)
- `integration-contract.yml` — contract testing
- `beefiscalapp-e2e.yml` — app E2E

### 7.2 Добавить: build + push образов

```yaml
# .github/workflows/build.yml
on:
  push:
    tags: ['v*']

jobs:
  build:
    runs-on: ubuntu-24.04
    steps:
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Build fiscal-backend
        uses: docker/build-push-action@v5
        with:
          context: ./fiscal-backend
          push: true
          tags: ghcr.io/${{ github.repository }}/fiscal-backend:${{ github.ref_name }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

### 7.3 GitOps: ArgoCD (Tier 2+)

ArgoCD синхронизирует K8s manifests из Git репозитория:
- Каждый environment (`dev`, `staging`, `prod`) — отдельная ветка или директория
- Production deploy = merge в `prod` ветку → ArgoCD обнаруживает изменение → rolling update
- Rollback = `git revert` → ArgoCD восстанавливает предыдущее состояние

```
deploy/
  k8s/
    base/           # общие манифесты
    overlays/
      dev/          # dev-специфичные настройки
      prod/         # prod-специфичные настройки
```

### 7.4 Secrets management

| Tier | Инструмент |
|------|-----------|
| 0–1 | `.env` файл на сервере (ограничить права: `chmod 600`) |
| 2 | K8s Secrets + sealed-secrets (Bitnami) — зашифрованы в Git |
| 3 | Vault (HashiCorp) или external-secrets-operator → AWS Secrets Manager / Doppler |

---

## 8. Disaster Recovery

### 8.1 RPO и RTO цели

| Tier | RPO (потеря данных) | RTO (время восстановления) |
|------|--------------------|-----------------------------|
| 0 | 6 часов (pg_dump interval) | 2–4 часа |
| 1 | 15 минут (WAL archiving) | 30–60 минут |
| 2 | ~0 (streaming replication) | 5–10 минут |
| 3 | ~0 (Patroni + sync commit) | <30 секунд (auto failover) |

### 8.2 Backup стратегия

```bash
# Tier 0–1: cron backup
# Ежедневный full backup + непрерывный WAL archiving

# Full backup
0 2 * * * pg_basebackup -D /backup/$(date +%Y%m%d) -Ft -z -P
# WAL archiving (postgresql.conf)
archive_command = 'mc cp %p minio/wal/%f'

# Retention: 7 дней full + 30 дней WAL
7 2 * * 0 find /backup -mtime +7 -exec rm -rf {} \;
```

### 8.3 Тест восстановления (обязательно проводить ежемесячно)

```bash
# Восстановить на тестовом сервере из backup
pg_restore --clean -d postgres /backup/latest/fiscal.dump
# Проверить количество записей
psql -c "SELECT count(*) FROM fiscal_runtime_sales;"
# Запустить smoke-тесты
./tests/smoke/run.sh https://restore-test.internal
```

---

## 9. Checklist для production-запуска

### Tier 0 (прямо сейчас)

- [ ] Выбрать сервер (минимум 2 GB RAM / 2 vCPU / 40 GB SSD)
- [ ] Настроить `.env` файлы со всеми required переменными (без `:-dev-only-*`)
- [ ] Сгенерировать секреты: `openssl rand -hex 32` для AUTH_HMAC_KEY, BLE_SIGNING_KEY, EMQX_JWT_SECRET, RABBITMQ_DEFAULT_PASS, FISCAL_DB_PASSWORD и т.д.
- [ ] Настроить DEVICE_CA_KEY_PATH и DEVICE_CA_CERT_PATH для Docker secrets
- [ ] Запустить с prod overlay: `docker compose -f compose.fiscalisation.yaml -f compose.fiscalisation.prod.yaml up -d`
- [ ] Убедиться что ALLOW_STUB_ADAPTERS=false в prod (проверить что fiscal-backend стартует — значит адаптер реального устройства активирован)
- [ ] Настроить OIDC IdP (Keycloak/Auth0) или AUTH_HMAC_KEY для legacy auth
- [ ] Включить WAL archiving в PostgreSQL → MinIO
- [ ] Настроить pg_dump cron backup
- [ ] Настроить мониторинг uptime (Uptime Kuma — 50 MB RAM, easy setup)
- [ ] Настроить Caddy с реальным доменом (TLS auto via Let's Encrypt)
- [ ] Firewall: закрыть все порты кроме 80/443 (Caddy) и SSH
- [ ] Увеличить graceful shutdown с 5 до 15 секунд в обоих main.go
- [ ] Проверить что edge-agents подключаются по HTTPS (ssl:// EMQX URI)

### Tier 1 (при росте нагрузки)

- [ ] Добавить resource limits в compose
- [ ] Включить PgBouncer перед PostgreSQL
- [ ] Настроить PostgreSQL streaming replication на standby-сервер
- [ ] Добавить Prometheus + Grafana
- [ ] Настроить Redis maxmemory policy
- [ ] Table partitioning для sales/events таблиц

### Tier 2 (при выходе за пределы одного сервера)

- [ ] Установить K3s на 3 сервера
- [ ] Развернуть Longhorn для persistent volumes
- [ ] Мигрировать PostgreSQL под управление CloudNativePG operator
- [ ] EMQX 3-node кластер (helm chart: emqx/emqx)
- [ ] RabbitMQ 3-node кластер (helm chart: bitnami/rabbitmq)
- [ ] Развернуть ArgoCD для GitOps
- [ ] HPA для stateless deployments

---

## Сводная таблица ресурсов

| Tier | TPS (пик) | Серверов | RAM total | CPU total | Storage | Стоимость/мес* |
|------|-----------|---------|-----------|-----------|---------|---------------|
| 0 | 10 | 1 | 2 GB | 2 vCPU | 40 GB | €5–20 (VPS) / $0 (own) |
| 1 | 50 | 1 | 8 GB | 4 vCPU | 200 GB | €20–60 (VPS) / $0 (own) |
| 2 | 200 | 3 | 24 GB | 24 vCPU | 1.5 TB | €150–300 (VPS) / $0 (own) |
| 3 | 1000 | 9+ | 200+ GB | 80+ vCPU | 10+ TB | €1000–3000 (VPS) / amortized (own) |

\* VPS — Hetzner/Contabo/OVH. Own hardware — одноразовая стоимость + электричество.

**Рекомендация по железу для собственного кластера Tier 2–3:**  
Dell PowerEdge R640 / HP ProLiant DL360 Gen10 с б/у рынка (~€500–1500/шт), 10 GbE switch (~€300). Полный кластер Tier 2 из 3 серверов: ~€2 500–5 000 разово vs. €1 800/год на VPS.

---

*Документ актуален для версии платформы от 2026-08-23. При изменении архитектуры требует обновления разделов 2 и 5.*
