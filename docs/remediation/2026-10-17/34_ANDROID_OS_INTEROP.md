# Android OS-key → issuer → S3 acceptance

2026-10-21: the previously pending Android interoperability item now passes for both offline leased and server-assigned online authorizations. [Versioned evidence](evidence/android-os-interop-2026-10-21/README.md) contains actual build/custody/vertical logs, exact module hashes and instrumentation APK SHA256. This supersedes earlier pending statements in doc28 and NATIVE_EXECUTION_AUTHORITY.md for this bounded harness only. Final merged Android application artifact and physical Android/iOS BLE HIL remain OPEN.

Separate BeeFiscalApp integration run:39 Node tests,95 UI tests, TypeScript and web export passed. Broad browser run failed:167 passed/122 failed. Its configuration mixed a synthetic-host-only presentation suite and production-auth-only tests into the ordinary app build; configuration separation is being verified and remaining UI failures are not accepted. The general UI gate remains OPEN.
