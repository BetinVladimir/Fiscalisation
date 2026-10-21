# Самодостаточный actual BLE transport CI

Дата:2026-10-21. Интегрирован test-only package `tests/ble-transport`; `make iot-cbor-encoder-interop-test` больше не требует npm install обоих приложений и соседнего BeeloyBackend. Production transport/codec/auth не подменяется. Полный UI/native build и HIL остаются отдельными gates. Ключи ESP32-S3/NVS; ATECC не используется.

Проверяется происхождение всех109 registry entries из двух actual app locks, точные pins/integrities и dependency graph. Обновление lock/origins воспроизводится versioned derive_lock.py. Новый temporary sandbox получает ровно11 actual source/test files с SHA до/после; UI aliases/configs/external file dependencies отсутствуют. Node22.14.0/npm10.9.2 проверяются до install; fresh npm ci использует пустые cache/node_modules и empty npm user/global configs, HOME не меняется.

Root независимо выполнил полный clean run из исходного репозитория после интеграции: producer12 synthetic int64/state rows → independent Go12 → actual MiniPOS waiter12 → все66 browser lifecycle tests PASS, оба named fixture cases PASS, skip0. GCC15, actual Go issuer/queue/processor/SQLite/CBOR/delivery, реальные cJSON/mbedTLS libraries; SDK publisher/radio boundaries fake. Это представление и доставка синтетических значений, не доказательство допустимости таких сумм финансовой операции. Все11 current source hashes повторно сверены.

16 отрицательных/helper gate tests PASS. Worker дополнительно выполнил реальный fixture-skipping counterfactual: Vitest exit0 при2passed/64skipped был отклонён gate; ambient npm11.5.2 отклонён до тестов. Эти counterfactual выполнял worker; root повторил все16 unit gates и clean positive vertical.

[Root evidence](evidence/transport-clean-root-2026-10-21/manifest.json), [dependency hashes](evidence/transport-clean-root-2026-10-21/dependency-summary.json), [actual browser report](evidence/transport-clean-root-2026-10-21/vitest.json). Публикуются только эти три safe файла; raw producer/npm logs и fixtures остаются локально. Workflow использует тот же явный allowlist.

S3 workflow дополнен pinned Node setup, transport gate, отрицательными тестами и path filters actual source graph/app locks/backend. Linux clean CI ещё требуется: успешный macOS run его не заменяет. Полный target rebuild после dispatch parse lifetime change также остаётся обязательным до обновления firmware release candidate.
