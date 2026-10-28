# DOC_DRIFT — Расхождения документации и кода

Дата: 2026-10-28 · Метод: grep-сверка · Файлы проверены: BLUECASH_POS_INTEGRATION.md, EXTERNAL_POS_INTEGRATION_PROTOCOL.md

## Таблица расхождений

| ID | Документ | Строка | Что в доке | Что в коде | Решение | Статус |
|---|---|---|---|---|---|---|
| DRIFT-01 | docs/BLUECASH_POS_INTEGRATION.md:294 | URL reconcile | `POST /public/v1/operations/{id}:reconcile` (colon-verb) | `POST /operations/{id}/reconcile` (path segment) | Обновить документацию: заменить `:reconcile` на `/reconcile` | OPEN |
| DRIFT-02 | docs/EXTERNAL_POS_INTEGRATION_PROTOCOL.md | URL style | Смесь colon-verb (`:reconcile`, `:finalize`) и path-style | Код использует только path-style `/reconcile`, `/finalize` | Стандартизировать документацию на path-style | OPEN |
| DRIFT-03 | docs/EXTERNAL_POS_INTEGRATION_PROTOCOL.md | X-Api-Version | `X-Api-Version: 2026-08-07` в примерах | Код принимает эту версию (актуально) | Нет расхождения — RESOLVED | RESOLVED |
| DRIFT-04 | README.md | URL prefix | `POST /public/v1/` | `POST /` (без `/public/v1` в fiscal-backend router) | Уточнить: gateway добавляет prefix `/public/v1` | OPEN |

## Критерий закрытия
Статус всех строк `RESOLVED` к моменту HIL. `OPEN` строки требуют либо правки документа (если код верен), либо правки кода (если документ верен).
