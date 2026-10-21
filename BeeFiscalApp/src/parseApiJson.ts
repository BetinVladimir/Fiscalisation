/** Keep unsafe integer wire tokens exact; safe numbers and quoted JSON strings retain their types. */
export function parseApiJson(source: string): any {
  // Validate the original grammar before quoting tokens (including invalid numeric object keys).
  const validated = JSON.parse(source);
  const number = /-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?/y;
  const parts: string[] = [];
  let cursor = 0;
  for (let at = 0; at < source.length;) {
    if (source[at] === '"') {
      at++;
      while (at < source.length) {
        if (source[at] === '\\') { at += 2; continue; }
        if (source[at++] === '"') break;
      }
      continue;
    }
    if (source[at] === '-' || (source[at] >= '0' && source[at] <= '9')) {
      number.lastIndex = at;
      const match = number.exec(source);
      if (match) {
        const token = match[0], end = at + token.length;
        if (!/[.eE]/.test(token) && !Number.isSafeInteger(Number(token))) {
          parts.push(source.slice(cursor, at), JSON.stringify(token));
          cursor = end;
        }
        at = end;
        continue;
      }
    }
    at++;
  }
  if (cursor === 0) return validated;
  parts.push(source.slice(cursor));
  // The native parser remains responsible for all JSON grammar validation.
  return JSON.parse(parts.join(''));
}
