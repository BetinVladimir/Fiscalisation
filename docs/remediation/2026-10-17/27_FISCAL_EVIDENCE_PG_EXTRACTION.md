# N5/N6: ограниченный экспорт ресурсов и финансового аудита Fiscal PostgreSQL

Статус: исполняемый read-only экспорт для dossier; не полная резервная копия, не подтверждение юридической достаточности и не готовность к НАП. Профиль `fiscal-evidence` дополняет независимые финансовый `fiscal`, MiniPOS `minipos`, S3 SQLite и BlueCash SQLite профили. Ключи ESP32-S3 находятся в NVS и не входят ни в один экспорт.

## Запуск и приобретение снимка

Использовать отдельного tenant-scoped читателя `beefiscal_tenant`, без SUPERUSER/BYPASSRLS и владения таблицами. DSN передаётся только через `DOSSIER_DATABASE_URL`; не помещать его в dossier, shell history или отчёты. Требуется точный проверенный словарь для фактически развёрнутой схемы. Словарь из `28_FISCAL_EVIDENCE_PG_DICTIONARY.generated.json` построен на disposable migration fixture, не на production. При отличии каталогов экспорт отказывается работать; нельзя автоматически принять новый production catalog как доверенный словарь.

```sh
cd fiscal-backend
# DOSSIER_DATABASE_URL устанавливается оператором через защищённый канал.
go run ./cmd/dossier-export -profile fiscal-evidence -schema public \
  -tenant '<tenant-id>' -source-commit '<captured-source-commit>' \
  -contract ../docs/remediation/2026-10-17/28_FISCAL_EVIDENCE_PG_DICTIONARY.generated.json \
  -output '/new/acquired/dossier-directory'
go run ./cmd/dossier-export -verify '/new/acquired/dossier-directory'
```

Архив получается в одной READ ONLY REPEATABLE READ транзакции; две таблицы читаются из одного PostgreSQL snapshot. Экспорт не запускает application persistence.Open, не восстанавливает journal, не обновляет projection и не выполняет checkpoint. Каталог назначения должен отсутствовать, staging имеет режим 0700, файлы 0600; manifest записывается последним. Проверка archive inventory/hash не аутентифицирует происхождение; source-commit является заявленной меткой приобретения, не подписью.

## Проверенный охват

`fiscal_runtime_resources`: kind/id/tenant/version/timestamps и выбранные поля data. Дублирующий raw payload не выгружается. `fiscal_runtime_audit`: идентификатор события/арендатор/actor/action/object/time/event_hash/typed previous_hash и выбранные before/after/UNP/hash pointers из payload. Audit остаётся неизменным в БД; экспорт представляет редактированную проекцию, не оригинальные байты подписанного события.

Правила каждого kind, поля и типы включены в reviewed dictionary и исполняются в `internal/dossier/evidence.go`. Поддержаны sale, operation, организация/объект/касса/оператор/устройство, binding и issued binding, execution authorization/revocation, reconciliation authorization и payment reconciliation events, отчёты, публичные workstation references, delegated sale/shift и UNP identity. Это whitelist, а не поиск по именам секретов. Неизвестное поле никогда не переносится автоматически.

Для issued binding сохраняются identity/generation/digests и выбранные fiscal/UNP ссылки; исключены canonical_json, signature, MQTT/localHTTP credentials и secret material. Для authorization сохраняются идентификаторы и envelope digest/key-id/algorithm; исполняемые payload/signature не экспортируются. Для платежей сохраняются сумма в minor units, RRN/auth/business references и evidence digests, но не bearer grants. Эти бизнес-данные могут быть чувствительными: применять ограниченный доступ и срок хранения dossier.

Provisioning session/delivery, device command outbox и webhook endpoint имеют EXCLUDED_CREDENTIAL_KIND. Любой неизвестный kind имеет EXCLUDED_UNREVIEWED_KIND. В обоих случаях остаются metadata и явная coverage inventory, fields=null. Неизвестные события не исчезают молча, но их содержимое не считается проверенным. Для audit отдельно сохраняются только исходные UNP/previous_hash/prev_hash pointers. Расширение требует review реального producer, явной схемы, dictionary regeneration и negative secret fixtures.

SQL CASE/JSONB projection выполняется внутри PostgreSQL до передачи по соединению. Go повторно проверяет типы и отсутствие неизвестных полей. Неверный тип известного поля приводит к отказу без печати содержимого. Все числа архива передаются текстом; вложенные JSON числа валидируются без float64, включая значения за 2^53. NULL отличается от строки и пустого значения. Coverage проверяется повторно при offline verification.

## Идентификаторы между хранилищами

Сопоставлять tenant + kind + id, sale.id/server_sale_id, operation.id/client_operation_id/receipt_session_id, fiscal device id/serial/FMIN, binding id/generation/hash, execution authorization id/hash, payment reconciliation grant/event/reference и UNP. Совпадение строки без tenant и namespace недостаточно. Это extraction contract; автоматическая межхранилищная сверка и доказательство полноты пока не реализованы.

## Известные разрывы аудита

Domain audit JSON использует `previous_hash`; исторические SQL 019 generated column и 017 projection читают `prev_hash`. Экспорт сохраняет оба source pointers и typed column и отмечает PROJECTED_HASH_POINTER_MISMATCH или PROJECTED_AMBIGUOUS_HASH_POINTER в coverage. Forward migration 021 исправляет canonical/legacy compatibility без изменения payload/hash. Исправление источника выполняется отдельной forward migration; exporter не переписывает историю и не меняет event_hash. После изменения схемы нужен новый reviewed fixture dictionary.

Цепочка domain repository глобальна между tenants. Tenant-filtered и отредактированный extract нельзя объявлять полной проверяемой hash chain, пересчитывать исходные подписи по отредактированным данным или выдавать успешную archive integrity за chain verification.

017 также ограничивает typed sale_events UUID-идентификатором продажи. Actual `service.go:newID("sale")` создаёт `sale-<hex>`; actual audit event id — UUID. Например tenant `11111111-1111-4111-8111-111111111111`, event `22222222-2222-4222-8222-222222222222`, object_id `sale-33333333-3333-4333-8333-333333333333` проходит tenant/event guard, но не sale object guard: runtime audit сохраняется, sale_events не пополняется. Дополнительно 017 читает fiscal_device_snapshot, тогда как domain Sale содержит fiscal_device. Typed event projection/backfill остаются OPEN. Текущие exporter fixtures tenant-evidence/audit-operation проверяют runtime extraction, не typed sale projection.

Отдельный регрессионный критерий для исправления: создать через actual domain OpenSale audit с UUID tenant, перенести actual serialized audit в migrated PostgreSQL, получить ровно одну sale_events запись с тем же source id, fiscal_device и hash pointer; повторная доставка идемпотентна, tenant isolation сохраняется. Не заменять source identifiers вымышленными UUID ради прохождения trigger.

## Проверка

```sh
sh scripts/dossier-evidence-postgres-integration.sh
sh scripts/dossier-postgres-integration.sh
sh scripts/dossier-minipos-postgres-integration.sh
```

Первый runner создаёт отдельный PostgreSQL 16.10 с actual migrations, случайным localhost портом и удаляет только собственный disposable container. Шесть реальных PG сценариев: pruning до транспорта/secret negatives/immutable evidence и precision; concurrent resource+audit consistent snapshot; unknown-kind inventory; malformed known field refusal; catalog drift/read-only; audit immutability и отсутствие изменений от экспорта. Fixtures явно синтетические и не являются внешним НАП/HIL доказательством.

## Integrated verification — 2026-10-21

Actual main working-tree runners all exit0: evidence6 (112.07s), Fiscal8 and MiniPOS5. Versioned logs/source hashes: evidence/pg-profiles-2026-10-21. Other-profile SKIPs within an individual runner are covered by the separate actual DB runner, not counted as PASS in that process.
