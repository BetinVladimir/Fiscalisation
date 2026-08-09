# Database and tenant isolation

SQL в `fiscal/` и `minipos/` является каноническим DDL для новых окружений и
применяется PostgreSQL entrypoint в лексикографическом порядке.

## Миграции

- `001_init.sql` — typed legal/reporting model и временный row-granular runtime
  compatibility store;
- `002_tenant_rls.sql` — non-owner/no-`BYPASSRLS` роль, tenant context function,
  `ENABLE ROW LEVEL SECURITY`, `FORCE ROW LEVEL SECURITY`, policies и минимальные
  grants.
- `003_runtime_typed.sql` — authoritative typed projection: Fiscal
  sales/operations/shifts/resources plus tenant-bound UNP sequences, API replay, webhook outbox, BLE sessions,
  connectivity probes, Edge pending commands and audit events; MiniPOS
  products/employees/shifts/orders/configuration plus API replay, webhook
  inbox and checkout result/hash checkpoints. Таблицы содержат явный tenant,
  domain constraints/indexes и FORCE RLS.
- `004_runtime_login.sh` — включает LOGIN для минимально привилегированной
  tenant-reader роли, используя отдельный обязательный secret из окружения.
- `005_typed_only_runtime.sql` — сохраняет точный domain payload рядом с
  constrained/indexed typed columns и добавляет versioned `storage_mode`.

Fiscal transaction обязан выполнить `SET LOCAL ROLE beefiscal_tenant` и
`SET LOCAL app.tenant_id = '<uuid>'` до typed SQL. MiniPOS использует
`SET LOCAL ROLE beeminipos_tenant` и
`SET LOCAL app.organization_id = '<uuid>'`. Контекст задаётся только из
проверенного JWT claim, никогда из request body/query/header с tenant id.
Отсутствующий или неверный context fail-closed: RLS не возвращает строки и
отклоняет mutation.

`FORCE RLS` обязателен, чтобы владелец таблицы не обходил policies. Backend имеет
два физически отдельных пула: system writer (`DATABASE_URL`) для переходной
aggregate transaction и non-owner tenant reader (`RLS_DATABASE_URL`) для typed
GET/list. В PROD разные database users обязательны и проверяются при старте.

## Проверка

`make postgres-integration` поднимает чистые PostgreSQL 16.10 instances,
применяет весь DDL, переключается на non-owner роли и доказывает:

- другой tenant/organization невидим;
- cross-tenant insert отклоняется самой БД;
- typed GET/list подключаются отдельным LOGIN/non-owner credential;
- persistence restart/legacy migration/differential update работают на
  настоящем PostgreSQL.

Legacy migration является исполняемым drill, а не только unit-проверкой: тесты
загружают прежний `runtime_snapshots`, запускают атомарную миграцию и проверяют
одновременно восстановленный row-store и tenant-bound typed projection
(`fiscal_runtime_sales` / `minipos_runtime_products`). Это гарантирует, что
cutover не объявляется готовым при потере typed-представления.

Gate входит в `make full-regression`.

## Typed-only runtime и rollback-слой

В bootstrap mode 1 `fiscal_state_rows` и `minipos_state_rows` восстанавливают
старый in-memory aggregate. SQL guard переключает mode 2 только когда каждая
поддерживаемая строка имеет точный typed payload. После переключения runtime
читает и пишет только `*_runtime_*` typed tables. Старые строки остаются
неизменным пассивным rollback-слоем до отдельно подтверждённой destructive migration.
Ключевые aggregates одновременно нормализуются в `*_runtime_*` typed tables в
той же serializable transaction. Каждый typed upsert/delete выполняется после
`SET LOCAL ROLE` и tenant context: constraint или cross-tenant failure
откатывает обе модели.
Single-entity и collection hot-path GET/authorization уже читают typed tables
через отдельный non-owner pool внутри read-only transaction с обязательным
`SET LOCAL`; RLS отсекает чужой tenant до domain layer. Fiscal artifacts и Edge
sync ACK также имеют tenant-bound typed storage и RLS; active device administration
использует `resources(kind=device)`, а не историческую `devices` map.
Все публичные hot-path и technical orchestration reads (включая replay,
BLE/probe, Edge pending/ACK, artifact/audit и MiniPOS checkout checkpoints)
переведены на typed repositories. Typed-only restart собирает coordinator cache
из constrained tables; следующий переход — удаление in-process maps и после
rollback window физическое удаление пассивного compatibility store.
До полного удаления coordinator обе БД используют monotonic generation CAS. `LoadVersioned`
читает rows и generation в одной `REPEATABLE READ` transaction; `SaveVersioned` блокирует
meta row и отклоняет stale generation до mutation. После conflict backend reload-ит
авторитетный snapshot, поэтому незафиксированное состояние не остаётся в памяти.
Нормальный runtime write использует `SaveDeltaVersioned`: coordinator хранит последний
подтверждённый baseline и вычисляет точные changed/deleted entities. В mode 1 PostgreSQL
атомарно заполняет compatibility и typed модели; в mode 2 после CAS меняются только typed
rows. Полный scan и database-wide stale-delete отсутствуют в рабочем mutation path.
Outbox/BLE/UNP/connectivity/Edge pending/audit/replay и MiniPOS API replay/webhook
inbox/checkout checkpoints уже проецируются атомарно и fail-closed.
