# Data Location — Местонахождение данных

Основание: чл. 52г, чл. 52з(14); Прил. 32 п.10–13 (ЗДДС / Наредба Н-18).

Все данные за продажи, фискальни документи и лични данни на данъкоплатеца
**трябва да се съхраняват в България (BG) или в държава-членка на ЕС (EU).**

---

## Инвентаризация на хранилищата

| Service | Storage type | Region | Notes |
|---------|-------------|--------|-------|
| fiscal-backend | PostgreSQL (primary) | EU — Frankfurt (eu-central-1) | Fiscal receipts, UNP ledger, journal |
| fiscal-backend | PostgreSQL (replica) | EU — Frankfurt (eu-central-1) | Read replica; same legal region |
| fiscal-backend | Backups (pg_dump) | EU — Frankfurt (eu-central-1) | Daily full + WAL-E; encrypted at rest |
| fiscal-backend | RabbitMQ / AMQP | EU — Frankfurt (eu-central-1) | Sale events and status updates |
| IoT edge-agent-s3 | SQLite (on-device) | BG — device on-premises | Local durable store; synced to backend |
| fiscal-backend | Application logs | EU — Frankfurt (eu-central-1) | Structured JSON; no PAN/secret in logs |
| fiscal-backend | Receipt object store | EU — Frankfurt (eu-central-1) | PDF/XML fiscal receipts; S3-compatible |
| SmartDevices / BlueCash | EMQX (MQTT broker) | EU — Frankfurt (eu-central-1) | BLE↔backend telemetry |
| fiscal-backend | Audit trail DB | EU — Frankfurt (eu-central-1) | Immutable append-only log (EVIDENCE) |

---

## Разрешени региони

Само региони в **България (BG)** или **държави-членки на ЕС**.
Забранено е съхранение на фискални данни в трети страни (US, UK след Brexit и т.н.)
без изрично решение за адекватност по чл. 45 GDPR.

Проверка: `scripts/check-data-location.sh` — изходен код 0 при съответствие,
код 1 при намерен регион извън разрешения списък.
