import { readFile, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const output = resolve('src/legalTranslations.generated.ts');
const locales = {
  bg: 'Bulgarian', hr: 'Croatian', cs: 'Czech', da: 'Danish', nl: 'Dutch', en: 'English',
  et: 'Estonian', fi: 'Finnish', fr: 'French', de: 'German', el: 'Greek', hu: 'Hungarian',
  ga: 'Irish', it: 'Italian', lv: 'Latvian', lt: 'Lithuanian', mt: 'Maltese', pl: 'Polish',
  pt: 'Portuguese', ro: 'Romanian', sk: 'Slovak', sl: 'Slovenian', es: 'Spanish', sv: 'Swedish', ru: 'Russian',
};
const english = {
  consent: 'I am authorised to bind the company and accept the Beeloy Business Terms (28 September 2026). I accept current features without a promise of unstated features or uninterrupted operation; mandatory rights remain unaffected.',
  terms: 'Read Business Terms',
  privacy: 'Privacy Notice',
};
const manual = {
  bg: {
    consent: 'Упълномощен съм да представлявам фирмата и приемам бизнес условията на Beeloy (28.09.2026). Приемам наличните функции без обещание за неупоменати възможности или непрекъсната работа; законовите права остават в сила.',
    terms: 'Прочети бизнес условията',
    privacy: 'Политика за поверителност',
  },
  en: english,
  ga: {
    consent: 'Tá údarás agam an chuideachta a cheangal agus glacaim le Téarmaí Gnó Beeloy (28 Meán Fómhair 2026). Glacaim leis na gnéithe atá ar fáil faoi láthair gan gealltanas maidir le gnéithe nach bhfuil luaite ná oibriú gan bhriseadh; ní dhéantar difear do chearta éigeantacha dlí.',
    terms: 'Léigh Téarmaí Gnó',
    privacy: 'Fógra Príobháideachta',
  },
  mt: {
    consent: "Jien awtorizzat/a li norbot lill-kumpanija u naċċetta t-Termini Kummerċjali ta' Beeloy (28 ta' Settembru 2026). Naċċetta l-karatteristiċi attwali mingħajr wegħda ta' karatteristiċi mhux imsemmija jew ta' tħaddim bla interruzzjoni; id-drittijiet legali obbligatorji jibqgħu mhux affettwati.",
    terms: 'Ara t-Termini Kummerċjali',
    privacy: 'Avviż dwar il-Privatezza',
  },
  ru: {
    consent: 'Я уполномочен представлять компанию и принимаю бизнес-условия Beeloy (28.09.2026). Принимаю текущие функции без обещаний неуказанных возможностей или бесперебойной работы; обязательные права сохраняются.',
    terms: 'Открыть бизнес-условия',
    privacy: 'Политика конфиденциальности',
  },
};
const progress = resolve('scripts/.legal-translations-progress.json');
let translated = {};
try { translated = JSON.parse(await readFile(progress, 'utf8')); } catch {}
Object.assign(translated, manual);

async function generate(locale, language) {
  const prompt = `Translate this mobile app onboarding copy into ${language} (${locale}). Return only a JSON object with exact keys consent, terms, privacy. Preserve the legal meaning, Beeloy brand, the date 28 September 2026, and the distinction between mandatory legal rights and optional features. Use natural, concise UI language. Do not add promises, legal claims or jurisdiction-specific terminology. Source JSON: ${JSON.stringify(english)}`;
  const response = await fetch('http://127.0.0.1:11434/api/generate', {
    method: 'POST', headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ model: 'translategemma:12b', prompt, stream: false, format: 'json', options: { temperature: 0, num_predict: 650 } }),
    signal: AbortSignal.timeout(180000),
  });
  if (!response.ok) throw new Error(`Ollama ${response.status}: ${await response.text()}`);
  const data = await response.json();
  const result = JSON.parse(data.response);
  if (Object.keys(result).sort().join(',') !== 'consent,privacy,terms') throw new Error(`${locale}: wrong keys`);
  for (const [key, value] of Object.entries(result)) {
    if (typeof value !== 'string' || value.trim().length < 4) throw new Error(`${locale}: empty ${key}`);
  }
  if (!result.consent.includes('Beeloy') || !result.consent.includes('2026')) throw new Error(`${locale}: brand/date lost`);
  return result;
}

for (const [locale, language] of Object.entries(locales)) {
  if (translated[locale]) continue;
  let lastError;
  for (let attempt = 0; attempt < 3; attempt++) {
    try { translated[locale] = await generate(locale, language); break; }
    catch (error) { lastError = error; }
  }
  if (!translated[locale]) throw lastError;
  await writeFile(progress, `${JSON.stringify(translated, null, 2)}\n`);
  process.stdout.write(`${locale} translated\n`);
}

const ordered = Object.fromEntries(Object.keys(locales).map(locale => [locale, translated[locale]]));
await writeFile(output, `// Generated with local Ollama translategemma:12b; legal review is still required.\nexport const legalTranslations = ${JSON.stringify(ordered, null, 2)} as const;\n`);
process.stdout.write(`Wrote ${output} for ${Object.keys(ordered).length} locales\n`);
