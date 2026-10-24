# fixes · пакеты задач по дефектам

Один файл = один дефект = вход для одной сессии OpenCode (qwen 64k). Генерация: `python3 docs/MVP1/tools/gen-fixes.py`. Порядок: P0 по номеру, затем P1, P2.

| Дефект | P | Кто | Код сейчас | ≈токенов | Суть |
|---|---|---|---|---|---|
| [BUG-001](BUG-001.md) | P0 | codex-first | PARTIAL | 3969 | знак теперь учитывается (−0.50→−50, −1.50→−150), но остались «+1.00», «01.00» и «1.-5»→95  |
| [BUG-003](BUG-003.md) | P0 | codex-first | PARTIAL | 5805 | Simulated берётся из s.simulated (SetSimulated); жёсткого true в service.go нет. Осталось: |
| [BUG-005](BUG-005.md) | P0 | codex-first | OPEN | 5352 | DeviceTime возвращает time.Now().UTC() |
| [BUG-020](BUG-020.md) | P0 | codex-first | OPEN | 2899 | нет отмены открытого чека при сбое |
| [BUG-021](BUG-021.md) | P0 | codex-first | OPEN | 2039 | компенсация карты при любой ошибке, независимо от unknown |
| [BUG-022](BUG-022.md) | P0 | codex-first | — | 1926 |  |
| [BUG-026](BUG-026.md) | P0 | codex-first | OPEN | 2342 | синтетический референс "DATECS-"+operationId |
| [BUG-028](BUG-028.md) | P0 | codex-first | OPEN | 3189 | классификация UNKNOWN по тексту сообщения |
| [BUG-029](BUG-029.md) | P0 | codex-first | OPEN | 1694 | DatecsSocketProtocol.execute: нет SYN/NAK/таймаута |
| [BUG-032](BUG-032.md) | P0 | codex-first | OPEN | 3275 | в манифесте нет сервиса и разрешений |
| [BUG-041](BUG-041.md) | P0 | codex-first | OPEN | 954 | now() возвращает 0 при часах раньше 2024-01-01, at=now() не проверяется |
| [BUG-050](BUG-050.md) | P0 | qwen | OPEN | 1236 | POS вызывает :reconcile, шлюз принимает /reconcile |
| [BUG-051](BUG-051.md) | P0 | codex-first | — | 1391 |  |
| [BUG-053](BUG-053.md) | P0 | codex-first | OPEN | 4109 | unp берётся с откатом на fiscalReference |
| [BUG-055](BUG-055.md) | P0 | codex-first | OPEN | 4235 | шлюз финализирует платёж как CASH |
| [BUG-063](BUG-063.md) | P0 | qwen | — | 317 | реализуется карточкой VG-01 (ARCH_09) |
| [BUG-064](BUG-064.md) | P0 | qwen | — | 317 | реализуется карточкой VG-02 (ARCH_09) |
| [BUG-065](BUG-065.md) | P0 | qwen | — | 317 | реализуется карточкой VG-03 (ARCH_09) |
| [BUG-066](BUG-066.md) | P0 | qwen | — | 317 | реализуется карточкой VG-04 (ARCH_09) |
| [BUG-071](BUG-071.md) | P0 | qwen | — | 1887 | reverse(): возврат карты до фискального сторно; решение D-12 storno-first |
| [BUG-072](BUG-072.md) | P0 | codex-first | OPEN | 1737 | нет блокировки между транспортами BlueCash |
| [SEQ-01](SEQ-01.md) | P0 | qwen | OPEN | 653 | S3 wrap 0xFF для Datecs |
| [BUG-002](BUG-002.md) | P1 | codex-first | OPEN | 1772 | validMoney по-прежнему принимает префикс +/− (service.go:1540-1542), хотя в статусе стояло |
| [BUG-004](BUG-004.md) | P1 | codex-first | — | 2889 |  |
| [BUG-006](BUG-006.md) | P1 | qwen | OPEN | 3133 | ReserveReceiptSaga нигде не вызывается кроме определений |
| [BUG-008](BUG-008.md) | P1 | codex-first | OPEN | 2137 | OriginalDocumentAt: original.UpdatedAt |
| [BUG-010](BUG-010.md) | P1 | codex-first | PARTIAL | 1398 | /readyz и /healthz → h.ready, /livez → h.live. Осталось: тест TestReadyzFailsWhenRepositor |
| [BUG-011](BUG-011.md) | P1 | qwen | PARTIAL | 1265 | проверки iss/aud/nbf/maxTTL (ParseTokenConfig) есть; осталось: откат на HMAC при ошибке OI |
| [BUG-012](BUG-012.md) | P1 | codex-first | — | 984 | TO VERIFY: start with hypothesis test |
| [BUG-023](BUG-023.md) | P1 | qwen | — | 1170 |  |
| [BUG-024](BUG-024.md) | P1 | qwen | OPEN | 2555 | require(current == null) остался |
| [BUG-025](BUG-025.md) | P1 | qwen | — | 744 |  |
| [BUG-027](BUG-027.md) | P1 | qwen | OPEN | 1379 | пустой ответ принимается как успех |
| [BUG-030](BUG-030.md) | P1 | qwen | — | 1734 |  |
| [BUG-040](BUG-040.md) | P1 | qwen | OPEN | 1061 | UNP резервируется до проверки готовности ФУ (строки 411 и 438) |
| [BUG-042](BUG-042.md) | P1 | qwen | OPEN | 971 | допустимы только CASH и CARD |
| [BUG-043](BUG-043.md) | P1 | qwen | OPEN | 724 | проверка сводится к total>0 |
| [BUG-052](BUG-052.md) | P1 | qwen | — | 895 |  |
| [BUG-054](BUG-054.md) | P1 | qwen | PARTIAL | 2295 | таймаут 4 с через AbortController есть. Осталось: разбор ответа и UNKNOWN при таймауте зап |
| [BUG-056](BUG-056.md) | P1 | qwen | — | 1487 |  |
| [BUG-057](BUG-057.md) | P1 | qwen | OPEN | 657 | пустой ключ заменяется uuid.NewString() |
| [BUG-058](BUG-058.md) | P1 | qwen | — | 535 |  |
| [BUG-059](BUG-059.md) | P1 | qwen | PARTIAL | 1246 | потребитель результатов использует inbox и ключ fiscal.operation.result.#. Осталось: скрип |
| [BUG-061](BUG-061.md) | P1 | qwen | — | 610 |  |
| [BUG-062](BUG-062.md) | P1 | qwen | — | 1906 |  |
| [BUG-067](BUG-067.md) | P1 | qwen | — | 317 | реализуется карточкой VG-05 (ARCH_09) |
| [BUG-068](BUG-068.md) | P1 | qwen | — | 317 | реализуется карточкой VG-06 (ARCH_09) |
| [BUG-069](BUG-069.md) | P1 | qwen | — | 317 | реализуется карточкой VG-07 (ARCH_09) |
| [BUG-070](BUG-070.md) | P1 | qwen | — | 317 | реализуется карточкой VG-09 (ARCH_09) |
| [BUG-031](BUG-031.md) | P2 | qwen | — | 308 |  |
| [BUG-044](BUG-044.md) | P2 | qwen | OPEN | 509 | PayloadConflict возвращает ошибку без result_sink_ |
