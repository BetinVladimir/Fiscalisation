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
- `004_ble_actor_subject.sql` — backward-compatible Fiscal BLE authority migration; projects the issuing OIDC subject, leaves legacy sessions unbound/fail-closed, and indexes active subject-scoped leases.
- `005_ble_client_public_key.sql` — projects and validates the ticket-bound X25519 public key used for client proof-of-possession at Edge HELLO.
- `004_runtime_login.sh` — включает LOGIN для минимально привилегированной
  tenant-reader роли, используя отдельный обязательный secret из окружения.
- `005_typed_only_runtime.sql` — сохраняет точный domain payload рядом с
  constrained/indexed typed columns и добавляет versioned `storage_mode`.
- `006_operation_register_and_shift_guard.sql` — backfill/index migration для
  durable `operation.register_id` и единственной активной либо требующей
  reconciliation смены на `(tenant_id, register_id)`.
- `007_api_idempotency_claim.sql` — durable `PENDING` claim для сериализации
  первого Fiscal mutation request между backend replicas до любого device или
  domain side effect.
- `008_resource_business_uniqueness.sql` — database-level enforcement для
  одной organization на tenant и регистронезависимых tenant-scoped business
  keys: location/register/operator `code` и device `serial`. Ограничения
  совпадают с атомарными проверками repository и защищают альтернативных
  writers/import tooling от расхождения с API.
- `009_sale_line_discount.sql` — обязательная неотрицательная абсолютная EUR
  скидка строки Fiscal, не превышающая её gross; поле остаётся частью
  неизменяемого snapshot и аппаратной команды.
- `010_sale_device_snapshot.sql` — backfill и индексированная typed-проекция
  неизменяемых `device_id`, serial, ИН ФУ/ФП, vendor/model/firmware, захваченных
  атомарно при payment reservation. Исторический receipt/export никогда не
  разрешает устройство через текущую mutable register binding.
- `011_sale_location_snapshot.sql` — backfill и индексированная typed-проекция
  точки продаж, зафиксированной при создании продажи. Фильтры исторических
  выгрузок не разрешают location через изменяемую конфигурацию кассового места.
- `012_ble_authority_identity.sql` — отделяет Edge advertising identity от
  конечного ФУ и индексирует неизменяемые location/register/fiscal-device
  поля подписанной BLE-authority session.
- MiniPOS migrations `006_operator_identity.sql`,
  `007_order_reversal_evidence.sql`, `008_order_fiscal_reference.sql` and
  `009_order_payment_evidence.sql` add
  durable passwordless identity/session bindings, constrained append-only
  reversal evidence and the original fiscal receipt reference in both the
  order and idempotent checkout projection. New `COMPLETED`/`REVERSED` rows
  cannot be stored without the original receipt reference; the constraint is
  `NOT VALID` only so historical rows can be backfilled without inventing data.
  `payments` is a constrained JSON array containing only validated tender type
  and exact EUR amount evidence. It supports restart-safe `[from,to)` sales
  reporting and must never contain PAN, PIN, track or CVV data.
- MiniPOS migration `010_api_idempotency_claim.sql` adds the durable
  `PENDING` claim projection used to serialize the first mutation request
  across backend replicas before any business or Fiscal side effect.
- MiniPOS migration `011_catalog_authority_consistency.sql` repairs legacy
  product/employee `active` projections from canonical `status`, updates their
  exact payload, then enforces `ACTIVE|INACTIVE`, `active=(status='ACTIVE')`
  and case-insensitive tenant uniqueness for SKU/operator code. This keeps
  alternative writers and typed-only restart aligned with domain authority.
- MiniPOS migration `012_order_line_discount.sql` хранит тот же абсолютный
  line-discount автономно и запрещает отрицательную скидку либо скидку выше
  gross до синхронизации с Fiscal API.

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
