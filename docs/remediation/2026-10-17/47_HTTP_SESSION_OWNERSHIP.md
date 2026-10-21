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

[Объединённое Headers evidence](evidence/http-headers-2026-10-21/manifest.json): root MiniPOS Node176/UI130, BeeFiscal Node61/UI98, оба TypeScript PASS.59 targeted HTTP/deadline scenarios включают24 header-form cases. Последующий scoped fresh actual-App browser auth результат приведён в [документе50](50_BLE_AUTHORITY_RETIREMENT.md); эти исторические HTTP логи сами по себе browser acceptance не доказывают. BLE/offline/НАП gates остаются открытыми.

## Active BLE retirement — software lifecycle подтверждён

Прежний uncovered interaction исправлен и проверен отдельно: committed auth/route owner синхронно отзывает локальную active/pending authority; remote revoke использует прежний bearer, а старые continuation не затрагивают новый lease. [Документ50 и raw evidence](50_BLE_AUTHORITY_RETIREMENT.md) фиксируют UI148/Node176/TypeScript, RED контрпримеры и свежий scoped browser auth proof. UI-MP-017 получает актуальные source/test anchors; это не физический BLE PASS.

MiniPOS theme-menu interaction при held configuration GET остаётся отдельно OPEN; MiniPOS current/stale401 browser PASS относится к viewport rerender. Fiscal current401 с theme toggle проверен. Native/HIL, полноценный authenticated BLE/offline и НАП gates остаются открытыми.

Scoped browser уточнение: шесть cases подтверждены после response.finished+render completion; Fiscal stale401/same-token ABA browser остаётся OPEN, поскольку старый read достиг прежнего4000ms deadline до доставки401 после повторного OTP. Исторический преждевременный visibility assert отозван; unit ABA coverage отдельно сохраняет силу. Подробности и неизменённые raw logs — в документе50.
