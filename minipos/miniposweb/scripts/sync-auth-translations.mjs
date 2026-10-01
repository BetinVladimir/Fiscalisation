import { readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

const sourcePath = join(import.meta.dirname, '..', '..', 'BeeMiniPOS', 'src', 'translations.generated.ts');
const targetPath = join(import.meta.dirname, '..', 'src', 'ui', 'authTranslations.generated.ts');
const keys = [
  'theme', 'systemTheme', 'lightTheme', 'darkTheme', 'language', 'signIn', 'email',
  'sendCode', 'oneTimeCode', 'continue', 'companyDetails', 'companyName',
  'address', 'taxIdentifier', 'fullName', 'createCompany',
];
const source = await readFile(sourcePath, 'utf8');
const objectStart = source.indexOf('{', source.indexOf('export const translations'));
const objectEnd = source.indexOf(' as const;', objectStart);
const translations = JSON.parse(source.slice(objectStart, objectEnd));
const languageMatch = source.match(/export const supportedLanguages = (\[[^\]]+\]) as const;/);
if (!languageMatch) throw new Error('BeeMiniPOS supportedLanguages not found');
const languages = JSON.parse(languageMatch[1]);
if (languages.length !== 25 || new Set(languages).size !== 25) throw new Error('Expected 25 distinct languages');
const authTranslations = Object.fromEntries(languages.map(language => {
  const copy = Object.fromEntries(keys.map(key => {
    const value = translations[language]?.[key];
    if (typeof value !== 'string' || !value.trim()) throw new Error(`Missing ${language}.${key}`);
    return [key, value];
  }));
  return [language, copy];
}));
const output = [
  '// Generated from BeeMiniPOS/src/translations.generated.ts by scripts/sync-auth-translations.mjs.',
  '// Checked in so the MiniPOS Web build is independent of its sibling app.',
  `export const supportedLanguages = ${JSON.stringify(languages)} as const;`,
  `export const translations = ${JSON.stringify(authTranslations, null, 2)} as const;`,
  '',
].join('\n');
await writeFile(targetPath, output);
console.log(`Wrote ${targetPath}: ${languages.length} languages × ${keys.length} auth keys`);
