// idempotentRequest — sends a POST/PATCH with a stable Idempotency-Key.
//
// createIdempotentRunner returns a runner bound to one logical mutation:
//   • Same payload on retry → same key (server ignores duplicate).
//   • After a successful response → key is cleared; next call gets a new key.
//   • Different payload → new key regardless.

export async function idempotentRequest<T>(
  url: string,
  key: string,
  body: unknown,
  options: { method?: string; headers?: Record<string, string> } = {},
): Promise<T> {
  const { method = 'POST', headers: extra = {} } = options;
  const res = await fetch(url, {
    method,
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': key,
      ...extra,
    },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${await res.text()}`);
  }
  return res.json() as Promise<T>;
}

export interface IdempotentRunner<T> {
  /** Run with the current stable key. Resets key after success. */
  run(body: unknown): Promise<T>;
  /** Current idempotency key (stable until reset or payload change). */
  key: string;
  /** Force-reset the key (e.g. on user-initiated new operation). */
  reset(): void;
}

/** Factory for a non-React context (closure-based state). */
export function createIdempotentRunner<T>(
  url: string,
  options: { method?: string; headers?: Record<string, string> } = {},
  newKey: () => string = () => crypto.randomUUID(),
): IdempotentRunner<T> {
  let currentKey = newKey();
  let currentPayload: string | null = null;

  return {
    get key() { return currentKey; },
    reset() {
      currentKey = newKey();
      currentPayload = null;
    },
    async run(body: unknown): Promise<T> {
      const payloadStr = JSON.stringify(body);
      if (currentPayload !== null && currentPayload !== payloadStr) {
        currentKey = newKey();
      }
      currentPayload = payloadStr;
      const usedKey = currentKey;
      const result = await idempotentRequest<T>(url, usedKey, body, options);
      // Success: clear so next distinct call gets a new key.
      currentKey = newKey();
      currentPayload = null;
      return result;
    },
  };
}
