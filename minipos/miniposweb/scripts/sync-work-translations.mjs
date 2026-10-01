import { readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

const root = join(import.meta.dirname, '..');
const sourcePath = join(root, '..', 'BeeMiniPOS', 'src', 'translations.generated.ts');
const extrasPath = join(root, 'src', 'ui', 'workExtras.generated.json');
const targetPath = join(root, 'src', 'ui', 'workTranslations.generated.ts');
const sourceKeys = [
  'sales', 'products', 'employees', 'currentSale', 'openShift', 'logout',
  'newProduct', 'newEmployee', 'name', 'priceEur', 'firstName', 'lastName',
  'operatorCode', 'save', 'add', 'total', 'discountEur', 'reverseSale',
];
const english = {
  settings: 'Settings', diagnostics: 'Diagnostics', printerTest: 'Test printer',
  shiftTitle: 'Cash register shift', loading: 'Loading…', retry: 'Retry',
  online: 'Online', offline: 'Offline', noProducts: 'No products yet.',
  chooseProduct: 'Select a product.', quantity: 'Quantity',
  cardRemainder: 'Card amount (remainder in cash)',
  checkoutFiscal: 'Pay and fiscalize', reverseLastSale: 'Reverse the last sale',
  cancel: 'Cancel', noEmployees: 'No employees yet.', saveFailed: 'Could not save',
  offlineReference: 'Offline reference data: a new shift cannot be opened without the backend',
  cloudPrepareUnavailable: 'Cloud preparation is unavailable; processing locally and syncing after recovery',
  adapter: 'Adapter', register: 'Register', generation: 'Generation', localUrl: 'Local URL',
  loadFailed: 'Could not load the list',
  outboxTitle: 'Offline operations', outboxEmpty: 'No saved operations yet.',
  outboxAwaiting: 'Awaiting fiscal result', outboxUncertain: 'Result uncertain — check before repeating',
  outboxFailed: 'Fiscal operation failed', outboxPendingSync: 'Fiscalized locally — waiting to sync',
  outboxSynced: 'Synced with the server', outboxFiscalized: 'Fiscalized', outboxRefresh: 'Refresh list',
};
const editorial = {
  bg: {
    settings: 'Настройки', diagnostics: 'Диагностика', printerTest: 'Тест на принтера',
    shiftTitle: 'Касова смяна', loading: 'Зареждане…', retry: 'Повтори',
    online: 'Онлайн', offline: 'Офлайн', noProducts: 'Все още няма продукти.',
    chooseProduct: 'Изберете продукт.', quantity: 'Количество',
    cardRemainder: 'С карта (остатъкът е в брой)',
    checkoutFiscal: 'Плащане и фискализация', reverseLastSale: 'Сторно на последната продажба',
    cancel: 'Отказ', noEmployees: 'Все още няма служители.', saveFailed: 'Неуспешно записване',
    offlineReference: 'Офлайн справочници: нова смяна не може да се отвори без backend',
    cloudPrepareUnavailable: 'Cloud подготовката е недостъпна; изпълнява се локално и ще се съгласува след възстановяване',
    adapter: 'Адаптер', register: 'Регистър', generation: 'Поколение', localUrl: 'Локален URL',
    loadFailed: 'Списъкът не може да се зареди',
    outboxTitle: 'Офлайн операции', outboxEmpty: 'Все още няма запазени операции.',
    outboxAwaiting: 'Очаква се фискален резултат', outboxUncertain: 'Резултатът е неясен — проверете преди повторение',
    outboxFailed: 'Фискалната операция е неуспешна', outboxPendingSync: 'Фискализирано локално — очаква синхронизация',
    outboxSynced: 'Синхронизирано със сървъра', outboxFiscalized: 'Фискализирано', outboxRefresh: 'Обнови списъка',
  },
  ru: {
    settings: 'Настройки', diagnostics: 'Диагностика', printerTest: 'Проверка принтера',
    shiftTitle: 'Кассовая смена', loading: 'Загрузка…', retry: 'Повторить',
    online: 'В сети', offline: 'Без сети', noProducts: 'Товаров пока нет.',
    chooseProduct: 'Выберите товар.', quantity: 'Количество',
    cardRemainder: 'С карты (остаток наличными)',
    checkoutFiscal: 'Оплатить и фискализировать', reverseLastSale: 'Сторнировать последнюю продажу',
    cancel: 'Отмена', noEmployees: 'Сотрудников пока нет.', saveFailed: 'Не удалось сохранить',
    offlineReference: 'Локальные справочники доступны без сети. Новую смену нельзя открыть без сервера.',
    cloudPrepareUnavailable: 'Облачная подготовка недоступна. Операция выполняется локально и синхронизируется после восстановления связи.',
    adapter: 'Адаптер', register: 'Касса', generation: 'Поколение', localUrl: 'Локальный URL',
    loadFailed: 'Не удалось загрузить список',
    outboxTitle: 'Офлайн-операции', outboxEmpty: 'Сохранённых операций пока нет.',
    outboxAwaiting: 'Ожидается фискальный результат', outboxUncertain: 'Результат неизвестен — проверьте перед повтором',
    outboxFailed: 'Фискальная операция завершилась ошибкой', outboxPendingSync: 'Фискализировано локально — ожидает синхронизации',
    outboxSynced: 'Синхронизировано с сервером', outboxFiscalized: 'Фискализировано', outboxRefresh: 'Обновить список',
  },
};
const languageNames = {
  bg: 'Bulgarian', hr: 'Croatian', cs: 'Czech', da: 'Danish', nl: 'Dutch',
  en: 'English', et: 'Estonian', fi: 'Finnish', fr: 'French', de: 'German',
  el: 'Greek', hu: 'Hungarian', ga: 'Irish', it: 'Italian', lv: 'Latvian',
  lt: 'Lithuanian', mt: 'Maltese', pl: 'Polish', pt: 'Portuguese', ro: 'Romanian',
  sk: 'Slovak', sl: 'Slovenian', es: 'Spanish', sv: 'Swedish', ru: 'Russian',
};
const source = await readFile(sourcePath, 'utf8');
const objectStart = source.indexOf('{', source.indexOf('export const translations'));
const objectEnd = source.indexOf(' as const;', objectStart);
const translations = JSON.parse(source.slice(objectStart, objectEnd));
const languageMatch = source.match(/export const supportedLanguages = (\[[^\]]+\]) as const;/);
if (!languageMatch) throw new Error('BeeMiniPOS supportedLanguages not found');
const languages = JSON.parse(languageMatch[1]);
if (languages.length !== 25 || new Set(languages).size !== 25) throw new Error('Expected 25 distinct languages');
let extras = {};
try { extras = JSON.parse(await readFile(extrasPath, 'utf8')); } catch { /* first generation */ }
extras.en = english;
extras.bg = editorial.bg;
extras.ru = editorial.ru;
const keys = Object.keys(english);

for (const language of languages) {
  const missingKeys = keys.filter(key => typeof extras[language]?.[key] !== 'string' || !extras[language][key].trim());
  if (!missingKeys.length) continue;
  const missingEnglish = Object.fromEntries(missingKeys.map(key => [key, english[key]]));
  const prompt = `Translate these user-facing point-of-sale UI strings from English into ${languageNames[language]} (${language}). Return ONLY a JSON object with exactly the same keys and translated string values. Use natural, concise product UI wording. Preserve EUR, URL, and the technical term backend. Do not leave English text as a fallback.\n${JSON.stringify(missingEnglish)}`;
  let translated;
  for (let attempt = 1; attempt <= 3; attempt++) {
    const response = await fetch('http://127.0.0.1:11434/api/generate', {
      method: 'POST', headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ model: 'qwen3.5:9b-mlx', prompt, stream: false, format: 'json', think: false, options: { temperature: 0 } }),
    });
    if (!response.ok) throw new Error(`Ollama HTTP ${response.status} for ${language}`);
    const data = await response.json();
    try {
      const candidate = JSON.parse(data.response);
      if (Object.keys(candidate).length !== missingKeys.length || !missingKeys.every(key => typeof candidate[key] === 'string' && candidate[key].trim())) throw new Error('missing or extra keys');
      translated = candidate;
      break;
    } catch (error) {
      if (attempt === 3) throw new Error(`Invalid Ollama JSON for ${language}: ${error}`);
    }
  }
  extras[language] = { ...extras[language], ...translated };
  await writeFile(extrasPath, `${JSON.stringify(extras, null, 2)}\n`);
  console.log(`Translated ${language}: ${missingKeys.length} strings`);
}

const merged = Object.fromEntries(languages.map(language => {
  const copy = Object.fromEntries([...sourceKeys.map(key => [key, translations[language]?.[key]]), ...keys.map(key => [key, extras[language]?.[key]])]);
  for (const [key, value] of Object.entries(copy)) if (typeof value !== 'string' || !value.trim()) throw new Error(`Missing ${language}.${key}`);
  return [language, copy];
}));
await writeFile(extrasPath, `${JSON.stringify(extras, null, 2)}\n`);
await writeFile(targetPath, [
  '// Existing BeeMiniPOS translations plus missing MiniPOS Web strings generated with local Ollama qwen3.5:9b-mlx.',
  '// Generated by scripts/sync-work-translations.mjs; checked in for standalone web builds.',
  `export const supportedLanguages = ${JSON.stringify(languages)} as const;`,
  `export const translations = ${JSON.stringify(merged, null, 2)} as const;`,
  '',
].join('\n'));
console.log(`Wrote ${targetPath}: ${languages.length} languages × ${sourceKeys.length + keys.length} keys`);
