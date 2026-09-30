import assert from 'node:assert/strict';
import test from 'node:test';
import { supportedLanguages, translations } from './translations.generated.ts';

test('selected register state has visible localized text in every supported language', () => {
  assert.equal(supportedLanguages.length, 25);
  for (const locale of supportedLanguages) {
    const selected = translations[locale].selected;
    assert.ok(selected.trim().length >= 3, `${locale}: missing selected label`);
    if (locale !== 'en') assert.notEqual(selected, translations.en.selected, `${locale}: English fallback`);
  }
});
