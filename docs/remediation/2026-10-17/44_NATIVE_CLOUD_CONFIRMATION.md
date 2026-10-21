# Native MiniPOS: подтверждение CLOUD checkout

Дата: 2026-10-21. Область: native checkout/reconcile и source backend. Изменение не подтверждает готовность физических устройств или сертификацию.

## Закрываемый разрыв

Ответ CLOUD dispatch сам по себе не разрешает удалить подписанный журнал. Клиент сначала читает source operation, затем source order. Терминальное подтверждение проверяет идентичность source заказа, смены, оператора, исходного регистра, external ID, root sale/register/operation, receipt session, суммы и commitment платежей, допустимость состояния и монотонность версий.

Успех требует source COMPLETED/REVERSED и совпадающего непустого фискального номера. FAILED/CANCELLED требуют source FAILED и отсутствия противоречащего фискального номера. Только после этого журнал удаляется. Окончательный отказ завершает текущую корзину; новая продажа требует нового source заказа. UNKNOWN, потеря ответа, неверная идентичность, противоречие или ошибка удаления сохраняют журнал. Ошибка проверки dispatch не переключает выполнение на BLE.

Полученный CLOUD FISCALIZED с номером сохраняется до последующих чтений как неизменяемое минимальное наблюдение в подписанном журнале. Оно переживает перезапуск и не может быть заменено поздним FAILED/CANCELLED или другим номером. UNKNOWN после такого наблюдения не создаёт source COMPLETED: требуется реальный source order с соответствующим состоянием и номером.

Source backend принимает только известные operation states, проецирует подтверждённый отказ, сохраняет COMPLETED/REVERSED против устаревших или противоречащих результатов и восстанавливает исходный заказ при ошибке сохранения.

## Публичный безопасный контракт

Фактический publicOrder намеренно удаляет payments, fiscal_register_id и execution_authorization_id. Прямое сравнение приватных полей с GET order несовместимо с реальным API. Вместо раскрытия этих полей добавлено optional checkout_confirmation для подготовленного заказа: version=1, root_register_id, payments_sha256. В proof нет сырых платежей, payment ID, card/provider/terminal metadata или execution authorization ID.

Digest фиксирует порядок и нормализованные поля каждого платежа: type, amount.amount/currency, непустые payment_id и terminal_policy. JSON канонический UTF-8 с сортировкой ключей, затем SHA256 lowercase hex. Клиент вычисляет commitment из сохранённого подписанного плана, сервер — из сохранённых нормализованных платежей. Отличие суммы, типа, payment ID, политики или порядка отклоняется. Отсутствующий proof не разрешает cleanup. Runtime OpenAPI определяет обязательные поля proof, запрещает лишние свойства; Go response validators и TypeScript contracts регенерированы, SourceOrder использует generated тип.

Обязательная вертикальная проверка `sh scripts/test-native-cloud-confirmation.sh` требует Go и Node и не пропускается при их отсутствии. Живой Go httptest исполняет реальные actor-scoped handlers/serializer и child Node выполняет actual TypeScript helper через два HTTP GET. Проверены FISCALIZED, FAILED, CANCELLED, различные source/root register, два упорядоченных платежа с optional terminal_policy, отсутствие приватных полей и значений в реальном ответе. Только generic Go suite может пропустить этот interop без opt-in; отдельная команда всегда включает opt-in. Это настоящий локальный HTTP, upstream fiscal operation и auth остаются тестовыми fixtures.

## Защита UI от повторного запуска

Синхронный per-owner flight guard блокирует два вызова одного checkout callback до React render. Проверки владельца сессии и текущей корзины не позволяют отложенному результату изменить другую сессию/корзину. Терминальный путь синхронно выводит корзину из обращения до освобождения guard, поэтому сохранённый callback не создаёт второй запрос после успеха или окончательного отказа. Повтор AUTHORIZING после неизвестного результата подготовки сохраняет существующую семантику ключа и журнала.

## Проверки и предварительная приёмка

- Production callback tests извлекают реальные checkout/reconcile из App.tsx; проверяют deferred prepare/GET, два одновременных вызова и повтор того же callback после терминала, потерю ответа, смену владельца/корзины и восстановление журнала после restart.
- Helper tests подменяют каждую связь, сумму, версию и состояние; противоречие dispatch/source и историческое положительное наблюдение не разрешают очистку.
- Journal tests проверяют append-only checkpoint, restart и повреждение сохранённого наблюдения.
- Domain/API tests проверяют FAILED/CANCELLED, replay, старые и одинаковые версии, неизвестные состояния, сохранение прежнего успеха, rollback при ошибке persistence и последовательность GET operation → GET order.
- Native Node: 147 PASS; TypeScript: PASS; native UI: 18 suites, 104 PASS. Source backend `go test ./... -count=1`: PASS; mandatory actual HTTP→TS interop: 3 состояния PASS; generated response contracts drift: PASS. Это локальные software fixtures; новый сценарий не заявлен проверенным на PostgreSQL или устройствах.
- До приёмки на интегрированной ветке повторить эти проверки вместе с соседними изменениями management/UI и целевым PostgreSQL профилем. Положительный ответ не должен очищать журнал до authoritative source GET; любой конфликт должен сохранять наблюдение, исходный план и возможность безопасного reconcile.

## Остаётся OPEN

1. Annex №29 п.8/п.9: готовность и UNP при первом вводе информации о продаже. Принятая source-first архитектура ADR-SUPTO-001 не отменяет этот срок; данный checkout patch его не исправляет.
2. Полный offline BLE цикл: новый source cart сейчас требует source backend; cached grant используется только при checkout существующей корзины. Нужен отдельный проект ранней идентичности/резервирования с полномочиями S3.
3. Криптографически подтверждённый ответ BLE устройства и долговечное offline завершение с последующей cloud синхронизацией. Существующая BLE политика не заменяется фиктивной проверкой подлинности.
4. Отдельный обнаруженный путь webhook persistence rollback (`applyFiscalEventLinkedForTenant`) требует самостоятельного исправления и fault-injection теста. Данный пакет не меняет этот путь.
5. Физические Datex/Daisy/POS, Android/iOS сборки и HIL, BLE/MQTT эквивалентность и доказательства для НАП остаются предметом следующих этапов.

## Root integration evidence

[Manifest and logs](evidence/native-cloud-confirmation-2026-10-21/manifest.json): Node 147/147; merged UI 114/114; TypeScript, full source backend and generated contracts drift PASS. The manifest separates staged financial source hashes from the tested worktree containing the first management fix; exact isolated financial patch UI baseline is 104/104. `make native-cloud-confirmation-test` requires real Go HTTP → TS confirmation and is included in regression. Native synthetic fixture now returns the public proof and versioned operation/source GETs; its tests call the actual confirmation helper and keep private payment fields absent. Full native-harness gate 30/30 PASS. No Android/iOS full-App launch or device fiscal operation is claimed.
