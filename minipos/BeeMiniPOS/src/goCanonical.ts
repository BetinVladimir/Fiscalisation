// Matches Go encoding/json for the fiscal wire subset. Decimal money and
// quantities are strings; numeric fields must be exactly representable integers.
const encoder = new TextEncoder();
const compareKeys = (a: string, b: string): number => {
  const left = encoder.encode(a), right = encoder.encode(b);
  for (let i = 0; i < Math.min(left.length, right.length); i++)
    if (left[i] !== right[i]) return left[i] - right[i];
  return left.length - right.length;
};
const quote = (value: string): string => JSON.stringify(value)
  .replace(/</g, "\\u003c").replace(/>/g, "\\u003e").replace(/&/g, "\\u0026")
  .replace(/\u2028/g, "\\u2028").replace(/\u2029/g, "\\u2029");
export function canonical(value: unknown): string {
  if (value === null) return "null";
  if (typeof value === "string") {
    // Refuse unpaired UTF-16 rather than signing bytes Go would replace.
    for (let i = 0; i < value.length; i++) {
      const c = value.charCodeAt(i);
      if (c >= 0xd800 && c <= 0xdbff) {
        const next = value.charCodeAt(++i);
        if (!(next >= 0xdc00 && next <= 0xdfff)) throw new Error("CANONICAL_INVALID_UNICODE");
      } else if (c >= 0xdc00 && c <= 0xdfff) throw new Error("CANONICAL_INVALID_UNICODE");
    }
    return quote(value);
  }
  if (typeof value === "boolean") return String(value);
  if (typeof value === "number") {
    if (!Number.isSafeInteger(value) || Object.is(value, -0)) throw new Error("CANONICAL_INTEGER_REQUIRED");
    return String(value);
  }
  if (Array.isArray(value)) {
    // Sparse arrays have no faithful JSON value at the omitted index.
    for (let i = 0; i < value.length; i++)
      if (!Object.hasOwn(value, i)) throw new Error("CANONICAL_JSON_REQUIRED");
    return `[${value.map(canonical).join(",")}]`;
  }
  if (typeof value === "object" && Object.getPrototypeOf(value) === Object.prototype)
    return `{${Object.entries(value as Record<string, unknown>)
      .sort(([a], [b]) => compareKeys(a, b))
      .map(([k, v]) => `${canonical(k)}:${canonical(v)}`).join(",")}}`;
  throw new Error("CANONICAL_JSON_REQUIRED");
}
