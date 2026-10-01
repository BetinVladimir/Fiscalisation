import { useMemo, useState } from "react";
import { Button, Card, HelperText, Surface, Text, TextInput, useTheme } from "react-native-paper";
import { EmailAuthClient, type AuthTokens } from "./api/client";
import { translations } from "./ui/authTranslations.generated";
import { PreferenceControls, readLanguage, type Language } from "./ui/PreferenceControls";

const title = (language: Language, signIn: string) => language === "en" ? "Sign in to BeeMiniPOS" : language === "bg" || language === "ru" ? "Вход в BeeMiniPOS" : `BeeMiniPOS · ${signIn}`;

export function AuthFlow({ backend, onAuthenticated }: { backend: string; onAuthenticated: (tokens: AuthTokens) => void }) {
  const [language, setLanguage] = useState<Language>(readLanguage);
  const theme = useTheme();
  const [stage, setStage] = useState<"email" | "code" | "onboarding">("email");
  const [email, setEmail] = useState(""), [code, setCode] = useState(""), [onboardingToken, setOnboardingToken] = useState("");
  const [companyName, setCompanyName] = useState(""), [address, setAddress] = useState(""), [taxIdentifier, setTaxIdentifier] = useState(""), [fullName, setFullName] = useState("");
  const [busy, setBusy] = useState(false), [error, setError] = useState("");
  const auth = useMemo(() => new EmailAuthClient(backend), [backend]);
  const t = translations[language];
  const save = (tokens: AuthTokens) => { localStorage.setItem("minipos-access-token", tokens.access_token); localStorage.setItem("minipos-refresh-token", tokens.refresh_token); onAuthenticated(tokens); };
  async function run(action: () => Promise<void>) { setBusy(true); setError(""); try { await action(); } catch (e) { setError(e instanceof Error ? e.message : String(e)); } finally { setBusy(false); } }
  return <main className="auth-shell"><div className="auth-card">
    <Surface elevation={1} style={{ backgroundColor: theme.colors.surface, flexDirection: "row", flexWrap: "wrap", justifyContent: "flex-end", gap: 8, padding: 8, marginBottom: 12, borderRadius: 12 }}>
      <PreferenceControls language={language} onLanguageChange={setLanguage} testIdPrefix="auth" />
    </Surface>
    <Card mode="outlined" style={{ backgroundColor: theme.colors.surface }}><Card.Content><div className="auth-form-stack">
      <Text variant="headlineSmall" accessibilityRole="header">{title(language, t.signIn)}</Text>
      {stage === "email" && <>
        <TextInput mode="outlined" label={t.email} accessibilityLabel={t.email} keyboardType="email-address" autoCapitalize="none" autoComplete="email" value={email} onChangeText={setEmail} testID="auth-email" />
        <Button mode="contained" contentStyle={{ minHeight: 48 }} loading={busy} disabled={busy || !email} onPress={() => run(async () => { await auth.requestCode(email, language); setStage("code"); })}>{t.sendCode}</Button>
      </>}
      {stage === "code" && <>
        <Text variant="bodyMedium">{email}</Text>
        <TextInput mode="outlined" label={t.oneTimeCode} accessibilityLabel={t.oneTimeCode} keyboardType="number-pad" maxLength={6} value={code} onChangeText={(value: string) => setCode(value.replace(/\D/g, ""))} testID="auth-code" />
        <Button mode="contained" contentStyle={{ minHeight: 48 }} loading={busy} disabled={busy || code.length !== 6} onPress={() => run(async () => { const result = await auth.verifyCode(email, code); if (result.onboarding_required) { setOnboardingToken(result.onboarding_token || ""); setStage("onboarding"); } else save(result); })}>{t.continue}</Button>
      </>}
      {stage === "onboarding" && <>
        <Text variant="titleLarge" accessibilityRole="header">{t.companyDetails}</Text>
        <TextInput mode="outlined" label={t.companyName} accessibilityLabel={t.companyName} value={companyName} onChangeText={setCompanyName} testID="auth-company-name" />
        <TextInput mode="outlined" label={t.address} accessibilityLabel={t.address} value={address} onChangeText={setAddress} testID="auth-address" />
        <TextInput mode="outlined" label={t.taxIdentifier} accessibilityLabel={t.taxIdentifier} value={taxIdentifier} onChangeText={setTaxIdentifier} testID="auth-tax-id" />
        <TextInput mode="outlined" label={t.fullName} accessibilityLabel={t.fullName} autoComplete="name" value={fullName} onChangeText={setFullName} testID="auth-full-name" />
        <Button mode="contained" contentStyle={{ minHeight: 48 }} loading={busy} disabled={busy || !companyName || !address || !taxIdentifier || !fullName} onPress={() => run(async () => save(await auth.onboarding(onboardingToken, { company_name: companyName, address, tax_identifier: taxIdentifier, full_name: fullName })))}>{t.createCompany}</Button>
      </>}
      {error ? <HelperText type="error" visible testID="auth-error">{error}</HelperText> : null}
    </div></Card.Content></Card>
  </div></main>;
}
