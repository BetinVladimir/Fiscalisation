<!-- Сгенерировано tools/gen-fixes.py; не править руками -->
# Дефект SEQ-01 · P0 · исполнитель: qwen

**Состояние кода на 2026-10-24:** OPEN — S3 wrap 0xFF для Datecs
**Проверка состояния:** `sed -n 23p IoT/firmware/edge-agent-s3/idf/main/serial_exchange.cpp`
**Приёмка подзадач:** `python3 docs/MVP1/tools/run-accept.py SEQ-01`
**Правила:** `AGENTS.md` (правь только файлы подзадачи; сначала тест и покажи, что он падает; целые центы; UNKNOWN ≠ FAILED).

---

# ЗАДАЧА SEQ-01  [P0] wave=3 status=NOT_STARTED
Документ: ARCH_22_TEST_E2E_REVIEW_ADDENDUM.md
Заметки: S3 оборачивает SEQ на 0xFF для Datecs; допустимо 0x20…0x7F (Kotlin уже верно). GAP-012 PARTIAL | [reverify 2026-10-24 OPEN] S3 wrap 0xFF для Datecs | проверка: sed -n 23p IoT/firmware/edge-agent-s3/idf/main/serial_exchange.cpp

## Зависимости
—

## Подзадачи
## SEQ-01.1
Тест в serial_exchange_native.cpp: 100 команд Datecs подряд → все SEQ в 0x20..0x7f, после 0x7f идёт 0x20; Daisy: после 0xff идёт 0x20
Файлы: IoT/firmware/edge-agent-s3/idf/tests/serial_exchange_native.cpp
Ожидание: тест красный до правки
Команда приёмки: bash IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh

## SEQ-01.2
Правка serial_exchange.cpp:23: wrap зависит от профиля (Datecs 0x7f, Daisy 0xff)
Файлы: IoT/firmware/edge-agent-s3/idf/main/serial_exchange.cpp
Ожидание: тест зелёный
Команда приёмки: bash IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh

## Карточка
### SEQ-01 · SEQ Datecs 0x20…0x7F на S3
Файл `IoT/firmware/edge-agent-s3/idf/main/serial_exchange.cpp` строка 23: `next_sequence_=sequence==0xff?0x20:…` одинаково для Datecs и Daisy. Для Datecs wrap на 0x7F, для Daisy на 0xFF (примеры вендора доходят до 0xDE и 0xC0). Сначала тест в `IoT/firmware/edge-agent-s3/idf/tests/serial_exchange_native.cpp` (96+ команд подряд: все SEQ Datecs в диапазоне, после 0x7F идёт 0x20), затем минимальная правка.

