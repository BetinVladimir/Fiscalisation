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

## Незавершённая связанная проверка

Normalize HeadersInit в реальные отправляемые headers до вычисления ownership: существующий spread init.headers не обрабатывает Headers и tuple arrays как Fetch. Additive исправление и тесты всех форм находятся в review. UI-MP028 остаётся OPEN до объединённой проверки этой границы. Общие BLE/offline/НАП gates остаются открытыми.
