# BLE authority: синхронный отзыв и владение сессией/маршрутом

2026-10-21. Исправлен программный lifecycle MiniPOS: current401, смена/потеря auth и unmount синхронно сбрасывают точную локальную active/pending BLE authority до async logout. Best-effort revoke использует сохранённый bearer прежней сессии. Старые401/revoke, connection-ready/error, expiry и logout continuation не сбрасывают новое подключение, включая A→B→A.

HTTP auth owner и committed BLE route owner независимы. Маршрут включает configuration id/version/binding generation, location/adapter/register и operator id/code. Его смена отзывает pending и active lease; saveConfiguration инвалидирует route до публикации новых настроек. Один synchronous connect ticket устанавливается до первого await, поэтому повторный вызов в том же событии не дублирует permission/register/session/connect. Только текущий flight может завершать busy/status.

Logout использует отдельный committed context auth+route+shift. Он сохраняет допущенное HeaderLeaveGuard действие через stay/discard и benign renders, но отвергает raw callback старой auth/ABA либо закрытой смены после открытия новой. Отправленная ранее разрешённая операция не объявляется отменённой. Checkout/payment journal этим исправлением не очищается и не переигрывается.

## Проверка и границы

[Manifest и raw evidence](evidence/ble-retirement-2026-10-21/manifest.json): MiniPOS UI148/Node176/TypeScript PASS. Контрпримеры воспроизведены на original/R1/R3 и при удалении logout context. React BLE transport замокан: физический READY/отзыв ESP32 не принят.

Свежие actual-App Expo web bundles в изолированной копии прошли шесть scoped browser cases: обычный OTP и configuration404 для manager/cashier, dirty logout, current401, старый401 после повторного OTP, Fiscal tenant login/retry . Фактические заголовки non-auth POST проверены на wire; финансовых записей нет. MiniPOS401 проверен с viewport rerender; Fiscal401 дополнительно с theme toggle.

MiniPOS theme-menu interaction при незавершённом configuration GET отдельно OPEN: theme-dark не появился до timeout. Этот результат не заменён PASS другого сценария. Browser source включает текущие незакоммиченные AppFrame/SafeArea/decorative header изменения; security commit93e13a9 и отдельная UI работа различаются в manifest. Общая UI/financial/offline/native/HIL/НАП готовность не заявляется.

## Предварительная приёмка

- Current401 немедленно теряет локальный READY до deferred storage logout; revoke ровно один, с прежним bearer.
- Потеря auth/route и A→B→A не дают старой ready/error/expiry/logout continuation затронуть новый lease.
- Double connect при held revoke/permission/register/session/ready выдаёт одну session/connection; старый finally не снимает busy нового flight.
- Same-auth saveConfiguration при pending old ready отзывает прежний route и позволяет новое подключение; старый success/error не публикует READY/status.
- Dirty logout stay сохраняет draft, discard завершает выход; raw old-owner/old-shift callbacks отвергнуты.
- Физическое подтверждение локального ESP32 session retirement, полноценная криптографическая BLE authority и HIL остаются отдельными открытыми gates.

## Ограничение Fiscal stale401 browser evidence

Предыдущее утверждение о browser same-token ABA отозвано: первоначальный assert видимости выполнялся без доказательства consumption старого401. Усиленный harness ждёт response.finished+два animation frames; Fiscal stale GET оказался aborted на3997ms при deadline4000ms, до завершения повторного OTP4594ms. Доставка4014625ms не дала response event. Этот browser gate OPEN; исторические logs/results сохранены как superseded/nonacceptance. Шесть остальных cases подтверждены повторным fresh-bundle прогоном; MiniPOS stale дополнительно проверяет последующую реальную React navigation commit. Unit ABA evidence не подменяется этим ограничением.
