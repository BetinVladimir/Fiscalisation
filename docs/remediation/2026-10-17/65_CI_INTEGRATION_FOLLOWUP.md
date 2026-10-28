# Проверка CI после typed CBOR и device time crypto

Дата: 2026-10-21. Ключи остаются во внутренней NVS ESP32-S3; ATECC608A/608 не используется.

## Подтверждённые результаты

Для `2f02765429c0bc70f1ef6169b526e4d7048278f6` GitHub Actions завершились успешно:

- [Native crypto gate](https://github.com/BetinVladimir/Fiscalisation/actions/runs/39554966702): обязательный crypto gate с фазой реального device time interoperability.
- [Протоколы устройств](https://github.com/BetinVladimir/Fiscalisation/actions/runs/39554966692).
- [Ядро reliable GATT](https://github.com/BetinVladimir/Fiscalisation/actions/runs/39554966705).

Это состояния конкретных workflow; они не доказывают полную сборку, физические испытания или приёмку НАП.

## Полная S3 сборка

[Run 39554966671](https://github.com/BetinVladimir/Fiscalisation/actions/runs/39554966671) завершился ошибкой: `device_time_anchor.cpp:112` использует `ESP_ERR_NVS_NOT_FOUND` без прямого include `nvs.h`. Host fake SDK предоставлял этот символ транзитивно и не выявил target compilation defect. Коммит `c9aed74` добавляет официальный заголовок, сохраняя семантику ошибок. Повторная полная target сборка обязательна; пока её успешность не подтверждена.

После исправления root повторил actual DeviceIdentity/mbedTLS/Go interoperability с GCC15: PASS. Первая попытка проверки не дошла до тестов из-за ошибочно выбранного локального каталога библиотек без libmbedx509; повторная использовала полный build-host.

SHA256 успешного локального лога: `8048dd8374703f5ee204e02099dbf44321f249be2e39a9f1b4fdaa0deb1985f0`. Raw лог хранится локально; не публикуется этим документом.

## Integration contract: отдельный незакрытый дефект контура

[Run 39554966717](https://github.com/BetinVladimir/Fiscalisation/actions/runs/39554966717): OpenAPI job PASS; server2server-e2e FAIL при ожидании localhost:8080. Enrollment/Rabbit и session concurrency не выполнялись. Это не успешный интеграционный тест.

Проверенные исходники workflow/Compose показывают следующие разрывы:

1. `up ... fiscal-backend` запускает зависимости backend, но не сервис caddy, который единственный публикует HTTP порт 8080 на host. Поэтому host health-check не может подтвердить backend.
2. Dev override задаёт пароли PostgreSQL, но DATABASE_URL/RLS_DATABASE_URL базового Compose получают пустые значения, если workflow не передаёт FISCAL_DB_PASSWORD/FISCAL_RLS_DB_PASSWORD.
3. PostgreSQL в этих двух Compose файлах не получает migration/init fixtures. Существующий disposable fixture builder и третий e2e Compose уже реализуют отдельную схему, runtime role и socket. Их необходимо использовать согласованно, а не ослаблять backend/RLS ради зелёного CI.

Требуемое исправление: изолированный CI Compose project с явными синтетическими секретами, проверенной подготовкой схемы/ролей, запуском host HTTP entrypoint, bounded readiness и тестами реального enrollment/projection/session concurrency. Все exec/cleanup должны адресовать тот же project и тот же набор Compose файлов. Не удалять чужие volumes/containers. Не выгружать необработанные логи с токенами. Приёмка: успешный fresh Ubuntu run с выполненными обоими E2E шагами, подтверждённой изоляцией и cleanup; один успешный health-check недостаточен.

## Следующие независимые проверки

Self-contained CBOR transport CI реализуется отдельно: чистый pinned Node/npm install, точные реальные модули, producer/Go/MiniPOS/full browser lifecycle, контроль fixture/source hashes и отрицательные проверки. До фактического clean run этот пункт остаётся OPEN. Production trusted-time provider/issuer/lifecycle, физический HIL и применимость актуальных норм НАП остаются отдельными открытыми требованиями.

## Исправление bootstrap, ожидающее fresh CI

Workflow теперь использует один уникальный COMPOSE_PROJECT_NAME на run/attempt и одинаковый COMPOSE_FILE для запуска, SQL exec и cleanup. Перед стартом формируется существующий disposable SQL fixture; третий e2e override монтирует миграции и socket. Явные синтетические DB credentials согласованы с обеими DSN. Запускается caddy с зависимостями; readiness ограничен числом попыток и timeout каждого HTTP запроса. Go выбирается по go.mod, concurrency test выполняется с count=1. Скрипт enrollment сохраняет прежний default Compose, но уважает явно переданный набор overrides.

Локально: sh -n PASS, git diff --check PASS; rendered Docker Compose JSON проверен на согласованность credentials/DSN, SQL init mount, socket и HTTP dependency graph; fixture generation PASS. Полный runtime E2E пока не выполнен для этого изменения; нужен actual Ubuntu CI result. Никакое прохождение статической проверки не закрывает E2E acceptance.

Первая версия workflow (`12198cf`, run39555975017) отклонена до запуска jobs: runner temporary path был указан на job env уровне. Исправление `19cb7d0` вычисляет `$RUNNER_TEMP` внутри шага и передаёт путь через GITHUB_ENV; GitHub принял workflow, run39556086939 фактически начал jobs. Это подтверждает валидацию workflow, но не runtime E2E успех.

## Полная S3 повторная проверка: PASS

[Run39555577175](https://github.com/BetinVladimir/Fiscalisation/actions/runs/39555577175) для `c9aed74c7fa96d5afcd3ab6f3e81244595b4946a` завершился SUCCESS: target compile/link, host IoT regression и финальная provenance проверка. Root скачал артефакты, независимо сопоставил все103 source hashes с точными Git blobs этого коммита и все7 artifact hashes с фактическими bytes. В compile_commands присутствуют device_time_anchor.cpp, time_anchor_core.cpp, canonical_cbor.cpp и ble_result_delivery.cpp. Это закрывает ошибку отсутствующего nvs.h и подтверждает target compilation новых модулей, но не включение production provider.

[Обезличенный summary и hashes](evidence/s3-time-cbor-ci-2026-10-21/summary.json). Сборка сохранена локально в ignored `.store-build/s3-idf556-ci-c9aed74-20261021`; BIN SHA256 `63247ab7bc0c361efc6e0f339e5a359ab343db2ddf15f39c26adc9c4d2398846`. Raw logs остаются локально. Устройства не прошивались.

## Сетевая причина readiness и отдельный CI overlay

Runs39556086939/39556143915 завершились FAIL: все services UP, PostgreSQL healthy, но PORTS пусты. Третий e2e Compose подключает всё только к internal dbroute; работающий two-compose стенд отдельно добавляет ingress. Для host-driven CI добавлен четвёртый `compose.fiscalisation.integration-ci.yaml`: только caddy получает routable ingress с127.0.0.1:8080, только PostgreSQL получает private bridge с127.0.0.1:5433 для session tests. Backend остаётся только internal dbroute и использует Unix DB socket. Исходный физический/production Compose не менялся.

Root повторил11 negative topology tests PASS и проверку фактического rendered четырёхфайлового Compose PASS. Worker дополнительно подтвердил original three-file config RED этим же validator. Workflow проверяет топологию до запуска. [Sanitized evidence](evidence/integration-ci-topology-2026-10-21/summary.json). Полный исправленный remote E2E ещё требуется; static/config evidence не заменяет его.
