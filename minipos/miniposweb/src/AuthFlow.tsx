import { persistAuthTokens } from './storage/authTokens';
import { foundationCopy } from '../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
import { AppButton as Button } from './ui/app-button';
import { FormField as TextInput } from './ui/form-field';
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { Card, HelperText, Surface, Text, useTheme } from "react-native-paper";
import { EmailAuthClient, type AuthTokens } from "./api/client";
import { translations } from "./ui/authTranslations.generated";
import { PreferenceControls, readLanguage, type Language } from "./ui/PreferenceControls";

const title = (language: Language, signIn: string) => language === "en" ? "Sign in to BeeMiniPOS" : language === "bg" || language === "ru" ? "Вход в BeeMiniPOS" : `BeeMiniPOS · ${signIn}`;

export function AuthFlow({ backend, onAuthenticated }: { backend: string; onAuthenticated: (tokens: AuthTokens) => void }) {
  const [language, setLanguage] = useState<Language>(readLanguage);
  const theme = useTheme();
  const errorId = `miniposweb-auth-error-${useId().replace(/:/g, '')}`;
  const pending = useRef<{ auth: EmailAuthClient; ticket: object } | null>(null);
  const [stage, setStage] = useState<"email" | "code" | "onboarding">("email");
  const [email, setEmail] = useState(""), [code, setCode] = useState(""), [onboardingToken, setOnboardingToken] = useState("");
  const [companyName, setCompanyName] = useState(""), [address, setAddress] = useState(""), [taxIdentifier, setTaxIdentifier] = useState(""), [fullName, setFullName] = useState("");
  const [busy, setBusy] = useState(false), [error, setError] = useState("");
  const auth = useMemo(() => new EmailAuthClient(backend), [backend]);
  const mounted = useRef(false), currentAuth = useRef(auth);currentAuth.current=auth;
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;pending.current=null;};},[]);
  useEffect(()=>{pending.current=null;setBusy(false);setError('');setStage('email');setCode('');setOnboardingToken('');setCompanyName('');setAddress('');setTaxIdentifier('');setFullName('');},[auth]);
  const snapshot = {}, latest = useRef(snapshot); latest.current = snapshot;
  const editable = () => mounted.current && currentAuth.current === auth && latest.current === snapshot && !busy && pending.current?.auth !== auth;
  const edit = (setter: (value: string) => void) => (value: string) => { if (editable()) setter(value); };
  const validEmail = /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim());
  const authenticatedRef = useRef(onAuthenticated); authenticatedRef.current = onAuthenticated;
  const current = () => mounted.current && currentAuth.current === auth;
  const t = translations[language];
  const save = (tokens: AuthTokens) => { if(!current())return;persistAuthTokens(localStorage, tokens); authenticatedRef.current(tokens); };
  async function run(action: () => Promise<void>) {
    if (!editable()) return;
    const ticket = {}; pending.current = {auth,ticket}; setBusy(true); setError('');
    try { await action(); }
    catch { if (current() && pending.current?.ticket === ticket) setError('AUTH_REQUEST_FAILED'); }
    finally { if (pending.current?.ticket === ticket) { pending.current = null; if (current()) setBusy(false); } }
  }
  const requestCode = () => run(async () => {
    if (stage !== 'email' || !validEmail) return;
    await auth.requestCode(email.trim(), language); if (current()) setStage('code');
  });
  const verifyCode = () => run(async () => {
    if (stage !== 'code' || !/^\d{6}$/.test(code)) return;
    const result = await auth.verifyCode(email.trim(), code); if (!current()) return;
    if (!result || typeof result.onboarding_required !== 'boolean') throw new Error('AUTH_RESPONSE_INVALID');
    if (result.onboarding_required) {
      if (typeof result.onboarding_token !== 'string' || !result.onboarding_token.trim()) throw new Error('AUTH_RESPONSE_INVALID');
      setOnboardingToken(result.onboarding_token); setStage('onboarding');
    } else save(result);
  });
  const validOnboarding = Boolean(onboardingToken.trim() && companyName.trim() && address.trim() && taxIdentifier.trim() && fullName.trim());
  const createCompany = () => run(async () => {
    if (stage !== 'onboarding' || !validOnboarding) return;
    save(await auth.onboarding(onboardingToken, {company_name:companyName.trim(),address:address.trim(),tax_identifier:taxIdentifier.trim(),full_name:fullName.trim()}));
  });
  return <Surface testID="minipos.auth.surface" mode="flat" elevation={0} style={{minHeight:'100%',minWidth:0,backgroundColor:theme.colors.background}}><main className="auth-shell"><div className="auth-card">
    <Surface elevation={1} style={{ backgroundColor: theme.colors.surface, position: 'relative', zIndex: 10, flexDirection: "row", flexWrap: "wrap", justifyContent: "flex-end", gap: 8, padding: 8, marginBottom: 12, borderRadius: 12 }}>
      <PreferenceControls language={language} onLanguageChange={setLanguage} testIdPrefix="auth" />
    </Surface>
    <Card mode="outlined" style={{ backgroundColor: theme.colors.surface }}><Card.Content><div className="auth-form-stack">
      <Text variant="headlineSmall" accessibilityRole="header">{title(language, t.signIn)}</Text>
      {stage === "email" && <>
        <TextInput editable={!busy} aria-describedby={error ? errorId : undefined} mode="outlined" label={t.email} accessibilityLabel={t.email} keyboardType="email-address" autoCapitalize="none" autoComplete="email" value={email} onChangeText={edit(setEmail)} onSubmitEditing={() => void requestCode()} testID="auth-email" />
        <Button mode="contained" contentStyle={{ minHeight: 48 }} loading={busy} disabled={busy || !validEmail} onPress={requestCode}>{t.sendCode}</Button>
      </>}
      {stage === "code" && <>
        <Text variant="bodyMedium">{email}</Text>
        <TextInput editable={!busy} aria-describedby={error ? errorId : undefined} mode="outlined" label={t.oneTimeCode} accessibilityLabel={t.oneTimeCode} textContentType="oneTimeCode" autoComplete="one-time-code" keyboardType="number-pad" maxLength={6} value={code} onChangeText={(value: string) => edit(setCode)(value.replace(/\D/g, ""))} onSubmitEditing={() => void verifyCode()} testID="auth-code" />
        <Button mode="contained" contentStyle={{ minHeight: 48 }} loading={busy} disabled={busy || code.length !== 6} onPress={verifyCode}>{t.continue}</Button>
      </>}
      {stage === "onboarding" && <>
        <Text variant="titleLarge" accessibilityRole="header">{t.companyDetails}</Text>
        <TextInput editable={!busy} aria-describedby={error ? errorId : undefined} mode="outlined" label={t.companyName} accessibilityLabel={t.companyName} value={companyName} onChangeText={edit(setCompanyName)} testID="auth-company-name" />
        <TextInput editable={!busy} aria-describedby={error ? errorId : undefined} mode="outlined" label={t.address} accessibilityLabel={t.address} value={address} onChangeText={edit(setAddress)} testID="auth-address" />
        <TextInput editable={!busy} aria-describedby={error ? errorId : undefined} mode="outlined" label={t.taxIdentifier} accessibilityLabel={t.taxIdentifier} value={taxIdentifier} onChangeText={edit(setTaxIdentifier)} testID="auth-tax-id" />
        <TextInput editable={!busy} aria-describedby={error ? errorId : undefined} mode="outlined" label={t.fullName} accessibilityLabel={t.fullName} autoComplete="name" value={fullName} onChangeText={edit(setFullName)} onSubmitEditing={() => void createCompany()} testID="auth-full-name" />
        <Button mode="contained" contentStyle={{ minHeight: 48 }} loading={busy} disabled={busy || !validOnboarding} onPress={createCompany}>{t.createCompany}</Button>
      </>}
      {error ? <HelperText nativeID={errorId} accessibilityRole="alert" accessibilityLiveRegion="polite" type="error" visible testID="auth-error">{foundationCopy[language].error}</HelperText> : null}
    </div></Card.Content></Card>
  </div></main></Surface>;
}
