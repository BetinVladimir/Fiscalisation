# BeeFiscal UI optimization evidence

Date: 2026-09-30. Overall status: **IN PROGRESS**. This file records verified increments, not completion of the BeeFiscal plan.

## Authentication increment

`App.tsx` now delegates the production email → OTP → company branch to `src/screens/AuthScreen.tsx`. The screen uses Paper `Card`, `Text`, outlined `TextInput`, `HelperText` and `Button`, with the active Paper theme and responsive `AuthForm`. Existing `useEmailOtpAuth` calls and API payloads remain unchanged. Email and OTP values remain visible after server errors. The privacy action uses a Paper button with link semantics.

The production and demo EAS profiles and web build script now set `EXPO_PUBLIC_APP_ENV=prod`. Before this change, neither release profile set the flag used by `App.tsx` to show authentication. The isolated auth E2E build explicitly enables the same mode; the existing developer E2E build keeps its original behavior.

Verified locally: `npx tsc --noEmit` PASS; `npm test` 8/8 PASS; `npm run ui:check` PASS; `npm run test:e2e` 6/6 PASS; `npm run test:e2e:auth` functional flow 1/1 PASS; `npm run build:web:production` PASS; `npm run build:web:demo` PASS. The auth E2E mocks the API and covers a failed challenge with preserved email, failed OTP with preserved code, successful verification, company selection, and arrival at the operational shell. No live authentication or fiscal mutation was exercised.

Strict email WCAG scans ran at xs 360×800, md 960×900 and lg 1440×1000 in both light and dark modes. All six reached the Paper email state, fit the expected form width and had no horizontal overflow or automatic Axe violations. Each had five `color-contrast` incomplete nodes, totaling 30. All six tests remain **FAIL** pending rendered-pixel/manual resolution; reports and screenshots were written under ignored `test-results/auth-paper-production-Paper-email-WCAG-AA-*/` and can be regenerated with `npm run build:e2e:auth && npx playwright test --config playwright.auth.config.ts -g 'production Paper email WCAG AA'`. The xs light/dark screenshots were visually inspected; this does not close AA or native checks.

Open: the auth screen has no persistent manual theme selector or 25-language dropdown; its copy remains Bulgarian. OTP and company WCAG matrices, email incomplete adjudication, native VoiceOver/TalkBack, 200% text and keyboard checks are not complete. Other `App.tsx` branches still contain custom interaction surfaces and fixed styles. Steps 02.01–02.04 and G1–G3 remain open.
