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
