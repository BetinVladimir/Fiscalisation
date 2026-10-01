import assert from 'node:assert/strict';
import test from 'node:test';
import { supportedLanguages, translations } from './translations.generated.ts';

test('selected register state and short cart tab have localized text in every supported language', () => {
  assert.equal(supportedLanguages.length, 25);
  for (const locale of supportedLanguages) {
    const selected = translations[locale].selected;
    assert.ok(selected.trim().length >= 3, `${locale}: missing selected label`);
    if (locale !== 'en') assert.notEqual(selected, translations.en.selected, `${locale}: English fallback`);
    const cart = translations[locale].cart;
    assert.ok(cart.trim().length >= 3 && cart.length <= 16, `${locale}: cart tab label must fit a small segment`);
    if (locale !== 'en') assert.notEqual(cart, translations.en.cart, `${locale}: English cart fallback`);
  }
});
