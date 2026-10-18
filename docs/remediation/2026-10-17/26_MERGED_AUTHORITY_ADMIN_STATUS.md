# Объединённый срез authority / ADMIN / quarantine

Дата: 2026-10-18. База d043368 плюс незакоммиченное рабочее дерево. Это промежуточная интеграция, не приёмка.

Интегрированы persistent terminal guard, S3/BlueCash read-only exports, MiniPOS delegated source identity/prepare/dispatch/browser PoP, typed authoritative denial cache revocation и отдельный root-signed ADMIN payment reconciliation. Два пересечения ADMIN/web в host runner и sync repository объединены; исправлены JSON fixture и выдача результата через result_sink. Ключи остаются в ESP32-S3 NVS, без ATECC608A.

## Проверено на основном дереве

- Backend domain/integration/API/MQTT PASS: `/private/tmp/fiscal-authority-admin-merged-backend-20261018.log`.
- MiniPOS backend `go test ./...` PASS: `/private/tmp/fiscal-authority-admin-merged-minipos-20261018.log`.
- MiniPOS TypeScript/Vite build PASS: `/private/tmp/fiscal-authority-merged-web-build-20261018.log`.
- S3 build PASS: RAM56716/327680, flash1999312/3145728; `/private/tmp/fiscal-authority-admin-merged-s3-20261018.log`.
- Terminal guard19 actual SQLite scenarios PASS; S3/BlueCash exporter16 tests PASS. Generated S3 dictionary обновлён по actual schema/source.
- Full web suite первоначально228/241PASS,13fail: три old checkout-path fixture и10Paper wrapper/button assertions. Исправлены stale callback production defect и Surface mock/font scaling expectations; targeted12testsPASS. Полный повтор web suite ещё не выполнен.

## Непройденные проверки и следующий шаг

Full IoT выявил устаревшую profile fixture (missing terminal ID и общий журнал независимых faultcases); она исправлена, targeted native saga PASS с явным Unknown→новаяCARD blocked assertion и CASH availability. Повторный merged gate дошёл до `money_host/money_native.cpp:45` и завершился exit2. Причина исследуется; gate не считается успешным. Лог `/private/tmp/fiscal-authority-admin-merged-iot-20261018.log`.

Нужны: устранение money fixture/runtime regression по доказательствам, новые checkout fixtures с настоящим prepared plan, полный merged IoT/web/browser/firmware interop, свежий source/build evidence bundle. ADMIN authenticated operator UI и полный MiniPOS JWT→HTTP issuer→UI→firmware pipeline ещё реализуются. Cloud PostgreSQL consistent extraction, offline revocation fence, cash/report/reversal permissions/contracts, native Android/iOS и реальный USB/COM/BLE HIL остаются открытыми. Программные тесты не подтверждают физический банковский/фискальный результат или применимость/одобрение НАП.

## Проверенный повтор после устранения регрессий

[Логи, source и build hashes](evidence/authority-admin-2026-10-18/README.md): full IoT exit0, backend/MiniPOS exit0, web241/241PASS, Chromium11/11PASS, webbuild exit0, contract gate exit0. Money fixture независимые журналы и actual restart quarantine проверены. TypeScript fixture guard добавлен. Contract smoke больше не требует удалённые Go-edge HTTP endpoints; сохранены S3 schema checks, actual S3 protocol tests и новые issuer/ADMIN route checks. Runtime base65→69: добавлены четыре reviewed authorization/dispatch/reconciliation операции.

Предыдущие неуспешные результаты выше сохраняют историю обнаружения, не являются текущим итогом этих gate. Полный deployed HTTP/UI pipeline всё ещё выявил реальные ApiClient missing API-version/idempotency header gaps; исправление отдельно. Native provider/checkout и ADMIN UI продолжаются. Это не общий SOFTWARE_COMPLETE и не HIL/НАП PASS.
