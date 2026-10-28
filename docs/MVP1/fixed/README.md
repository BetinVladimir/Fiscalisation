# Полная очередь кодогенерации MVP1

`plan.json` — исполняемый overlay над всеми 314 задачами `execution/status.json`, карточками ARCH_03/12/18 и дополнениями ревью. Исторический tracker не редактируется и его DONE не является разрешением пропустить проверку. INDEX содержит код, тесты, алиасы и внешние gates. Генерируемые пакеты исправляются через `build_plan.py` и `review_overrides.py`, затем пересобираются. Старый `tools/gen-fixes.py` делегирует новому генератору.

## Что входит

- Реализация и статические исправления всех обязательных путей, POS, MiniPOS, BeeFiscalApp и связанной инфраструктуры.
- TST-* из ARCH_03/18, SCRIPT-01…36 из ARCH_12 (отдельный namespace устраняет конфликт SCR-36).
- Явные S3FIX-01…07, PERSIST-01, MATRIX-01, SUPTO-01, REFUND-01, EVIDENCE-01.
- Написание тестов непосредственно в `tests/e2e`: 36 пакетов E2E-GAP-001…036, E2EMATRIX-01 и TST-E2E-01…26 (канонические сценарии в `tests/e2e/scenarios`). Логика всех GAP: [E2E_COVERAGE.md](E2E_COVERAGE.md), целевой документ: `tests/e2e/MVP1_COVERAGE.md`.
- Внешние vendor/legal/HIL задачи перечислены, но не исполняются моделью; пакет фиксирует их как external_requires. Это разрешает писать код на известных протоколах и тестировать LAB, но не закрывает внешний gate или путь MVP.

## Правила для qwen 3.6 35B / 64k

Одна подзадача — одна свежая сессия, максимум три попытки. Test-first шаги объединены с ближайшей реализацией, чтобы runner не требовал зелёного теста до исправления. Политика последнего решения владельца в каждом пакете приоритетнее исторической карточки. Алиасы не повторяют реализацию; BUG-070 → VG-09.

Входной prompt ограничен консервативной оценкой 12000 токенов (UTF-8 bytes/2); при превышении убирается дублированная карточка, а не требования подзадачи. Полная карточка доступна по точному пути. Для провайдера модели необходимо выделить контекст 65536: runner не меняет настройки Ollama/OpenCode. Читать максимум три исходника одновременно, крупные файлы — диапазонами; бюджет исходников примерно 12000 токенов, хвост ошибки до 12000 символов, ответ до 8000 токенов. Это планирование бюджета, не измерение tokenizer конкретного провайдера.

У каждой исполняемой подзадачи есть явные files и адресный unittest-harness в `tests/mvp1`. Его тоже пишет qwen: он запускает реальный native runner/production code/script и проверяет результат, число выполненных тестов и ожидаемые значения. Он не должен моделировать продукт вместо его вызова. Для документных шагов проверяются содержательные поля и связи. Legacy-команды сохранены как справка, чтобы ошибочный cwd, ls, grep или отсутствующий скрипт не стали автоматическим PASS. Независимое ревью harness обязательно: runner не доказывает семантическую полноту произвольного сгенерированного теста.

E2E-GAP запускаются адресно в `tests/e2e/gaps`, три раза подряд. Фейки допустимы только на внешних границах. Пропуски, пустые наборы, отсутствующие инструменты, timeout и поддельный текст PASS/OK — не доказательство. Доверенный `trusted_check.py` считает фактически выполненные unittest tests/assertions и проверяет отсутствие skips/expected failures; для code-пакетов требуется наблюдаемый запуск runtime/native subprocess или импорт production Python module. Продуктовые runtime/HIL gates не заменяются текстовыми проверками.

## Проверка без запуска модели

```bash
python3 docs/MVP1/fixes/run_fixes.py --validate
python3 docs/MVP1/fixes/run_fixes.py --list
python3 docs/MVP1/fixes/run_fixes.py --list --only E2E-GAP-018
python3 -m unittest discover -s docs/MVP1/fixes -p test_runner.py
```

Порядок топологический; `--only` включает зависимости. Неизвестная зависимость, цикл, пустая задача, пустые files/accept и выход пути за workspace — ошибка. Перед полным запуском проверить baseline среды: Python, Go, Android/Gradle, Node, C++/ESP-IDF, Postgres и broker fixtures по выбранным задачам. Runner ничего не устанавливает и не подключается к реальным ФУ/банку.

## Исполнение

```bash
python3 docs/MVP1/fixes/run_fixes.py --execute --create-workspace \
  --workspace-root /tmp/beeloy-mvp1-qwen --model provider/model
```

Создаёт detached git worktrees `Fiscalisation`, `POS`, `BeeloyBackend` из HEAD соответствующих исходных репозиториев, затем копирует проверенные fixes-пакеты. Не копирует незакоммиченный продуктовый код. Для уже подготовленного изолированного workspace убрать `--create-workspace`. Все sibling-пути ведут внутрь этого workspace; symlink в основной проект запрещён. Не запускать две очереди в одном workspace.

Возобновление — та же команда **без** `--create-workspace`. Receipts и логи находятся в `<workspace>/.mvp-fixes-state`. Изменение пакета останавливает resume; приёмка перепроверяется. Приёмочные тесты не могут менять checkout ни при первом запуске, ни при resume, ни в финальном gate. Evidence сохраняется в `MVP1_ARTIFACT_DIR` под state/artifacts/<unit>/<red|green>/<run>, вне checkout. При провале остаются изменения для разбора, автоматических reset/clean/commit/merge нет. Изменения вне files или исходного проекта останавливают очередь. Сгенерированный diff остаётся доступным для независимого ревью.

`--validate`/`--list` не создают директорий и не запускают модель. Прежние `--order`, `--routes`, `--accept-changed`, автоматические коммиты и пропуск по старым Fix-Task trailers удалены: они не обеспечивали корректный порядок, изоляцию нескольких репозиториев и повторную приёмку. Новая очередь использует только schema 2 plan.json.

Коды выхода: 0 — выбранный код прошёл тесты и независимое ревью; 1 — тесты не прошли; 2 — некорректный план/параметры/среда; 3 — ревью кода принято, но остались external/blocked; 4 — TESTED_REVIEW_REQUIRED. Автоматического финального PASS по выводу модели нет. Завершение генерации не означает независимое approve, физический HIL PASS, соответствие НАП или разрешение PROD.

## Перегенерация

```bash
python3 docs/MVP1/fixes/build_plan.py
python3 docs/MVP1/fixes/run_fixes.py --validate
```

Добавлять новые задачи в overlay можно без ручного изменения legacy status.json. Сопоставление legacy/новых пакетов хранится в source/depends/alias_of/external_requires. Не использовать старый tools/run-accept.py для этой очереди: он читает другой tracker и допускает пустой набор.


## Независимое ревью после генерации

Успех автоматических проверок записывается как `TESTED_REVIEW_REQUIRED`, не как окончательное принятие. В state создаётся `review-request.json` с SHA плана, SHA содержимого/mode файлов workspace и точным набором выбранных code-задач. Независимый reviewer проверяет diff, oracle, вызовы реального production runtime и полноту сценариев, затем создаёт **отдельный** JSON, сохраняя эти поля:

```json
{"schema":1,"plan_sha":"<из review-request.json>","workspace_sha":"<из review-request.json>","tasks":["<весь список из request>"],"verdict":"approve","independent":true,"reviewer":"<имя независимого reviewer>"}
```

Повторный запуск с `--review /absolute/path/review.json` перепроверяет тесты и принимает только ревью точного состояния кода. Qwen не должен создавать этот файл; self-approval с reviewer=qwen отклоняется. Это доверенный ввод оператора, не криптографическая аутентификация reviewer. Контроллер не является sandbox и не доказывает смысл произвольного теста: динамический tautology/фиктивный native runner выявляется независимым ревью. Не выдавать TESTED за compliance/HIL/PROD.

При изменении исходников, executable mode, плана или состава задач старое ревью не принимается. После timeout завершается process group; оставшийся дочерний процесс даже после rc0 делает запуск неуспешным. Решения/карточки prompt берутся из неизменяемого snapshot source_documents в plan.json: последующая правка документа в workspace не меняет старый fingerprint.

Первой выполняется W0-1: сбор исходной базовой линии. На этом этапе допустим отчёт с честными FAIL отдельных модулей; это доказательство сбора baseline, не успешности продукта. Далее выполняются адресные тесты; только REGRESSION-01 требует всю применимую регрессию. E2E-HARNESS-01 и соответствующие EMU-зависимости идут до E2E-сценариев. LEGAL/HIL/vendor answers, включая BP-06 и реальную ротацию ключей, остаются внешними gates.

Дополнительно: git index/HEAD входят в контроль изменений; модель не может staging/commit даже разрешённых исходников. Подбор целевого пути по одному basename отключён: существующий `main_test.go` другого приложения не становится целью нового Fiscal-теста. Карточки обрезаются по границам заголовков, конфликтующие старые clock/SEQ/e-чек/HIL критерии заменены canonical spec. Retry prompt с хвостом ошибки тоже укладывается в бюджет12000; весь packet/requirements сохраняется.


Повторное ревью: [REVIEW.md](REVIEW.md). Все TST-DEEP-01…14 имеют конкретные native test deliverables; повреждённые legacy пути не превращаются в docs/spec заглушки. Поле read_files разрешает только чтение (включая sibling source для статического анализа), files — изменения. Сценарии E2E зависят и от harness/EMU, и от задач реализации проверяемого поведения. В конце выбранной очереди runner повторяет **все** её приёмки: последняя правка не может оставить ранее зелёный тест сломанным и получить review request.

## Контролируемый red/green (receipts schema 3)

Каждая unit запускается в двух свежих сессиях модели. Red принимается только при выполненном unittest assertion failure: без loader/import/tool errors, skip, пустого набора или константных assert. Для runtime-задач также обязателен наблюдаемый вызов продукта или реально упавший native test. Снимки фактически изменённых red-исходников и их SHA сохраняются в приватном каталоге artifacts/<unit>/red/mutations/<attempt> для проверки чувствительности. Green запускается после red; существовавшие после red тестовые файлы нельзя менять. Их изменение останавливает очередь.

Для test-only задач поле `mutation_files` разрешает временную red-мутацию production-кода. Контроллер сохраняет исходные байты/mode scoped файлов, после red восстанавливает их и удаляет созданные red production-заглушки. Это восстановление только мутаций данной попытки в изолированном checkout; Git reset/clean не выполняется. Другие `read_files` остаются read-only. При timeout/нарушении scope работа останавливается для ручного разбора. Старые green-only receipts schema 2 не принимаются.

`python --version`, арифметика в тесте и один лишь import не подтверждают runtime. Контроллер наблюдает вызовы Python product-функций; успешный локальный I/O к запущенному product runtime; native test counts из Go JSON, GTest, unittest/pytest/Jest/CTest/TAP и свежих Gradle JUnit XML. Процессы должны захватывать stdout/stderr; Gradle запускается с `--rerun-tasks`. Python test-wrapper с нарисованной сводкой не является native evidence. Native scripts из IoT/tests принимаются только при совпадении с доверенным исходным runner. Для нового native бинарника вне checkout нужен наблюдаемый compile из production sources либо запуск существующего доверенного runner.

Raw native stdout/stderr и свежие JUnit XML архивируются в `MVP1_ARTIFACT_DIR`. Игнорируемые build/cache файлы native toolchain допустимы; acceptance не меняет контролируемые исходники. Native assertion failure отделён от compiler/import/environment failure. Эти проверки подтверждают исполнение, но семантика oracle и связь результатов с требованием всё равно требуют независимого ревью.

Полнота: `source_cards` содержит все ARCH-карточки с canonical ID/alias. Validator независимо извлекает ID из snapshots, проверяет все тестовые строки ARCH_03/18 (включая S3), dependencies, aliases, scopes и бюджет. Исторические R-01…06 имеют явные актуальные реализации/aliases; superseded Pxx layout не возвращает отменённые post-HIL исключения.

Проверка актуальных документов и воспроизводимости всех generated packets: `python3 docs/MVP1/fixes/audit_coverage.py`. Команда читает реальные документы, сравнивает snapshots, все ARCH/корневые task headings и таблицы тестов с очередью, затем пересобирает пакеты во временный каталог. Новая карточка, TST-S3 строка, изменённый документ или устаревший MD вызывают nonzero.

Обычный C/C++ assertion abort принимается как red только с runtime assert diagnostic и provenance наблюдаемого compile либо доверенного native runner. Compiler/static_assert ошибки этим доказательством не являются. Core dumps отключены в тестовом controller, чтобы падение не записывало файлы в checkout.

Для задач, читающих fixes/plan.json: выбирать нужные ID/поля через Python/json; не печатать весь manifest или escaped source_documents. Это данные скрипта, не целиком вход модели.

Наблюдение localhost socket — только диагностика, не доказательство владельца сервиса. Для runtime приёмки нужны product calls или завершённый native/product subprocess; для red — assertion failure продукта/native suite. TAP plan без последовательных выполненных ok строк не принимается. Testdata/fixtures/test helpers не являются production Python.

## Пилот одной подзадачи

`--pilot-unit` требует `--only` и явно пропускает зависимости. Результат всегда частичный `PILOT_TESTED_REVIEW_REQUIRED` (rc4), никогда `ACCEPTED`; `--review` запрещён. Не означает завершения задачи или MVP. Использовать отдельный checkout и замороженную копию источников, чтобы чужие параллельные правки не блокировали исходный guard. Пример:

```sh
python3 docs/MVP1/fixes/run_fixes.py --execute --only SEQ-01 --pilot-unit SEQ-01.1 --workspace-root /path/to/isolated --opencode-run-arg=--standalone --opencode-run-arg=--auto
```

Флаги OpenCode передаются после `run`; отдельный wrapper не нужен. `--auto` — явная настройка оператора для изолированного workspace. Provider limit.context=65536 не устанавливает Ollama num_ctx: перед пилотом проверить `/api/ps` context_length=65536. Полные stage logs сохраняются до 1 MiB, отдельно от хвоста ошибки12k, mode0600. Без обязательной независимой проверки смысла oracle нет приёмки.

Inventory кэширует SHA только пока совпадают inode/device/size/mtime/ctime/mode: изменение содержимого с восстановленным mtime, удаление и chmod обнаруживаются.

Если Qwen преждевременно реализовал fix в red, контроллер сохраняет предложение отдельно, восстанавливает точные baseline bytes и повторяет red на том же тесте. Только реальный assertion failure исходного продукта допускает green; просто зелёный прогон не принимается. Native wrappers должны использовать phase-independent `native_harness.check_native_tests`; шаблоны есть в SEQ-01/BUG-002. Бизнес-oracle всё равно проверяется независимо.

Дочерний PWD всегда равен resolved workspace cwd; OLD PWD удаляется из окружения. Иначе OpenCode 2 может привязаться к каталогу исходного процесса, несмотря на Popen(cwd=workspace).

Для unit с harness_template контроллер заранее создаёт ровно этот Python harness и запрещает его изменение моделью. Существующий отличающийся файл не перезаписывается: нужен review/archive. Новые API могут иметь минимальный явно объявленный scaffold, только если файл отсутствует; рабочий код не заменяется. Golden vectors неизменны и проверяются до модели. BUG-002 native Go тест загружает fixed JSON вместо ручного перепечатывания кейсов. Проверяется исполнение всех требуемых Go cases ещё до оценки rc: discovery/fixture error не служит red assertion.
