# Full fiscal E2E

Этот набор поднимает изолированное dev/CI-окружение и проверяет весь путь от новой компании MiniPOS до фискальных операций.

## Что проверяется

1. Запуск настоящих PostgreSQL, Redis, RabbitMQ, EMQX, `fiscal-backend`, `beeminipos-backend` и web-прокси.
2. Создание внешней системы в Fiscal и передача bootstrap system token в MiniPOS только через environment.
3. Регистрация нового пользователя MiniPOS по email OTP, onboarding компании и сохранение данных компании.
4. Enrollment компании MiniPOS в Fiscal по второму OTP; проверка `ACTIVE` binding непосредственно в Fiscal PostgreSQL.
5. Синхронизация location, register и operator из MiniPOS в integration API Fiscal.
6. Настройка активных Fiscal resources и настоящие CASH и CARD продажи через публичный Fiscal API: товарная строка, оплата, фискализация, получение чека и возврат.
7. Локальный HTTP-контракт mock `edge-agent-s3`: readiness, CASH intent, чтение результата и reconcile.
8. Локальный HTTP-контракт mock `bluecash-app`: readiness, CARD intent, RRN/authorization и idempotency replay.
9. Повтор CASH/CARD payment с тем же `Idempotency-Key` возвращает ту же операцию и не создаёт повторную фискализацию.
10. Устаревший `If-Match` при конкурентном изменении register отклоняется с conflict/precondition response.

SMTP mock принимает обычный SMTP на `127.0.0.1:18025`, хранит письма только в памяти и предоставляет API `http://127.0.0.1:18080/messages`. Тест никогда не читает OTP напрямую из БД.

Device mocks моделируют сетевую границу `/beeloy/local/v1`, а не ESP-IDF/Android runtime и не реальное фискальное или банковское оборудование. Hardware certification, BLE, secure element и vendor SDK должны проверяться отдельными HIL/instrumentation suites.

## Требования

- Docker Engine с Compose v2;
- Node.js 20+;
- `curl` и `jq`;
- свободные localhost-порты `18000`, `18001`, `18025`, `18080`, `18443`, `18444`, `19001`, `19002`, `15433`, `16380`, `15674`, `25674`, `11884`, `18884`, `28083`–`28085`.

## Запуск

Из корня репозитория:

```sh
./tests/e2e/full-fiscal/run.sh
```

По умолчанию runner всегда удаляет свои контейнеры и volumes. Для диагностики:

```sh
KEEP_E2E=1 ./tests/e2e/full-fiscal/run.sh
docker compose -p beeloy-full-e2e-fiscal logs -f fiscal-backend
docker compose -p beeloy-full-e2e-minipos logs -f beeminipos-backend
```

После диагностики удалите окружение командами с теми же project names и compose-файлами. Стенд использует отдельные project names и не затрагивает обычные dev volumes.

Поддерживаются переменные `E2E_FISCAL_URL`, `E2E_MINIPOS_URL`, `E2E_AUTH_KEY`, `E2E_CREDENTIAL_KEY` (base64 ровно 32 bytes) и `KEEP_E2E`. Значения по умолчанию предназначены только для одноразового CI/dev стенда и не являются production secrets.

## CI

Workflow `.github/workflows/full-fiscal-e2e.yml` запускается вручную и при изменениях E2E/integration кода. Он не выгружает логи или OTP во внешние artifacts. Для диагностики воспроизведите ошибку с `KEEP_E2E=1` на доверенном runner.

## Быстрая проверка только mocks

```sh
docker compose -f tests/e2e/full-fiscal/compose.yaml up -d --build --wait
curl -fsS http://localhost:18080/healthz
curl -fsS http://localhost:19001/beeloy/local/v1/readyz -H 'Authorization: Bearer test'
curl -fsS http://localhost:19002/beeloy/local/v1/readyz -H 'Authorization: Bearer test'
docker compose -f tests/e2e/full-fiscal/compose.yaml down -v
```
