# NAP-N0 · P0 · ARCH_10_NAP_LATEST_CHANGES.md

Notes: повторить на заморозке релиза (W9); см. ARCH_10

Цель: POS + MiniPOS, MQTT и BLE, BlueCash, Datecs/S3/COM, BluePay/S3/BLE, Daisy Compact/S3/USB, Smart S/Android/HTTP. D-1…D-20: последние подтверждённые решения DECISIONS §5 приоритетнее старых карточек. Транзакции до HIL. Деньги: целые центы, half-up; golden 1200+167=1367, CASH 500/CARD 867. UNKNOWN после записи: только lookup, никаких повторных денежных команд/автовозврата. Сторно до refund; crash/timeout refund сохраняет отдельное durable состояние. Compact: CARD/split запрещены до I/O. Smart S: только LAB без номера; PROD UNKNOWN, не синтетический номер. E-чек выключен: проверять отказ, не реализовывать отправку. BC-50: сторно cmd43, внешний терминал; часы читать cmd62, WARN >5мин/BLOCK >2ч, business_date Europe/Sofia; валютный режим 2/3. Datecs SEQ 0x20..0x7f, Daisy 0x20..0xff, CP1251. Статусы исторического трекера не доказывают готовность. Никогда не менять status.json и не записывать HIL PASS по эмулятору.

### NAP-N0 · P0 · Заморозка нормативного baseline (шаги)
1. Скачать консолидированный Н-18 НАП (актуальную редакцию) и посчитать SHA256; сравнить титульный перечень изменений (последний ДВ) с `…2025-05-23…pdf`.
2. Для каждого ДВ после 23.05.2025 из реестра (`evidence/nap-publication-register-2026-10-21/coverage.json`) выполнить поиск «Н-18|фискал|СУПТО|ИАСУТД» и записать результат в `docs/SUPTO/nap-baseline-<date>.md` (дата, ДВ, вывод «не затрагивает»/«затрагивает → NORM-xx»).
3. Прочитать перви

Source: ARCH_10_NAP_LATEST_CHANGES.md
Task: NAP-N0  Unit: NAP-N0