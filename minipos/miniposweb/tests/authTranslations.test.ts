import { describe, expect, it } from 'vitest';
import { supportedLanguages, translations } from '../src/ui/authTranslations.generated';

const authKeys = [
  'theme', 'systemTheme', 'lightTheme', 'darkTheme', 'language', 'signIn', 'email',
  'sendCode', 'oneTimeCode', 'continue', 'companyDetails', 'companyName',
  'address', 'taxIdentifier', 'fullName', 'createCompany',
] as const;

describe('MiniPOS Web Paper auth copy', () => {
  it('has every visible auth label for the 24 EU languages plus Russian', () => {
    expect(supportedLanguages).toHaveLength(25);
    expect(new Set(supportedLanguages).size).toBe(25);
    for (const language of supportedLanguages) {
      for (const key of authKeys) {
        expect(translations[language][key].trim(), `${language}.${key}`).not.toBe('');
      }
    }
  });
});
