# DEMO-OTP-UI-20261027

Human authorized demo email OTP support everywhere, frontend-only part. Production accepts six numeric digits; literal `demo` only with strict Expo extra isDemo=true and appVariant=demo. Device pairing/PIN policy, fiscal money, backend/config/deploy are excluded.

files: BeeFiscalApp/src/emailOtpAuth.ts; BeeFiscalApp/src/screens/AuthScreen.tsx; BeeFiscalApp/src/emailOtpCode.ts; BeeFiscalApp/src/ui/__tests__/auth-screen-current.test.tsx; minipos/BeeMiniPOS/App.tsx; minipos/BeeMiniPOS/src/emailAuth.ts; minipos/BeeMiniPOS/src/emailOtpCode.ts; minipos/BeeMiniPOS/src/ui/email-otp-fields.tsx; minipos/BeeMiniPOS/__tests__/email-auth-safe-error-current.test.tsx; minipos/BeeMiniPOS/__tests__/email-otp-fields-current.test.tsx.

test: local existing Jest UI config, selected actual AuthScreen and email-auth hook/field suites. Red test before source changes; then same tests green. No execution/status.json edits. New ID is not registered; run-accept NOT_REGISTERED must remain explicit, direct commands recorded.

Existing old BeeFiscal UI/hook strip/require numeric six-digit OTP. Existing MiniPOS UI gates length6 and hook forwards arbitrary codes. New tests cover demo/prod/invalid flag and unchanged numeric support, genuine Paper field interactions; no native/runtime acceptance inferred.
