# Database and tenant isolation

SQL в `fiscal/` и `minipos/` является каноническим DDL для новых окружений и
применяется PostgreSQL entrypoint в лексикографическом порядке.

## Миграции

- `001_init.sql` — typed legal/reporting model и временный row-granular runtime
  compatibility store;
- `002_tenant_rls.sql` — non-owner/no-`BYPASSRLS` роль, tenant context function,
  `ENABLE ROW LEVEL SECURITY`, `FORCE ROW LEVEL SECURITY`, policies и минимальные
  grants.
- `003_runtime_typed.sql` — authoritative typed hot-path projection: Fiscal
  sales/operations и MiniPOS products/employees/shifts/orders, EUR/money/version/
  uniqueness constraints, indexes и FORCE RLS.
- `004_runtime_login.sh` — включает LOGIN для минимально привилегированной
  tenant-reader роли, используя отдельный обязательный secret из окружения.

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

Gate входит в `make full-regression`.

## Переходный runtime store

`fiscal_state_rows` и `minipos_state_rows` пока восстанавливают полный in-memory
aggregate и поэтому намеренно не выдают ложную гарантию request-scoped RLS.
Они применяют дифференциальные upsert/delete и не трогают неизменённые строки.
Ключевые aggregates одновременно нормализуются в `*_runtime_*` typed tables в
той же serializable transaction: constraint failure откатывает обе модели.
Single-entity и collection hot-path GET/authorization уже читают typed tables
через отдельный non-owner pool внутри read-only transaction с обязательным
`SET LOCAL`; RLS отсекает чужой tenant до domain layer. Оставшийся переход —
tenant-bound typed mutations и остальные aggregates.
