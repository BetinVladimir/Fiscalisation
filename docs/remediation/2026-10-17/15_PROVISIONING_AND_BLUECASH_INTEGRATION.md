# Provisioning, S3 activation и BlueCash wire integration

Дата: 2026-10-18. База: e66b4f7. Полный объединённый IoT gate завершён exit 0; evidence и hashes: [provisioning-2026-10-18](evidence/provisioning-2026-10-18/README.md).

Интегрирован защищённый onboarding: manufacturing registry связывает ключ устройства, backend выдаёт подписанный device-encrypted пакет; приложение передаёт opaque framing; S3 атомарно сохраняет единственный encrypted NVS blob, перечитывает его перед подписанным install proof. Мобильное installed-состояние не является доказательством. Native/Web BLE form, restart/lost-ACK lookup и WiFi runtime подключены. Внешнего ATECC608A нет.

Typed EDGE_AGENT_S3 activation использует отдельный ACTIVE физический фискальный ресурс маршрута; серийный номер платы не становится fiscal serial, существующий fiscal/payment route не подменяется. Неподтверждённая legacy embedded activation остаётся FISCAL_IDENTITY_PROOF_REQUIRED.

BlueCash canonical JSON исправляет UTF-8 порядок, Go escaping, точный int64 и portable float32/64; typed ACK/event/batch сохраняют Go struct order и null/[]; hash использует захваченный previous-hash snapshot. Backend MQTT UseNumber и targeted EdgePendingCommand decoder сохраняют вложенные authenticated Payload/ResultPayload после рестарта и отклоняют trailing JSON.

## Проверено в основном дереве

- Actual Go → mbedTLS/device identity → signed schema3/encrypted NVS → actual firmware install proof → Go: PASS, включая commit failure, replay, wrong subject/secret, restart/lost ACK.
- BeeFiscalApp unit tests 24/24 PASS; actual BLE runtime host PASS.
- Actual Go encoding/json/domain structs → production Kotlin: 10660 vectors PASS. Первый integrated oracle запуск выявил новую missing provisioningseal dependency в test copy; dependency добавлена, повторный прогон PASS.
- Backend domain/MQTT/API после всех описанных интеграций: exit 0.
- `git diff --check`: PASS.

Логи пока в `/private/tmp/fiscal-provisioning-vertical-integrated-20261018.log`, `/private/tmp/fiscal-bluecash-json-integrated-20261018.log`, `/private/tmp/fiscal-provisioning-int64-backend-20261018.log`. Целевая S3 сборка `/private/tmp/fiscal-provisioning-integrated-s3-20261018.log`: terminal exit 0, RAM 56636/327680, Flash 1978268/3145728. Android BlueCash unit tests 43/43 PASS и assembleProdDebug exit 0; лог `/private/tmp/fiscal-bluecash-integrated-android-20261018.log`. Предыдущий evidence bundle относится к e66b4f7, а не к этому пакету.

Общий IoT gate выявил missing provisioningseal в изолированных GoWire/REVERSAL oracle copies и устаревший startup source guard. Зависимости и guard исправлены; повторный полный `make iot-test` завершён exit 0. Guard проверяет выход setup до operational ingress, а обязательный native installer runner — фактические crypto/NVS/replay сценарии. Физический boot/power-cut этим не подтверждён.

Открыты: MiniPOS authorization/client proof vertical, setup-entry/reconfiguration и физический отзыв cached offline authority, аппаратные Android/iOS USB/COM/BLE испытания и внешние применимые требования НАП. Подробные границы: PROVISIONING_V3_VERTICAL.md и backend migration/activation документы.
