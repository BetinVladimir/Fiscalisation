# Повторное ревью fixes / run_fixes.py — 2026-10-25

Проверено: цель MVP_GOAL, решения владельца, все 314 legacy-задач, тестовые и SCRIPT-карточки, новые S3/E2E задачи, генератор и execution controller. Продуктовый runtime, модель qwen, реальные устройства и эквайринг в этом ревью не запускались.

| Приоритет | Находка | Исправление |
|---|---|---|
| P1 | rc0 + напечатанный PASS/OK принимались за тестовый успех | Shell-приёмки запрещены; selector unittest разбирается строго; отдельный trusted controller пишет структурированный result после фактического прогона |
| P1 | skip/expectedFailure/пустой suite/no-assert/константный assert могли выглядеть зелёными | Проверяются testsRun, выполненные assertions, skips, expected/unexpected results, loader errors; простые константные asserts отвергаются |
| P1 | Сгенерированный слабый oracle всё ещё мог быть формально зелёным | Автостатус только TESTED_REVIEW_REQUIRED; окончательное ACCEPTED требует независимого review JSON, связанного с SHA текущего плана/workspace и полным набором задач |
| P1 | Resume и финальный gate могли менять исходники вне scope | До/после каждого тестового прогона сравниваются checkout и оригинальные репозитории; тестам запрещены source writes |
| P1 | Некоторые задачи требовали менять запрещённый status.json; весь execution ошибочно был запрещён | TG-017 стал aggregate gate, CL-02 исправляет реальный клиент; status/reverify/routing защищены, mvp-cut config разрешён в явном scope |
| P1 | BP-06 мог генерировать ответы вендора; ротация настоящих секретов смешана со статикой | BP-06/CL-01 external; SECURITY-ROTATION-01 — отдельное внешнее действие без выдуманного evidence |
| P1 | Изменяемые документы меняли условия старых prompt на resume | Source snapshots включены в plan.json; prompt использует их; исходные документы при создании workspace копируются из snapshot |
| P1 | E2E могли идти до harness/эмулятора | E2E-HARNESS-01 и профильные EMU prerequisites; W0-1 до кодогенерации; 3/3 обязательны |
| P1 | Транзакции/bootstrap/СУПТО поставлены слишком крупно; SMS-02 не имел явного шага переноса runtime | PERSIST-01: 6 границ транзакций; S3FIX-07: 3 части; SUPTO-01: 3 части; SMS-02: 2 дополнительных implementation units |
| P1 | Scope не разрешал реализацию импорта/момента УНП, а SCOPE-анализ не имел исходников для чтения | Дополнены NAP-04/POS-11/CL-02/SCOPE-02…06; shared-runtime source paths добавлены явно |
| P1 | При timeout/завершении могли оставаться native дочерние процессы | Отдельные process groups, TERM/KILL, bounded output; leaked children не считаются успехом |
| P2 | E2E evidence записывалось бы в checkout и ломало read-only acceptance | MVP1_ARTIFACT_DIR вне репозиториев, отдельные каталоги прогонов |
| P1 | Basename guessing перенаправлял Fiscal main_test.go в MiniPOS | Удалён fuzzy retarget; оставлены только явные проверенные moves и новые файлы в нужном модуле |
| P1 | Карточки захватывали следующий раздел ##; Datecs0xFF, clock/e-чек/48HIL противоречили решениям | Парсер обрезает карточку по следующему заголовку; canonical spec overrides для конфликтующих задач, HIL checker по20 маршрутам и capabilities |
| P1 | Git index/HEAD и часть sibling checkout не контролировались | Мониторятся все присутствующие sibling репозитории; staging/commit моделью изменяет git-state digest и останавливает очередь |
| P1 | TST-DEEP с повреждёнными путями превращались в docs/spec вместо native теста | Для всех14 задач заданы native deliverables, read-only source context и implementation prerequisites; отчёт/README не заменяет код теста |
| P1 | Последняя правка могла сломать уже прошедшую приёмку | Финальная перепроверка всего выбранного closure до выдачи review request; отдельный negative integration fixture |
| P2 | Retry error мог превысить бюджет prompt | Ошибка обрезается по общему бюджету12000 без изменения требований; максимум150 строк |
| P2 | Изменение executable mode не инвалидировало review hash | Mode включён в inventory digest |

## Покрытие

474 пакета / 711 подзадач. Все 314 ID трекера сохранены (code/external/alias). Отдельно: 36 SCRIPT, 66 тестовых карточек, 36 GAP E2E, аппаратная capability matrix, транзакции, S3 storage/recovery/sync, shared Android runtime, профильное СУПТО, refund recovery и финальная регрессия. Canonical E2E: tests/e2e/gaps и tests/e2e/scenarios. Физические и юридические gates не превращены в software PASS.

## Ограничения доказательства

Реальный запуск qwen 35B/64k и всего продукта не подтверждён этим ревью. Проверена структура/порядок/бюджет пакетов и механизм исполнения на локальных fixtures. Независимое семантическое ревью обязательно: нельзя автоматически доказать, что произвольный сгенерированный assertion покрывает бизнес-требование. Trusted controller не является защитой от намеренно вредоносного Python-кода; запускать только в изолированном workspace/test environment.

Проверка покрытия также машинная: coverage.legacy_tasks, script_tasks, gap_scenarios и required_routes в plan.json; отсутствие POS/BLE маршрута ломает validate. Это проверка полноты постановки, не результата реализации.

## Повторная проверка и исправления

- TST-S3-01…06 восстановлены: исправлен parser ID, добавлены native deliverables и implementation prerequisites.
- ARCH_07 R-01…06 теперь имеют явные canonical aliases либо актуальные R-02/R-04 tasks. Полнота ARCH source cards и таблиц тестов проверяется отдельно от legacy tracker.
- BUG-005: восстановлен полный контракт DEVICE_TIME/ProjectedResult, исправлен Sofia golden (2026-10-21T22:30Z -> 2026-10-22T01:30+03:00), исключён mqttclient/client.go вне Go-модуля.
- Любой subprocess/import больше не служит runtime-доказательством. Проверяются product calls, native counts/fresh JUnit либо локальный runtime I/O; version/no-op/fake test-wrapper отклоняются.
- Обязательный red -> восстановление production-мутации -> green с неизменными red-тестами. Native compile/import failures не заменяют воспроизведение дефекта. Старые receipts отклоняются.
- Проверки касаются плана и контроллера; продуктовая очередь и физический HIL здесь не выполнялись.

## Результат контрольного прогона

79 тестов контроллера/плана прошли, включая локальный runtime fixture и компиляцию native fixture вне checkout. Три новые регрессии отдельно воспроизвели ошибки на старой реализации (3 assertion failures, без ошибок среды).

Read-only audit: 310 ARCH-карточек, 316 нормализованных document IDs (включая E-1…6 -> EMU), 314 legacy ID, 86 тестовых строк ARCH_03/18, 36 SCRIPT, 36 GAP, 20 маршрутов — пропусков нет. Все generated packets воспроизводятся без изменений. Максимальная консервативная оценка начального prompt: 7878 токенов; stage header и retry также проверяются по общему лимиту12000. Это не замер tokenizer и не контроль всех будущих tool outputs.

## Независимая семантическая перепроверка

Сверены MVP_GOAL и последние решения DECISIONS, S3 intent/reserve/ready и фактический host runner. TST-S3-01 получает scope для общего host build и интеграции в make iot-test; остальные S3 тесты зависят от него. Реальный intent_processor.cpp должен компилироваться, hardware boundary doubles допустимы, копия бизнес-алгоритма запрещена.

Дополнительно закрыты ложные свидетельства: testdata/fixtures/src/test больше не production calls; TAP требует выполненные последовательные ok строки; Ran N без FAILED не является red; script exit 1 без assertion не является red; green native suite не подтверждает red. Локальный socket connect остаётся диагностикой: наблюдатель не устанавливает владельца порта, поэтому этот сигнал исключён из достаточной приёмки в run_fixes.py. Для сервиса требуется завершённый проверяемый product subprocess либо native suite.

Полнота ID не доказывает семантическую полноту реализации. Лимит prompt проверяется оценкой bytes/2; фактическая исполнимость qwen 35B требует пилотного запуска с настроенным 64k. HIL/legal/vendor gates остаются открытыми до внешних evidence.
