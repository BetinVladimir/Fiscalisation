# HTTP401: владение запросом и жизненный цикл сессии

2026-10-21. Software fix для MiniPOS source/Fiscal HTTP и BeeFiscalApp HTTP.

Каждая committed auth activation получает отдельный owner object. HTTP запрос фиксирует owner до transport;401 отзывает только тот же ещё актуальный owner. Перед logout owner инвалидируется, поэтому параллельные401 не повторяют callback. Смена токена, unmount/remount, ручной logout и последовательность A→B→A не возвращают права прежнему запросу. Callback logout обновляется только в layout commit; обычный перерендер не меняет owner.

MiniPOS current401 сразу очищает runtime bearer, отзывает management capability, инвалидирует refresh и закрывает Admin. BeeFiscal инвалидирует read authority. Поздняя transport ошибка прежней fiscal сессии не запускает mutation retry; разрешённый current-owner retry сохраняет исходные bearer/idempotency key. HTTP401/другие HTTP ошибки финансовых запросов не повторяются. Общий deadline fetch+body не изменён.

## Предварительная приёмка

- Старый401 после A→B и A→B→A не вызывает logout текущего оператора.
- Current401 отзывает authority один раз; benign theme/pending rerender не блокирует этот отзыв.
- Unmount/remount и foreign Authorization не дают старому запросу завершить новую сессию.
- Retired transport failure не повторяет отправку; current-owner transport retry использует тот же запрос. HTTP401 не вызывает retry.
- Тесты выполняют реальные extracted HTTP bodies. Actual MiniPosApp React tests дополнительно проверяют layout wiring и немедленный отзыв management, а не только отдельный helper.

[Evidence](evidence/http-session-2026-10-21/manifest.json): root MiniPOS Node160/UI130, BeeFiscal Node53/UI98 и TypeScript обоих приложений PASS. RED logs воспроизводят исходные stale401/ABA ошибки. Полный браузер/native/HIL этими tests не заменяется.

## Фактические отправляемые заголовки

Все три HTTP-функции теперь строят Headers(defaults), накладывают new Headers(init.headers) через case-insensitive set и фиксируют owner по итоговым заголовкам, которые реально передаются fetch. Record, lowercase record, Headers и tuple array сохраняют override semantics, idempotency, If-Match и custom headers без изменения caller input. Чужой Bearer не отзывает текущую сессию; её собственный401 отзывает один раз. HTTP ошибки не повторяются; fetch/body deadline сохранён.

[Объединённое Headers evidence](evidence/http-headers-2026-10-21/manifest.json): root MiniPOS Node176/UI130, BeeFiscal Node61/UI98, оба TypeScript PASS.59 targeted HTTP/deadline scenarios включают24 header-form cases. Fresh actual-App browser verification готовится отдельно; unit/React gates не подменяют её. BLE/offline/НАП gates остаются открытыми.

## Active BLE retirement — OPEN

Review found an uncovered interaction: layout token replacement/current401 clearing may precede the old passive BLE revoke path. Local BLE READY and cached transport authority must retire synchronously under the old committed owner; best-effort remote revoke must use the captured old session/bearer and cannot affect a new session. Active-BLE tests and correction are in progress. Node/React HTTP PASS does not close this lifecycle gap (UI-MP017).
