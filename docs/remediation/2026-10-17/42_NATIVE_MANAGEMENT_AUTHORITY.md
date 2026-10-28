# MiniPOS: права management и владение асинхронными результатами

2026-10-21. Software scope: актуальный MiniPosApp, committed role authority, callback ownership; физические устройства не участвовали.

## Исправлено

Отсутствующая конфигурация больше не открывает редакторы CASHIER/AUDITOR. Настройка разрешена ADMIN/SUPERVISOR после свежего operator-session. Непривилегированный кассир не загружает общий employee directory. Рендер Admin и вход в него проверяют актуальные права; четыре сохранения, скидки и отчёт независимо проверяют capability при вызове.

useManagementAccess выдаёт capability только committed render. Свежий ответ сервера может немедленно отозвать её до React rerender; старые props не возвращают права. Retained callbacks не переживают смену роли, сессии, ABA или unmount. Уже отправленный запрос невозможно отменить задним числом: после await его последствия допускаются только в той же committed auth scope. Обычный theme/pending rerender сохраняет эту scope. Поэтому корректный отчёт или сохранение не теряется, а поздний ответ прежнего оператора не очищает новый draft/config, не отзывает новую BLE-сессию и не запускает refresh.

Refresh сверяет epoch и токен после каждого await. Cleanup инвалидирует epoch: ответ configuration после unmount не запускает clock-sync, readiness или workstation-session POST. Реальная кнопка отчёта использует pending wrapper и блокирует повторный запуск.

## Предварительная приёмка и тесты

- Configuration404 для CASHIER/AUDITOR: Admin отсутствует, записи отсутствуют; ADMIN/SUPERVISOR получают редактор.
- Старые mutation/discount/report callbacks после downgrade, session change, ABA и unmount не выполняют привилегированное действие.
- Прерванный concurrent ADMIN render не выдаёт capability; свежий server downgrade действует до rerender и не отменяется старыми props.
- Deferred сохранения всех четырёх типов и поздние ошибки не меняют новую административную сессию; deferred configuration после unmount не продолжает startup POST.
- Реальные кнопки report/product save с parent-pending и theme rerender публикуют корректный ответ при неизменной authority.
- [Evidence](evidence/native-management-2026-10-21/manifest.json): root Node147/147, UI127/127 в19 suites, TypeScript PASS;23 management cases. Counterexample logs фиксируют потерянный отчёт при строгом render-token после await и поздние последствия старых write callbacks. Это React actual-App tests с mock transport, не нативный запуск приложения.

## Отдельно OPEN

[Отдельное исправление HTTP session ownership](47_HTTP_SESSION_OWNERSHIP.md) теперь защищает от старого401, включая ABA и реальные HeadersInit формы. Объединённые Node/UI/TypeScript проверки прошли; fresh browser/native приёмка отслеживается отдельно. Полная аппаратная/БД/НАП приёмка не заявляется.
