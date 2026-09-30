# BeeMiniPOS UI optimization evidence

Status: **IN PROGRESS**. The app-wide manifest, WCAG matrix, native screen-reader checks, and Paper migration remain open.

## Operator login, email and OTP

The unauthenticated screen now places the title, theme/language menus, fields, actions and error message in a Paper Card. Email and OTP actions use Paper Button, the fields resolve to Paper TextInput through `FormField`, and server errors use Paper HelperText. The operator onboarding body still contains legacy Pressable controls and is **not complete**. The form width uses the shared xs/md/lg thresholds with a 320–480 px target capped by available space.

Validation on 2026-09-30: `npx tsc --noEmit`, `npm run ui:check`, and `npm run build:web:production` exited 0. The production E2E fixture was rebuilt with the environment from `Fiscalisation/Makefile` before `npm run test:web-interaction`; it exited 0 and checked xs/md/lg form widths, the working theme and language menus, and fail-closed login with no personal email. This E2E also exercised the existing sales and BeeFiscal journeys. The generated production export is tracked in `dist-cloudflare/production`.

No WCAG AA claim follows from these checks: the complete state/viewport/theme scan and visual contrast review have not run. Native iOS/Android, 200% text, keyboard, screen-reader and onboarding states remain to be tested.
