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

- Fiscal and autonomous MiniPOS Go backends with separate PostgreSQL/FORCE-RLS persistence.
- Two independent DEV/PROD Compose products with separate Caddy ingress.
- Generated and runtime-enforced OpenAPI/AsyncAPI/webhook/BLE contracts.
- Edge agent with SQLite journal, BLE authority, signed sync/ACK, retention and OTA policy.
- Daisy/Datecs protocol abstraction with explicit semantic/unsupported matrices.
- BeeMiniPOS and BeeFiscalApp Expo applications for Android, iOS and Web.
- Daisy SMART S contract STUB, hard-disabled in production.
- Regression, evidence, recovery and Stage-25 handover gates.

## IoT Protocol Build Model

- `protocol-abstraction` is built as part of IoT firmware, not as a standalone service.
- The target fiscal protocol is selected at compile time via preprocessor configuration.
- Different firmware builds can enable different adapters (for example Epson/Datecs/Tremol).

## Remaining milestones

The base non-production software profile is implemented and regression-tested. Pilot and Bulgarian production remain NO-GO until a selected device/payment track has vendor, electrical, firmware, HIL and acquirer evidence; an organization passwordless IdP is deployed; an approved clean vulnerability scan and protected release signature exist; and NAP/BIM/legal/service acceptance is signed. These external gates are authoritative in `MVP_GATES.md` and cannot be closed by simulator results.
