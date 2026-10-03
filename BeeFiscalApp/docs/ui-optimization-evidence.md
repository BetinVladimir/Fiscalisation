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

The dedicated strict `npm run test:e2e:auth-wcag` matrix now scans email, expanded language list, OTP and company selection at xs/md/lg × light/dark. On 2026-10-01 all **24 scans** reached their intended states and reported **0 automatic Axe violations**. Axe returned **42** `color-contrast` incomplete nodes each for email, OTP and company selection, and **150** with the list open: **276 incomplete nodes total**. All 24 strict tests intentionally FAIL pending rendered-pixel/manual adjudication. The reports and screenshots are saved under `docs/ui-optimization-evidence/auth-wcag/` with `summary.json` carrying source commit, test hash, bundle hash and per-state counts. The OTP xs/dark and company xs/light screenshots were visually inspected for fit, not certified contrast.

A subsequent pixel diagnostic collected computed foreground/ancestor background, rendered bounds, screenshot color samples and opacity for all 276 incomplete nodes. **157** have candidate-supported pixel evidence (minimum observed candidate ratio **9.25:1**); **119** remain manual-review, including clipped menu items, empty inputs and differing DOM/screenshot backgrounds. This does not convert any Axe incomplete into WCAG PASS. The 24 `axe.json` reports and `summary.json` now include the diagnostic details; the strict matrix still fails. `e2e/contrast-evidence.spec.ts` passed 2/2 synthetic cases covering strong contrast and refusal of invisible, low-contrast and patterned evidence.

The backend now passes `language` from `POST /public/v1/app-auth/challenges` to `StartAppChallenge` and queues a localized OTP email for all 25 selectable languages. Generation and editorial-review details are recorded in the server increment below. Server errors remain raw API messages; the operational app is still largely untranslated.

On 2026-10-01 the auth privacy action was changed to use `https://legal.beeloy.org/{language}/privacy`, matching all 25 routes in the legal site's `app/seo.ts`. `npm test` passed 9/9 including a full 25-locale URL check; `npx tsc --noEmit`, `npm run ui:check` and `npm run test:e2e:auth` passed (7/7). The five xs E2E locale smokes clicked the Paper privacy link and confirmed the target URL without navigating to the live legal site. The legal site's translation quality and live availability were not assessed in this increment.

On 2026-10-01 the Fiscal backend OTP outbox path was localized without changing the public JSON request. `fiscal-backend/scripts/generate-app-otp-locales.mjs` generated 18 language pairs through local Ollama `qwen3.5:9b-mlx`; BG/EN/RU and four corrected languages are authored directly. The generated Go table contains 25 subject/body prefixes; the six-digit OTP is appended after translation and unknown language falls back to English. The SMTP worker now MIME-encodes non-ASCII subjects and quoted-printable bodies. `go test ./...` passed, including checks of 25 entries, code preservation, fallback and MIME round-trip. The HTTP→PostgreSQL outbox integration test was added but **skipped** locally because `PG_INTEGRATION_URL` is not configured. Translation quality still requires native-speaker editorial review and a live SMTP delivery check.

Open: contrast incomplete adjudication for all four auth states, native VoiceOver/TalkBack and preference persistence, 200% text and keyboard checks, editorial review of 25 locales, and migration of the remaining `App.tsx` branches. The operational shell still uses fixed styles and Bulgarian text; theme preference there is not yet visually accepted. Steps 02.01–02.04 and G1–G3 remain open. No deployment was performed.

## Operations and responsive shell increment (2026-10-01)

The operations list moved from `App.tsx` into `src/screens/OperationsScreen.tsx`, using actual Paper Card/Content/Text/Button leaves. It preserves the backend `allowed_actions` gate and the existing reconcile callback; the E2E still proves one reconcile request and no duplicate fiscal sale. The operational navigation now uses Paper Drawer.Item in a persistent section at md/lg and a Paper Modal/Drawer on xs. This fixes a measured xs horizontal overflow caused by the prior fixed 190 px sidebar. Header, toolbar and bottom status adapt to xs width; Paper theme colors now drive their surfaces. The shared `ReferencePicker` uses Paper TextInput/List.Item/HelperText/Button, and the refresh and probe actions are Paper Buttons. The previous xs/dark screenshot exposed an almost unreadable light search input; the current screenshot shows theme-consistent dark Paper input and visible selection.

Verification on current source: `npx tsc --noEmit`, `npm run ui:check`, `npm test` (9/9), `npm run test:e2e` (14/14, including six xs/md/lg × light/dark operations layouts), `npm run test:e2e:auth` (7/7), and production/demo web exports passed. The strict `npm run test:e2e:operations-wcag` reached the populated operations screen in all six size/theme combinations, with no runtime errors, horizontal overflow or automatic Axe violations. It remains **FAIL** because Axe reported **124 `color-contrast` incomplete nodes**; pixel diagnostics preliminarily support 98 and leave 26 for manual review. Reports and screenshots are stored at `docs/ui-optimization-evidence/operations-wcag/`, with source-file/test/bundle hashes in `summary.json`. The xs light and dark screenshots were visually inspected for fit and the corrected search-field appearance; this is not a full WCAG judgment.

Remaining for this branch: localize operational copy to all 25 locales, finish list empty/loading/error and details states, date/amount formatting, keyboard/200% text and native checks, and adjudicate all contrast incomplete nodes. The other operational branches and much of `App.tsx` still use legacy wrappers and fixed styles. Step 02.01 and gates G1–G3 remain open; no Cloudflare deployment or store submission occurred.

## Reports increment (2026-10-01)

The reports branch now uses `src/screens/ReportsScreen.tsx` with Paper Card, Text, Button, HelperText and ActivityIndicator components. It has distinct populated, empty, loading and API-error states with a retry action. Switching tabs clears stale records; a failed list request cannot display devices or prior reports. Report commands stay disabled during their request, and a pending guard prevents duplicate POSTs while preserving the existing idempotency key and role/register checks.

Verification on this source passed: `npx tsc --noEmit`, `npm run ui:check`, `npm test` (9/9), `npm run test:e2e` (23/23, including six xs/md/lg × light/dark layouts, empty/error/retry and one pending POST), `npm run test:e2e:auth` (7/7), and both production/demo web exports. The E2E tests use a mocked fiscal API; no live report command ran.

The strict `npm run test:e2e:reports-wcag` inspected populated, empty and error states at xs/md/lg in both themes: 18 scans, zero automatic Axe violations, and 438 `color-contrast` incomplete nodes. Pixel diagnostics preliminarily support 360 candidates and leave 78 for manual review. Thus the strict command remains **FAIL**, and WCAG AA is not certified. Screenshots and Axe reports are saved under `docs/ui-optimization-evidence/reports-wcag/`; the summary records test/source/bundle hashes. The xs/dark error screenshot was visually inspected for fit, not accepted as a contrast audit.

Remaining: operational translations for all 25 locales, final manual contrast adjudication, 200% text and keyboard/native checks, and migration of the other operational screens. Gates G1–G3 remain open. No deployment or store submission occurred.

## Audit increment (2026-10-01)

The read-only audit journal moved into `src/screens/AuditScreen.tsx` with Paper Card, TextInput, Button, Text, HelperText and ActivityIndicator. The seven existing query filters and the `ADMIN`/`AUDITOR` role gate remain wired to the public API. Technical record fields, including the hash, are disclosed through a separate Paper Button; the summary shows event type and timestamp. Switching tabs or reapplying filters clears stale records. Empty, loading and API-error states are distinct, and an error offers a retry without displaying old events.

Local checks: `npx tsc --noEmit`, `npm run ui:check`, `npm test` (9/9), `npm run test:e2e` (31/31), `npm run test:e2e:auth` (7/7), and production/demo web exports passed. The E2E matrix covers xs/md/lg × light/dark, expanded details scrolling above the fixed footer, empty and error/retry states, and preservation of filter query parameters. It uses a mocked API; no live audit data was read.

The strict `npm run test:e2e:audit-wcag` reached populated, expanded, empty and error states on xs/md/lg in both themes: **24 scans, zero automatic Axe violations, 834 `color-contrast` incomplete nodes**. Pixel diagnostics preliminarily support 367 candidates and leave 431 for manual review. The strict suite is **FAIL**; this is not WCAG AA certification. The full reports, screenshots and hashes are under `docs/ui-optimization-evidence/audit-wcag/`. The xs/light expanded and xs/dark populated screenshots were visually inspected for layout, not accepted as a final contrast audit.

Remaining: finish locale coverage of operational text, manually adjudicate incomplete contrast nodes, verify native screen reader/keyboard and 200% text behavior, and migrate devices, administration and settings. Step 02.03 and gates G1–G3 remain open. No deployment or store submission occurred.

## Regulatory settings and operational preferences increment (2026-10-01)

The settings branch moved into `src/screens/SettingsScreen.tsx`. It now presents the public country-policy contract fields and the `/tax-groups` array in Paper Card/Text, with Paper loading, empty and API-error/retry states. The E2E fixture was corrected to match the real API contract: `official_currency` and a bare tax-group array instead of the former mock-only `currency` and `{items}` response. Refresh clears stale policy before loading; a failed request cannot show old data.

The operational shell now renders the same bounded Paper language list and system/light/dark theme controls as authentication. E2E checks 25 available choices, scrolling on xs, persisted RU/theme after reload and no horizontal overflow. This is control availability only: most operational copy still remains Bulgarian for the selected non-BG locales and must be translated before locale completion can be claimed.

Local checks passed: `npx tsc --noEmit`, `npm run ui:check`, `npm test` (9/9), `npm run test:e2e` (40/40), `npm run test:e2e:auth` (7/7), and production/demo web exports. The E2E matrix covers settings xs/md/lg × light/dark, populated policy and tax groups, empty tax groups, error/retry and scrolling above the fixed footer. The strict `npm run test:e2e:settings-wcag` reached populated, tax-empty and error states on six size/theme combinations: **18 scans, zero automatic Axe violations, 456 `color-contrast` incomplete nodes**. Pixel diagnostics preliminarily support 370 and leave 86 for manual review. Its strict result remains **FAIL**; WCAG AA is not certified. Reports and screenshots with source/test/bundle hashes are under `docs/ui-optimization-evidence/settings-wcag/`. The xs/light populated screenshot was visually inspected for layout only.

Remaining: translate operational copy across all 25 locales, adjudicate contrast incomplete nodes, test native assistive technology, keyboard and 200% text, and migrate devices and administration. Gates G1–G3 remain open. No deployment or store submission occurred.

## Devices and administration shared Paper increment (2026-10-01)

The device list now uses Paper List.Item and Chip for its name, model, transport, selection and backend status. It never substitutes a disconnected device with a ready label. The former light device-control panel with an unreadable pale heading in dark mode was replaced by a theme-aware Paper Card and Paper Text. Shared `Field`, `Action`, `AdminCard` and existing-record choices now render actual Paper TextInput, Button, Card, Text and List.Item leaves; this changes device workflows and administration forms without changing their API mutation handlers.

Refreshing devices clears the previous selection, telemetry and records before the request. A failed request now shows a Paper error/retry card and hides stale device controls; an empty response has its own state. Functional E2E covers the existing diagnostics, printer, provisioning, BLE and SmartDevice mutation flows, device list xs/md/lg × light/dark, disconnected and empty states, error/retry with stale-record clearing, and administration card layout at the same six size/theme combinations. Final checks passed: `npm test` (9/9), `npm run ui:check`, `npx tsc --noEmit`, `npm run test:e2e` (55/55), `npm run test:e2e:auth` (7/7), and production/demo web exports. E2E used a mocked public API; no live device mutations ran.

The strict `npm run test:e2e:devices-admin-wcag` scanned device populated/disconnected/empty/error plus administration top/device-form on xs/md/lg in both themes. The first run found 48 `aria-prohibited-attr` incomplete nodes in choice groups; changing them to a list role exposed 48 critical `aria-required-children` violations because interactive buttons are not list items. React Native's supported `toolbar` role describes the interactive choice collection. On final source the **36 scans reported zero automatic Axe violations and 1127 `color-contrast` incomplete nodes**. Pixel diagnostics preliminarily support 741 and leave 386 for manual review. The strict suite remains **FAIL**, not WCAG AA certification. Reports, screenshots and hashes are in `docs/ui-optimization-evidence/devices-admin-wcag/`. The xs/dark device screenshot was visually inspected to confirm the corrected panel appearance.

Remaining: extract complete `DevicesScreen` and `AdministrationScreen`, replace their remaining legacy text and tenant/logout controls, add native/keyboard/200% checks, finish 25-locale operational translations and manually adjudicate contrast. Steps 02.01, 02.02 and 02.04 and gates G1–G3 remain open. No deployment or store submission occurred.

## Paper shell leaves and refreshed accessibility evidence (2026-10-01)

`App.tsx` no longer imports or renders `AppText`, `AppPressable` or `FormField`. Its remaining text uses Paper Text, the tenant switcher and logout use Paper Button, and loading uses active Paper theme colors. `scripts/check-ui-guideline.mjs` now fails if these legacy leaves return to `App.tsx`. The production auth E2E also clicks logout and verifies a return to the email screen. On final source `npm test` (9/9), `npm run ui:check`, `npx tsc --noEmit`, `npm run test:e2e` (55/55), `npm run test:e2e:auth` (7/7), and production/demo web exports passed.

After this change the six strict WCAG suites were rerun and their reports refreshed: auth 24 scans/276 incomplete; operations 6/136; reports 18/474; audit 24/826; settings 18/456; devices and administration 36/1127. Across **126 scans**, there were **zero automatic Axe violations** and **3295 `color-contrast` incomplete nodes**. Pixel diagnostics preliminarily support 2156 candidates and leave **1139 for manual review**. The audit rerun additionally removed 36 `aria-prohibited-attr` incomplete nodes by dropping unsupported labels from passive containers; the final audit report contains only color-contrast incomplete. Each suite still exits **FAIL** by design until those nodes are adjudicated. This is evidence of coverage and progress, not a WCAG AA pass.

The reports for audit were regenerated after its final ARIA source edit; the other five suites were regenerated immediately before that audit-only edit. Their rendered screens and the shared `App.tsx` were unchanged by it, while the overall JS bundle hash changed. A final whole-app audit will refresh every artifact together after all screen migrations. Native screen readers, keyboard and 200% text checks, translations outside auth, and the remaining screen extraction still block G1–G3. No deployment or store submission occurred.

## Device extraction, search and register context increment (2026-10-01)

The device branch moved to `src/screens/DevicesScreen.tsx`; shared Paper fields, buttons, cards and record choices moved to `src/components/FiscalControls.tsx`. Device search now distinguishes no matches from an empty API response. Selecting another register clears stale device details and reloads the active list. Operations and reports requests receive the newly selected register ID immediately, rather than the previous value captured by React state. Selected record choices expose an explicit selected label to assistive technology on web, where Paper's `List.Item` did not forward `aria-selected`.

`npx tsc --noEmit` and `npm run ui:check` passed. Functional `npm run test:e2e` passed **57/57**, including the new two-register request test and search state. The strict devices/administration matrix reached seven states at xs/md/lg × light/dark: **42 scans, zero automatic Axe violations, 1318 color-contrast incomplete nodes**. Pixel diagnostics preliminarily support 886 candidates and leave **432 for manual review**. The suite exits **FAIL** pending adjudication; it does not establish WCAG AA. Reports and screenshots are in `docs/ui-optimization-evidence/devices-admin-wcag/`; `summary.json` records the source hashes and bundle. The xs/dark populated device screenshot was visually inspected for fit and theme consistency.

Remaining: extract administration into its own screen, complete register-scoped device behavior against the real API contract, localize operational copy, and run keyboard, 200% text and native assistive-technology checks. Steps 02.01–02.04 and G1–G3 remain open. No deployment or store submission occurred.

## Register-bound device context (2026-10-01)

The fiscal API contract confirms that `GET /devices` lists company devices without a `register_id` filter, while `GET /registers` exposes `fiscal_device_id` and `payment_terminal_id`. The operational device list now uses those actual binding IDs to show only devices for the selected register; unbound devices remain available in administration. Switching tenant clears the prior register selection. The two-register E2E fixture verifies R01 → R02 → R01 changes the visible device and never shows the other register's device. `npx tsc --noEmit`, `npm test` (10/10), `npm run ui:check`, functional `npm run test:e2e` (58/58), production auth `npm run test:e2e:auth` (7/7), and both `npm run build:web:production` and `npm run build:web:demo` passed.

The refreshed strict devices/administration WCAG run produced **42 scans, zero automatic Axe violations, 1325 incomplete nodes**. Pixel diagnostics preliminarily support 886 candidates; **439 remain manual-review**. The suite still exits **FAIL** and does not prove WCAG AA. Its saved `summary.json`, Axe reports and screenshots now match this source and web bundle. Native assistive-technology and 200% text checks remain open; no deployment occurred.

## Device readiness versus registration status (2026-10-01)

An `ACTIVE` device registration does not prove the fiscal device is reachable. The selected device now shows a separate readiness line from `GET /devices/{id}/readiness`. Until that response arrives it says “Проверява се”; a false or unavailable response says “Не е готово”. The registry chip remains explicitly labelled as registration status. The device row includes the selected register code. `/health` is adapter telemetry and may legitimately be absent for a fiscal device; readiness, health and activity now load independently, so a 404 from `/health` does not suppress a valid readiness response. Switching devices or registers clears old readiness and telemetry before the next request.

Typecheck and `ui:check` passed, `npm test` passed 10/10, functional E2E passed **60/60**, and production/demo web exports passed. E2E covers false readiness despite `ACTIVE`, missing adapter health, and R01 → R02 → R01 without stale readiness. The strict devices/administration matrix now covers an unavailable device as a separate state: **48 scans, zero automatic Axe violations, 1512 incomplete nodes**. Pixel diagnostics preliminarily support 1038 candidates and leave **474 for manual review**. The strict result remains **FAIL**, not WCAG AA certification. Axe reports, screenshots and source/bundle hashes are under `docs/ui-optimization-evidence/devices-admin-wcag/`. The xs light/dark unavailable screenshots were inspected: the registration status and “Не е готово” are separately legible and the page has no horizontal overflow. Native screen reader and 200% text checks are still open; no deployment occurred.

## Administration screen extraction and form validation (2026-10-01)

The five administrative Paper cards moved from `App.tsx` into `src/screens/AdministrationScreen.tsx` through typed props. The existing CRUD and binding callbacks, role gate and test IDs remain connected. `ui:check` now asserts that `App.tsx` delegates the branch and that the screen imports actual Paper components. A new location-form E2E exposed a success message that was immediately overwritten by list refresh. The mutation flow now reports success after a successful refresh and reports a saved-but-not-refreshed state if the refresh fails; server mutation errors keep the form input for retry.

The xs screenshot exposed a clipped “read-only” transport JSON label on an editable field. This is now a shorter Paper `TextInput` label with Paper `HelperText`; invalid JSON or a JSON value other than an object displays a local error and disables device save. Functional E2E verifies invalid → valid input and location save error → retry with the same payload. On final source `npx tsc --noEmit`, `npm run ui:check`, `npm test` (10/10), `npm run test:e2e` (62/62), `npm run test:e2e:auth` (7/7), and production/demo web exports passed.

The strict devices/administration matrix now includes invalid JSON: **54 scans, zero automatic Axe violations, 1718 incomplete nodes**. Pixel diagnostics preliminarily support 1148 candidates; **570 require manual review**. The strict suite remains **FAIL**, so WCAG AA is not certified. Reports and screenshots are under `docs/ui-optimization-evidence/devices-admin-wcag/`, with source hashes for both extracted screens and the shared Paper controls. The invalid JSON xs/dark screenshot was inspected for label fit and error visibility. Remaining: complete field-by-field form validation and localization, extract the shell, run keyboard/200% text and native checks, adjudicate contrast, and verify all 11 applications. No deployment or store submission occurred.


## Privacy action affordance (2026-10-03)

The auth privacy policy action was a text-only Paper Button without a persistent shape. It now uses actual outlined Paper Button,48px target and wrapping label; existing linkrole,privacyPolicyUrl(language),Linking handler unchanged. Baseline7authPASS;newframecheck first5FAIL/2PASS. Afterpatch8authPASS,including EN/BG/RU/DE/FR xs samelegalURLs and6 xs/md/lg lightdark frame/height/theme/overflowcontexts. Typecheck andui:checkPASS. Evidence privacy-affordance retains logs/sourcehash/contract. No liveauth/fiscalcommands. FullAA/nontextframecontrast/focus/200%/all25privacybrowser/native/manual andotherappcriteriaOPEN.


## Audit expansion action affordance (2026-10-03)

AuditEventCard detailsaction now actualoutlinedPaperButton,minHeight48,wrappinglabel. Eventfields,hash/filtercallbacks/localexpanded/showhide unchanged. Baseline62functionalPASS;newframeassert6FAIL beforepatch. Afterpatch62functionalPASS,including6xs/md/lg-lightdark frame/48px/sameHashdisclosure/showhide/footerbounds cases.Typecheck/uiPASS. Evidence audit-affordance. InstalledPaperButton overwrites accessibilityState withdisabled:retainedexpandedprop isnotverifiedsemanticexpansion;followupneeded. No realauditAPI/fiscalcommands. All25auditcopy/200%/focus/nontextcontrast/strictAA/native/manual/full11criteriaOPEN.


## Actual Paper audit disclosure semantics (2026-10-03)

Details usescontrolled genuinePaperList.Accordion,same localexpanded/onPress/labels/recordfields/hash. NativePaperAccordion forwardsaccessibilityState.expanded;installedRNWeb ignores this deprecated object,so aweb-onlycompatibilityeffect onownheaderrolebutton setsaria-expanded throughstructuralViewref. NoPaperfork/custombutton. Header keepsvisibleoutline,min48,unlimitedtitlelines. BaselineButton6FAIL;firstAccordion56PASS/6FAIL proved RNWebgap;final62functionalPASS including6xs/md/lg-lightdark ariafalse→true→false/Enterexpand/Spacecollapse/samehash/footerbounds/frame48. Typecheck/uiPASS. Evidence audit-expanded. NativeVoiceOver/TalkBack/manual/themechangewhileexpanded/200%/all25copy/strictAA/nontext/focuscontrast/currentaudits remainOPEN;no liveAPI/fiscalcommands.


### Audit decorative Paper chevron (3b7701f)

24 strict scans: role-img-alt violations 12 before, 0 after; 826 color-contrast incomplete nodes retained. AA remains INCOMPLETE. Six xs/md/lg light/dark journal functional tests, TypeScript and ui:check PASS. Evidence audit-chevron and refreshed audit-wcag. Native/manual, 200%, all25 copy, nontext/focus contrast and full app criteria remain open.


### Persistent action frames on Paper lists

Device/reference/choice/language rows and Drawer navigation now have permanent theme-outline frames; actual Paper alert Buttons use outlined mode/minimum48. Handlers unchanged. TypeScript/ui PASS; full62 functional E2E PASS, with six xs/md/lg light/dark device frame and Enter assertions and shell language frame/selection checks. Reference/Choices/navigation/alert require dedicated runtime evidence; latest strict WCAG evidence predates these source edits. Nontext/focus contrast, native/manual,200%,all25 and full criteria remain open. Evidence list-affordance.


### Navigation/reference/Choices browser affordance

Six xs/md/lg light/dark dedicated tests PASS: permanent frame/48 target/viewport bounds, Enter register and form choice, Space navigation, no API mutations. All68 functional E2E and TypeScript PASS. Production e9c051a unchanged; second-register test API composite endpoint added, selected initialization reflected in test. Initial interrupted fixture failure retained. Evidence navigation-affordance. Alert runtime, numerical frame/focus contrast, strictAA/native/manual/200%/all25/full criteria remain open.

Current devices/admin strict audit on unchanged production source e9c051a: 54 scans, 0 violations, 1718 incomplete nodes. Strict command exit1, AA remains INCOMPLETE; 9 states/3 sizes/2 themes only. Screens/full axe JSON retained in devices-admin-wcag.


### Frame and keyboard focus contrast (supplemental)

7 dedicated tests PASS including negative frame control. Six xs/md/lg light/dark contexts x reference/Choices/Drawer:36 frame comparisons min4.959729578789477;18 focus comparisons min3.1109458559692036. Computed sRGB/RGBA backgrounds, reject unsupported layers, settled opaque animation; Chromium auto outline visible/width-positive. Keyboard-only menu opening Enter, selection Enter and navigation Space, bounds/48/no mutations retained. This is supplemental numerical evidence, not pixel/occlusion/manual/native adjudication or full AA. Existing1718 strict incomplete nodes remain open. Evidence frame-focus-contrast.


### Navigation CSS200 clipping and shell reflow

Baseline six contexts fail truncated Drawer labels. Genuine Paper List.Item multiline navigation fixes labels; header/toolbar wrap fixes md overflow; ScrollView nav190 with no flex-grow fixes visual regression and preserves lower-tab scrolling. Six CSS200 xs/md/lg light/dark action scenarios PASS, doubled actual font measurements/screens retained;69 normal functional/types/ui PASS. Dedicated action frame/focus/keyboard/no-mutations retained. Other200% labels, actual zoom/native/all25/pixel/manual and current full strictAA refresh remain open; older strict evidence predates this source. Evidence navigation-text200.


### Multiline genuine Paper field labels

FormField now shows full label with genuine Paper Text above genuine outlined Paper TextInput (floating label undefined); accessibility name/values/change/errors/help preserved. Actual FiscalControls Field/ReferencePicker migrated. Six CSS200 size/theme contexts x3 search/location/X25519 labels PASS, actual doubled metrics/width-height bounds and input/filter/noAPI mutations checked.75 full functional/types/ui PASS; xs-light screenshot visually inspected. Earlier screenshot baseline demonstrated truncation, not dedicated red test. Remaining fields/auth/currentstrictAA/all25/realzoom/native/manual/textcontrast/full criteria OPEN. Evidence field-labels.


### Remaining extracted-screen fields use genuine shared Paper labels

Auth email/OTP, Audit7 filters, device search and final Administration multiline Transport JSON use FormField. Input types/autocomplete/maxLength/value/onChange/error/multiline unchanged.75 final functional/types/uiPASS;14 authPASS incl6 CSS200 email/OTP contexts (Auth source unchanged since run; later Administration-only change).6 operational CSS200contexts x12labels=72metrics, including7Audit/JSON/search/device/location/X25519, with JSON-invalid save disabled/noAPI mutations. Auth fake challenge means no actual email/OTP. Evidence all-field-labels. No direct TextInput consumers remain in these extracted screens; generic Paper FormField owns leaf. CurrentstrictAA/contrast/native/manual/all25/realzoom/full11criteriaOPEN.


### Supplemental Paper label text contrast and current devices/admin strict audit

78 computed comparisons (72labels+6JSONerror), six size/themeCSS200 contexts, min7.097957347416672;7label tests incl invisible-text negative control PASS,7action/frame/focus regressionPASS,typesPASS. RGBA composition rejects unsupported layers; notpixel/occlusion/native/manual/fullAA. Current strict devices/admin54scans: 0 violations, 1586 incomplete nodes;commandexit1/AA INCOMPLETE. All shared field/helper source hashes and screenshots retained. Remainingauth/othertext/currentauditstrict/native/all25/realzoom/full11criteriaOPEN. Evidence label-text-contrast and devices-admin-wcag.


### Genuine Paper language menu keyboard and expanded semantics

Six baseline keyboard cases fail missing aria-expanded. Own web-trigger compatibility and genuine Paper list roving focus/menuitemradio checked semantics added. ArrowUp/Down wrap/Home/End/scroll/Escape/select restore/TabShiftTab exit/reopen and unique aria-controls target rolemenu tested.82finalfunctional/14auth/types/uiPASS. Strict6auth-language-open scans:0violations,6aria-valid-attr-value incomplete +138color-contrast nodes, exit1/AAINCOMPLETE. Initial missing-ID interpretation was wrong: installed axe precheck always requests review with haspopup; nativeID already existed. Extra root ID temporarily introduced duplicate-ID6, retained and removed. Final unique original nativeID verified. No waivers. Native expanded/screenreaders/manual/fullmenuCSS200/all25/fullcriteriaOPEN. Evidence language-keyboard.


### All25 language menu CSS200

Six xs/md/lg light/dark scenarios PASS,150 actual locale selections. Canonical source literal read without native runtime/eval.150 doubled labelmetrics/textcontrast +300frame comparisons, labels no clip/item48/bounded225/focused item within scrollviewport, keyboard select/return/real preferences persisted. Pageerrors/API mutations/horizontaloverflow0. Productiondc902b6 unchanged; tests/typecheckPASS. Evidence language-all25-text200. This proves menu names/selection only, not fullapp translations/fullAA/native/manual/realzoom; existing6ARIA+138contrast strictincomplete remainopen.


## BeeFiscal Portal/Card overlay fix — 2026-10-04

Closed shared Paper Dialog portals unmounted; actual Axe stack proved decorative card-outline above content, so genuine Auth/language Card uses contained mode with theme border/background on container. Baseline24scans0VI/270incomplete nodes; portal-only228; current0VI/97nodes (91color/6aria),13strictPASS11FAIL.14auth functionalPASS,2diagnostic capturesPASS (not AA),types/UI/exportPASS. Remaining OTP virtual stack: transient Paper ripple above input; investigate motion/stability. Evidence docs/ui-optimization-evidence/empty-dialog-portal.02-001/007/008/016/020 IN_PROGRESS; remainingstates/native/manual/all11 fullcriteria OPEN.


## BeeFiscal real reduced motion — 2026-10-04

02-001/010/016/020 IN_PROGRESS. Actual OTP Axe virtual stack showed transient ripple span. Genuine PaperProvider now honors live web prefers-reduced-motion and native AccessibilityInfo through official rippleEffectEnabled and animation.scale, cleans subscriptions and restores defaults when off.6 real mouse ripple present/absent/present testsPASS xs/md/lg light/dark,14auth functionalPASS/types/UI/exportPASS. Strict24tests18PASS6FAIL:0VI/12incomplete78nodes (72color+6aria, all language-menu), previous97. Email/OTP/company sixcontexts each pass automatic checks; menu stillINCOMPLETE. Evidence docs/ui-optimization-evidence/reduced-motion. Native/manual/alloperational/all11 fullcriteria remain OPEN.
