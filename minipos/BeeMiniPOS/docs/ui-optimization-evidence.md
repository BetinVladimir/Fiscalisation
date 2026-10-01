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
