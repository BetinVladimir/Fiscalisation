# ESP32-S3: официальная сборка ESP-IDF5.5.6 и TLS1.3

Дата:2026-10-21. Изолированный эксперимент; это основание для отдельного ревью миграции SDK, а не выполненная миграция production. Исходный репозиторий и установленный глобальный ESP-IDF5.4.0 не изменены. Прошивка устройств не выполнялась. Рабочие ключи остаются внутри ESP32-S3/NVS; ATECC608A/608 не используется.

## Результат

**Полное приложение S3 успешно скомпилировано и слинковано официальным idf.py5.5.6, включая bootloader, partition table и проверку размера app. Вторая полная сборка с TLS1.3=y также PASS.** Для успешного пути не использовались патчи ESP-IDF/mbedTLS, CMake hooks, отключение проверки версии компилятора или замена vendor submodule на произвольный upstream.

Результат снимает препятствие «не собирается полный образ с исправленной библиотекой». Он не закрывает интеграцию защищённого BLE-канала, проверку pin/binding/PoP, надёжный GATT stream, runtime heap/stack, реальные устройства, сертификацию и полный аудит применимости всех SDK advisories.

| Вариант | Итог | App.bin SHA256 | Размер |
| --- | --- | --- | --- |
| Исходные sdkconfig.defaults + явные board/GPIO настройки, TLS1.3 OFF | PASS; baseline metadata limitation ниже | `039fbec7a518b0670d401267918ef2e1a64c8a72b001c808a52eb8494e2df649` |2091920bytes |
| Та же board/config основа + TLS1.3=y, чистое состояние SDK | **PASS, предпочтительный проверенный артефакт** | `6f111a7a02c3b8563277fb7a6e21384f5a8b019cb9068d9eb765212679ef42ce` |2146768bytes |

Предпочтительный файл: `artifacts-tls13/beefiscal_edge_agent_s3.bin`. ELF: SHA256 `341f94051ec382cb98610b69e77ef41efa5706c329d430082f12f997fa05a4c6`. Bootloader.bin20464bytes, SHA256 `cb4bd9fa500191453ec9363e72b4f6296a4a31ec9c5f35ab2b1716c5f3cd0009`. Partition table SHA256 `bd9ab5aace828042720cde613ccb9840fc22f643f818383aeb2cea4f46c9abc2`. Это отдельные flash-компоненты; merged flash image не создавался и не проверялся.

## Точная основа

- Официальный ESP-IDF tag5.5.6, commit `880a203ae603d08a94d06aa01e0d6761958e5e9e`.
- Официальный vendor mbedTLS submodule `2b96dd8eebe880f304c69976b3c2fa0c5100cbb6`, версия3.6.7; `framework` — tracked Git tree `50e13842aeec74b4164df86fdfcfe4827aff6b13` внутри vendor, не отдельный submodule. Первоначальный `git -C framework rev-parse HEAD` поднимался к parent vendor; исправленная запись использует `git ls-tree HEAD framework`. Остальные подмодули перечислены в `SDK_FINAL_SUBMODULES`.
- Поддерживаемый Xtensa GCC14.2.0, release `esp-14.2.0_20260121`; установлен официальным idf_tools.py только в `/tmp`.
- CMake3.31.10; Ninja1.13.2; Python3.11.7; зависимости Python установлены с официальным `espidf.constraints.v5.5.txt`; точные пакеты — `PYTHON_FREEZE.txt`.
- Приложение скопировано из working tree при HEAD `e7aefae130c468cd17d42798f40ea8529aa6764c`. Авторитетная спецификация проверенных файлов — `TESTED_SOURCE_SHA256SUMS`, поскольку это снимок working tree. Сопоставление с текущим original показало только два отличия: `main/idf_component.yml` и сгенерированный `dependencies.lock`.
- Кандидатный manifest фиксирует `idf: "==5.5.6"`; USB CDC ACM остаётся2.3.0 с исходным component hash. Точный diff — `component-manifest.patch`, lock — `dependencies.lock.tested`.
- Весь исходный production C/C++ и CMake приложения сохранён без изменений. Обнаруженной необходимости миграции API приложения нет.

## Board и конфигурация

Официальная сборка получает repo `sdkconfig.defaults` и явный `board.defaults`: ESP32-S3,8MB flash, custom partition `partitions_8mb.csv`. Это перенос настроек `platformio.ini`, которые один sdkconfig.defaults не выражает. App partition3MiB; TLS1.3 образ оставляет `0xf3e30` bytes (32%) свободными. Bootloader оставляет38% своей области.

В C/C++ flags явно переданы исходные `EDGE_SD_CLK=39`, `EDGE_SD_CMD=38`, `EDGE_SD_D0=40`; реальные response files сохранены в `config-evidence[-tls13]/toolchain/`. Итоговые sdkconfig сохранены отдельно. `CONFIG_TLS13_DIFF.patch` показывает включение TLS1.3 и автоматически выбранные vendor Kconfig параметры. TLS1.2 сохранён для существующей системы; это эксперимент включения поддержки TLS1.3, а не внедрение TLS1.3-only бизнес-канала. Параметры PSK/tickets остаются defaults SDK, и будущий secure-channel wrapper должен отдельно запретить их согласно doc46.

Официальный SDK по умолчанию компилирует C++ как `gnu++2b`, тогда как PlatformIO передавал `gnu++17`; CMake приложения требует минимум17. Эта разница явно отражена в compile_commands. Перед production-миграцией следует согласовать сохранение17 для app либо принять официальный default и прогнать общие проверки; текущий PASS не заявляет побайтовое соответствие старой PlatformIO сборке.

`idf.py size` для TLS1.3 варианта: DIRAM156687bytes занято,185073bytes остаётся в описанной linker region; .data30696, .bss21600, DIRAM .text104391. Отдельный IRAM region16384bytes занят векторами/кодом согласно linker layout. Это **статический отчёт**, не измерение свободной heap на работающей плате. Одновременные BLE/MQTT/SQLite/TLS handshake и минимальный запас stack не проверены.

Baseline ограничение: первоначальное shutil-копирование превратило несколько tracked symlinks SDK examples/RIOT в обычные файлы, из-за чего configure сохранил `IDF_VER=v5.5.6-dirty`. Типы файлов восстановлены из Git; изменения не касались используемого SDK-кода. Для второго свежего TLS1.3 build дерево SDK было чистым до configure и после сборки, compile_commands содержит `IDF_VER=v5.5.6`. Поэтому использовать для дальнейшей оценки следует второй артефакт. Не скрывать это различие при сравнении hashes/размеров.

## Причины прежних failures

1. PlatformIO6.10 не обеспечивает готовую интеграцию5.5.6: коллизия basename объектов `esp_efuse_fields.c` блокирует SCons. Подмена одного framework package в старом platform не является поддержанной миграцией.
2. ESP-IDF5.5.6 сравнивает REALPATH каталога SDK с dirname toolchain path, который не canonicalized тем же способом. На macOS `/tmp` и `/private/tmp` расходятся; это создавало ссылку на несуществующий `tools/cmake/cflags`. Для окончательного PASS выбран физический путь без alias; vendor patch и дополнительный hook не нужны.
3. Первый ручной bootloader configure выполнялся без compiler directory в PATH, затем сохранял системные ar/ranlib в CMake state. Повторная работа с таким cache не исправляла вложенный configure. Полностью новая официальная сборка с корректным окружением сама выбрала поддерживаемые Xtensa ar/ranlib, что сохранено в bootloader/CMakeCache.txt. Нельзя переносить старые CMake cache между toolchains.
4. Физический путь в `Beeloy ERP` содержит пробел: SDK Bluetooth linker-fragment handling разбивал путь. Проверенный физический путь не содержит пробелов.
5. Один sdkconfig.defaults без board-настроек даёт default1MiB app partition, и полный образ правильно отвергается проверкой размера. Успешный вариант использует существующую3MiB partition, без отключения проверки размера.

## Проверки и оставшиеся ворота

- Полная стандартная сборка и полная TLS1.3 сборка: terminal0, `Project build complete`; логи `build-official-final.log` и `build-official-tls13.log`.
- Native identity/startup entropy/RNG budget harness с **vendor mbedTLS3.6.7**: PASS, `identity-entropy-sdk556.log`.
- Реальный Go issuer → actual mbedTLS decrypt/подпись/schema3/encrypted NVS/install proof: PASS, `provisioning-sdk556.log`.
- Полный `make iot-test` на сохранённом vendor mbedTLS3.6.7: **PASS, terminal0**, подтверждён агрегатором; raw log `evidence/s3-idf556-build-2026-10-21/root-full-iot-sdk556.log.gz`. Это запуск исходного software suite до отдельного patch нового build entrypoint; его новые gate tests и новый firmware artifact учитываются отдельно.
- Нет физического запуска/flash, HIL, runtime memory/latency/entropy/RF measurements, BLE secure stream и сертификатной lifecycle интеграции. Ключи существующих устройств не регенерировались.

## Воспроизведение и миграция на ревью

`build-official-final.sh` и `build-official-tls13.sh` — фактически выполненные команды официального idf.py, без flash. Они используют временную физическую директорию без пробелов/alias и изолированные `/tmp` tools/Python/cache. Для повторения после cleanup нужно воссоздать именно эту новую временную директорию, распаковать `source-snapshot.tar.gz` в неё как `repo`, получить официальный5.5.6 с подмодулями (`git clone --branch v5.5.6 --recursive`), сохранить symlinks и проверить commits. Восстановить `board.defaults`; baseline config формируется из defaults, TLS1.3 config — его копия с `CONFIG_MBEDTLS_SSL_PROTO_TLS1_3=y`. Скрипты содержат точные пути сохранённого toolchain и аргументы. Указанные hashes удостоверяют эти артефакты; воспроизводимость бинарника побайтно между разными путями/временем не заявляется.

Минимальный будущий change для ревью: закрепить официальный SDK/toolchain и Python constraints в отдельном build entrypoint/CI; перенести8MB partition/GPIO параметры явно; изменить IDF manifest+lock; согласовать C++ standard; выполнить полный software regression и ограниченный HIL memory/communication gate. Исходный global PlatformIO/SDK не обновлять побочным эффектом и не объявлять старый PIO6.10 совместимым с5.5.6 по одному override. Применимость advisories всех включённых компонентов и будущая TLS policy остаются отдельными критериями.

Сохранённый vendor для повторных host-тестов: `MBEDTLS_SOURCE=/private/tmp/fiscal-idf556-official-20261021/vendor/mbedtls-3.6.7-2b96dd8e`; framework subtree включён, административные `.git` pointers исключены, содержимое byte-for-byte сверено с официальным vendor. `CJSON_SOURCE`/`CJSON_SOURCE_DIR` можно направить на соседний `vendor/cJSON`. Отдельные SHA256 manifests приложены. Generated PEM/device-private assets в пакет не добавлены.

## Cleanup

Оба созданных физических временных каталога удалены после сохранения исходников, vendor, конфигурации, логов и артефактов в `/private/tmp/fiscal-idf556-official-20261021`: `/Users/freelancer/Documents/Beeloy ERP/.tmp-fiscal-idf556-20261021` и отдельный `.tmp-fiscal-idf556-20261021` в разрешённом visualization root. `CLEANUP.json` содержит точные пути и время проверки отсутствия. Существующие ERP/visualization файлы не изменялись. Все команды flash в логах — только подсказки idf.py; они не запускались.

## Сохранённые evidence и граница истории

[Evidence manifest](evidence/s3-idf556-build-2026-10-21/manifest.json) содержит hashes ограниченных gzip raw logs, config/source/toolchain manifests и исторический CLEANUP. Бинарники, ELF и generated private assets не включены. Предпочтительный исторический TLS1.3 candidate остаётся `6f111a7a...`; последующая production-entrypoint сборка, её gate tests, provenance и новый hash — отдельный результат. Повторно созданная директория для этого нового эксперимента имеет отдельный cleanup record; исторический CLEANUP не переписывается.

## Последующая проверка production entrypoint

Отдельный reviewed change использует официальный idf.py, SDK/vendor/toolchain/USB lock и fail-closed provenance gates. Его полный final build и повторный verify — PASS; app SHA256 `c63abbed7a80b9c8aaf13251b638f1c55cd0e2d11391146e493002a1f5edab6f`. Все 94 проверенных source inputs совпали с применённым агрегатором original. Это новый артефакт; исторический `6f111a7a...` выше не подменяется. Кандидат принимает официальный C++23 default, TLS1.3 library support и прежние GPIO/8MB partition. Production-канал BLE этим не реализован.

[Новые evidence](evidence/s3-idf556-entrypoint-2026-10-21/README.md) содержат 12 gate tests, final build/verify, negative-check logs, provenance и отдельный cleanup повторно созданного workspace. Исторический SDK5.4.0 глобально не обновлялся. Новый entrypoint готов к отдельному ревью/CI; удалённый GitHub CI и HIL ещё не запускались.

После применения entrypoint полный агрегированный `make iot-test` с фактическим vendor3.6.7 также PASS, terminal0: [raw log](evidence/s3-idf556-entrypoint-2026-10-21/root-integrated-iot.log.gz). Этот запуск заменяет прежний pre-entrypoint regression как актуальное подтверждение; прежний log оставлен только в историческом evidence.

## Интеграция в репозиторий

Build entrypoint, SDK/config lock, негативные тесты и CI workflow приняты в коммите `925c64f`. Это закрывает программный путь сборки и локальную проверку происхождения артефактов; предыдущие разделы сохраняют историю экспериментов. Удалённый CI, target runtime/HIL и production BLE security остаются OPEN.

Проверенные BIN/ELF и provenance также сохранены локально в игнорируемом `.store-build/s3-idf556-verified-c63abbed-20261021/`; все artifact hashes и 94 исходника сверены перед переносом. Это архив software candidate без flash/HIL приёмки и без полного signed release/flash bundle. Стандартный новый build directory остаётся свободным.
