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
