// idempotentRequest — stable Idempotency-Key per logical mutation.
// Shared logic mirroring AdminApp/src/idempotentRequest.ts.

export function idempotentKey(
  current: string | null,
  payload: string | null,
  newPayload: string,
  newKey: () => string,
): { key: string; payload: string } {
  if (current === null || payload !== newPayload) {
    return { key: newKey(), payload: newPayload };
  }
  return { key: current, payload: newPayload };
}

export interface IdempotentRunner<T> {
  run(body: unknown): Promise<T>;
  key: string;
  reset(): void;
}

export function createIdempotentRunner<T>(
  url: string,
  options: { method?: string; headers?: Record<string, string>; fetchFn?: typeof fetch } = {},
  newKey: () => string = () => crypto.randomUUID(),
): IdempotentRunner<T> {
  const { method = 'POST', headers: extra = {}, fetchFn = fetch } = options;
  let currentKey = newKey();
  let currentPayload: string | null = null;

  return {
    get key() { return currentKey; },
    reset() { currentKey = newKey(); currentPayload = null; },
    async run(body: unknown): Promise<T> {
      const payloadStr = JSON.stringify(body);
      if (currentPayload !== null && currentPayload !== payloadStr) {
        currentKey = newKey();
      }
      currentPayload = payloadStr;
      const usedKey = currentKey;
      const res = await fetchFn(url, {
        method,
        headers: { 'Content-Type': 'application/json', 'Idempotency-Key': usedKey, ...extra },
        body: payloadStr,
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}: ${await res.text()}`);
      currentKey = newKey();
      currentPayload = null;
      return res.json() as Promise<T>;
    },
  };
}
