# MVP_GOAL — зафиксированная цель MVP (источник истины)

> **Статус: ЗАФИКСИРОВАНО владельцем 2026-10-23.** Любой документ MVP1, противоречащий этому файлу, считается устаревшим и правится по этому файлу. Менять цель можно только решением владельца с записью в `DECISIONS.md`.

## 1. Цель (формулировка владельца, без сокращений)

**POS + MiniPOS → MQTT/BLE →**
1. **BlueCash-50** — прямое подключение;
2. **нативный протокол DATECS** через IoT ESP32-S3 и **COM** (`IoT/firmware/edge-agent-s3`);
3. **BluePay** через IoT ESP32-S3 и **BLE**;
4. **Daisy (Smart S)** — Android, **прямое подключение, как BC-50**;
5. **Daisy Compact S 01** — через IoT ESP32-S3 и **USB** (нативный протокол Daisy);
6. **соответствие требованиям СУПТО** (Н-18, Приложение 29, последние изменения НАП).

## 2. Матрица «клиент × транспорт × устройство» (что должно работать к MVP)

| # | Путь | Клиенты | Транспорт до устройства | Исполнитель | Интерфейс к ФУ/пинпаду |
|---|---|---|---|---|---|
| P-A | BlueCash-50 | POS, MiniPOS | MQTT, BLE (и локальный HTTP, если включён) | Android-приложение BlueCash | прямое подключение к ФУ BC-50 (Datecs v2.11.4) |
| P-B | Datecs native | POS, MiniPOS | MQTT, BLE | ESP32-S3 `edge-agent-s3` | UART/COM (профиль `DatecsDp150BluePad50`) |
| P-C | BluePay | POS, MiniPOS | MQTT, BLE | ESP32-S3 | BLE central к BluePad (`bluepad_ble_central.cpp`) |
| P-S | Daisy (Smart S) | POS, MiniPOS | MQTT, BLE (и локальный HTTP, как у BlueCash) | Android-приложение Smart S (`SmartDevices/daisy-smart-app`) | прямое подключение к ФУ/терминалу устройства через vendor SDK/API |
| P-D | Daisy Compact S 01 | POS, MiniPOS | MQTT, BLE | ESP32-S3 `edge-agent-s3` | USB (CDC/ACM host, `usb_cdc_runtime.cpp`, профиль `DaisyCompactS01`) |
| P-E | СУПТО | все пути | — | — | УНП, документы, журнал, экспорты, часы, сторно, e-чек по ARCH_06/08/10 |

Каждая ячейка «клиент × транспорт × путь» — отдельная строка HIL-матрицы (`ARCH_17`, §5). Путь считается достигнутым только когда пройдены **все** строки матрицы для него, а не один сквозной сценарий.

## 3. Что это меняет в ранее принятом

| Было (ARCH_09/11, DECISIONS) | Стало |
|---|---|
| MVP = лабораторный HIL только на BlueCash-50 (D-1) | MVP = шесть путей P-A, P-S, P-B, P-C, P-D, P-E; BlueCash — один из путей |
| Daisy «после HIL» (D-2), `MVP_GATES`: `STUB_ONLY`/`DOCUMENTED_NOT_APPROVED` | Daisy **в MVP** (путь P-D); профиль `DaisyCompactS01` и Daisy-ветка `profile_executor.cpp` в прошивке уже есть — задача: довести и доказать |
| S3-прошивка — только BUG-040…043 и ссылки на карточки плана 70 | S3-пути P-B/P-C/P-D обязательны; карточки P05–P11, P17, P19, P23–P27, P32 — до первого HIL |
| Tremol, Eltrade, Fibank | вне цели MVP |
| Daisy Smart S — заглушка, PROD запрещён (`MVP_GATES`: `STUB_ONLY`) | Smart S **в MVP**; заглушка заменяется реальной интеграцией после получения SDK (DOC-V4); запрет PROD сохраняется до evidence |
| `minipos/` в части вызовов Fiscal (D-11) | MiniPOS — полноценный клиент MVP (MQTT/BLE) |
| POS только через шлюз и REST | POS-путь до устройств — MQTT через backend; BLE для POS — вопрос D-13 |

Что **не** меняется: `MVP_GATES.md` (BlueCash-50 `EXCLUDED_FROM_PROD`, DP-150 MX и Daisy `DOCUMENTED_NOT_APPROVED`, НАП-сертификация — отдельный release-blocker). MVP по этой цели — **тестируемый MVP на реальных устройствах**, а не разрешение на продуктивную эксплуатацию.

## 4. Критерии достижения (автоматически проверяемые)
1. Для каждой строки HIL-матрицы `ARCH_17 §5` есть запись `docs/vendor/hil_results.json` с `result=PASS` и ссылкой на evidence (команда: `python3 scripts/check-hil-matrix.py` — SCR-40).
2. Для каждого пути P-A, P-S, P-B, P-C, P-D выполнены сценарии: наличные; карта (где есть пинпад); смешанная оплата; сторно; обрыв связи (UNKNOWN→resolve); закрытие дня; повтор с тем же ключом.
3. СУПТО: `scripts/check-nap-baseline.sh`, трассировка Приложения 29 по каждому профилю устройства (`NAP-P-*`, ARCH_17 §6), ответы NORM-01…16 или консервативные интерим-правила.
4. Нет `SKIP` и нет «PASS на симуляторе» в графе HIL.

> **Две разные Daisy-интеграции** (см. `docs/remediation/2026-10-17/03_DEVICE_PROTOCOLS.md:81`): **Smart S** — Android-устройство, прямое подключение (путь P-S, по образцу BlueCash); **Compact S 01** — принтер с USB CDC/ACM через ESP32-S3 (путь P-D). Сейчас P-S — **заглушка** `DaisySmartStub` (debug-only, запрещена в PROD); для реализации нужен vendor SDK/API/лицензия (DOC-V4) — без них P-S = `BLOCKED_SPEC`.

## 5. Связанные документы
`ARCH_17_MVP_GOAL_PATHS_GAPS.md` (разрывы по путям, факты по Daisy, HIL-матрица, новые задачи), `DECISIONS.md` (D-1/D-2/D-11 пересмотрены, D-13 новый), `MASTER_PLAN.md`, `ARCH_11` (критерии E1–E10 дополнены).


> **Smart S — транспорт и протокол (2026-10-23).** Android-приложение; POS/MiniPOS → MQTT/BLE как у BC-50; к ФУ — JSON по HTTP через localhost (`docs/vendor/DAISY_SMART_PROTOCOLS.md`, D-17). Карта — через Payment Bridge (D-19). Блокеры PROD: в ответе продажи нет номера документа (F-1) и нет чтения документа (F-2); путь P-S остаётся `LAB` до ответов DOC-V4B.

> **Решения владельца 2026-10-23** (подробно DECISIONS §5): клиенты POS и MiniPOS оба используют MQTT и BLE (нативный BLE в POS); HIL волнами; персистентность на транзакциях до HIL; Smart S в PROD — после ответа вендора (LAB).
