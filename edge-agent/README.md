# Edge Agent

Локальный компонент для фискальных устройств (Raspberry Pi, mini PC, POS, embedded).

## Задачи
- Подключение к устройствам через USB/UART.
- Локальная буферизация транзакций (offline-first).
- Надежная доставка событий в облако через MQTT.
- Безопасный канал связи (TLS/mTLS).

## Планируемые модули
- `device-adapters/` — драйверы и transport-адаптеры.
- `store/` — локальная очередь и журнал.
- `sync/` — синхронизация и reconciliation.
- `security/` — сертификаты и attestations.
