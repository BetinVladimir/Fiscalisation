# Static UI acceptance matrix review — 2026-10-21

This review follows the current extracted MiniPOS/BeeFiscal source and the newly integrated runtime fetch/body deadline helpers. It checks every original42 case and adds an explicit active BLE authority case. Static source evidence is necessary, but does not prove browser execution, UI geometry, physical BLE, cryptographic interoperability or legal acceptance.

The checker now requires each extracted source block **and caller wiring**, reports all failures, rejects empty evidence and keeps every OPEN case as a failing acceptance condition. No runtime implementation is changed by this review. It does not treat a random32-byte value, a manually entered key or a detached handshake module as authenticated BLE evidence.

## Review of every case

`EVIDENCE` means the required source locations were inspected and the updated static anchors resolve. It is not a runtime PASS. `OPEN` identifies behavior or an unresolved prior acceptance/design baseline.

| Case | Category | Review | Source evidence |
|---|---|---|---|
| UI-MP-001 | touch | EVIDENCE | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-002 | accessibility | EVIDENCE | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-003 | test_ids | EVIDENCE | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-004 | unknown | OPEN | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-005 | final_device | OPEN | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-006 | web_ble | EVIDENCE | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-007 | public_api | EVIDENCE | `minipos/BeeMiniPOS/App.tsx`, `minipos/BeeMiniPOS/src/nativeExecutionManager.ts`, `minipos/BeeMiniPOS/src/nativeExecutionAuthority.ts` |
| UI-MP-008 | barcode | EVIDENCE | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-009 | shift_close | EVIDENCE | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-010 | operator_identity | EVIDENCE | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-011 | reversal | EVIDENCE | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-012 | split_tender | EVIDENCE | `minipos/BeeMiniPOS/App.tsx`, `minipos/BeeMiniPOS/src/nativeExecutionAuthority.ts` |
| UI-MP-013 | fiscal_result_contract | OPEN | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-014 | ble_lifecycle | EVIDENCE | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-015 | network_timeout | EVIDENCE | `minipos/BeeMiniPOS/App.tsx`, `minipos/BeeMiniPOS/src/http.ts`, `minipos/BeeMiniPOS/src/http.test.ts` |
| UI-MP-016 | oidc_lifecycle | EVIDENCE | `minipos/BeeMiniPOS/src/emailAuth.ts` |
| UI-MP-017 | auth_loss | EVIDENCE | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-018 | shift_recovery | EVIDENCE | `minipos/BeeMiniPOS/App.tsx`, `minipos/BeeMiniPOS/src/sourceOrders.ts` |
| UI-MP-019 | z_reconciliation | EVIDENCE | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-020 | least_privilege | OPEN | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-021 | reports | EVIDENCE | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-022 | fiscal_identity | EVIDENCE | `minipos/BeeMiniPOS/src/fiscalIdentity.ts` |
| UI-MP-023 | device_resolution | EVIDENCE | `minipos/BeeMiniPOS/src/fiscalDeviceResolution.ts` |
| UI-MP-024 | shift_recovery_scope | EVIDENCE | `minipos/BeeMiniPOS/App.tsx` |
| UI-MP-025 | discount | EVIDENCE | `minipos/BeeMiniPOS/App.tsx` |
| UI-FISC-001 | public_api | EVIDENCE | `BeeFiscalApp/App.tsx` |
| UI-FISC-002 | readiness | EVIDENCE | `BeeFiscalApp/App.tsx` |
| UI-FISC-003 | reports | EVIDENCE | `BeeFiscalApp/App.tsx`, `BeeFiscalApp/src/screens/ReportsScreen.tsx` |
| UI-FISC-004 | accessibility | OPEN | `BeeFiscalApp/App.tsx`, `BeeFiscalApp/src/components/FiscalControls.tsx`, `BeeFiscalApp/src/ui/app-button.tsx` |
| UI-FISC-005 | admin_editors | EVIDENCE | `BeeFiscalApp/App.tsx`, `BeeFiscalApp/src/screens/AdministrationScreen.tsx` |
| UI-FISC-006 | allowed_actions | EVIDENCE | `BeeFiscalApp/App.tsx`, `BeeFiscalApp/src/screens/OperationsScreen.tsx` |
| UI-FISC-007 | network_timeout | EVIDENCE | `BeeFiscalApp/App.tsx`, `BeeFiscalApp/src/http.ts`, `BeeFiscalApp/src/http.test.ts` |
| UI-FISC-008 | admin_identity | EVIDENCE | `BeeFiscalApp/App.tsx`, `BeeFiscalApp/src/screens/AuthScreen.tsx` |
| UI-FISC-009 | oidc_lifecycle | EVIDENCE | `BeeFiscalApp/src/emailOtpAuth.ts` |
| UI-FISC-010 | auth_loss | EVIDENCE | `BeeFiscalApp/App.tsx` |
| UI-FISC-011 | least_privilege | EVIDENCE | `BeeFiscalApp/App.tsx`, `BeeFiscalApp/src/screens/OperationsScreen.tsx` |
| UI-FISC-012 | device_control_plane | OPEN | `BeeFiscalApp/App.tsx`, `BeeFiscalApp/src/screens/DevicesScreen.tsx`, `BeeFiscalApp/src/provisioningProtocol.ts`, `BeeFiscalApp/src/screens/AdministrationScreen.tsx` |
| UI-FISC-013 | audit | EVIDENCE | `BeeFiscalApp/App.tsx`, `BeeFiscalApp/src/screens/AuditScreen.tsx` |
| UI-FISC-014 | filters | EVIDENCE | `BeeFiscalApp/src/registerFilter.ts` |
| UI-PAGE-001 | pagination | EVIDENCE | `minipos/BeeMiniPOS/src/pagination.ts` |
| UI-BUILD-001 | platforms | EVIDENCE | `Makefile` |
| UI-E2E-001 | interaction_e2e | EVIDENCE | `minipos/BeeMiniPOS/e2e/web-interaction.mjs` |
| UI-MP-026 | authenticated_ble | OPEN | `minipos/BeeMiniPOS/App.tsx`, `minipos/BeeMiniPOS/src/nativeBleTransport.ts`, `minipos/BeeMiniPOS/src/webBle.ts` |

## Open requirements and decisions

- **UI-MP-004**: The UI assigns allowed_actions:[READ] locally when checkout becomes uncertain, so the claimed backend-owned READ permission is not demonstrated. Define and enforce the authoritative recovery-action contract rather than inferring permission from local state.
- **UI-MP-005**: Baseline decision remains unresolved: the invariant requires atomic Fiscal sale open on first product, while the accepted source-first implementation creates a source order and defers root preparation to payment. Reconcile the approved design/compliance baseline before closing this case.
- **UI-MP-013**: Current checkout maps every non-FISCALIZED result, including known FAILED, to UNKNOWN and clears success journal without authoritative source operation/order reload. Preserve final rejection semantics and obtain source-owned outcome before journal cleanup.
- **UI-MP-020**: Missing configuration sets admin=true regardless of production cashier role, while the Admin render and management callbacks lack canManage guards. Hiding the ordinary header menu alone does not prove that a cashier cannot see or invoke editors.
- **UI-FISC-004**: Extracted controls preserve accessible roles and 48dp minimum, but the previous 56dp primary-action evidence has no equivalent in shared AppButton. Confirm the accepted touch-size baseline and primary action coverage rather than treating a 56dp header as CTA evidence.
- **UI-FISC-012**: Provisioning delivery and metrics moved to extracted modules, but a manually entered X25519 public-key string plus REST session issuance is not proof of possession or an authenticated BLE session. Device response authentication and operational session protocol remain open.
- **UI-MP-026**: Both active transports prepare an unrelated random 32-byte value and support only OPEN_MVP. Signed command authority does not authenticate peripheral replies or provide the claimed session proof of possession; the separately implemented handshake is not active transport evidence.

The first-product and56dp entries retain the previous acceptance expectations as explicit design questions; they do not invent a new architecture or reject the approved source-first design without review. Closing them requires reconciling the approved design/specification and the matrix. The source-order recovery entry is updated to the accepted commercial source ownership and exact active-shift scope, with durable ambiguous checkout remaining in reconciliation.

The runtime network cases now inspect `fetchTextWithTimeout` at every App API-helper call site, independent Promise deadline, body consumption and the noncooperative-fetch/body-stall regression tests. This evidence applies to those runtime call helpers. Separate authentication-module raw fetches are not covered by those tests and must not be described as universally bounded networking.

Browser entrypoint anchors remain source evidence only. The interaction script's presence and assertion strings cannot replace an actual current browser report. Likewise, bundle command strings do not prove final Android/iOS artifacts.

## Verification

- `ruby scripts/test_verify_ui_acceptance.rb`:10 tests,14 assertions, PASS. Negative fixtures remove caller wiring, remove extracted guards, reintroduce a forbidden payment repeat, declare OPEN, use unsupported status, remove sources and erase required anchors. Existing platform/category/minimum coverage constraints remain enforced.
- `ruby scripts/verify_ui_acceptance.rb`: exits1 with seven explicit OPEN cases and no missing/moved-source anchors against the reviewed tree. This is the expected honest result; it is not a green readiness claim.

The checker/matrix alone cannot close these gaps. Runtime repairs need tests and reviewed evidence before their OPEN markers are removed.

Subsequent investigation adds UI-MP-027: a fresh cart currently requires source HTTP even when cached checkout authority exists. See [offline cart and UNP timing follow-up](43_OFFLINE_CART_AND_UNP_TIMING.md). The earlier seven-case count is the review baseline, not the current full backlog.
