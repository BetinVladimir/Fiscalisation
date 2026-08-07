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

Fiscal transaction обязан выполнить `SET LOCAL ROLE beefiscal_tenant` и
`SET LOCAL app.tenant_id = '<uuid>'` до typed SQL. MiniPOS использует
`SET LOCAL ROLE beeminipos_tenant` и
`SET LOCAL app.organization_id = '<uuid>'`. Контекст задаётся только из
проверенного JWT claim, никогда из request body/query/header с tenant id.
Отсутствующий или неверный context fail-closed: RLS не возвращает строки и
отклоняет mutation.

`FORCE RLS` обязателен, чтобы владелец таблицы не обходил policies. Production
runtime должен подключаться не superuser-ролью. Миграционный владелец и runtime
identity должны быть разными секретами/ролями.

## Проверка

`make postgres-integration` поднимает чистые PostgreSQL 16.10 instances,
применяет весь DDL, переключается на non-owner роли и доказывает:

- другой tenant/organization невидим;
- cross-tenant insert отклоняется самой БД;
- persistence restart/legacy migration/differential update работают на
  настоящем PostgreSQL.

Gate входит в `make full-regression`.

## Переходный runtime store

`fiscal_state_rows` и `minipos_state_rows` пока восстанавливают полный in-memory
aggregate и поэтому намеренно не выдают ложную гарантию request-scoped RLS.
Они применяют дифференциальные upsert/delete и не трогают неизменённые строки.
Ключевые aggregates одновременно нормализуются в `*_runtime_*` typed tables в
той же serializable transaction: constraint failure откатывает обе модели.
Single-entity hot-path GET/authorization уже читает typed tables внутри
read-only transaction под non-owner ролью с обязательным `SET LOCAL`; RLS
отсекает чужой tenant до domain layer. Оставшийся переход — collection reads,
mutations и остальные aggregates.
