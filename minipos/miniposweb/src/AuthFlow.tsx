import { useMemo, useState } from "react";
import { EmailAuthClient, type AuthTokens } from "./api/client";

type Language = "bg" | "ru" | "en";
const copy = {
  bg: { title: "Вход в BeeMiniPOS", email: "Имейл", send: "Изпрати код", code: "Еднократен код", verify: "Продължи", company: "Данни за компанията", name: "Наименование", address: "Адрес", tax: "Данъчен идентификатор", full: "Вашите имена", finish: "Създай компания" },
  ru: { title: "Вход в BeeMiniPOS", email: "Email", send: "Отправить код", code: "Одноразовый код", verify: "Продолжить", company: "Данные компании", name: "Название", address: "Адрес", tax: "Налоговый идентификатор", full: "Ваше ФИО", finish: "Создать компанию" },
  en: { title: "Sign in to BeeMiniPOS", email: "Email", send: "Send code", code: "One-time code", verify: "Continue", company: "Company details", name: "Company name", address: "Address", tax: "Tax identifier", full: "Your full name", finish: "Create company" },
};

export function AuthFlow({ backend, onAuthenticated }: { backend: string; onAuthenticated: (tokens: AuthTokens) => void }) {
  const [language, setLanguage] = useState<Language>((localStorage.getItem("minipos-language") as Language) || "bg");
  const [stage, setStage] = useState<"email" | "code" | "onboarding">("email");
  const [email, setEmail] = useState(""), [code, setCode] = useState(""), [onboardingToken, setOnboardingToken] = useState("");
  const [companyName, setCompanyName] = useState(""), [address, setAddress] = useState(""), [taxIdentifier, setTaxIdentifier] = useState(""), [fullName, setFullName] = useState("");
  const [busy, setBusy] = useState(false), [error, setError] = useState("");
  const auth = useMemo(() => new EmailAuthClient(backend), [backend]);
  const t = copy[language];
  const save = (tokens: AuthTokens) => { localStorage.setItem("minipos-access-token", tokens.access_token); localStorage.setItem("minipos-refresh-token", tokens.refresh_token); onAuthenticated(tokens); };
  async function run(action: () => Promise<void>) { setBusy(true); setError(""); try { await action(); } catch (e) { setError(e instanceof Error ? e.message : String(e)); } finally { setBusy(false); } }
  return <main className="auth-shell"><section className="login auth-card">
    <div className="auth-heading"><h1>{t.title}</h1><select aria-label="Language" value={language} onChange={e => { const value=e.target.value as Language; setLanguage(value); localStorage.setItem("minipos-language", value); }}><option value="bg">Български</option><option value="ru">Русский</option><option value="en">English</option></select></div>
    {stage === "email" && <><label>{t.email}<input type="email" autoComplete="email" value={email} onChange={e=>setEmail(e.target.value)} /></label><button disabled={busy || !email} onClick={()=>run(async()=>{ await auth.requestCode(email, language); setStage("code"); })}>{t.send}</button></>}
    {stage === "code" && <><p>{email}</p><label>{t.code}<input inputMode="numeric" autoComplete="one-time-code" maxLength={6} value={code} onChange={e=>setCode(e.target.value.replace(/\D/g,""))} /></label><button disabled={busy || code.length!==6} onClick={()=>run(async()=>{ const result=await auth.verifyCode(email,code); if(result.onboarding_required){ setOnboardingToken(result.onboarding_token || ""); setStage("onboarding"); } else save(result); })}>{t.verify}</button></>}
    {stage === "onboarding" && <><h2>{t.company}</h2><label>{t.name}<input value={companyName} onChange={e=>setCompanyName(e.target.value)} /></label><label>{t.address}<input value={address} onChange={e=>setAddress(e.target.value)} /></label><label>{t.tax}<input value={taxIdentifier} onChange={e=>setTaxIdentifier(e.target.value)} /></label><label>{t.full}<input autoComplete="name" value={fullName} onChange={e=>setFullName(e.target.value)} /></label><button disabled={busy || !companyName || !address || !taxIdentifier || !fullName} onClick={()=>run(async()=>save(await auth.onboarding(onboardingToken,{company_name:companyName,address,tax_identifier:taxIdentifier,full_name:fullName})))}>{t.finish}</button></>}
    {error && <p className="error">{error}</p>}
  </section></main>;
}
