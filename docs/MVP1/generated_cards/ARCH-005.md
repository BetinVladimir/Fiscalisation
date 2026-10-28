# ARCH-005 · P1 · ARCH_04_ARCHITECTURE_CODE_STYLE_DOCS.md

Alias: BUG-023. No own implementation; close only via canonical task.

Dependencies: BUG-023

Цель: POS + MiniPOS, MQTT и BLE, BlueCash, Datecs/S3/COM, BluePay/S3/BLE, Daisy Compact/S3/USB, Smart S/Android/HTTP. D-1…D-20: последние подтверждённые решения DECISIONS §5 приоритетнее старых карточек. Транзакции до HIL. Деньги: целые центы, half-up; golden 1200+167=1367, CASH 500/CARD 867. UNKNOWN после записи: только lookup, никаких повторных денежных команд/автовозврата. Сторно до refund; crash/timeout refund сохраняет отдельное durable состояние. Compact: CARD/split запрещены до I/O. Smart S: только LAB без номера; PROD UNKNOWN, не синтетический номер. E-чек выключен: проверять отказ, не реализовывать отправку. BC-50: сторно cmd43, внешний терминал; часы читать cmd62, WARN >5мин/BLOCK >2ч, business_date Europe/Sofia; валютный режим 2/3. Datecs SEQ 0x20..0x7f, Daisy 0x20..0xff, CP1251. Статусы исторического трекера не доказывают готовность. Никогда не менять status.json и не записывать HIL PASS по эмулятору.

Алиас BUG-023. Собственной реализации/приёмки нет; завершение только по канонической задаче.

Source: ARCH_04_ARCHITECTURE_CODE_STYLE_DOCS.md
Task: ARCH-005  Unit: ARCH-005