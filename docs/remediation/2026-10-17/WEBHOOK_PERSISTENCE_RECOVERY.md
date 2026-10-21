# Source webhook: восстановление после отказа persistence

2026-10-21. Дополнение к ограничению webhook в 44_NATIVE_CLOUD_CONFIRMATION.md.

Все ProcessFiscalWebhook wrappers сходятся в ProcessFiscalWebhookLinkedForTenantWithReference. Три стадии сохраняются отдельно: входящее событие, проекция source order, отметка обработки. Первоначальное подозрение, что mutation-before-persist всегда оставляет ложный COMPLETED, не подтвердилось: persistLocked уже вызывает restoreLocked, и обычные отказы на каждой стадии корректно восстанавливаются.

Воспроизведён реальный дополнительный отказ: Save ошибается, затем recovery Load тоже ошибается или возвращает пустой/повреждённый snapshot. Старый restoreLocked молча оставлял изменённую память. Следующее сохранение ошибки inbox могло записать такой недоказанный COMPLETED в хранилище.

Save может выполнить COMMIT, затем вернуть timeout. Один fallback без блокировки новых записей небезопасен: сохранение ошибки inbox может стереть уже сохранённый COMPLETED в обычном Store. Сценарий воспроизведён RED; versioned CAS защищался конфликтом generation, но generic интерфейс этого не гарантирует.

Исправление сохраняет последний подтверждённый snapshot также для обычного Store. При неудачном recovery read restoreLocked использует этот snapshot; generation из невалидного recovery ответа не принимается. Fallback является только последним подтверждённым состоянием памяти. Пока authoritative reload недоступен, persistenceRecoveryPending запрещает новые Save, включая запись ошибки inbox. Первый успешный recovery reload заменяет память, принимает подтверждённую generation и возвращает retry-required без записи текущей speculative mutation. Следующий повтор использует authoritative state. Успешный свежий reload остаётся приоритетным. Ошибка только последней отметки inbox не отменяет уже сохранённые фискальные факты: COMPLETED, номер и версия сохраняются; повтор завершает inbox без повторного fiscal dispatch.

Предварительная приёмка:

- Четыре публичных wrapper × три стадии Save: ошибка возвращена; память соответствует durable order; неподтверждённая отметка ProcessedAt не видна; повтор проецирует ровно один раз; processed replay ничего не записывает.
- Три стадии × recovery error/empty/corrupt: недоказанный успех не попадает в память или последующее сохранение inbox; durable успех третьей стадии сохраняется; повтор и restart согласованы.
- Versioned store × три невалидных recovery ответа: чужая/недоказанная generation не принимается; retry проходит с подтверждённой generation.
- COMMIT → timeout на трёх стадиях с recovery error/empty/corrupt: plain, versioned и delta stores не выполняют дополнительного Save до authoritative recovery. Durable bytes неизменны; номер и версии сохраняются; retry/restart/replay согласованы. Проверен ambiguous первый Save при первоначально пустом snapshot.
- Runtime Postgres реализует DeltaVersionedStore; SaveVersioned и SaveDeltaVersioned возвращают ошибку tx.Commit, которая может означать неопределённый исход. Barrier применяется и к этому пути; реальные сетевые разрывы PostgreSQL здесь не эмулировались.
- Focused regression: 65 cases/subcases PASS. Полный source backend `go test ./... -count=1`: PASS. PostgreSQL outage/HIL не заявлены проверенными данным software fixture.

Граница Store contract: успешный Load(nil,nil) означает authoritative первоначально пустое хранилище только если ни один непустой snapshot ещё не подтверждён. Для versioned/delta дополнительно обязательна generation=0. Отдельный snapshotEstablished фиксирует только реальные успешные Save/Load; synthetic fallback '{}' не создаёт acknowledged baseline. После любого подтверждённого непустого snapshot пустой recovery read остаётся невалидным. Так definite первый Save failure + LoadERROR → последующий healthy empty не требует рестарта процесса и не блокируется навсегда.

В generic Store невозможно отличить действительно пустой successful read от устаревшего/ложного nil,nil после скрытого COMMIT первого Save: наблюдения идентичны. Поддерживается описанный authoritative read contract; stale/inconsistent successful reads, нарушающие его, не считаются доказанно безопасными. Production Postgres LoadVersioned читает snapshot и generation в одной repeatable-read транзакции. Данные тесты моделируют контракт и ошибки, не доказывают сетевое поведение живого PostgreSQL.

Добавлены definite first-write failure → healthy empty/retry/restart для plain/versioned/delta, включая предшествующий recovery error/corruption; ambiguous committed first-write + true read error/recovery/restart для всех трёх; initial empty с nonzero generation отклоняется.

RED definite failure: /private/tmp/fiscal-webhook-boundary-red.log; ambiguous COMMIT: /private/tmp/fiscal-webhook-ambiguous-red.log. GREEN: /private/tmp/fiscal-webhook-ambiguous-green.log. RED initial empty: /private/tmp/fiscal-webhook-empty-red.log. Финальный GREEN: /private/tmp/fiscal-webhook-empty-green.log. Полный backend: /private/tmp/fiscal-webhook-empty-backend.log. Runtime изменение ограничено persistLocked/restoreLocked; webhook не отправляет новых фискальных команд.

## Root integration evidence

[Manifest and raw-log hashes](evidence/webhook-persistence-2026-10-21/manifest.json): full source backend and `go test -race ./internal/domain ./internal/api` PASS. Focused runtime fault tests PASS. `make postgres-integration` also PASS: disposable typed projection, fixture preparer and both products persistence/integration. This is a real PostgreSQL regression run, not PostgreSQL network-outage fault injection.

Further structural validation remains OPEN: syntactically valid JSON `null` or an object missing mandatory snapshot collections may currently decode as empty state. The present tests establish the documented Save/Load error, empty-byte and syntax-corruption boundaries; they do not establish arbitrary persisted-object integrity.
