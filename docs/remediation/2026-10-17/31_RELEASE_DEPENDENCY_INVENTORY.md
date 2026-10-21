# Release dependency inventory: bounded scope, remaining SBOM gaps

The release evidence remains **PARTIAL_LOCKFILE_INVENTORY**, not a complete runtime or deployed-artifact SBOM. `PROD_NO_GO`, external hardware/vendor/legal gates, Ed25519 manifest trust and SBOM-bound vulnerability scan checks are unchanged. A signed inventory is evidence of its bytes and stated inputs; it does not establish coverage of omitted components or physical certification.

## Included and required inputs

- `BeeFiscalApp/package-lock.json`
- `minipos/BeeMiniPOS/package-lock.json`
- `minipos/miniposweb/package-lock.json` (previously omitted)
- `fiscal-backend/go.sum` and `minipos/beeminipos-backend/go.sum`

Missing known inputs now stop generation instead of silently producing a smaller inventory. npm lockfile versions 2/3 require a packages map; a normal installed package without a version also fails. Root package entries and local/workspace links are not invented as registry packages. Nested install paths are reduced to the actual package name; explicit package names identify npm aliases. Scoped npm namespaces and version characters are percent encoded. Equal package/version identities deduplicate across clients; distinct versions remain separate components. All five exact input SHA-256 hashes are included in the signed manifest file_hashes. Existing compose image enumeration remains unchanged.

This is a union of lockfile records, including development/optional/platform dependencies. go.sum records can include modules not linked into a release binary. This inventory does not identify which dependencies reach a particular Android/iOS/web/firmware artifact. Local Beeloy shared sources are not registry identities and need their own source/material provenance.

## BlueCash remaining coverage

`SmartDevices/bluecash-app` and shared Android modules are not represented by the three npm locks. Add reproducible per-release-variant Gradle/Maven resolved dependency inventory, plugin/toolchain versions, transitive graph, native libraries and the exact packaged APK/AAB contents. Include separately identified vendor jars (`com.android.fiscal.jar`, `com.android.pinpad.jar`) with hashes, vendor/version provenance and available license information; do not infer their contents from filenames. Datecs/Daisy/native integrations and local shared code require explicit source/build identity. Bind the resulting SBOM to the actual signed Android artifact and scanner report before claiming Android coverage.

## IoT remaining coverage

Current inventory omits ESP-IDF and managed components, C/C++/vendored dependencies (including serialization and SQLite/crypto sources), compiler/linker/SDK/toolchain identities, bootloader/partition/application binaries and their linkage. Produce dependency/source manifests from the exact reproducible S3 build and bind hashes to actual firmware segments and release artifact manifest. Separate build tools/factory tooling from runtime components. Device/NVS key material is never an SBOM component or exportable evidence secret; hardware remains ESP32-S3 internal NVS, without ATECC.

## Executable regression and verification

`ruby scripts/tests/release_npm_inventory_test.rb` runs the actual generator against isolated lockfile fixtures: three-client coverage and input hashes, nested/scoped/alias identity, multiple versions, absent known inputs, unsupported/missing packages maps and missing versions. `make evidence-test` includes this regression before existing evidence and signing tests. Existing Ed25519 trust/tamper and foreign-SBOM scan rejection tests continue to pass. These checks do not remove any remaining SBOM or release gate.

Integrated verification on 2026-10-21: `PATH=/opt/homebrew/opt/ruby/bin:$PATH make evidence-test` exited 0: 5 tests / 41 assertions; actual repository inventory 1023 components; unsigned and production NO-GO checks plus Ed25519 trust/provenance/tamper and SBOM-bound scan regressions passed. Component count is snapshot-specific, not a completeness criterion.
