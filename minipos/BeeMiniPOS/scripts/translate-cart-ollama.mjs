import { readFile, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const path = resolve('src/translations.generated.ts');
const source = await readFile(path, 'utf8');
const match = source.match(/^export const translations = (\{[\s\S]*?\}) as const;/m);
if (!match) throw new Error('translations.generated.ts format changed');
const translations = JSON.parse(match[1]);
const locales = Object.keys(translations);
const prompt = `Translate the short English point-of-sale interface label "Cart" (the current basket of products being sold, not a vehicle) into each locale in this exact list: ${locales.join(', ')}. Return only a JSON object with those locale codes as keys and concise natural UI strings as values. No explanations. The English value is "Cart".`;
const response = await fetch('http://127.0.0.1:11434/api/generate', {
  method: 'POST', headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ model: 'translategemma:12b', prompt, stream: false, format: 'json', options: { temperature: 0, num_predict: 1000 } }),
  signal: AbortSignal.timeout(180000),
});
if (!response.ok) throw new Error(`Ollama ${response.status}: ${await response.text()}`);
const generated = JSON.parse((await response.json()).response);
if (Object.keys(generated).sort().join(',') !== [...locales].sort().join(',')) throw new Error('Ollama returned an incomplete locale set');
for (const locale of locales) {
  if (typeof generated[locale] !== 'string' || !generated[locale].trim()) throw new Error(`Empty ${locale} translation`);
  translations[locale].cart = generated[locale].trim();
}
translations.en.cart = 'Cart';
// Fix model outputs that named a vehicle or used another language's word.
Object.assign(translations.bg, { cart: 'Кошница' });
Object.assign(translations.hr, { cart: 'Košarica' });
Object.assign(translations.ga, { cart: 'Ciseán' });
Object.assign(translations.mt, { cart: 'Qoffa' });
Object.assign(translations.sl, { cart: 'Košarica' });
await writeFile(path, source.replace(match[1], JSON.stringify(translations, null, 2)));
process.stdout.write(`Added cart label in ${locales.length} locales\n`);
