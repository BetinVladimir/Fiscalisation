# DEMO-OTP-001 — explicit demo email OTP, server only

Human/root scope: existing demo email-OTP issuers, never production bypass, PIN/pairing/step-up unchanged. Entry per AGENTS fixes-card option. No execution/status.json modification.

## DEMO-OTP-001.1
files: fiscal-backend/internal/integration/appauth.go; fiscal-backend/internal/integration/demo_otp_test.go
Test red first: real StartAppChallenge against disposable PostgreSQL must persist digest('demo') and localized outbox code only when exact ENVIRONMENT=demo and PRODUCTION!=true. Real VerifyAppChallenge must keep token/instance/expiry/status/membership/code guards; non-demo literal demo rejected even seeded hash. Normal numeric production works.
accept: cd fiscal-backend && GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/integration -run TestDemoOTP -count=1

## DEMO-OTP-001.2
files: minipos/beeminipos-backend/internal/emailauth/service.go; minipos/beeminipos-backend/internal/emailauth/demo_otp_test.go
Test red first: real RequestCode and VerifyCode with disposable PostgreSQL, same environment/code invariants, expiry/attempt-limit/unknown challenge unchanged. No changes onboarding/financial/PIN semantics.
accept: cd minipos/beeminipos-backend && GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/emailauth -run TestDemoOTP -count=1

Read-only context: existing postgres_fixture.py for isolated Unix-socket DB. Tests never use live demo DB, SMTP worker, real accounts or credentials. Three green executions, original red preserved. run-accept.py requires registered task ID; invoke/report its result without inventing status registration.

## Current execution constraint / incomplete acceptance

Root restated inherited rule: backend/DB execution only on demo; no further local DB launches. Initial disposable local-DB red/green evidence is preserved but NOT demo E2E acceptance. Current tests are pure production code-selection/Verify early-rejection tests only, three repeats; full persisted challenge acceptance remains NOT_RUN on approved demo fixture. No protected status registration was modified; run-accept reports unknown new ID, exact direct pure commands passed.
