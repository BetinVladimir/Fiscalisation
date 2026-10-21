# MP004 — Source recovery authority

Статус: source-bound recovery и typed PostgreSQL projection интегрированы в bb91423; root verification приведён ниже. Source-less legacy migration остаётся OPEN. Это не HIL и не сертификация.

Ранее клиент назначал READ локально в трёх UNKNOWN ветках и искал journal только при OPEN shift. Теперь UNKNOWN содержит пустой список действий; отдельная кнопка запрашивает действующее разрешение сервера. GET `/minipos/orders/{order_id}/recovery` проверяет tenant, фактический shift возвращённого order, текущего оператора и живую operator session. Разрешение допускает только READ, не prepare/dispatch/retry платежа. Для гранта необходимы явные fiscal_register_id, checkout_operation_id, receipt_session_id и согласованный fiscal_operation_id; fallback исходного register не используется.

Readiness и snapshot order читаются под одним Service lock. После авторизации фактического snapshot проверяется epoch, меняющийся перед каждой попыткой сохранения, включая uncertain save. Успешная посторонняя запись тоже отменяет старый snapshot. Детерминированные тесты RED без финальной проверки epoch выдают ошибочный READ; GREEN возвращают503 без дополнительных Save.

Клиент сначала читает owner-scoped исходный order, затем точный recovery grant. Сопоставляются source ID, shift, register, operator, external ID, версия, текущий configured fiscal register и неизменяемый signed intent/план. Новые journal фиксируют source_context; legacy source-bound контекст выводится только из авторизованного source GET и гранта. Потерянный AUTHORIZING получает сохранённую ассоциацию через сервер без переписывания платежного плана. Отсутствующий root sale разрешается только GET конкретной операции исходного order. Существующий confirmCloudCheckout дополнительно проверяет root operation, исходный order, receipt, fiscal version и платежный digest перед удалением journal.

Поиск journal независим от OPEN shift. Старый CLOSED shift остаётся видимым UNKNOWN, даже если открыт другой shift; checkout и shift-toggle quarantined. Подтверждение старого journal сохраняет другой активный cart. При нескольких journal quarantine переходит к следующему. Потеря auth, смена сессии и ABA отменяют поздний результат и не удаляют journal.

Предварительная приёмка: owner получает только readonly capability; чужие tenant/operator/session отклонены; отсутствие гранта,403/401, malformed association и outage сохраняют journal; восстановление выполняет GET-only цикл; ни одна ошибка не повторяет финансовый POST. Source-less legacy остаётся явно OPEN: требуется проверяемая миграция/ручная сверка, произвольный root GET запрещён.

Проверки: Node208/208; configured UI19 suites160/160; TypeScript; generated TypeScript и request/response drift gates; actual Go HTTP→TypeScript interop и backend race suite. Полный backend race suite завершён успешно (MINIPOS_RECOVERY_NODE_INTEROP=1). Parent дополнительно расширяет Jest discovery nested UI tests; этот пакет не меняет Jest/helper файлы. Тесты используют synthetic fixtures, без устройств и внешних финансовых API.

Evidence: [manifest](evidence/source-recovery/manifest.json). Исторические RED logs сохраняют исходный обнаруженный дефект; финальные GREEN имеют отдельные файлы. SQL typed projection и actual PostgreSQL acceptance поставляются независимым persistence пакетом, не входят в этот patch.

## Integrated root verification — 2026-10-21

Root merged API/domain/client with the separate typed PostgreSQL whitelist projection. Node208, configured UI21suites/169tests, TypeScript, full MiniPOS Go race with actual HTTP→TS, and both generated contract drift gates PASS. All8 persistence/dependency hashes exactly match the actual PostgreSQL/RLS full race evidence. [Root logs and staged/working source hashes](evidence/source-recovery/root-integrated/manifest.json); [actual PostgreSQL evidence](evidence/typed-order-recovery-2026-10-21/README.md). Existing unrelated AppFrame/header changes were present in the working UI test environment and remain outside this package; staged and tested hashes distinguish that scope. Legacy journals without proven source identity remain quarantined, with migration/discovery work OPEN. Physical HIL and full BLE authority remain separate OPEN requirements.

## Legacy export: committed owner scope and privacy

Replacement legacy incremental binds export/report authority to committed auth/cloud/recovery context, pending journal ID, selected employee/operator and full fiscal route (register/configuration/binding). Each committed transition clears the report before paint. Delayed exports and retained callbacks cannot publish across auth, same-token configuration, operator or operator ABA changes. Reconciliation report publication uses the same guard.

Global journal presence is not ownership evidence. Unsigned/unowned source-less journals export only kind/version, journal hash, bounded status and enumerated reason. No amount, operation/receipt ID, journal ID, source ID or timestamps are emitted by default. Detailed export requires exact signed current operator/register/intent/receipt anchors; no implicit manager override is granted. Signed payloads, authorizations, signatures, client proof, JWT and session hashes remain excluded. Malformed monetary objects are rejected.

Persisted sidecars are strictly validated: fixed keys, SHA256 string, status/reason enum with compatible combinations, exact complete source_context for LINKED and no linkage for QUARANTINED. Unknown persisted reasons and arbitrary nested fields fail closed and are never echoed. Record hash must match frozen journal.

Acceptance: Node234/234, configured UI19 suites166/166, TypeScript PASS. Previous legacy export against the four new actual App delayed-export tests: auth case PASS, configuration/operator/operator ABA cases RED; corrected committed scope all GREEN. Node tests cover unowned financial privacy, malformed reason/status/context/digest, nested monetary objects and arbitrary private extras. Separate raw logs and hashes are in legacy evidence manifest. Physical HIL/native/browser acceptance remains separate.


Root integrated verification: Node234, configured UI21suites/175tests and TypeScript PASS. Source-less signed journals require unique server linkage and fresh monetary proof; records without sufficient evidence remain quarantined. [Root logs and staged/tested source hashes](evidence/legacy-source-recovery/root-integrated/manifest.json). Existing unrelated AppFrame/header changes remain outside this package and were present during UI verification.
