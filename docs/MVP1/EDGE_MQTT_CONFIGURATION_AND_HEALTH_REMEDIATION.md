# Edge MQTT configuration и фактическое состояние устройств

Статус: `P0 OPEN`  
Дата аудита: 2026-08-14

## 1. Что уже реализовано

Backend создаёт подписанный composite binding generation и публикует его в
`tenants/{tenant}/devices/{adapter}/bindings`. ESP-IDF проверяет подпись,
tenant/device, монотонность generation, сохраняет binding в NVS, перезапускает
runtime и публикует apply ACK. Поддержаны профили:

- Datecs DP-150 MX по RS-232 + BluePad-50 Plus по BLE;
- Daisy Compact S 01 по USB без payment terminal.

Это рабочая основа, но не полный управляемый configuration/health plane.

## 2. P0-EDGE-CFG-001 — явная конфигурация драйвера и протокола

Текущий envelope выбирает реализацию неявно по `profile/vendor/model/transport`.
Добавить в каждый endpoint и OpenAPI/data dictionary:

- `role: FISCAL_DEVICE|PAYMENT_TERMINAL`;
- `vendor`, `model`, `transport`;
- стабильные `driver_id`, `protocol_id`, `protocol_version`;
- `transport_parameters` с discriminated schema:
  UART/RS-232 (baud, data bits, parity, stop bits, TX/RX pins), USB CDC
  (VID/PID/interface/serial), BLE GATT (identity/address policy, service,
  TX/RX characteristics);
- expected device identity/capabilities и connection timeout/retry policy.

ESP-IDF обязан валидировать allow-list совместимости до записи, применить всю
generation атомарно, сохранить last-known-good, выполнить rollback при ошибке
инициализации и отправить signed `APPLIED|REJECTED|ROLLED_BACK` ACK с причиной.
Неизвестный driver/protocol/version отклоняется fail-closed.

## 3. P0-EDGE-HEALTH-001 — живой MQTT status plane

Сейчас edge публикует binding ACK и sync batches, но не heartbeat/endpoint
health. Backend `Probe()` проверяет в основном соединение bridge с брокером и не
доказывает доступность конкретного ФУ/POS. Добавить:

```text
tenants/{tenant}/devices/{adapter}/status
tenants/{tenant}/devices/{adapter}/probes/requests
tenants/{tenant}/devices/{adapter}/probes/results
```

`status` публикуется retained, QoS 1, периодически и при изменении. MQTT Last
Will выставляет `adapter_state=OFFLINE`. Payload содержит schema version,
adapter/device/register IDs, boot ID, monotonically increasing sequence,
observed/config generation, firmware, timestamp и для каждого endpoint:

- configured/transport state/reachable;
- observed vendor/model/serial/FMIN либо terminal ID;
- protocol/driver versions;
- printer/paper/fiscal-memory/open-receipt status для ФУ;
- POS link/acquirer readiness для терминала;
- last successful I/O, last error code/time и certainty.

Статус `READY` допустим только при свежем heartbeat, совпавшей generation и
успешной проверке всех обязательных endpoint roles. Broker connection сам по
себе не означает READY.

## 4. P0-BE-HEALTH-001 — materialization MQTT → REST

Backend должен:

1. подписаться на status/probe-result topics и строго сверять tenant/device с
   topic, binding и credential;
2. валидировать schema, sequence, boot ID, timestamp и generation;
3. хранить current snapshot и append-only state transitions в PostgreSQL;
4. вычислять `ONLINE|DEGRADED|OFFLINE|MISCONFIGURED|STALE` по server receive time;
5. предоставить OpenAPI endpoints:
   `GET /devices/{id}/activity`, `GET /devices/{id}/health`,
   `POST /devices/{id}/probes`, `GET /device-probes/{id}`;
6. возвращать отдельно adapter, fiscal endpoint и payment endpoint, `last_seen`,
   age, observed/applied generation и причину;
7. публиковать WebHook только на state transition, с outbox/retry/signature, а
   не на каждый heartbeat;
8. не подменять telemetry глобальным `driver.Probe()`.

## 5. Acceptance

- cold boot, reconnect, stale heartbeat, LWT, wrong generation, unplugged
  fiscal device, absent BluePad, paper-out и recovery моделируются тестами;
- status другого tenant/device отклоняется и аудируется;
- duplicate/out-of-order sequence не откатывает current snapshot;
- applied generation видна одинаково в binding ACK, health REST и UI;
- нагрузочный тест подтверждает bounded retained heartbeat traffic и отсутствие
  webhook storm;
- PostgreSQL restart не теряет current state и transition history.

