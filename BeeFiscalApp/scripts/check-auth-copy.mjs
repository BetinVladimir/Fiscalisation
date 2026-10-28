import { readFile } from 'node:fs/promises';

const source = await readFile(new URL('../src/ui/auth-copy.generated.ts', import.meta.url), 'utf8');
const copy = JSON.parse(source.replace(/^\/\/[^\n]*\nexport const authCopy = /, '').replace(/ as const;\s*$/, ''));
const locales = ['bg', 'hr', 'cs', 'da', 'nl', 'en', 'et', 'fi', 'fr', 'de', 'el', 'hu', 'ga', 'it', 'lv', 'lt', 'mt', 'pl', 'pt', 'ro', 'sk', 'sl', 'es', 'sv', 'ru'];
const keys = ['title', 'description', 'email', 'requestCode', 'code', 'verify', 'chooseCompany', 'chooseCompanyPrefix', 'privacyPolicy', 'language', 'theme', 'themeSystem', 'themeLight', 'themeDark'];
if (JSON.stringify(Object.keys(copy)) !== JSON.stringify(locales)) throw new Error('Auth copy locale set/order mismatch');
for (const locale of locales) {
  if (JSON.stringify(Object.keys(copy[locale])) !== JSON.stringify(keys)) throw new Error(`${locale}: auth copy key set/order mismatch`);
  for (const [key, value] of Object.entries(copy[locale])) {
    if (typeof value !== 'string' || !value.trim() || /[<>\u0000-\u001f]/.test(value)) throw new Error(`${locale}.${key}: invalid string`);
  }
  if (!copy[locale].title.startsWith('BeeFiscal • ')) throw new Error(`${locale}: product title changed`);
}
console.log(`Auth copy: ${locales.length} locales × ${keys.length} keys structurally complete; editorial review still required.`);
