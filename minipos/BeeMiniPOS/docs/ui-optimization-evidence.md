# BeeMiniPOS UI optimization evidence

Status: **IN PROGRESS**. The app-wide manifest, WCAG matrix, native screen-reader checks, and Paper migration remain open.

## Operator login, email and OTP

The unauthenticated screen places the title, fields, actions and error message in a Paper Card; theme and language menus sit in the header. Email and OTP actions use Paper Button, the fields resolve to Paper TextInput through `FormField`, and server errors use Paper HelperText. The form width uses the shared xs/md/lg thresholds with a 320–480 px target capped by available space.

Validation on 2026-09-30: `npx tsc --noEmit`, `npm run ui:check`, and `npm run build:web:production` exited 0. The production E2E fixture was rebuilt with the environment from `Fiscalisation/Makefile` before `npm run test:web-interaction`; it exited 0 and checked xs/md/lg form widths, the working theme and language menus, and fail-closed login with no personal email. This E2E also exercised the existing sales and BeeFiscal journeys. The generated production export is tracked in `dist-cloudflare/production`.

No WCAG AA claim follows from these checks: the complete state/viewport/theme scan and visual contrast review have not run. Native iOS/Android, 200% text, keyboard, screen-reader and additional onboarding states remain to be tested.

## Onboarding continuation

The long onboarding form now scrolls on xs and uses Paper TextInput, Checkbox.Item and Buttons. Theme and language controls sit in a separate header so the scroll area cannot clip their menus. On web, Paper Menu's animation remained at opacity 0 in this layout; the two menus use a zero animation scale while retaining the standard Paper menu and items. `@expo/vector-icons` was installed through `expo install` because Paper logged missing icon-provider warnings.

The rebuilt production fixture passed the expanded web interaction E2E: email → OTP → onboarding, required fields and consent, exact onboarding API payload, preserved input after a mocked 500 response, and no horizontal overflow at 360 px. The suite also verifies the theme and language menus on a fresh page. `npx tsc --noEmit`, `npm run ui:check`, all 46 unit tests, and `npm run build:web:production` passed. The browser suite does not cover native keyboard avoidance or screen readers.

## Legal text in 25 languages

`scripts/translate-legal-ollama.mjs` generated the 22 missing locale entries with local Ollama `translategemma:12b`, preserving the existing BG/RU/EN text. Irish and Maltese machine outputs had material meaning errors and were corrected in the generator before use. `src/legalTranslations.test.ts` checks the exact 25-locale set, required keys, non-empty labels, the Beeloy brand and the terms year; it cannot certify legal equivalence. The generated dictionary is used directly by the onboarding Checkbox.Item and two links. The rebuilt production fixture passed E2E after switching onboarding to French. TypeScript, 46 unit tests and production web export passed. Critical legal copy still requires review by qualified language/legal reviewers; do not mark this G2 item DONE from automated translation alone. The availability and language of linked legal pages also remain unverified.

## Single Paper theme provider

The saved `system`/`light`/`dark` choice now lives in the root `BeeloyAppProvider`, which loads it before rendering the app. A single `PaperProvider` supplies the selected MD3 theme to the login form, menus, portals and the rest of the app. The duplicate nested provider and duplicate preference state were removed from `App.tsx`. The UI static check rejects a second provider in that file.

Validation on 2026-10-01: TypeScript, `npm run ui:check`, 46 unit tests and `npm run build:web:production` passed. `make ui-interaction-test` rebuilt both apps' development and production fixtures; the first browser run exposed an obsolete BeeFiscal fixture without a register-to-device binding. After updating that binding and the audit/settings steps to match the current Paper screens, `npm run test:web-interaction` passed the complete MiniPOS and BeeFiscal journeys, including the saved theme choice and production login fail-closed checks. The suite does not yet verify every theme token in the rendered DOM or prove WCAG AA on all states.

## Sale shell and header

The authenticated app root and header now read the active Paper theme's `background`, `surface`, `outlineVariant`, `onSurface` and `onSurfaceVariant` roles. The former fixed light/dark styles for these surfaces were removed. The brand and context are Paper Text, while logout, BLE connection and sales/configuration navigation are Paper Buttons. This is a partial migration of the sale shell; the shift action, product grid, cart, payment panel and management editors still contain legacy controls and colors.

Validation on 2026-10-01: TypeScript, `npm run ui:check`, all 46 unit tests, production and demo web exports, and the full local web interaction E2E passed. The E2E now verifies that root and header computed background colors both change between explicit light and dark themes, and that dark mode survives reload. It does not establish WCAG AA for the remaining controls or the six required viewport/theme combinations on every state.

## Shift actions, unknown operation and register choice

The open/close/reconcile shift control uses Paper Button with modes that reflect the action. The pending unknown operation uses Paper Card, Text and Button with semantic tertiary theme colors, preserving the restriction that a payment cannot be repeated while its outcome is unknown. The register picker uses Paper RadioButton.Item, and its selected item includes a visible, accessible translated state label. Its empty action uses Paper Button. The `selected` label was generated with local Ollama `translategemma:12b` for all 25 locales; the incorrect Maltese result was corrected in the generator and dictionary. The installed Paper web radio item did not expose `aria-checked`, so selection is also stated in the accessible name and visually; this remains a limitation to check on native screen readers.

Validation on 2026-10-01: TypeScript, `npm run ui:check`, all 47 unit tests, full local MiniPOS/BeeFiscal interaction E2E, and production/demo web exports passed. The browser test changes register selection twice, verifies the accessible selected label, then saves the original register before continuing the sales, unknown-result and Z-reconciliation journeys. Full screen/state WCAG AA and native checks remain open.

## Sale catalog and mobile segments

The xs catalog/cart switch now uses Paper SegmentedButtons. Product tiles use actionable Paper Cards and Paper Text; the product search uses the active Paper surface instead of a fixed light background. The cart segment keeps its full amount in the accessible label while its shorter visible label fits at 360 px. The status footer now uses the active theme's surface variant and a visible text label. Decorative Paper icons in the theme/language controls and selected segment were removed after Axe found `role-img-alt` failures; selected menu items retain a localized text marker. The accessible name on a nonsemantic Paper Card container and an ARIA label on the status View were also removed after Axe reported `aria-prohibited-attr` as incomplete.

Validation on 2026-10-01: TypeScript, `npm run ui:check`, 47 unit tests, local MiniPOS/BeeFiscal interaction E2E, and production/demo web exports passed. The E2E scans the populated catalog at xs 360×800, md 960×900 and lg 1440×1000 in both explicit themes, plus the empty mobile cart at xs/light and xs/dark. All eight scans have **zero automatic Axe violations**, with one `color-contrast` incomplete result in each scan. Node-level incomplete results are retained in `docs/ui-optimization-axe-catalog.json`. Screenshots for all eight combinations are in `docs/ui-screenshots/`; visual inspection of xs light/dark found the old dark search field defect and confirmed its correction. The remaining incomplete nodes need measured manual review. This is not a full WCAG AA pass: other states/screens, 200% text, keyboard, native screen readers and the remaining manual checks are open.

## Cart and payment controls

The populated cart now uses Paper Text, outlined TextInput, Paper IconButtons for quantity, and Paper Buttons for discount and cash/card/split actions. The theme supplies the cart surface, borders and foregrounds. Checkout, discount and quantity handlers were left intact. At xs the entire cart panel scrolls, so quantity, split amount and payment actions remain reachable instead of being clipped by a small inner line list. Split amount and its action stack vertically, keeping their Bulgarian labels readable. The repeated register UUID is omitted from the xs cart detail block; UNP and state remain visible, and the active register is still shown in the header. Quantity IconButtons were enlarged after the first browser measurement found only a 46×46 px hit area; E2E now requires at least 48×48 px in both themes.

Validation on 2026-10-01: TypeScript, `npm run ui:check`, 47 unit tests, complete MiniPOS/BeeFiscal interaction E2E and both web exports passed. Ten local Axe scans now cover the six catalog combinations plus empty and populated xs carts in light/dark: zero automatic violations, one unresolved `color-contrast` incomplete per scan. The node-level report and screenshots are saved alongside this file; xs screenshots include separate quantity, split-payment and cash/card scroll positions. Payment mode consolidation, confirmation flows, native keyboard and full WCAG review remain open.

## Sticky total and 320 px labels

The cart total now sits outside the scrollable lines/payment region and stays immediately above the status footer. The xs E2E compares its bounding box before and after scrolling to the cash action; it remains fixed in both themes. At 320 CSS px, the long current-sale tab label and split-cash field label were visibly clipped. A short `cart` label was generated with local Ollama `translategemma:12b` for all 25 languages; Bulgarian, Croatian, Irish, Maltese and Slovenian outputs were corrected to concise basket terms in the generator and dictionary. The visible split-cash field label is the existing `cashEur` translation, while its full accessible name still explains that this is the split payment cash amount. The 320 px E2E checks no horizontal document overflow and saves a screenshot.

Validation on 2026-10-01: TypeScript, `npm run ui:check`, 47 unit tests and the complete interaction E2E passed on the fresh local web fixture. The same ten Axe scans retained zero automatic violations and ten unresolved `color-contrast` incomplete results. The sticky-total and 320 px checks cover the populated xs cart; 200% text, every state at all breakpoints, native screen readers and full WCAG AA remain open.

## Management screens and register choices

The active management home, product, tax-group, employee, configuration, reversal and report editors now use Paper Card, Text, Button, List and TextInput components with theme-derived surfaces and borders. The old unreachable duplicate editor markup and fixed light field colors were removed. Register and tax-group choices use Paper Buttons with visible selected text and a selected state; Paper RadioButton.Item had generated nested interactive radio elements on web, which Axe flagged as critical/serious ARIA violations.

Validation on 2026-10-01: TypeScript, `npm run ui:check`, 47 unit tests, the complete MiniPOS/BeeFiscal interaction E2E, and production/demo web exports passed. The management home was scanned at xs/md/lg in light/dark: six scans with zero automatic Axe violations and one unresolved `color-contrast` incomplete each. Combined with the existing catalog/cart cases, `docs/ui-optimization-axe-catalog.json` now has 16 scans. Screenshots for the six management combinations are under `docs/ui-screenshots/`; xs light and dark were visually inspected. Nested management editors, native controls, keyboard flow, 200% text and manual contrast review still need coverage before WCAG AA or the whole plan can be marked complete.

The local interaction suite now also visits the products list, new-product form, tax-group list and new-group form, employee list and new-employee form through their actual Paper buttons. For each screen it asserts the expected control is present, scans xs/md/lg in light/dark, and saves six screenshots. On the fresh web fixture the complete MiniPOS/BeeFiscal journey passed. The report now contains 52 scans with zero automatic Axe violations and 52 unresolved `color-contrast` incomplete results. The new-product xs light, new-tax-group xs dark and employee-list xs light screenshots were inspected without finding clipping or theme mismatch; the remaining captures still need visual review. These additional automated scans do not resolve manual contrast, keyboard, native or full-state coverage gates.

## Demo badge

The demo variant's corner badge now uses Paper Surface/Text and the active theme's `errorContainer`/`onErrorContainer` roles instead of a legacy Text wrapper and fixed error colors. TypeScript, `npm run ui:check`, 47 unit tests, and production/demo web exports passed. A local browser opened the freshly built demo bundle at 360×800 and switched light/dark: the badge remained visible at the upper-right without covering the theme or language controls. Computed foreground/background were `rgb(65, 14, 11)` / `rgb(249, 222, 220)` in light and `rgb(255, 218, 214)` / `rgb(140, 29, 24)` in dark. Screenshots are `docs/ui-screenshots/demo-login-xs-light.png` and `demo-login-xs-dark.png`. This checks only the demo login viewport, not all badge placements or WCAG manual criteria.


## Onboarding legal action affordance

Two genuine Paper outlined Buttons now have link role/min56/wrapping and own web href/Enter compatibility; original locale URLs/Linking handler unchanged. Six baseline border failures retained; keyboard role-only/href-only failed and corrected using normal Paper click.11final tests (6size/theme legal Enter+pointer+single URL and5auth),types/uiPASS. Required consent test updated; trace showed missing GET/registers fixture404, added read fixture without bypassing auth. Generated Playwright reports excluded from source lint. FullApp/AA/text200/all25/native/manual/contrast remain OPEN. Evidence onboarding-legal-affordance;03-001 IN_PROGRESS.


## BeeMiniPOS shared field labels — 2026-10-03

03-009 IN_PROGRESS: genuine Paper Text wraps the full field label above genuine Paper TextInput; email/OTP use existing translated descriptive labels. Baseline xs light/dark clipped (409 px text in304 px), md/lg passed. Final17 tests PASS (6 CSS200 field,6 legal actions,5 auth); TypeScript, UI checker and isolated web export PASS. Six measurements/screenshots retained in BeeMiniPOS docs/ui-optimization-evidence/form-field-text200; xs light inspected. This is CSS text enlargement, not actual browser zoom/native DynamicType. All consumers/locales/states, strict AA/manual/native/fullapp remain OPEN.


## BeeMiniPOS admin action affordance — 2026-10-03

03-001/03-007/03-008 IN_PROGRESS: outlined back button, framed genuine Paper tax-group List.Item with button role/status in name/min48/wrapping; shared dialog secondary actions outlined, primary min48. Baseline6 missing-frame failures, final23 selected E2E PASS (6 admin xs/md/lg light/dark frame/min48/Enter editor/prefill/Enter return +17 auth/legal/field regressions). TypeScript/UI checker/web export PASS; xs light screenshot inspected. Test uses fake ADMIN operator-session rather than bypassing production permissions. Evidence docs/ui-optimization-evidence/admin-action-affordance. Shared dialog runtime, all states/locales/actions, numeric contrast/strict AA/native/manual/fullcriteria remain OPEN.


## BeeMiniPOS framed language/theme choices — 2026-10-04

03-001 IN_PROGRESS: genuine Paper Menu remains; choices now genuine framed Paper List.Item/min48/full wrapping titles. Six baseline missing-frame failures. Selected regression29 PASS, enhanced menu6 PASS:150 language selections on xs/md/lg starting light/dark with CSS200 labels, frames/min48/no clipping/Enter and actual translated email accessible name;18 theme selections. Types/UI checker/export PASS. Evidence docs/ui-optimization-evidence/menu-action-affordance; screenshots finish in dark after theme sequence, xs inspected. Strict AA/numeric contrast, arrow/Escape/Tab/focus return, persistence/native/truezoom/allpages/full11 criteria OPEN.


## BeeMiniPOS menu keyboard — 2026-10-04

03-001 IN_PROGRESS. Baseline6 ArrowDown failures; own scoped web key handler supports arrows wrap/HomeEnd/focus+nearest scroll on genuine Paper menuitems, with listener cleanup. Paper initial focus/Escape/return preserved and tested. Selected35 E2E PASS; enhanced6 both-menu keyboard cases PASS xs/md/lg light/dark. Types/UI/export PASS. Evidence docs/ui-optimization-evidence/menu-keyboard. Tab/roving/expanded-selected semantics, numeric contrast/strict AA/native/manual/full11 criteria remain OPEN.


## BeeMiniPOS selected menu semantics — 2026-10-04

03-001 IN_PROGRESS: web genuine Paper menuitem DOM role menuitemradio/checked derived from current preferences; trigger expanded/haspopup, mount observer cleanup. Baseline6 missing checked failures;35 selected E2E PASS with radio/checked/unchecked/expanded assertions in6 xs/md/lg light/dark keyboard cases,150 locale choices retained. Types/UI/export PASS. Evidence docs/ui-optimization-evidence/menu-selected-state. Tab/roving, full ARIA strict Axe/needsReview, state change/reload/native/manual/numeric contrast/full11 remain OPEN.


## BeeMiniPOS strict auth/menu Axe and parent roles — 2026-10-04

03-001 IN_PROGRESS. Added strict WCAG2/2.1 A+AA Playwright test saving violations AND incomplete. Baseline18 scans had12 aria-required-parent findings/168 nodes; structural theme View/language ScrollView menu roles fixed, Paper leaves unchanged. Current18 scans0 automatic violations but18 color-contrast incomplete/108 nodes: strict6 FAIL/INCOMPLETE, not AA PASS. Functional35 PASS/types/UI/export PASS. Full compressed baseline/current retained in docs/ui-optimization-evidence/auth-menu-strict-aa. Contrast incomplete resolution/Tab/roving/native/manual/allpages/full11 remain OPEN.


## BeeMiniPOS auth contrast diagnosis — 2026-10-04

Strict findings messageKey bgOverlap for6 auth text nodes per scan. Removing decorative image experimentally did not change18 scans0VI/108 incomplete nodes, so source experiment reverted. Browser diagnostic xs/light label hit test showed actual label topmost and all root ancestors360x800; no collapsed root at this point. Strict test now saves layout/hit-stack evidence. docs/ui-optimization-evidence/auth-contrast-diagnostic retains experiment and diagnostic reports. Root cause and contrast adjudication OPEN; no AA PASS or production contrast fix claimed.


## BeeMiniPOS all incomplete text hit stacks — 2026-10-04

Expanded strict test diagnostics to every incomplete target/text Range rectangle, recording actual hit stacks/colors/opacity/z-index. Re-exported restored current source (decorative image retained).18 scans still0VI/108 color incomplete; six strict failures preserved. Login samples actual target topmost; open-menu samples covered by menu/backdrop. Evidence docs/ui-optimization-evidence/auth-text-hit-stacks with18-state summary and raw stacks. This does not establish AA or false positives. Next: match background interaction/accessibility to open overlay and diagnose login Axe virtual stacking. Original11-app scope remains OPEN.


## BeeMiniPOS open-menu background interaction — 2026-10-04

03-001 IN_PROGRESS. Web app-root inert while genuine Paper portal menu/backdrop open, counted locks preserve existing inert and cleanup restores. Native unchanged. Baseline6 missing inert failures;35 functional PASS incl real background email focus rejection/menu focus retained/Escape restore and150 locale choices. Types/UI/export PASS.18 strict scans0VI:12 open-menu scans0incomplete,6 login scans6 color-contrast incomplete36nodes (previous108). Six strict tests FAIL/INCOMPLETE retained. Evidence docs/ui-optimization-evidence/menu-background-inert. Login/Tab/roving/native/manual/truezoom/all11 fullcriteria remain OPEN.


## BeeMiniPOS modal menu Tab/roving — 2026-10-04

03-001 IN_PROGRESS. Baseline6FAIL:25 language tab-stops. Genuine Paper menu now one current item Tab-stop with arrows updating it; Tab/ShiftTab cycle current item and actual Paper close action while background inert; close label existing25-locale back translation. Native unchanged.35 regression PASS plus enhanced6 both-menu keyboard cases PASS xs/md/lg light/dark. Types/UI/export PASS. Strict18scans0VI;12menu scans0incomplete;6login scans36 incomplete nodes, strict6FAIL retained. Evidence docs/ui-optimization-evidence/menu-tab-navigation. Login/native/manual/truezoom/screen readers/full11 criteria remain OPEN.


## BeeMiniPOS actual preference persistence — 2026-10-04

03-001 IN_PROGRESS. New preferences-persistence E2E uses actual UI/storage/reload, no storage seed/mock.6contexts xs/md/lg light/dark PASS51.5s with all25locale selections and150 reloads; translated email accessible name and opposite-to-system chosen theme bg retained;150language+150theme reopened checked/focus-on-selected assertions and background inert cleanup. TypeScript PASS; production source unchanged. Evidence docs/ui-optimization-evidence/preferences-persistence. This verifies web preferences only; native/copy/allstates/manual/truezoom/login incomplete/fullAA/all11 remain OPEN.
