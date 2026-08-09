# MVP completion audit

Дата среза: 2026-08-09. Статус цели: **IN PROGRESS / PROD NO-GO**.

Этот документ не подменяет canonical roadmap. Он связывает фактическую реализацию в данном репозитории с критериями этапов 20–25 и запрещает считать simulator evidence доказательством hardware/legal readiness.

## Доказано текущим кодом

| Область | Статус | Авторитетное evidence |
|---|---|---|
| Fiscal и MiniPOS public API | PASS для реализованного MVP surface | 73 canonical + 8 runtime operations, `make contract-test`, API/race tests |
| Автономность MiniPOS | PASS | отдельная PostgreSQL/Compose; E2E читает MiniPOS при остановленной Fiscal PostgreSQL |
| EUR cash fiscalization | PASS на simulator | two-Compose sale через оба Caddy, immutable operation/receipt references |
| Ambiguous outcome | PASS | E2E требует 502 + durable `UNKNOWN`; replay того же key не повторяет fiscal sale |
| REST/BLE route freeze | PASS на software transport | MiniPOS tests: REST preferred, BLE only after READY/live probe, no REST→BLE retry after ambiguous send |
| Registry-aware final-device gate | PASS на software transport | readiness requires tenant-owned ACTIVE fiscal device; connectivity requires ACTIVE register binding; unknown/foreign/DRAFT/BLOCKED/unbound fixtures all return BLOCK before driver probe |
| BLE authority issuance | PASS на software transport | issue requires tenant-owned ACTIVE register with ACTIVE fiscal/smart-device binding; refresh is rejected after the bound device becomes BLOCKED; cross-tenant and unknown-register cases are race-tested |
| Registry-aware fiscal execution | PASS на central software path | sale, shift, payment, reversal, cash/report execution require tenant-owned ACTIVE register and ACTIVE fiscal-device binding before driver invocation; counting-driver negative test plus public-API-provisioned two-Compose sale prove fail-closed and success paths |
| Operator authority | PASS на central software path | sale/shift require a same-tenant operator within `active_from`/`active_to`; future and expired fixtures fail, while public-API-provisioned operator completes the two-Compose sale |
| Edge durability | PASS на SQLite implementation | WAL/FULL, durable-before-device, restart replay, signed sync/ACK, three-month ACK-gated GC |
| Edge storage pressure | PASS на software storage | 70/85/95/100 states; authenticated status API; ≥95% blocks before sequence/journal/device |
| Authn/authz and public flow control | PASS для software MVP | PROD OIDC RS256/JWKS with issuer/audience/key-rotation tests; DEV-only HMAC; scope/RBAC/cross-tenant service checks; tenant/IP rate limit with 429/Retry-After |
| Webhook security | PASS | canonical `t,kid,v1`, 300 s window, constant-time HMAC, SSRF/redirect controls, rotation/410/retry tests |
| Backup/restore | PASS для pilot drill | custom-format dump/restore обеих БД, restored-row assertion, RTO <120 s |
| Supply-chain inventory | PASS как NO-GO evidence | CycloneDX 1.6, file hashes and manifest; unsigned/unscanned build cannot become PROD_GO |
| Daisy SMART S | PASS только STUB boundary | debug/release builds and tests; release/PROD instantiation hard-disabled |
| Daisy/Datecs protocols | PASS для inventory/frame baseline | 88/73 classification, frame/BCC compilation and negative unknown-command behavior |
| Regression stability | PASS текущего software slice | после Stage 21/23, typed-only и BG evidence изменений два последовательных `make full-regression` на неизменённом tree; ранее также зафиксированы три последовательных цикла после tenant-bound artifact/Edge-sync cutover |

## Незакрытая software implementation

| Gap | Почему completion не доказан | Следующее доказательство |
|---|---|---|
| PostgreSQL typed persistence + RLS | PASS для canonical MVP: storage mode 2 использует typed tables как единственное активное durable representation; exact CAS delta, typed-only restart и rollback доказаны на PostgreSQL. Compatibility corruption игнорируется. Legacy `devices` исключён | удаление in-process coordinator maps является post-MVP refactor; физическое удаление пассивных compatibility rows — отдельная подтверждаемая destructive migration после rollback window |
| Production OIDC/OAuth 2.1 | PASS на уровне software: inbound RS256/JWKS cache+rotation/issuer/audience; outbound MiniPOS→Fiscal client credentials с cache, refresh и однократным 401 retry; HMAC/static service token запрещены в PROD | для deployment GO требуется evidence принятого внешнего IdP/client registration |
| Device provisioning enforcement | PASS на уровне software: creation is DRAFT-only; lifecycle transitions, ACTIVE-only register binding, active-register guard, PROD evidence references and provisioning eligibility are fail-closed and race/E2E-tested | application 24, approved-type registry and authorized-service dossier remain external deployment evidence |
| Full card orchestration | Simulator/STUB не доказывает approve/decline/timeout/reversal/STAN/RRN semantics реального terminal SDK | vendor/acquirer adapter plus failure/reconciliation HIL |
| Per-command protocol semantics | PASS для Datecs `Supported`: 88 Daisy/73 Datecs классифицированы; 8 Daisy/40 Datecs commands имеют validated builders/parsers и golden gate, включая все Datecs с disposition `Supported`. Покрыты storno с original-document/invoice/УНП, programmed PLU, PC mode, typed 13-field NRA delivery status, FM date/Z-range, operator/PLU reports, VAT/daily totals, FM structured records, EJ transport и modem identity/signal. Datecs 135 остаётся diagnostics, а регуляторное состояние берётся только из 71. Полная семантика EJ CSV columns и optional/privileged команды остаются отдельно gated | vendor/HIL proof for EJ CSV column variants; optional/privileged semantics only when their activation track is approved |
| BG-020 cross-boundary export | Реализация корректно формирует EUR export, но canonical `ComplianceExport` содержит один `artifact` и `official_currency: const EUR`; это не позволяет контрактно представить требуемые отдельные BGN/EUR файлы для периода через 31.12.2025/01.01.2026 | сначала additive OpenAPI revision с per-period artifact manifests/currency, затем two-period golden export и totals proof |
| BG-001…BG-025 executable trace | Машиночитаемый requirement→rule→API→test→evidence register и verifier реализованы; 6 PASS, 13 PARTIAL, 5 EXTERNAL_BLOCKED, 1 EXCLUDED_MVP. Реестр намеренно не повышает hardware/legal gaps до PASS | закрыть оставшиеся software-части PARTIAL; vendor/HIL/legal строки допускают переход только по внешнему evidence |
| Signed release/provenance/vulnerability evidence | SBOM создан, но подпись и release scan намеренно NO-GO | signed immutable artifacts, provenance attestation and attached critical/high-zero scan |
| OTA lifecycle | Нет signed manifest verification, A/B rollback, anti-downgrade и recovery implementation | approved hardware boot/update implementation and destructive tests |
| UI acceptance matrix | A 13-case executable Android/iOS/Web source/build gate covers both apps. MiniPOS has 48/56dp targets, live status, stable transport/BLE/payment/UNKNOWN/reconcile IDs and GET-only reconciliation. FiscalApp has accessible selected tabs, live Core/transport status, device/UNKNOWN/report semantics, optimistic public-API administration editors and loading/empty/error feedback. Full interaction automation and physical assistive-tech/BLE/browser evidence are not complete | Playwright/native interaction runners plus physical screen-reader/BLE/browser gates |
| Long-running resilience | PASS для accelerated software model: executable 72h/5-minute network-flap and 7d/10-minute journal scenarios produce a machine-readable report and prove zero loss/duplicate commit, exact ambiguous ACK replay, daily restart, final cursor/hash, three-month retention and purge-chain continuity. Physical wall-clock SD/hardware endurance is not claimed | physical 72h/7d selected-hardware HIL remains deployment evidence, not a base software implementation gap |

## Внешние блокеры, формально исключённые из base MVP

Полный список и re-entry evidence находится в `MVP_GATES.md`. Daisy SMART SDK, Daisy Compact USB/electrical profile, DP-150 EUR firmware/electrical approval, BlueCash/BluePad/acquirer, microSD destructive power-loss, physical GATT/USB/UART HIL и НАП/БИМ/legal certification остаются `STUB_ONLY`, `DOCUMENTED_NOT_APPROVED`, `UNSUPPORTED` или `PROD_NO_GO`. Их нельзя закрыть simulator-тестом или конфигурационным флагом.

## Решение текущего среза

- Software regression: **PASS**.
- Base functional MVP: **IN PROGRESS**, пока не закрыты перечисленные software gaps либо не оформлено их допустимое исключение canonical roadmap.
- Pilot: **NO-GO** без выбранного hardware track и HIL evidence.
- Production Bulgaria: **NO-GO** без vendor/legal/НАП/БИМ evidence, завершённого runtime RLS, внешнего IdP deployment evidence и подписанного release pack.
