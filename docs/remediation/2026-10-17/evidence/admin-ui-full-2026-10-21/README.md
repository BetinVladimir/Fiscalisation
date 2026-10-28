# BeeFiscalApp normal browser regressions, 2026-10-21

Scope: optimized production-format Expo export of the current real App using the normal scripts' **demo-auth** environment and synthetic HTTP fixtures. This is neither the synthetic presentation host nor evidence of authenticated production or physical BLE operations.

Baseline main 3211ef8 normal browser run: **33 failed, 215 passed** (`/private/tmp/admin-browser-full-3211-run.log`). Fixed pre-HTTP full normal run: **249 passed**, terminal exit 0, 1.5 minutes (`/private/tmp/admin-browser-full-3211-fixed-run.log`). One additional test explicitly covers cancellation of the real provisioning dialog without claiming successful installation.

Runtime fixes in FiscalControls:

- Web radio Space/Enter now activates the explicitly current visible item; the layout-bound keyboard handler retains guards and bypasses stale Paper synthetic-click callbacks.
- Decorative nested Paper radios remain hidden and nonfocusable through later Pressability tabindex updates; a cleaned-up observer preserves that boundary.
- A speculative/interrupted React render no longer retires the still-committed control. Ownership changes only on commit. Retained callbacks remain rejected after replacement, disabling, and unmount.
- Deterministic React concurrent Suspense regression: old hook fails the interrupted-render assertion (other two safety tests pass), new hook passes all three. Actual browser key-event diagnostics also proved the former rejected current legitimate callbacks.

Browser contracts corrected without removing their invariants:

- Measure the exact visible Paper row frame, preserving contrast >=3, height >=48, containment, viewport and keyboard/focus checks. The contrast negative control still detects a fill-matching frame.
- Pending admin/diagnostic commands keep navigation locked, exactly one request and unchanged drafts; navigating resumes after completion. Their former tests waited for a response after the control's four-second timeout while trying disabled navigation.
- Check current accessible names and actual localized alerts/status, retaining retry, payload and fail-closed checks.
- Remove the fabricated success of legacy provisioning on an ACTIVE fixture. The real backend explicitly returns DEVICE_NOT_PROVISIONABLE for that case. Real dialog cancellation checks no BLE access, no installation mutation and secret clearing. Physical installation remains **OPEN**.

Legacy provisioning control-plane coverage remains separately verified in real Go handler/domain tests: TestAdministrativeSurfaceTenantIsolationAndBinding and TestProvisioningSessionIsTenantBoundAndSurvivesRestart. The current JS provisioning protocol's eight tests also pass, covering ciphertext frames, installed-proof confirmation, recovery, foreign identity, generation and cancellation. These are not a substitute for full real-device installation.

Root's later HTTP deadline commit 234c13d was copied into the isolated App/http/http.test and freshly built. Merged Node: 47/47; UI Jest: 98/98; TypeScript: exit 0. Merged API browser: **127/127 PASS**, terminal exit 0, 32.1 seconds (`/private/tmp/admin-browser-full-234c-merged-api.log`). Key modified source and test bytes match the original repository exactly; see MERGED_TESTED_SOURCE_SHA256SUMS. PRE_HTTP_TESTED_SOURCE_SHA256SUMS identifies the earlier full run.

Only temporary entry/module-resolution harness paths, local config port 19216 and output paths differ from repository defaults. No original repository mutations were performed by this task. Patches are in this directory; root applied them during integration. No checker/matrix, assertions, axe exclusions or financial gates were relaxed.

Versioned evidence: exact raw logs are inside deterministic gzip, with raw/compressed SHA256 and terminal outcomes in manifest. Baseline and owner-red logs intentionally record failures. Source manifests distinguish the pre-HTTP full249 run from fresh merged127 API cases. No claim that249 cases were all rerun on the merged HTTP source.
