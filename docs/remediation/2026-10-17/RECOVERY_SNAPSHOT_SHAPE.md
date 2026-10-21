# Recovery snapshot: структурная проверка

2026-10-21. Дополнение к WEBHOOK_PERSISTENCE_RECOVERY.md. Изменение касается только recovery read после отказа persistence; правила initial constructor и миграции не меняются.

Предыдущая проверка json.Unmarshal различала синтаксическую ошибку, но считала JSON null, {}, unrelated object или объект с пропущенными коллекциями подтверждённым snapshot. При известном сохранённом фискальном факте это могло очистить memory и снять write barrier. Воспроизведение на прежнем recovery даёт RED; дополнительная baseline-aware проверка сохраняет подтверждённый snapshot и persistenceRecoveryPending.

Поддерживаемая структура:

- Корень — непустой JSON object с распознаваемой snapshot map collection. Корневые null, {}, unrelated-only object не подтверждают recovery.
- Domain serializer сохраняет12 map collections и sequence. Actual PostgreSQL rebuildSnapshot сохраняет9 core maps и sequence; исторически может опускать пустые tax_groups, identity_bindings и operator_sessions.
- Core collections: products, employees, shifts, orders, checkouts, checkout_hashes, api_replays, webhook_inbox, configurations. Каждая core collection, присутствовавшая в acknowledged baseline, должна присутствовать в recovery как JSON object. Sequence обязателен, если присутствовал в baseline; его значение — uint64, не null.
- Три исторически optional collections разрешено опускать только если acknowledged значение отсутствовало/было пустым. Пропуск ранее непустой optional collection отклоняется. Исторический null пустой optional map допускается; populated optional map не заменяется null.
- Для sparse legacy baseline не навязываются отсутствовавшие новые core/optional поля; уже acknowledged keys сохраняются. Initial constructor не переводится на новый mandatory schema.
- Легитимный первоначально пустой Store по ранее утверждённому контракту обрабатывается отдельно: nil,nil и отсутствие acknowledged непустого baseline, дополнительно generation0 для versioned/delta. Synthetic '{}' не становится acknowledged baseline.

Проверено13 shape cases: восемь повреждённых recovery форм; PG9/full12 совместимость; три populated optional omissions; sparse legacy compatibility; полный source backend PASS. Существующая65-case матрица Save/recovery/ambiguous COMMIT и initial empty остаётся в полном backend suite.

Локальные логи: /private/tmp/fiscal-recovery-shape-red.log, /private/tmp/fiscal-recovery-shape-green.log, /private/tmp/fiscal-recovery-shape-backend.log. PG9 fixture повторяет фактическую структуру rebuildSnapshot; новый тест не утверждает live PostgreSQL transport fault injection.

Граница: проверка базовой структуры не является доказательством истинности произвольного structurally-valid полного snapshot. Она не обнаруживает хранилище, возвращающее согласованную по форме, но ложную/устаревшую карту сущностей. Authoritative read contract остаётся обязательным.

## Integrated evidence

[Root manifest and logs](evidence/recovery-shape-2026-10-21/manifest.json): all13 structural scenarios, full source backend, domain/API race and actual `make postgres-integration` PASS. The PostgreSQL run covers disposable typed projections, fixture preparation and both products persistence/integration; it does not inject database network faults.
