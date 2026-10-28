# MiniPOS Web: uncertain fiscal submission must not switch transport

Status: **OPEN — production routing change requires explicit authorization** (2026-10-01).

## Reproduction

The E2E fixture returns HTTP 500 from the cloud `/checkout-batch` request. A separate local-device route records `/intents` calls, while `/operations/{id}` returns 404 for an operation that does not exist. The test `e2e/checkout.spec.ts` now fails with **expected 0, received 1** local submissions after that cloud HTTP 500. The cart clears and a reversal action appears because the mocked local device reports `FISCALIZED`. This does not prove a real duplicate fiscal receipt occurred, but it proves a second transport attempt was made after an uncertain first submission.

The current `src/fiscal/routeController.ts` says route changes are allowed only before a transport attempt, but its catch path continues after `route.submit()` raises a 5xx or timeout. Cloud failure before `submit()` (probe/lookup) is a different case: local fallback remains appropriate there.

## Proposed behavior and acceptance

1. Track whether `route.submit(intent, signal)` has started. Probe failures and a confirmed absent prior operation may still select another available route before submission.
2. A definite 4xx rejection from the chosen route remains an error and does not switch routes. For 5xx, network failure or timeout after submission starts, save the existing operation ID and payload digest with `UNKNOWN` certainty in the outbox. Do not call another transport's `submit()` for that sale.
3. Show a clear pending/uncertain result in the sale UI, keep the cart, and prevent a new operation ID from being created by a second click or after reload until recovery resolves the original operation. Recovery must query by the original operation ID and may only finalize or allow a new attempt after a verified terminal result. Preserve the existing API/idempotency contract.
4. E2E must prove zero secondary `/intents` calls after cloud 500 and after an ambiguous timeout, while keeping cart and operation identity. It must also prove pre-submit cloud probe failure still falls back to local, 4xx remains immediate, and recovery after reload uses lookup without resubmitting the sale. Unit tests should cover the same state transitions.

`npx playwright test --workers=2 --retries=0` currently reports **41 passed, 1 failed of 42**, with this safety regression as the sole failure. The exact red test is `cloud submit 500 stays uncertain without a second transport attempt`. No production payment or fiscal device was used; all routes were mocked. The full app E2E and UI-optimization G1/G3 gates remain FAIL until this is resolved and reverified.
