import type { Language } from './preferences';

export function privacyPolicyUrl(language: Language): string {
  return `https://legal.beeloy.org/${language}/privacy`;
}
