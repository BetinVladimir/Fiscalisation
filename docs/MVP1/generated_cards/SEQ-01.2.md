# SEQ-01.2 · P0 · ARCH_22_TEST_E2E_REVIEW_ADDENDUM.md

Dependencies: W0-1
Notes: S3 оборачивает SEQ на 0xFF для Datecs; допустимо 0x20…0x7F (Kotlin уже верно). GAP-012 PARTIAL

Цель: POS + MiniPOS, MQTT и BLE, BlueCash, Datecs/S3/COM, BluePay/S3/BLE, Daisy Compact/S3/USB, Smart S/Android/HTTP. D-1…D-20: последние подтверждённые решения DECISIONS §5 приоритетнее старых карточек. Транзакции до HIL. Деньги: целые центы, half-up; golden 1200+167=1367, CASH 500/CARD 867. UNKNOWN после записи: только lookup, никаких повторных денежных команд/автовозврата. Сторно до refund; crash/timeout refund сохраняет отдельное durable состояние. Compact: CARD/split запрещены до I/O. Smart S: только LAB без номера; PROD UNKNOWN, не синтетический номер. E-чек выключен: проверять отказ, не реализовывать отправку. BC-50: сторно cmd43, внешний терминал; часы читать cmd62, WARN >5мин/BLOCK >2ч, business_date Europe/Sofia; валютный режим 2/3. Datecs SEQ 0x20..0x7f, Daisy 0x20..0xff, CP1251. Статусы исторического трекера не доказывают готовность. Никогда не менять status.json и не записывать HIL PASS по эмулятору.

Исправить profile-specific SEQ в SerialSession::exchange, не менять wire codec/ответы Fake. Datecs: 0x20..0x7F включительно = 96 значений. Golden индекс→SEQ: 0→0x20,95→0x7F,96→0x20,97→0x21,112→0x30,192→0x20. Daisy: 0x20..0xFF = 224 значения; 0→0x20,223→0xFF,224→0x20,225→0x21. Индекс 112 НЕ является Datecs wrap. Повтор передачи одной команды использует прежний immutable packet/SEQ. Использовать существующий Fake из serial_exchange_native.cpp, не придумывать новый ответ ФУ/парсер и не создавать отдельный .cpp. Старую безусловную проверку f.sent[224]==f.sent[0] заменить на profile-specific golden assertions: для Datecs период 96, для Daisy 224. Datecs SEQ находится в sent[i][5], Daisy в sent[i][2]. Все старые тесты сохранить, исправить только устаревший Datecs oracle. В конце main вывести реальный TAP plan и ok после выполненных assertions. Python harness до 40 строк: subprocess bash IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh с capture_output/text/timeout, проверить rc0 и TAP; не читать/парсить implementation как oracle. Red оставляет native assertion FAIL на исходном Datecs коде; green минимально исправляет serial_exchange.cpp. Не менять тесты в green. Запрещён новый serial_exchange_datecs_wrap.cpp и прочие дополнительные файлы. Не писать длинный план: прочитать cpp/test/header (3 файла), добавить golden тест и harness, запустить. Полная приёмка — обе serial profiles, 226+ команд, immutable retry SEQ. После завершения каждого профиля вывести соответствующую TAP строку ok 1 - Datecs cycle96 / ok 2 - Daisy cycle224; в конце 1..2. Печатать после assertions, не вместо них. Контроллер заранее создаёт этот фиксированный harness; не менять его. Использовать этот harness без phase/source inspection; не писать собственный parser/вызов Python вместо Go. Менять только native тест/продукт по этапу.
```python
from pathlib import Path
import sys,unittest
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/"docs/MVP1/fixes"))
from native_harness import check_native_tests
class T(unittest.TestCase):
    def test_native(self):
        check_native_tests(self,['bash', 'IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh'],root=ROOT,cwd='.',markers=('ok 1 - Datecs cycle96','ok 2 - Daisy cycle224'),timeout=120)
```

Source: ARCH_22_TEST_E2E_REVIEW_ADDENDUM.md
Task: SEQ-01  Unit: SEQ-01.2