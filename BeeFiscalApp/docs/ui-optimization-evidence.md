# BeeFiscal UI optimization evidence

Date: 2026-09-30. Overall status: **IN PROGRESS**. This file records verified increments, not completion of the BeeFiscal plan.

## Authentication increment

`App.tsx` now delegates the production email → OTP → company branch to `src/screens/AuthScreen.tsx`. The screen uses Paper `Card`, `Text`, outlined `TextInput`, `HelperText` and `Button`, with the active Paper theme and responsive `AuthForm`. Existing `useEmailOtpAuth` calls and API payloads remain unchanged. Email and OTP values remain visible after server errors. The privacy action uses a Paper button with link semantics.

The production and demo EAS profiles and web build script now set `EXPO_PUBLIC_APP_ENV=prod`. Before this change, neither release profile set the flag used by `App.tsx` to show authentication. The isolated auth E2E build explicitly enables the same mode; the existing developer E2E build keeps its original behavior.

Verified locally: `npx tsc --noEmit` PASS; `npm test` 8/8 PASS; `npm run ui:check` PASS; `npm run test:e2e` 6/6 PASS; `npm run test:e2e:auth` functional flow 1/1 PASS; `npm run build:web:production` PASS; `npm run build:web:demo` PASS. The auth E2E mocks the API and covers a failed challenge with preserved email, failed OTP with preserved code, successful verification, company selection, and arrival at the operational shell. No live authentication or fiscal mutation was exercised.

Before adding the theme/language header, strict email WCAG scans ran at xs 360×800, md 960×900 and lg 1440×1000 in both light and dark modes. All six reached the Paper email state, fit the expected form width and had no horizontal overflow or automatic Axe violations. Each had five `color-contrast` incomplete nodes, totaling 30. This is historical baseline evidence; the current strict command and counts are recorded below. The xs light/dark screenshots were visually inspected, which did not close AA or native checks.

## Theme and language increment

`src/ui/preferences.tsx` now provides one persisted language choice and system/light/dark theme mode. Web uses localStorage and native uses SecureStore; storage failure falls back to usable in-memory controls. The Paper provider waits for preferences to hydrate before displaying the app and uses one theme object for all Paper leaves. The auth header exposes the controls on email, OTP and company steps. The 25-language picker is one bounded 224 px scroll list composed of Paper `Card` and `List.Item`; selected state is visible as ✓ and exposed through `accessibilityState`. A direct Paper `Menu` experiment did not mount its list in this Expo Web setup, so the working dropdown composition uses Paper leaf components. Escape closes it on web.

Local Ollama `qwen3.5:9b-mlx` generated 22 missing auth locales from English; EN/BG/RU were authored directly. The generated catalog contains **25 locales × 14 auth keys** and `ui:check` verifies exact key coverage, non-empty strings and preserved BeeFiscal product name. Obvious action, role and brand errors in generated GA/MT and several other titles were corrected. This structural check and limited correction do **not** prove semantic accuracy in every language; native-speaker editorial review remains open. The rest of the operational app still has untranslated Bulgarian strings.

Functional checks after this increment: `npx tsc --noEmit`, `npm test` (8/8), `npm run ui:check`, `npm run test:e2e` (6/6), `npm run test:e2e:auth` (7/7), and both production/demo web exports passed. The auth E2E includes EN/BG/RU/DE/FR xs smoke, all 25 picker options with last-item selection, Escape, persistence of RU and dark theme after reload, `language: ru` in the challenge request, and email → OTP → company with error retention.

The dedicated strict `npm run test:e2e:auth-wcag` matrix scanned email and expanded language list at xs/md/lg × light/dark. All **12 scans** reached their intended states and reported **0 automatic Axe violations** after decorative Paper icons with unnamed `role=img` were removed. Axe still returned **42** `color-contrast` incomplete nodes on email and **150** with the list open. All 12 strict tests intentionally FAIL pending rendered-pixel/manual adjudication. Reports and screenshots are transient under ignored `test-results/auth-wcag-*/` until the next Playwright run.

The backend currently decodes `language` on `POST /public/v1/app-auth/challenges` but calls `StartAppChallenge` without it; `internal/integration/appauth.go` writes the OTP email in English. The UI sends the selected locale, but the email is **not localized**. Server errors remain raw API messages. These are open cross-system localization gaps, not completed translations.

On 2026-10-01 the auth privacy action was changed to use `https://legal.beeloy.org/{language}/privacy`, matching all 25 routes in the legal site's `app/seo.ts`. `npm test` passed 9/9 including a full 25-locale URL check; `npx tsc --noEmit`, `npm run ui:check` and `npm run test:e2e:auth` passed (7/7). The five xs E2E locale smokes clicked the Paper privacy link and confirmed the target URL without navigating to the live legal site. The legal site's translation quality and live availability were not assessed in this increment.

Open: OTP and company WCAG matrices, contrast incomplete adjudication, native VoiceOver/TalkBack and preference persistence, 200% text and keyboard checks, editorial review of 25 locales, and migration of the remaining `App.tsx` branches. The operational shell still uses fixed styles and Bulgarian text; theme preference there is not yet visually accepted. Steps 02.01–02.04 and G1–G3 remain open. No deployment was performed.
