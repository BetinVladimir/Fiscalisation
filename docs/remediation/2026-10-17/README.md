# Fiscalisation: аудит и задания на доработку

## Уточнение текущего контура

По прямому запросу пользователя отдельный Go-модуль `edge-agent/` удалён из репозитория. Его команды, генераторы и ссылки на тестовые evidence исключаются из действующего контура. Исторические результаты не подтверждают готовность S3. Текущий IoT runtime — `IoT/firmware/edge-agent-s3/idf`; обязательны actual-runtime тесты и сборка этой прошивки. Go oracle в `idf/tests/go_wire_host` использует `fiscal-backend/internal`, а не удалённый модуль, и остаётся применимым. Недостающие эквивалентные проверки S3 остаются OPEN до реализации, а не закрываются удалением reference tests.

## Текущий интеграционный срез — 2026-10-21

- [Статус native/ADMIN/экспорта и открытые ограничения](28_INTEGRATED_NATIVE_ADMIN_STATUS.md): actual ADMIN HTTP → браузер → S3 processor/SQLite PASS; iOS OS-key → issuer → S3 interoperability PASS. Физический BLE этими стендами не проверен.
- [Обезличенный evidence ADMIN и S3 authority boundary](evidence/admin-http-2026-10-21/README.md): логи, SHA256 исходников и границы выводов. Authority boundary дополнительно прошёл AddressSanitizer/UndefinedBehaviorSanitizer без ошибок.
- Независимые профили PostgreSQL: [MiniPOS](24_MINIPOS_POSTGRES_EXTRACTION.md) и [Fiscal](21_FISCAL_POSTGRES_EXTRACTION.md); [миграция audit predecessor](29_AUDIT_PREDECESSOR_MIGRATION.md).
- [Android OS-key → issuer → S3](34_ANDROID_OS_INTEROP.md): offline и online interoperability прошли; это instrumentation harness, окончательная сборка приложения и физический BLE ещё не приняты.
- [Интеграция UI](36_UI_INTEGRATION_RECHECK.md): отдельные auth/operational/presentation проверки прошли. Последующий baseline215/33 исправлен: [обычный набор249/249 и свежий merged API127/127](40_ORDINARY_BROWSER_REGRESSIONS.md) прошли. [Статическая матрица](39_UI_STATIC_ACCEPTANCE_REVIEW.md) сохраняет явные OPEN требования; общий UI/release gate остаётся OPEN.
- [Canonical PostgreSQL и typed audit](37_POSTGRES_CANONICAL_INTEGRATION.md): миграция022,46 SQL-проверок, persistence обоих продуктов, три профиля досье и полный two-compose E2E прошли. Логи и хеши сохранены; production rollout внешнего BeeloyDB и полная нормативная модель не подтверждены.
- [Management authority](42_NATIVE_MANAGEMENT_AUTHORITY.md): actual MiniPosApp guards закрывают role/configuration404, retained callbacks и поздние write/report результаты; Node147/UI127/TypeScript PASS. Отдельно проверяется привязка HTTP401 к сессии запроса.
- [CLOUD checkout: authoritative source proof](44_NATIVE_CLOUD_CONFIRMATION.md): actor-scoped operation/order GET, безопасный payment commitment, долговечное положительное наблюдение; Node147, merged UI114 и реальный Go HTTP → TS helper PASS. Подтверждение BLE и полный offline цикл остаются OPEN.
- [Стартовая энтропия ключа S3](45_S3_STARTUP_ENTROPY.md): actual mbedTLS identity/DRBG и target IDF build PASS; источник энтропии включается до RF, ключ сохраняется во внутренней NVS. Происхождение ранее выпущенных ключей, SDK security baseline и физические испытания отдельно OPEN.
- Открытые программные требования: [электронные чеки](30_ELECTRONIC_RECEIPT_GAP.md), полный offline equivalence, отзыв автономных разрешений и защищённый BLE; полная программа HIL и внешняя нормативная приёмка также открыты.
- [Официальная процедура НАП и ограничения актуальности](16_NAP_OFFICIAL_PROCEDURE_RECHECK.md) повторно проверены 2026-10-21. Ключи — внутри NVS ESP32-S3; ATECC608 не используется.

- [Стандартный защищённый BLE: варианты и feasibility](46_BLE_STANDARD_CHANNEL_OPTIONS.md): actual TLS1.3 memory-BIO/Go interoperability и отрицательные проверки сертификатов; Android/iOS пока compile/link, браузер — synthetic WASM, production GATT/authority OPEN.
- [Первичные нормативные источники и недостающий п.24](49_NAP_PRIMARY_REQUIREMENTS_RECHECK.md): исторические локальные копии помечены; полнота нормативного baseline по дате выпуска остаётся OPEN.

Ниже — исторические срезы; их pending-формулировки не заменяют текущий статус выше.

## Результаты реализации на 2026-10-18

- Проверенный и отправленный пакет `e66b4f7`: [online/offline authority, MQTT validity, serial-ledger](14_ONLINE_AUTHORITY_AND_MQTT_VALIDITY.md), [логи и build/source hashes](evidence/authority-2026-10-18/README.md). Полные IoT и backend domain/MQTT/API проверки и S3 build завершились exit 0 для этого среза.
- Следующая незакоммиченная волна: [provisioning, typed S3 activation, BlueCash canonical wire и int64](15_PROVISIONING_AND_BLUECASH_INTEGRATION.md). Полный IoT gate, interop/App/backend, S3 build и Android BlueCash 43 tests/build PASS; [новые логи и hashes](evidence/provisioning-2026-10-18/README.md). Предыдущие hashes не доказывают эту волну.
- MiniPOS authorization/client proof и доверенная делегация операторской сессии ещё реализуются. MiniPOS order ID не является fiscal sale ID. Реальный Android/iOS USB/COM/BLE HIL, legacy authority fence и внешние требования НАП остаются открытыми.
- Ключи текущей версии находятся внутри ESP32-S3; внешний ATECC608A не используется.

- Следующий проверенный срез: [BluePad, SQLite extraction и route revocation](20_BLUEPAD_SQLITE_AND_REVOCATION.md), [логи и hashes](evidence/bluepad-sqlite-2026-10-18/README.md). IoT/S3/backend PASS; persistent quarantine и trusted ADMIN/MiniPOS flow ещё не закрыты.

- [Объединённый authority/ADMIN/quarantine срез](26_MERGED_AUTHORITY_ADMIN_STATUS.md): IoT/backend/web241/browser11/build/contracts PASS; [evidence](evidence/authority-admin-2026-10-18/README.md). Полный HTTP/UI pipeline, native и HIL остаются открытыми.

## Активная цель и ускоренное исполнение

2026-10-17 пользователь поставил полную цель: закрыть все разрывы/ошибки, добавить и выполнить тесты, устранить регрессии, подготовить устройство-испытания и MiniPOS к применимой процедуре НАП/СУПТО. [Цель, три параллельных потока и общая приёмка](08_GOAL_AND_FAST_TRACK.md); [подготовка пакета НАП](09_SUPTO_NAP_PREPARATION.md). Сохраняется CASH → CARD/split/recovery, регуляторная дорожка начинается сразу. Пользователь разрешил делегирование; общий backlog, contracts и merged-snapshot gate обязательны. Это расширение цели, не свидетельство уже выполненных исправлений.

**Исполнение начато:** приняты первые исправления; результаты и ограничения см. [журнал реализации](10_IMPLEMENTATION_EVIDENCE.md). Ниже сохранён исходный аудит; его формулировки описывают первоначальный срез, а не отсутствие последующих изменений. Общая готовность по-прежнему не подтверждена.

Дата: 2026-10-17. Срез Git: `cfff038` **плюс существующее dirty working tree**. Исследован именно рабочий каталог, включая незакоммиченные изменения пользователя. Исполняемый код в рамках аудита не изменялся.

**Предварительный статус: SOFTWARE_GAPS_OPEN / NO_GO для приёмки целевых сквозных сценариев.** Формулировка `SOFTWARE_COMPLETE_HIL_PENDING` в документах от 2026-08-14 больше не описывает этот срез. Обнаружены программные разрывы, которые следует устранить до основной программы испытаний устройств. Диагностические подключения к устройствам допустимы для уточнения протокола; они не означают приёмку.

## Согласованная стратегия

2026-10-17 принят **вариант 2 — поэтапная автоматизация с контрольными точками**: baseline → эмуляторы/red tests → **CASH** → **CARD/split/recovery** → полный software gate и сборки к HIL. [Принятое решение M0–M4](04_REMEDIATION_PLAN.md). Согласование плана не означает закрытия GAP или выполнения исправлений; физическое evidence остаётся HIL_PENDING.

## Документы для работы

1. [Охват модулей и результаты проверок](01_MODULE_AUDIT.md).
2. [Реестр разрывов и ошибок](02_GAP_REGISTER.md): GAP-001…GAP-034; факт, последствия, задание, критерий закрытия.
3. [Протоколы и источники производителей](03_DEVICE_PROTOCOLS.md): локальные PDF/SDK, официальные интернет-источники, сопоставление с runtime.
4. [Порядок доработок](04_REMEDIATION_PLAN.md): зависимости, пакеты работ, предварительная приёмка.
5. [Тесты, которые необходимо дописать](05_TESTS_AND_ACCEPTANCE.md): T-01…T-25, ожидаемые результаты и аппаратные gates; W9/T-25 — расширяемые эмуляторы USB/COM/BLE устройств для IoT сборки.
6. [Исходный план испытаний MiniPOS](../../MINIPOS_HARDWARE_TEST_PLAN_2026-10-17.md): применять после программных gates из настоящего аудита.

## Ревью исполнимости плана

[Ревью REV-01…10](06_PLAN_REVIEW.md): план — достаточный backlog, но пока не готовый полностью автоматический runner. [Дополнение A0–A11](07_AUTOMATION_EXECUTION.md) задаёт порядок исправлений, manifest/case inventory, новые CI targets и отдельный PRE_HIL_SOFTWARE_READY. Эти механизмы предстоит реализовать; hardware acceptance остаётся отдельной.

## Что установлено

- Backend и реальные MQTT-получатели используют разные command topics; S3 не принимает topic business ACK с batch suffix.
- S3 runtime и Android BlueCash используют ошибочные fiscal frame codecs; тестируемый common driver не равен используемому runtime codec.
- BLE onboarding BeeFiscalApp и S3 имеют разные UUID/диалог; Android/iOS-клиент настройки содержит явный отказ crypto provider, Wi-Fi provisioning не завершён.
- В S3 есть разрывы durability, sync canonicalization/retry, recovery и платёжного BLE transport, требующие software fault-тестов.
- В MiniPOS Web повторный запуск UNKNOWN может снова отправить команду; lookup errors смешиваются с отсутствием операции.
- Unit-тесты ряда модулей проходят, но это не доказывает совместимость реальных runtime-компонентов. Три API-теста MiniPOS сейчас падают.

## Правила интерпретации

**C — подтверждено:** непосредственно видно в текущем коде, воспроизведено тестом либо независимым probe. **R — риск:** найден разрыв/непроверенный инвариант, для окончательного диагноза нужен указанный тест. **E — внешний gate:** нужен конкретный firmware/SDK/железо/эквайер; отсутствие evidence не объявляется доказанной программной ошибкой.

P0 означает блокер целевого денежного/фискального цикла или достоверности результата; P1 — обязательная доработка до предварительной приёмки соответствующего профиля; P2 — расширение охвата и эксплуатационной устойчивости. Это инженерная приоритизация, а не юридическое заключение о фискальной сертификации.

Проверка включала чтение модулей, контрактов и протоколов и выборочный запуск тестов. Полный построчный аудит всех файлов, security certification, проверка PostgreSQL на чистом стенде, native APK/IPA/firmware rebuild и физический HIL в этом исследовании не выполнены. «Просмотрен модуль» не означает «доказано отсутствие ошибок». Для неисполненных направлений ниже явно указаны проверки к добавлению.

Закрывать GAP следует ссылкой на изменение, исполняемый тест, immutable build hash и evidence. Само наличие функции, README-статуса или JSON trace со значением PASS недостаточно. Новые исходные тесты в этом этапе не добавлялись: пользователь запросил документацию без изменения кода.
