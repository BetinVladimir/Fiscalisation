import assert from 'node:assert/strict';
import test from 'node:test';
import { privacyPolicyUrl } from './ui/legal.ts';
import { authCopy } from './ui/auth-copy.generated.ts';
import type { Language } from './ui/preferences';

test('privacy policy links follow every selectable legal-site locale', () => {
  const languages = Object.keys(authCopy) as Language[];
  assert.equal(languages.length, 25);
  for (const language of languages) {
    assert.equal(privacyPolicyUrl(language), `https://legal.beeloy.org/${language}/privacy`);
  }
});
