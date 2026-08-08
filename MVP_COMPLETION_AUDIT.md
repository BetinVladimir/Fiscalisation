# MVP completion audit

Дата среза: 2026-08-07. Статус цели: **IN PROGRESS / PROD NO-GO**.

Этот документ не подменяет canonical roadmap. Он связывает фактическую реализацию в данном репозитории с критериями этапов 20–25 и запрещает считать simulator evidence доказательством hardware/legal readiness.

## Доказано текущим кодом

| Область | Статус | Авторитетное evidence |
|---|---|---|
| Fiscal и MiniPOS public API | PASS для реализованного MVP surface | 73 canonical + 8 runtime operations, `make contract-test`, API/race tests |
| Автономность MiniPOS | PASS | отдельная PostgreSQL/Compose; E2E читает MiniPOS при остановленной Fiscal PostgreSQL |
| EUR cash fiscalization | PASS на simulator | two-Compose sale через оба Caddy, immutable operation/receipt references |
| Ambiguous outcome | PASS | E2E требует 502 + durable `UNKNOWN`; replay того же key не повторяет fiscal sale |
| REST/BLE route freeze | PASS на software transport | MiniPOS tests: REST preferred, BLE only after READY/live probe, no REST→BLE retry after ambiguous send |
| Edge durability | PASS на SQLite implementation | WAL/FULL, durable-before-device, restart replay, signed sync/ACK, three-month ACK-gated GC |
| Edge storage pressure | PASS на software storage | 70/85/95/100 states; authenticated status API; ≥95% blocks before sequence/journal/device |
| Authn/authz and public flow control | PASS для software MVP | PROD OIDC RS256/JWKS with issuer/audience/key-rotation tests; DEV-only HMAC; scope/RBAC/cross-tenant service checks; tenant/IP rate limit with 429/Retry-After |
| Webhook security | PASS | canonical `t,kid,v1`, 300 s window, constant-time HMAC, SSRF/redirect controls, rotation/410/retry tests |
| Backup/restore | PASS для pilot drill | custom-format dump/restore обеих БД, restored-row assertion, RTO <120 s |
| Supply-chain inventory | PASS как NO-GO evidence | CycloneDX 1.6, file hashes and manifest; unsigned/unscanned build cannot become PROD_GO |
| Daisy SMART S | PASS только STUB boundary | debug/release builds and tests; release/PROD instantiation hard-disabled |
| Daisy/Datecs protocols | PASS для inventory/frame baseline | 88/73 classification, frame/BCC compilation and negative unknown-command behavior |
| Regression stability | PASS текущего software slice | два последовательных `make full-regression` после последнего deterministic fix |

## Незакрытая software implementation

| Gap | Почему completion не доказан | Следующее доказательство |
|---|---|---|
| PostgreSQL typed persistence + RLS | Ключевые aggregates имеют atomic differential typed projections; single/list GET выполняется через отдельный LOGIN/non-owner pool и fail-closed RLS transaction. PROD требует раздельных writer/reader users; реальные PostgreSQL tests используют reader credential и покрывают allow/deny | tenant-bound typed mutation repositories и оставшиеся aggregates |
| Production OIDC/OAuth 2.1 | PASS на уровне software: inbound RS256/JWKS cache+rotation/issuer/audience; outbound MiniPOS→Fiscal client credentials с cache, refresh и однократным 401 retry; HMAC/static service token запрещены в PROD | для deployment GO требуется evidence принятого внешнего IdP/client registration |
| Full card orchestration | Simulator/STUB не доказывает approve/decline/timeout/reversal/STAN/RRN semantics реального terminal SDK | vendor/acquirer adapter plus failure/reconciliation HIL |
| Per-command protocol semantics | Полная классификация не равна encoder/parser semantics каждой поддерживаемой команды | golden request/response vector and semantic test for every `SUPPORTED` command |
| BG-001…BG-025 executable trace | Части требований покрыты, но нет одного generated report, доказывающего 100% applicable rows | machine-readable requirement→rule→test→evidence register and verifier |
| Signed release/provenance/vulnerability evidence | SBOM создан, но подпись и release scan намеренно NO-GO | signed immutable artifacts, provenance attestation and attached critical/high-zero scan |
| OTA lifecycle | Нет signed manifest verification, A/B rollback, anti-downgrade и recovery implementation | approved hardware boot/update implementation and destructive tests |
| UI acceptance matrix | Builds проходят, но accessibility/touch/background/permission/browser matrix не автоматизирована полностью | Playwright/native runner evidence plus physical BLE/browser gates |
| Long-running resilience | Нет 72h network-flap и 7d accelerated journal soak | reproducible soak harness and reports with zero duplicate/loss |

## Внешние блокеры, формально исключённые из base MVP

Полный список и re-entry evidence находится в `MVP_GATES.md`. Daisy SMART SDK, Daisy Compact USB/electrical profile, DP-150 EUR firmware/electrical approval, BlueCash/BluePad/acquirer, microSD destructive power-loss, physical GATT/USB/UART HIL и НАП/БИМ/legal certification остаются `STUB_ONLY`, `DOCUMENTED_NOT_APPROVED`, `UNSUPPORTED` или `PROD_NO_GO`. Их нельзя закрыть simulator-тестом или конфигурационным флагом.

## Решение текущего среза

- Software regression: **PASS**.
- Base functional MVP: **IN PROGRESS**, пока не закрыты перечисленные software gaps либо не оформлено их допустимое исключение canonical roadmap.
- Pilot: **NO-GO** без выбранного hardware track и HIL evidence.
- Production Bulgaria: **NO-GO** без vendor/legal/НАП/БИМ evidence, завершённого runtime RLS, внешнего IdP deployment evidence и подписанного release pack.
