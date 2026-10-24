# fixes · пакеты задач по дефектам

Один файл = один дефект = вход для одной сессии OpenCode (qwen 64k). Генерация: `python3 docs/MVP1/tools/gen-fixes.py`. Порядок: P0 по номеру, затем P1, P2.

**Автозапуск:** `python3 docs/MVP1/fixes/run_fixes.py --model provider/model` — по одной свежей сессии OpenCode на дефект в отдельном git worktree, проверка, коммит, возобновление после остановки; `--list` показывает очередь, `--only ID` один дефект. Выполняются **все** дефекты локальной моделью, каждая подзадача — отдельной сессией (малый контекст), по умолчанию строго по номерам. Перед запуском закоммитьте `AGENTS.md` и `docs/MVP1/fixes/`.

| Дефект | P | Кто | Код сейчас | ≈токенов | Подзадач | Суть |
|---|---|---|---|---|---|---|
| [BUG-001](BUG-001.md) | P0 | qwen | PARTIAL | 5931 | 5 | знак теперь учитывается (−0.50→−50, −1.50→−150), но остались «+1.00», «01.00» и «1.-5»→95  |
| [BUG-003](BUG-003.md) | P0 | qwen | PARTIAL | 6140 | 6 | Simulated берётся из s.simulated (SetSimulated); жёсткого true в service.go нет. Осталось: |
| [BUG-005](BUG-005.md) | P0 | qwen | OPEN | 5427 | 6 | DeviceTime возвращает time.Now().UTC() |
| [BUG-020](BUG-020.md) | P0 | qwen | OPEN | 3808 | 5 | нет отмены открытого чека при сбое |
| [BUG-021](BUG-021.md) | P0 | qwen | OPEN | 2109 | 4 | компенсация карты при любой ошибке, независимо от unknown |
| [BUG-022](BUG-022.md) | P0 | qwen | — | 2027 | 4 |  |
| [BUG-026](BUG-026.md) | P0 | qwen | OPEN | 2401 | 3 | синтетический референс "DATECS-"+operationId |
| [BUG-028](BUG-028.md) | P0 | qwen | OPEN | 5947 | 5 | классификация UNKNOWN по тексту сообщения |
| [BUG-029](BUG-029.md) | P0 | qwen | OPEN | 2559 | 6 | DatecsSocketProtocol.execute: нет SYN/NAK/таймаута |
| [BUG-032](BUG-032.md) | P0 | qwen | OPEN | 3345 | 6 | в манифесте нет сервиса и разрешений |
| [BUG-041](BUG-041.md) | P0 | qwen | OPEN | 969 | 3 | now() возвращает 0 при часах раньше 2024-01-01, at=now() не проверяется |
| [BUG-050](BUG-050.md) | P0 | qwen | OPEN | 1253 | 3 | POS вызывает :reconcile, шлюз принимает /reconcile |
| [BUG-051](BUG-051.md) | P0 | qwen | — | 1406 | 4 |  |
| [BUG-053](BUG-053.md) | P0 | qwen | OPEN | 4124 | 5 | unp берётся с откатом на fiscalReference |
| [BUG-055](BUG-055.md) | P0 | qwen | OPEN | 4250 | 5 | шлюз финализирует платёж как CASH |
| [BUG-063](BUG-063.md) | P0 | qwen | — | 334 | 1 | реализуется карточкой VG-01 (ARCH_09) |
| [BUG-064](BUG-064.md) | P0 | qwen | — | 334 | 1 | реализуется карточкой VG-02 (ARCH_09) |
| [BUG-065](BUG-065.md) | P0 | qwen | — | 334 | 1 | реализуется карточкой VG-03 (ARCH_09) |
| [BUG-066](BUG-066.md) | P0 | qwen | — | 334 | 1 | реализуется карточкой VG-04 (ARCH_09) |
| [BUG-071](BUG-071.md) | P0 | qwen | — | 1962 | 4 | reverse(): возврат карты до фискального сторно; решение D-12 storno-first |
| [BUG-072](BUG-072.md) | P0 | qwen | OPEN | 1807 | 3 | нет блокировки между транспортами BlueCash |
| [SEQ-01](SEQ-01.md) | P0 | qwen | OPEN | 670 | 2 | S3 wrap 0xFF для Datecs |
| [BUG-002](BUG-002.md) | P1 | qwen | OPEN | 1830 | 3 | validMoney по-прежнему принимает префикс +/− (service.go:1540-1542), хотя в статусе стояло |
| [BUG-004](BUG-004.md) | P1 | qwen | — | 2892 | 3 |  |
| [BUG-006](BUG-006.md) | P1 | qwen | OPEN | 3121 | 3 | ReserveReceiptSaga нигде не вызывается кроме определений |
| [BUG-008](BUG-008.md) | P1 | qwen | OPEN | 2195 | 3 | OriginalDocumentAt: original.UpdatedAt |
| [BUG-010](BUG-010.md) | P1 | qwen | PARTIAL | 1455 | 3 | /readyz и /healthz → h.ready, /livez → h.live. Осталось: тест TestReadyzFailsWhenRepositor |
| [BUG-011](BUG-011.md) | P1 | qwen | PARTIAL | 1282 | 3 | проверки iss/aud/nbf/maxTTL (ParseTokenConfig) есть; осталось: откат на HMAC при ошибке OI |
| [BUG-012](BUG-012.md) | P1 | qwen | — | 1058 | 4 | TO VERIFY: start with hypothesis test |
| [BUG-023](BUG-023.md) | P1 | qwen | — | 1187 | 5 |  |
| [BUG-024](BUG-024.md) | P1 | qwen | OPEN | 2629 | 3 | require(current == null) остался |
| [BUG-025](BUG-025.md) | P1 | qwen | — | 816 | 3 |  |
| [BUG-027](BUG-027.md) | P1 | qwen | OPEN | 1451 | 3 | пустой ответ принимается как успех |
| [BUG-030](BUG-030.md) | P1 | qwen | — | 1806 | 4 |  |
| [BUG-040](BUG-040.md) | P1 | qwen | OPEN | 1078 | 3 | UNP резервируется до проверки готовности ФУ (строки 411 и 438) |
| [BUG-042](BUG-042.md) | P1 | qwen | OPEN | 988 | 3 | допустимы только CASH и CARD |
| [BUG-043](BUG-043.md) | P1 | qwen | OPEN | 741 | 3 | проверка сводится к total>0 |
| [BUG-052](BUG-052.md) | P1 | qwen | — | 912 | 2 |  |
| [BUG-054](BUG-054.md) | P1 | qwen | PARTIAL | 2312 | 3 | таймаут 4 с через AbortController есть. Осталось: разбор ответа и UNKNOWN при таймауте зап |
| [BUG-056](BUG-056.md) | P1 | qwen | — | 1504 | 2 |  |
| [BUG-057](BUG-057.md) | P1 | qwen | OPEN | 674 | 2 | пустой ключ заменяется uuid.NewString() |
| [BUG-058](BUG-058.md) | P1 | qwen | — | 552 | 2 |  |
| [BUG-059](BUG-059.md) | P1 | qwen | PARTIAL | 1263 | 4 | потребитель результатов использует inbox и ключ fiscal.operation.result.#. Осталось: скрип |
| [BUG-061](BUG-061.md) | P1 | qwen | — | 627 | 3 |  |
| [BUG-062](BUG-062.md) | P1 | qwen | — | 1923 | 3 |  |
| [BUG-067](BUG-067.md) | P1 | qwen | — | 334 | 1 | реализуется карточкой VG-05 (ARCH_09) |
| [BUG-068](BUG-068.md) | P1 | qwen | — | 334 | 1 | реализуется карточкой VG-06 (ARCH_09) |
| [BUG-069](BUG-069.md) | P1 | qwen | — | 334 | 1 | реализуется карточкой VG-07 (ARCH_09) |
| [BUG-070](BUG-070.md) | P1 | qwen | — | 334 | 1 | реализуется карточкой VG-09 (ARCH_09) |
| [BUG-031](BUG-031.md) | P2 | qwen | — | 325 | 1 |  |
| [BUG-044](BUG-044.md) | P2 | qwen | OPEN | 526 | 1 | PayloadConflict возвращает ошибку без result_sink_ |
