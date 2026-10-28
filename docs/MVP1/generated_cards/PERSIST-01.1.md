# PERSIST-01.1 · P0 · MVP_GOAL.md

Dependencies: W0-1
Notes: Явное закрытие разрыва ревью; реальные внешние gates остаются открыты.

Цель: POS + MiniPOS, MQTT и BLE, BlueCash, Datecs/S3/COM, BluePay/S3/BLE, Daisy Compact/S3/USB, Smart S/Android/HTTP. D-1…D-20: последние подтверждённые решения DECISIONS §5 приоритетнее старых карточек. Транзакции до HIL. Деньги: целые центы, half-up; golden 1200+167=1367, CASH 500/CARD 867. UNKNOWN после записи: только lookup, никаких повторных денежных команд/автовозврата. Сторно до refund; crash/timeout refund сохраняет отдельное durable состояние. Compact: CARD/split запрещены до I/O. Smart S: только LAB без номера; PROD UNKNOWN, не синтетический номер. E-чек выключен: проверять отказ, не реализовывать отправку. BC-50: сторно cmd43, внешний терминал; часы читать cmd62, WARN >5мин/BLOCK >2ч, business_date Europe/Sofia; валютный режим 2/3. Datecs SEQ 0x20..0x7f, Daisy 0x20..0xff, CP1251. Статусы исторического трекера не доказывают готовность. Никогда не менять status.json и не записывать HIL PASS по эмулятору.

Fresh schema ONLY. Controller has supplied BOTH immutable native test and Python harness. Do not write or change tests, receipt_session inserts, infrastructure, helper or harness. Read only database/fiscal/013_supto_identifier_foundation.sql and the supplied native test if needed. RED: temporarily remove the existing UNIQUE (tenant_id, unp) constraint from migration013, leaving other SQL valid; run exact unit acceptance, confirm expected duplicate-UNP assertion, STOP. GREEN: controller restores SQL; run unchanged unit acceptance and STOP, no test edits. SQLSTATE 23505 and exact constraint names, valid seed inserts and cross-tenant controls are already implemented. Do not add 999 migrations or another test.

Source: MVP_GOAL.md
Task: PERSIST-01  Unit: PERSIST-01.1