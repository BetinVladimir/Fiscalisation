# Промежуточные evidence wave2

**SOFTWARE_GAPS_OPEN. Не release approval и не готовность к HIL.** Baseline commit `f5a6d1ff43ef9c01a0c8aa588319097e1c024c49`, последующие изменения в рабочем дереве. SOURCE_SHA256SUMS фиксирует файлы на момент архивации.

- IoT/factory gate PASS до добавления generator; отдельный factory-bundle.log подтверждает16 тестов после generator.
- S3 cross-build PASS после MQTT frozen batch, но до будущего canonical JSON fix. Сборка не доказывает совместимость с backend: GAP022 открыт и воспроизведён.
- MiniPOS backend,113 web tests и web production build PASS после первого CLOUD recovery; последующие followup сюда не включены.
- sync.log:10 actual SQLite/runtime сценариев.
- S3 manifest соответствует binary artifacts в локальном `/private/tmp/fiscal-wave2-s3-bundle-20261017/` и исходным `.pio/build/edge-agent-s3-idf/`. Binaries здесь не дублируются; доступность временного bundle не является долгосрочным хранением release artifacts.
- Для final gate нужен новый точный snapshot, полный required case inventory, долговременное хранение binaries и проверка всех оставшихся GAP.

При сохранении логов удалены только пробелы на концах строк. RAW_LOG_SHA256.json фиксирует SHA-256 исходных временных логов до этой нормализации.
