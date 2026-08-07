# Cloud Core (IoT Middleware)

Облачное ядро event-driven платформы.

## Подмодули
- `mqtt-router/` — маршрутизация MQTT-сообщений.
- `transaction-engine/` — exactly-once на уровне логики, idempotency и retry.
- `audit-compliance/` — immutable audit trail, hash chaining, подписи.
- `security-layer/` — mTLS, authn/authz, управление ключами.
- `erp-api/` — REST API и webhooks для ERP.
