import { useMemo, useState } from "react";
import { Button, Card, HelperText, Menu, Surface, Text, TextInput, useTheme } from "react-native-paper";
import { ScrollView, View } from "react-native";
import { EmailAuthClient, type AuthTokens } from "./api/client";
import { translations, supportedLanguages } from "./ui/authTranslations.generated";
import { useThemePreference } from "./ui/theme";

type Language = typeof supportedLanguages[number];
const labels: Record<Language, string> = {
  bg: "Български", hr: "Hrvatski", cs: "Čeština", da: "Dansk", nl: "Nederlands",
  en: "English", et: "Eesti", fi: "Suomi", fr: "Français", de: "Deutsch",
  el: "Ελληνικά", hu: "Magyar", ga: "Gaeilge", it: "Italiano", lv: "Latviešu",
  lt: "Lietuvių", mt: "Malti", pl: "Polski", pt: "Português", ro: "Română",
  sk: "Slovenčina", sl: "Slovenščina", es: "Español", sv: "Svenska", ru: "Русский",
};
const title = (language: Language, signIn: string) => language === "en" ? "Sign in to BeeMiniPOS" : language === "bg" || language === "ru" ? "Вход в BeeMiniPOS" : `BeeMiniPOS · ${signIn}`;
const readLanguage = (): Language => {
  try { return supportedLanguages.find(value => value === localStorage.getItem("minipos-language")) || "bg"; }
  catch { return "bg"; }
};

export function AuthFlow({ backend, onAuthenticated }: { backend: string; onAuthenticated: (tokens: AuthTokens) => void }) {
  const [language, setLanguage] = useState<Language>(readLanguage);
  const [languageOpen, setLanguageOpen] = useState(false);
  const [themeOpen, setThemeOpen] = useState(false);
  const { preference, setPreference } = useThemePreference();
  const theme = useTheme();
  const [stage, setStage] = useState<"email" | "code" | "onboarding">("email");
  const [email, setEmail] = useState(""), [code, setCode] = useState(""), [onboardingToken, setOnboardingToken] = useState("");
  const [companyName, setCompanyName] = useState(""), [address, setAddress] = useState(""), [taxIdentifier, setTaxIdentifier] = useState(""), [fullName, setFullName] = useState("");
  const [busy, setBusy] = useState(false), [error, setError] = useState("");
  const auth = useMemo(() => new EmailAuthClient(backend), [backend]);
  const t = translations[language];
  const save = (tokens: AuthTokens) => { localStorage.setItem("minipos-access-token", tokens.access_token); localStorage.setItem("minipos-refresh-token", tokens.refresh_token); onAuthenticated(tokens); };
  async function run(action: () => Promise<void>) { setBusy(true); setError(""); try { await action(); } catch (e) { setError(e instanceof Error ? e.message : String(e)); } finally { setBusy(false); } }
  const chooseLanguage = (value: Language) => { setLanguage(value); setLanguageOpen(false); try { localStorage.setItem("minipos-language", value); } catch { /* private browsing */ } };
  return <main className="auth-shell"><div className="auth-card">
    <Surface elevation={1} style={{ backgroundColor: theme.colors.surface, flexDirection: "row", flexWrap: "wrap", justifyContent: "flex-end", gap: 8, padding: 8, marginBottom: 12, borderRadius: 12 }}>
      <Menu visible={themeOpen} onDismiss={() => setThemeOpen(false)} theme={{ animation: { scale: 0 } }} contentStyle={{ maxHeight: 224 }} anchor={<Button mode="outlined" onPress={() => setThemeOpen(true)} testID="auth-theme-open">{t.theme}: {preference === "system" ? t.systemTheme : preference === "dark" ? t.darkTheme : t.lightTheme}</Button>}>
        <View accessibilityRole="menu">
        {(["system", "light", "dark"] as const).map(value => <Menu.Item key={value} testID={`auth-theme-${value}`} title={`${value === "system" ? t.systemTheme : value === "dark" ? t.darkTheme : t.lightTheme}${preference === value ? " ✓" : ""}`} onPress={() => { setPreference(value); setThemeOpen(false); }}/>) }
        </View>
      </Menu>
      <Menu visible={languageOpen} onDismiss={() => setLanguageOpen(false)} theme={{ animation: { scale: 0 } }} contentStyle={{ maxHeight: 224, width: 240 }} anchor={<Button mode="outlined" onPress={() => setLanguageOpen(true)} testID="auth-language-open">{t.language}: {labels[language]}</Button>}>
        <ScrollView style={{ maxHeight: 224 }} nestedScrollEnabled>
        <View accessibilityRole="menu">
        {supportedLanguages.map(value => <Menu.Item key={value} testID={`auth-language-${value}`} title={`${labels[value]}${language === value ? " ✓" : ""}`} onPress={() => chooseLanguage(value)} />)}
        </View>
        </ScrollView>
      </Menu>
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
