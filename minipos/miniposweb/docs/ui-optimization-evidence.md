# MiniPOS Web UI-optimization evidence

Status: **IN PROGRESS** (2026-10-01). Scope is the complete MiniPOS Web plan in `BeeloyBackend/docs/UI-optimization/04-miniposweb.md`; the evidence below covers the auth foundation and auth screens only.

## Paper foundation and auth flow

- Vite resolves React Native imports through React Native Web, including `.web` platform files in both the production build and dev prebundle. The shared `@beeloy/design-tokens` light/dark palettes drive the sole PaperProvider and the existing CSS variables. Theme preference supports system/light/dark, responds to OS changes in system mode, and persists explicit choices in localStorage.
- The email, OTP and onboarding states now use real Paper Card, Surface, Text, TextInput, Button, HelperText and Menu.Item leaves. The language and theme controls remain visible on all three steps. The single language menu contains 25 EU/RU choices and scrolls inside a 224 px popover; the background page does not scroll when the last language is reached. Both controls retain the user's choice after reload.
- The 16 auth labels per locale are generated from the existing BeeMiniPOS dictionary by `npm run translations:sync-auth` and checked into `src/ui/authTranslations.generated.ts`. This keeps MiniPOS Web's build independent of the sibling app's source tree. The structural test verifies 25 unique languages and non-empty labels; it does not certify linguistic quality. A server error with an empty HTTP body now displays `HTTP <status>` instead of silently showing no error.

## Validation

- `npm test`: **2/2 passed**. `npm run build` and `npm run deployment`: **passed**. The production JS asset is 596 kB uncompressed; Vite warns that this exceeds its 500 kB chunk threshold.
- `npx playwright test e2e/auth.spec.ts e2e/auth-wcag.spec.ts --workers=1 --retries=0`: **8/8 passed**, covering email → OTP, onboarding, errors, theme/language persistence and the matrix below.
- `docs/ui-optimization-auth-axe.json` and `docs/ui-screenshots/` contain **30 scans/screenshots**: email, OTP, onboarding, expanded language menu and expanded theme menu at xs 360×800, md 960×900 and lg 1440×1000 in light/dark. There were **zero automatic Axe WCAG A/AA violations** and **30 `color-contrast` incomplete results**. The test also checks page reflow, 25 distinct language choices, internal menu scroll and full visibility of both header controls after OTP. The xs dark auth and onboarding, xs light onboarding and expanded language menu screenshots were visually inspected; the remaining screenshots and incomplete nodes need manual review.
- The complete `npm run test:e2e` run: **34 passed, 8 failed of 42**. Failures remain in checkout, employee management, offline and route failover. One checkout test uses an ambiguous decimal-input locator; other failures need fixture/UI diagnosis. The earlier pre-auth-migration run had **31 passed, 9 failed of 40**; this is not evidence that every remaining failure is unrelated to the migration. The whole-app E2E gate is therefore **FAIL**.

## Open gates

Management, sale and offline screens still contain native DOM controls and mixed copy; Paper migration, 25-locale coverage and the full screen/state manifest remain open. The 30 Axe `incomplete` contrast nodes need measured manual review. Keyboard, 200% text, native assistive technology, published runtime and Cloudflare deployment are unverified. `Dockerfile` still installs a file dependency on the sibling `BeeloyBackend/mobile/design-tokens` without copying that directory into its build context; Docker build is unverified. Do not mark MiniPOS Web or the 11-app goal DONE from this auth result.
