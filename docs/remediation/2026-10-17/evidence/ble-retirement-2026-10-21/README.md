# BLE retirement и actual browser auth evidence

Raw logs сжаты без изменения содержимого; manifest хранит SHA256 исходных и gzip bytes. Browser source/export manifests фиксируют свежие изолированные сборки, а не прежние dist. Harness и скорректированный только в tmp synthetic header fixture приложены как gzip source; для воспроизведения harness размещается в `minipos/BeeMiniPOS/e2e/` с двумя bundle directories, указанными в source harness. Никакие внешние API/устройства не использовались.

PASS: MiniPOS UI148, Node176, TypeScript; шесть browser cases: manager OTP/config404/dirty logout, cashier404, MiniPOS current401/viewport, MiniPOS stale401/newOTP, Fiscal OTP/retry/tenant, Fiscal current401/theme. Non-auth POST использует фактические Headers с bearer/content-type/idempotency; финансовых записей нет.

RED evidence сохранено отдельно: original active BLE cases, R1 double-connect/route cases, R3 dirty logout, removed logout scope. Ошибки raw logs не обозначены PASS.

MiniPOS theme-menu под held configuration GET остаётся OPEN: выбор theme-dark не появился до timeout, после deadline read authority fail-closed сброшена. Это не доказательство theme PASS и не расширяет HTTP/viewport результат. Отдельный raw browser-theme-open.log.gz сохранён.

Исходный browser helper отклонял дополнительный законный GET `/orders?limit=100` при сбросе Admin каталога. Tmp fixture дополнен только этим точным чтением; ограничения записей сохранены. AppFrame/SafeArea и декоративные иконки взяты из текущего working tree, не откатывались и не считаются частью security patch.

## Уточнение completion milestone

Подтверждённые шесть cases ждут `response.finished()` и два animation frames. MiniPOS stale401 дополнительно выполняет реальную навигацию Admin→products и проверяет product-add после React commit.

Fiscal stale401/same-token ABA browser **OPEN**: diagnostic зафиксировал abort held locations GET на3997ms при неизменённом deadline4000ms; обычный повторный OTP закончился4594ms, ответ401 пытались доставить4625ms.401 не наблюдался браузером. Прежние семь-case raw log/results/harness сохранены с суффиксом historical и явно SUPERSEDED, не PASS acceptance. Unit ABA-тесты остаются самостоятельным валидным evidence. Новые failure и timing raw logs сохранены; production deadline не ослаблялся.
