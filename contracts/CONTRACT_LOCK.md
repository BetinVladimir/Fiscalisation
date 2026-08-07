# Contract lock

Canonical public contracts are maintained with the regulatory documentation and are consumed by this implementation without redefining business schemas:

- `../../BeeloyBackend/docs/Fiscal/api/openapi-public-v1.yaml` — SHA-256 `5aeacae5be26b5f8c6cb19b48e6725bb751dd7ad2ce6bee2773044964cdce203`;
- `../../BeeloyBackend/docs/Fiscal/events/asyncapi-device-v1.yaml` — SHA-256 `16c9ebb272c272189d08843739b1e6ca439aff16844e7d307cab835212cfe797`.
- `openapi-runtime-v1.yaml` — SHA-256 `7d4a80efa3d62bd2df9166d217309313160f2f1beaf9fde60702f8845db5071f`; implementation completion contract for MiniPOS configuration/shifts/webhook receiver and DEV/HIL Edge endpoints, including authenticated storage-pressure telemetry.

The lock is intentionally explicit: changing any source requires a reviewed contract diff, regenerated clients and an update of these hashes. REST, webhook and BLE payloads must reference the canonical schemas rather than introduce transport-specific business models. The runtime completion document may only close implementation-surface gaps; it cannot redefine a business schema already owned by the regulatory contract.
