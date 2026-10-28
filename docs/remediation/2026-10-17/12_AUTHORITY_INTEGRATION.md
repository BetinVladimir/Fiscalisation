# Авторизация исполнения: промежуточная проверка

Дата: 2026-10-17. Статус: интеграция продолжается, общий gate ещё не подтверждён.

Проверен C++ verifier с настоящей mbedTLS из ESP-IDF и независимо подписанными Go-векторами. 17/17 сценариев PASS: valid, expired, future, long TTL, чужие tenant/binding/range/серийный номер кассы/номер фискальной памяти/operator/command/client key, подмена issuer signature/client proof/payload, конфликт operation alias и неканонический JSON. Подмена идентификаторов кассы проверяется с валидной подписью issuer: отказ обусловлен несовпадением binding. Временный лог: `/private/tmp/fiscal-execution-authorization-20261017.log`. До включения runner в репозиторий и общего прогона это локальное свидетельство, не завершённая приёмка.

Ключи находятся во внутренней NVS ESP32-S3; ATECC608A не используется.

## Открытые условия сквозной приёмки

- Включить actual issuer → verifier → IntentProcessor → SQLite тесты и воспроизводимый mbedTLS runner в обязательный gate.
- Исправлено в MiniPOS: отсутствующий backend `server_sale_id` остаётся null в envelope и canonical payload; surrogate хранится отдельно. Online canonical server_sale_id теперь содержит настоящий backend ID. Добавлены два regression сценария, full Vitest: 13 файлов, 121/121 PASS. Это не закрывает ещё не подключённые authorization/client proof.
- MiniPOS передаёт operator code как operator_id и не включает canonical operator_code. Требуются отдельные доверенные resource ID и код, сопоставленные с подписанной авторизацией и клиентской подписью.
- Backend online sale уже может иметь UNP, выданный до SALE_FINALIZE; firmware выделяет UNP из leased range. Нельзя молча заменить UNP или ослабить равенство при merge: нужен отдельный подписанный источник полномочий для существующего online UNP и тест отсутствия повторного расходования offline range.
- Отдельно доказать offline create/merge после рестарта, UNKNOWN → terminal, concurrent CAS, immutable authority и отказы при чужих identity/range/binding.
- После объединения выполнить общий software gate и S3 build на одном source snapshot. Реальные USB/COM/BLE испытания и внешнее подтверждение НАП остаются самостоятельными условиями.

## MiniPOS canonical wire: исправление и проверка

Вынесен `src/fiscal/goCanonical.ts`: порядок ключей по UTF-8 вместо localeCompare; Go-compatible HTML/U+2028/U+2029 escaping. Денежные значения и количества остаются decimal strings. Числа ограничены safe integers, невалидный Unicode и значения вне JSON-подмножества отклоняются до отправки. Это исключает подпись округлённых/непредставимых значений; расширение на дробные JSON numbers требует отдельного Go-compatible контракта.

Новый обязательный Vitest тест запускает настоящий Go `encoding/json` и сравнивает байты на 265 векторах, включая Unicode keys, HTML, управляющие символы и вложенную продажу. Full MiniPOS Vitest: 14 файлов, 123/123 PASS; `tsc --noEmit` PASS. Новая schema3 проверка `unp_prefix == fiscal_device_number` интегрирована; actual profile/money host runner PASS. Общий интеграционный gate ещё не выполнен.
