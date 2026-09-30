import { readFile, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const english = {
  title: 'BeeFiscal • Administrator sign-in',
  description: 'Sign in with a code sent to your email. Only registered Fiscal users can access this app.',
  email: 'Email',
  requestCode: 'Send code',
  code: 'Code from email',
  verify: 'Verify code',
  chooseCompany: 'Choose a company',
  chooseCompanyPrefix: 'Choose company',
  privacyPolicy: 'Privacy policy',
  language: 'Language',
  theme: 'Theme',
  themeSystem: 'System',
  themeLight: 'Light',
  themeDark: 'Dark',
};

const bulgarian = {
  title: 'BeeFiscal • административен вход',
  description: 'Влезте с код по email. Достъп имат само регистрирани потребители на Fiscal.',
  email: 'Имейл', requestCode: 'Изпрати код', code: 'Код от имейл', verify: 'Потвърди кода',
  chooseCompany: 'Изберете компания', chooseCompanyPrefix: 'Изберете компания',
  privacyPolicy: 'Политика за поверителност', language: 'Език', theme: 'Тема',
  themeSystem: 'Системна', themeLight: 'Светла', themeDark: 'Тъмна',
};

const russian = {
  title: 'BeeFiscal • вход администратора',
  description: 'Войдите с помощью кода из письма. Доступ есть только у зарегистрированных пользователей Fiscal.',
  email: 'Электронная почта', requestCode: 'Отправить код', code: 'Код из письма', verify: 'Подтвердить код',
  chooseCompany: 'Выберите компанию', chooseCompanyPrefix: 'Выбрать компанию',
  privacyPolicy: 'Политика конфиденциальности', language: 'Язык', theme: 'Тема',
  themeSystem: 'Системная', themeLight: 'Светлая', themeDark: 'Тёмная',
};

const languages = {
  hr: 'Croatian', cs: 'Czech', da: 'Danish', nl: 'Dutch', et: 'Estonian', fi: 'Finnish',
  fr: 'French', de: 'German', el: 'Greek', hu: 'Hungarian', ga: 'Irish', it: 'Italian',
  lv: 'Latvian', lt: 'Lithuanian', mt: 'Maltese', pl: 'Polish', pt: 'Portuguese',
  ro: 'Romanian', sk: 'Slovak', sl: 'Slovenian', es: 'Spanish', sv: 'Swedish',
};

// Review corrections for conspicuous role, action, product-name and grammar errors.
const corrections = {
  hr: { title: 'BeeFiscal • Administratorska prijava' },
  da: { title: 'BeeFiscal • Administratorlogin', description: 'Log ind med en kode sendt til din e-mail. Kun registrerede Fiscal-brugere kan bruge denne app.' },
  nl: { title: 'BeeFiscal • Beheerdersaanmelding' },
  et: { title: 'BeeFiscal • Administraatori sisselogimine' },
  fi: { title: 'BeeFiscal • Ylläpitäjän kirjautuminen' },
  de: { title: 'BeeFiscal • Administrator-Anmeldung' },
  el: { title: 'BeeFiscal • Σύνδεση διαχειριστή', themeDark: 'Σκούρο' },
  ga: {
    title: 'BeeFiscal • Logáil isteach mar riarthóir',
    description: 'Logáil isteach le cód a seoladh chuig do ríomhphost. Ní féidir ach le húsáideoirí cláraithe Fiscal an aip seo a úsáid.',
    email: 'Ríomhphost', requestCode: 'Seol cód', code: 'Cód ón ríomhphost', verify: 'Fíoraigh an cód',
    privacyPolicy: 'Polasaí príobháideachais', themeDark: 'Dorcha',
  },
  lv: { title: 'BeeFiscal • Administratora pieteikšanās', description: 'Piesakieties ar savu e-pastu nosūtīto kodu. Šai lietotnei var piekļūt tikai reģistrēti Fiscal lietotāji.' },
  lt: { title: 'BeeFiscal • Administratoriaus prisijungimas' },
  mt: {
    title: 'BeeFiscal • Dħul tal-amministratur',
    description: 'Idħol b’kodiċi mibgħut fl-email tiegħek. L-app hija disponibbli biss għal utenti Fiscal irreġistrati.',
    requestCode: 'Ibgħat kodiċi', code: 'Kodiċi mill-email', verify: 'Ivverifika l-kodiċi',
    chooseCompany: 'Agħżel kumpanija', chooseCompanyPrefix: 'Agħżel kumpanija', privacyPolicy: 'Politika tal-privatezza',
    themeLight: 'Ċar', themeDark: 'Skur',
  },
  pl: { title: 'BeeFiscal • Logowanie administratora' },
  pt: { description: 'Entre com o código enviado para seu e-mail. Apenas usuários registrados no Fiscal podem acessar este aplicativo.' },
  sl: { description: 'Prijavite se s kodo, poslano na vaš e-poštni naslov. Do aplikacije lahko dostopajo samo registrirani uporabniki Fiscal.', privacyPolicy: 'Pravilnik o zasebnosti' },
  es: { title: 'BeeFiscal • Inicio de sesión de administrador' },
  sv: { title: 'BeeFiscal • Administratörsinloggning' },
};

const keys = Object.keys(english);
let translations = { bg: bulgarian, en: english, ru: russian };
if (process.argv.includes('--corrections-only')) {
  const current = await readFile(resolve('src/ui/auth-copy.generated.ts'), 'utf8');
  translations = JSON.parse(current.replace(/^\/\/[^\n]*\nexport const authCopy = /, '').replace(/ as const;\s*$/, ''));
} else for (const [locale, language] of Object.entries(languages)) {
  const prompt = `Translate the JSON string values from English into ${language} for a fiscal administration mobile app. Keep the JSON keys unchanged. Keep BeeFiscal and Fiscal as product names. Use concise, natural UI language. Output a JSON object only, with every key present and no extra keys. Input JSON: ${JSON.stringify(english)}`;
  const response = await fetch('http://127.0.0.1:11434/api/generate', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ model: 'qwen3.5:9b-mlx', prompt, stream: false, think: false, format: 'json', options: { temperature: 0 } }),
    signal: AbortSignal.timeout(120_000),
  });
  if (!response.ok) throw new Error(`${locale}: Ollama HTTP ${response.status}`);
  const result = await response.json();
  const translated = JSON.parse(result.response);
  if (JSON.stringify(Object.keys(translated).sort()) !== JSON.stringify([...keys].sort())) throw new Error(`${locale}: key mismatch`);
  if (Object.values(translated).some(value => typeof value !== 'string' || !value.trim() || value.length > 250 || /[<>]/.test(value))) throw new Error(`${locale}: invalid value`);
  translations[locale] = translated;
  process.stdout.write(`${locale} `);
}

for (const [locale, values] of Object.entries(corrections)) Object.assign(translations[locale], values);

const ordered = Object.fromEntries(['bg', 'hr', 'cs', 'da', 'nl', 'en', 'et', 'fi', 'fr', 'de', 'el', 'hu', 'ga', 'it', 'lv', 'lt', 'mt', 'pl', 'pt', 'ro', 'sk', 'sl', 'es', 'sv', 'ru'].map(locale => [locale, translations[locale]]));
await writeFile(resolve('src/ui/auth-copy.generated.ts'), `// Generated with local Ollama qwen3.5:9b-mlx; editorial review is still required.\nexport const authCopy = ${JSON.stringify(ordered, null, 2)} as const;\n`);
process.stdout.write('\nWrote 25 locales.\n');
