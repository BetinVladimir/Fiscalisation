import assert from 'node:assert/strict';
import test from 'node:test';
import { legalTranslations } from './legalTranslations.generated.ts';
import { supportedLanguages } from './translations.generated.ts';

test('onboarding legal copy exists for every supported language', () => {
  assert.deepEqual(Object.keys(legalTranslations), [...supportedLanguages]);
  for (const locale of supportedLanguages) {
    const copy = legalTranslations[locale];
    assert.deepEqual(Object.keys(copy), ['consent', 'terms', 'privacy']);
    assert.ok(copy.consent.length >= 100, `${locale}: consent is incomplete`);
    assert.match(copy.consent, /Beeloy/);
    assert.match(copy.consent, /2026/);
    assert.ok(copy.terms.trim().length >= 4, `${locale}: terms label is missing`);
    assert.ok(copy.privacy.trim().length >= 4, `${locale}: privacy label is missing`);
    if (locale !== 'en') assert.notEqual(copy.consent, legalTranslations.en.consent, `${locale}: English fallback`);
  }
});
