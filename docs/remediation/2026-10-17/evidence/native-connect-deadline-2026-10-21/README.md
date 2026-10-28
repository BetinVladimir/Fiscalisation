# Native BLE connection deadlines — 2026-10-21

A non-settling adapter connect previously retained the connecting flag indefinitely. Connect, service discovery and Android MTU negotiation now each have a 10-second deadline; failures reset the session and permit a fresh user connection. Late completion from an old generation requests disconnection. No monetary bytes are sent during these stages.

Actual NativeBleTransport regression with mocked timers proves connect timeout, connecting flag release, late disconnect request, discovery timeout and MTU timeout. Full npm test67/67 PASS; npx tsc --noEmit exit0. Intermediate test-only counter typing error was corrected before this final run. Adapter simulation does not establish physical radio behavior or authenticated peripheral response.
