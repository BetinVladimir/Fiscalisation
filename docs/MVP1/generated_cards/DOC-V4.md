# DOC-V4 · P1 · ARCH_09_VENDOR_DOCS_AND_PROTOCOL_GAPS.md

Notes: ПОЛУЧЕНО 2026-10-23: Protocol_Smart_Pro_v2_2025.pdf и Protocol_Daisy_Smart_Payment_v1_2026.pdf (docs/vendor/, sha256 в manifest.json; разбор DAISY_SMART_PROTOCOLS.md). Осталось: ответы вендора на Q-S1…Q-S8, Q-P1…Q-P6 (DOC-V4B)

Цель: POS + MiniPOS, MQTT и BLE, BlueCash, Datecs/S3/COM, BluePay/S3/BLE, Daisy Compact/S3/USB, Smart S/Android/HTTP. D-1…D-20: последние подтверждённые решения DECISIONS §5 приоритетнее старых карточек. Транзакции до HIL. Деньги: целые центы, half-up; golden 1200+167=1367, CASH 500/CARD 867. UNKNOWN после записи: только lookup, никаких повторных денежных команд/автовозврата. Сторно до refund; crash/timeout refund сохраняет отдельное durable состояние. Compact: CARD/split запрещены до I/O. Smart S: только LAB без номера; PROD UNKNOWN, не синтетический номер. E-чек выключен: проверять отказ, не реализовывать отправку. BC-50: сторно cmd43, внешний терминал; часы читать cmd62, WARN >5мин/BLOCK >2ч, business_date Europe/Sofia; валютный режим 2/3. Datecs SEQ 0x20..0x7f, Daisy 0x20..0xff, CP1251. Статусы исторического трекера не доказывают готовность. Никогда не менять status.json и не записывать HIL PASS по эмулятору.

Source: ARCH_09_VENDOR_DOCS_AND_PROTOCOL_GAPS.md
Task: DOC-V4  Unit: DOC-V4