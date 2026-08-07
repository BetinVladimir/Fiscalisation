# System Overview

Этот документ фиксирует суть платформы как общеевропейского middleware-слоя
для фискальных устройств и cloud ERP.

## Product Thesis

Платформа предоставляет унифицированный API для работы с фискальными устройствами,
скрывая аппаратные и протокольные различия.

## Positioning

- Stripe-like infra for fiscal transactions
- API-first and compliance-first
- pay-per-use business model

## Scope of Current Monorepo

- IoT firmware and shared modules
- Protocol abstraction stubs in `IoT/protocol-abstraction` (Arduino/C++)
- Edge agent stub
- Cloud core layer stubs
- Backend APIs (Go)
- Mobile apps (Expo)
- DB migrations (PostgreSQL)
- Messaging (EMQX MQTT, RabbitMQ, Redis)

## IoT Protocol Build Model

- `protocol-abstraction` is built as part of IoT firmware, not as a standalone service.
- The target fiscal protocol is selected at compile time via preprocessor configuration.
- Different firmware builds can enable different adapters (for example Epson/Datecs/Tremol).

## Next Technical Milestones

1. Define canonical transaction contract and protocol adapter interface.
2. Implement edge local queue + reconciliation policy.
3. Implement transaction engine with idempotency persistence.
4. Implement audit chain and signature workflow.
5. Publish first OpenAPI spec for ERP integration.
