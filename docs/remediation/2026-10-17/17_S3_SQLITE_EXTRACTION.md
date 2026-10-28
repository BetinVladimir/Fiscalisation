# N5/N6: извлечение локальных данных ESP32-S3

Статус: **LOCAL_EXTRACTION_IMPLEMENTED / HYBRID_DOSSIER_OPEN**. Это технический комплект извлечения и словарь локальной БД, не юридическое заключение, подача или подтверждение полной готовности НАП.

## Инвентаризация и граница

Отдельного S3 SQLite extractor в исходном репозитории не обнаружено. Существующие `fiscal-backend/internal/domain/exports.go` и API periodized exports относятся к облачным представлениям продаж и не извлекают полный локальный SQLite. Новый `scripts/export_s3_sqlite.py` расширяет комплект инструментов dossier отдельным локальным exporter без изменения firmware runtime/schema.

Фактическая S3 БД задаётся в `IoT/firmware/edge-agent-s3/idf/main/app_main.cpp`: `/sdcard/beefiscal/edge-agent.db`. DDL и существующая миграция читаются непосредственно из `main/durable_storage.cpp`. Словарь [18_S3_SQLITE_DICTIONARY.generated.md](18_S3_SQLITE_DICTIONARY.generated.md) содержит все таблицы/поля, SQLite types/defaults/PK/FK/indexes, SQL definitions и хеш исходника. `schema_version=3` недостаточно для различения всех позднее добавленных таблиц: проверяются вся фактическая схема и source SHA256.

| Store | Связь в гибридной системе | Охват этого инструмента |
|---|---|---|
| S3 SQLite | operations/receipts/payments/payment saga; event outbox → frozen sync → backend ACK; local sales; UNP authority/claims | Полный текущий файл, все 15 прикладных таблиц и SQLite `sqlite_sequence`, включая исторические route generations, NULL/BLOB/raw text |
| S3 NVS | DeviceIdentity private key, signed/encrypted provisioning и конфигурация | Не извлекается. Ключи находятся внутри ESP32-S3 NVS; ATECC не используется |
| Fiscal/MiniPOS backend PostgreSQL и связанные persistent audit/event/object stores | Облачная проекция локальных событий и собственные cloud lifecycle/authorization/configuration данные | Не извлекаются; отдельные full-store dictionary/export и сверка проекций остаются OPEN. `database/fiscal`, `database/minipos` и actual repository mappings требуют отдельного inventory |
| Android BlueCash | `beefiscal-bluecash-journal.db`: journal_event, sync_checkpoint, payment_transaction, ble_local_sale, ble_unp_range; bootstrap/deployment preferences | Не извлекаются; source `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/AndroidJournal.kt` и preferences требуют отдельного инструмента |
| MiniPOSWeb и приложения администратора | IndexedDB/outbox, localStorage, native secure storage/preferences | Не извлекаются; актуальные device/app instances и tenant scopes входят в отдельный dossier inventory |
| Фискальный аппарат / POS-терминал | Собственная память документов/транзакций, RRN/auth references | Не извлекаются; локальная строка состояния не подменяет физическое доказательство |

Синхронизированные локальные строки могут уже быть удалены retention-политикой. Экспорт не восстанавливает удалённую историю и не доказывает полноту облачной истории. Хеши payload/event сохраняются как данные; инструмент не объявляет их подписи, цепочки ACK, полномочия или фактическую фискализацию проверенными. Вложенные JSON/text payloads сохраняются побайтно по SQLite storage class, но их прикладной JSON-field dictionary и cross-store reconciliation остаются отдельной частью N5/N6.

## Получение входной копии

1. Ответственный оператор фиксирует adapter/device ID, источник/версию firmware, дату и способ получения, владельца носителя и доступ. Остановить всех писателей и получить согласованную offline-копию по процедуре обслуживания устройства. Этот инструмент не добавляет удалённый dump/shutdown endpoint и не останавливает runtime.
2. Сохранить исходный носитель/образ и его хеши, включая возможные `-wal`, `-shm`, `-journal`. Не удалять sidecars и не запускать восстановление на единственном оригинале. При наличии sidecars exporter откажет: отдельная контролируемая процедура должна получить consolidated copy на рабочем дубликате с записью исходных и результирующих хешей.
3. Передать инструменту отдельный regular SQLite-файл. Флаг `--quiescent-copy` — явное заявление оператора, а не автоматическое доказательство отсутствия писателей. Инструмент проверяет отсутствие sidecars, источник до/после чтения и integrity; одних хешей недостаточно для доказательства аппаратного происхождения или корректности acquisition.
4. Доступ к выводу предоставлять как к полным коммерческим/персональным данным: export не редактирует строки и не является tenant-filtered API. Файл может содержать исторические данные разных binding contexts. Извлечение другого tenant/device store не выполняется; выбор самого приобретённого файла и право доступа контролируются процедурой владельца.

Не извлекать NVS/private keys для этого сценария. Для capture, содержащего действующие платежные/персональные данные, применяются подтверждённые правила доступа/хранения владельца; данный документ не устанавливает правовые сроки хранения.

## Исполнение

Нужен Python 3 со стандартным `sqlite3` (без внешних пакетов). Из корня принятой версии:

```sh
python3 -m unittest discover -s tests -p 'test_s3_sqlite_export.py' -v
python3 scripts/export_s3_sqlite.py dictionary --output /private/tmp/s3-dictionary-review.md
python3 scripts/export_s3_sqlite.py export \
  --database /private/tmp/acquired-s3/edge-agent.db \
  --output /private/tmp/s3-dossier-extract \
  --device-id 'ACTUAL-ADAPTER-ID' \
  --source-commit 'ACTUAL_FULL_40_OR_64_HEX_SOURCE_REVISION' \
  --acquisition-note 'ACTUAL acquisition record reference; operator; method; firmware artifact reference' \
  --quiescent-copy
python3 scripts/export_s3_sqlite.py verify --directory /private/tmp/s3-dossier-extract
```

Заменить значения-заглушки фактическими. `--source-commit` и device/acquisition metadata — заявленные оператором значения, не доверенная аттестация firmware. Хеш фактического schema source и хеш исполняемого Python source сохраняются отдельно. Конкретный binary/build и устройство связываются внешним acquisition/release evidence; exporter это не фабрикует. `--schema-source /path/to/durable_storage.cpp` перед subcommand допускает явный выбранный исходник; несовпадение всей DB schema вызывает отказ, миграция на месте не выполняется.

Экспортер открывает acquired copy с `mode=ro&immutable=1`, `query_only=ON`, `trusted_schema=OFF`, без extension loading. `immutable` допустим здесь только для заявленной неизменяемой offline-копии: это не live-backup. Источник не открывается на запись. Облако, BLE/MQTT и скрытые caches не используются. Файлы создаются с режимом 0600, каталог — 0700; существующий output не заменяется. `manifest.json` публикуется последним и служит маркером завершения. При ошибке незавершённые файлы инструмента удаляются, вход сохраняется.

Результат: `manifest.json`, `dictionary.json`, `dictionary.md`, отдельный JSONL для каждой таблицы. Manifest содержит SHA256/размер каждого payload artifact, точные количества строк, порядок колонок, integrity/FK diagnostics и явный `not_included`. `verify` обнаруживает изменение/пропуск/лишние файлы и расхождение inventories; он **не** аутентифицирует сам manifest, устройство или происхождение данных. Повторное вычисление хешей злоумышленником не является цифровой подписью. Применимый внешний evidence/trust workflow подключается отдельно.

## Формат и проверка полноты

Каждая JSONL-строка — массив значений в порядке колонок словаря. `integer.decimal` хранится строкой без потери 64-bit precision; `real.hex` сохраняет SQLite REAL; `text.value` — точный текст, включая NUL/newlines; `blob.base64` — точные bytes; `text_bytes.base64` сохраняет TEXT с невалидным UTF-8 с исходным storage class; `null` отличен от пустой строки. При независимом восстановлении в disposable DB `text_bytes` требует `CAST(? AS TEXT)`, не обычного BLOB binding. В production DB инструмент ничего не восстанавливает.

Порядок строк детерминирован по всем колонкам; duplicative rows не дедуплицируются. Таблица `sqlite_sequence` включена, поскольку AUTOINCREMENT high-water может превосходить текущий max key. Проверяются schema inventory и row counts в одной read transaction. Повреждённая DB/неизвестная schema/изменение входа вызывают отказ, а не неполный PASS. FK issues сохраняются вместе с данными: CLI code2, `EXPORTED_WITH_FOREIGN_KEY_ISSUES`; обычное извлечение — code0 `EXPORTED`; отказ — code1. `verify` может подтвердить целостность архива с исходными FK issues, но не объявляет их исправленными.

Тесты создают DB фактическим firmware DDL, заполняют каждую таблицу, извлекают все rows и восстанавливают в disposable SQLite с точным сравнением значений/типов/PK/FK; проверяют 64-bit values, invalid UTF-8 TEXT, BLOB/NULL/Unicode/NUL, AUTOINCREMENT metadata, неизменность readonly source, отказ при sidecars/schema drift/corruption/concurrent writer, isolation от соседних stores/NVS, no-clobber, tamper/omission/symlink и executable CLI. Фикстуры не являются данными реальных устройств.

## Критерии принятия и оставшиеся задания

Техническая предварительная приёмка локального комплекта: тесты PASS; generated dictionary совпадает с exact accepted source; CLI запускается без внешней БД/сервиса; вход не меняется; каждая таблица/колонка/строка извлечена либо весь экспорт отклонён; FK issues не скрываются; sensitive NVS не входит. Это не закрывает N5/N6 для всей системы.

До полного dossier нужны: контролируемая физическая acquisition rehearsal и внешний provenance; cloud и Android/web store inventories/extractors и access tests; словарь вложенных payloads; сверка local/cloud/history/retention/restore; installation/version/network inventory для конкретной поставки; independent review применимости и полноты требований НАП. N0/N1/N5/N6 и юридическая/организационная готовность остаются открытыми в соответствующей части.
