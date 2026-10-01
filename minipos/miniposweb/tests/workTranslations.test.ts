import { describe, expect, it } from 'vitest';
import { supportedLanguages as authLanguages } from '../src/ui/authTranslations.generated';
import { supportedLanguages, translations } from '../src/ui/workTranslations.generated';

describe('MiniPOS Web working translations', () => {
  it('covers the same 25 EU/RU locales with every visible work key', () => {
    expect(supportedLanguages).toEqual(authLanguages);
    expect(supportedLanguages).toHaveLength(25);
    const keys = Object.keys(translations.en);
    expect(keys).toHaveLength(52);
    for (const language of supportedLanguages) {
      expect(Object.keys(translations[language])).toEqual(keys);
      for (const key of keys) {
        const value = translations[language][key as keyof typeof translations.en];
        expect(value.trim(), `${language}.${key}`).not.toBe('');
      }
    }
    expect(translations.de.checkoutFiscal).not.toBe(translations.en.checkoutFiscal);
    expect(translations.fr.noEmployees).not.toBe(translations.en.noEmployees);
    expect(translations.ru.currentSale).not.toBe(translations.en.currentSale);
  });
});
