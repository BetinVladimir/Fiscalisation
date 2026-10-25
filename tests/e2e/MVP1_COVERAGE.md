# План E2E: GAP-001…036

Назначение: tests/e2e. Это спецификация генерации, не отчёт PASS.

| GAP | Логика Given / When / Then | Пакет |
|---|---|---|
| GAP-001 | MQTT backend publish -> реальный ingress S3/Android; exact tenant/device topic, msg_id; один результат и документ; неправильный namespace не принимается. | [E2E-GAP-001](../../docs/MVP1/fixes/E2E-GAP-001.md) |
| GAP-002 | S3 batch отправлен, business ACK доставлен в реальный ACK handler; checkpoint двигается один раз; дубликат ACK не удаляет новые события; неверный hash отвергается. | [E2E-GAP-002](../../docs/MVP1/fixes/E2E-GAP-002.md) |
| GAP-003 | BeeFiscalApp onboarding -> fake BLE peripheral с production BFPE parser; authenticate, Wi-Fi, binding, restart; крипто-заглушка запрещена. | [E2E-GAP-003](../../docs/MVP1/fixes/E2E-GAP-003.md) |
| GAP-004 | Factory reset -> CA/trust/network/secrets -> TLS broker -> READY; неверный CA или потеря trust записи блокирует деньги; проверить restart. | [E2E-GAP-004](../../docs/MVP1/fixes/E2E-GAP-004.md) |
| GAP-005 | Provisioning payload больше MTU23: фрагментация и сборка без oversized write; потеря/перестановка фрагмента не применяет partial config. | [E2E-GAP-005](../../docs/MVP1/fixes/E2E-GAP-005.md) |
| GAP-006 | Production advertising builder сериализует реальные поля; payload <=31 bytes, scan response разрешён; scanner находит UUID/name; overflow -> отказ. | [E2E-GAP-006](../../docs/MVP1/fixes/E2E-GAP-006.md) |
| GAP-007 | MTU23/247, payload1KB через production BLE codec; exact bytes; lost fragment -> timeout/UNKNOWN после записи, без повторного денежного send. | [E2E-GAP-007](../../docs/MVP1/fixes/E2E-GAP-007.md) |
| GAP-008 | Два BLE клиента/сессии: чужой origin/proof, stale nonce, replay и переключение владельца; ни одного unauthorized execute, законная сессия работает. | [E2E-GAP-008](../../docs/MVP1/fixes/E2E-GAP-008.md) |
| GAP-009 | Daisy production codec + 17 golden frames, Cyrillic и STAT; bad BCC/length отвергаются; full cash receipt через эмулятор USB. | [E2E-GAP-009](../../docs/MVP1/fixes/E2E-GAP-009.md) |
| GAP-010 | Datecs Kotlin/C++ production codec: один oracle LEN=data+0x2A, TX/RX/BCC/status и битые кадры; несоответствие хотя бы байта ломает тест. | [E2E-GAP-010](../../docs/MVP1/fixes/E2E-GAP-010.md) |
| GAP-011 | Serial SYN, NAK, late frame, silent deadline: byte-identical resend только по протоколу; timeout after write UNKNOWN; no double document. | [E2E-GAP-011](../../docs/MVP1/fixes/E2E-GAP-011.md) |
| GAP-012 | 100+ Datecs и 225+ Daisy команд, wrap/restart/cache; корректные SEQ/CP1251/slots, новая команда не получает cached чужой ответ. | [E2E-GAP-012](../../docs/MVP1/fixes/E2E-GAP-012.md) |
| GAP-013 | BluePad wake/flow/chunks production adapter: completion каждого chunk; disconnect между chunks -> UNKNOWN/NotSent по реальной границе записи. | [E2E-GAP-013](../../docs/MVP1/fixes/E2E-GAP-013.md) |
| GAP-014 | BluePad два operation IDs, reordered/late responses и STAN/RRN/AuthID; чужой result не завершает операцию; missing STAN -> unresolved/manual. | [E2E-GAP-014](../../docs/MVP1/fixes/E2E-GAP-014.md) |
| GAP-015 | S3 negative price/qty/tax/discount/payment sum: rejected до reservation/card/fiscal I/O; golden1367 проходит. | [E2E-GAP-015](../../docs/MVP1/fixes/E2E-GAP-015.md) |
| GAP-016 | Reserve -> kill -> reboot с той же SD DB; тот же UNP с 7-digit sequence, тот же operation ID; нет повторной покупки/печати. | [E2E-GAP-016](../../docs/MVP1/fixes/E2E-GAP-016.md) |
| GAP-017 | Каждый storage write/flush/checkpoint возвращает ошибку по очереди; денежные calls=0 после сбоя; prior committed evidence сохранено. | [E2E-GAP-017](../../docs/MVP1/fixes/E2E-GAP-017.md) |
| GAP-018 | Kill между terminal write/outbox write/commit; после рестарта оба либо ни одного; delivery повторяется, device execution нет. | [E2E-GAP-018](../../docs/MVP1/fixes/E2E-GAP-018.md) |
| GAP-019 | 2 CARD + CASH, отдельные payment IDs; один approved, другой timeout, reboot; evidence и компенсация индивидуальны, approved payment не повторяется. | [E2E-GAP-019](../../docs/MVP1/fixes/E2E-GAP-019.md) |
| GAP-020 | Recovery запущен при USB/UART not ready и затем чужом open receipt; ждёт readiness/ownership; чужой чек не отменяется. | [E2E-GAP-020](../../docs/MVP1/fixes/E2E-GAP-020.md) |
| GAP-021 | ФУ success с реальным номером и success без номера; первый сохраняет evidence, второй UNKNOWN; synthetic reference никогда не FISCALIZED. | [E2E-GAP-021](../../docs/MVP1/fixes/E2E-GAP-021.md) |
| GAP-022 | Один sync event C++ -> Go: exact canonical bytes/hash, Cyrillic/quotes/newline/amount, порядок map не меняет digest. | [E2E-GAP-022](../../docs/MVP1/fixes/E2E-GAP-022.md) |
| GAP-023 | Batch отправлен, ACK потерян, новые события добавлены и reboot; resend exact batch ID/hash/body; ACK удаляет только исходный batch. | [E2E-GAP-023](../../docs/MVP1/fixes/E2E-GAP-023.md) |
| GAP-024 | Все команды и results по schema backend/S3; sale ID vs operation ID, UNKNOWN и device association сохраняются; invalid envelope -> structured refusal. | [E2E-GAP-024](../../docs/MVP1/fixes/E2E-GAP-024.md) |
| GAP-025 | Expired command, full queue/SD и retention cutoff; expired не исполняется, unacked не вытесняется; bounded storage и безопасный отказ. | [E2E-GAP-025](../../docs/MVP1/fixes/E2E-GAP-025.md) |
| GAP-026 | MiniPOS Web после submit теряет ответ, переключает MQTT/BLE; только lookup тем же key/digest, submitCalls==1. | [E2E-GAP-026](../../docs/MVP1/fixes/E2E-GAP-026.md) |
| GAP-027 | Native checkout: kill/background после durable outbox и после BLE write; restart/resume/sync не повторяют документ/покупку и сохраняют pending. | [E2E-GAP-027](../../docs/MVP1/fixes/E2E-GAP-027.md) |
| GAP-028 | MiniPOS production backend PostgreSQL full regression с webhook-before-link, dispatch timeout, restart; no downgrade SUCCEEDED/no lost result. | [E2E-GAP-028](../../docs/MVP1/fixes/E2E-GAP-028.md) |
| GAP-029 | Чистая сборка Android production composition и startup runtime service; реалистичный lifecycle; README commands исполняются в CI, demo/stub не попадают в prod. | [E2E-GAP-029](../../docs/MVP1/fixes/E2E-GAP-029.md) |
| GAP-030 | EMU evidence имеет явный kind=EMU; HIL verifier отказывает при предъявлении EMU вместо physical, отсутствии фото/raw logs/device firmware; HIL PASS не синтезировать. | [E2E-GAP-030](../../docs/MVP1/fixes/E2E-GAP-030.md) |
| GAP-031 | Manufacturing flash manifest app vs merged: адреса/partition/hash; неверный тип образа отклоняется до flash; физическую прошивку не выполнять. | [E2E-GAP-031](../../docs/MVP1/fixes/E2E-GAP-031.md) |
| GAP-032 | Tremol вне MVP: capability/route запрещены явным structured error до I/O; не реализовывать неподтверждённый Tremol adapter. | [E2E-GAP-032](../../docs/MVP1/fixes/E2E-GAP-032.md) |
| GAP-033 | Smart S production HTTP port + FakeSmartFiscalServer/Bridge: деньги из cents, drop after body UNKNOWN без второй печати; LAB без номера, PROD UNKNOWN; refund manual. | [E2E-GAP-033](../../docs/MVP1/fixes/E2E-GAP-033.md) |
| GAP-034 | Подписанный evidence bundle с ключом из самого bundle; незнакомый ключ отвергается внешним trust store; доверенный fixture принимается, tamper отвергается. | [E2E-GAP-034](../../docs/MVP1/fixes/E2E-GAP-034.md) |
| GAP-035 | Подпись результата валидна, но tenant/device/binding generation чужие или устарели; result не применяется; верный immutable association применяется один раз. | [E2E-GAP-035](../../docs/MVP1/fixes/E2E-GAP-035.md) |
| GAP-036 | Device reference содержит quotes, backslash, Cyrillic и длинную строку; production JSON serializer не обрезает/не портит; restart сохраняет exact reference. | [E2E-GAP-036](../../docs/MVP1/fixes/E2E-GAP-036.md) |

Общие требования: реальный production path; fake только внешней границы; lost response после записи; counters на fiscal/purchase/refund; exact operation ID/digest/UNP; crash/restart; deterministic oracle; 3 из 3; нет SKIP/синтетического HIL PASS.

Дополнительные разрывы: E2EMATRIX-01 (20 маршрутов и capabilities), SUPTO-01 (профили и архив/экспорты), REFUND-01 (crash refund), EVIDENCE-01 (внешний trust anchor), TST-E2E-01…26 (продажа, скидки, НДС, сторно, offline, Smart S, BluePay, POS BLE).
