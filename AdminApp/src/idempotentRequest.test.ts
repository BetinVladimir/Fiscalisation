import { createIdempotentRunner, idempotentRequest } from './idempotentRequest';

// Counter for deterministic key generation in tests.
let keySeq = 0;
function nextKey() { return `key-${++keySeq}`; }
beforeEach(() => { keySeq = 0; });

// Mock fetch globally.
const mockFetch = jest.fn();
(global as unknown as { fetch: typeof fetch }).fetch = mockFetch;

function okResponse(body: unknown) {
  return Promise.resolve({
    ok: true,
    status: 200,
    json: () => Promise.resolve(body),
  } as Response);
}
function failResponse(status = 500) {
  return Promise.resolve({
    ok: false,
    status,
    text: () => Promise.resolve('Error'),
  } as Response);
}

afterEach(() => jest.clearAllMocks());

describe('idempotentRequest', () => {
  it('sends Idempotency-Key header', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true, status: 200, json: () => Promise.resolve({ ok: true }),
    } as Response);
    await idempotentRequest('/api/sale', 'test-key-abc', { amount: 1367 });
    const [, init] = mockFetch.mock.calls[0];
    expect((init!.headers as Record<string, string>)['Idempotency-Key']).toBe('test-key-abc');
  });

  it('throws on non-ok response', async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 409, text: () => Promise.resolve('Conflict') } as Response);
    await expect(idempotentRequest('/api/sale', 'k1', {})).rejects.toThrow('HTTP 409');
  });
});

describe('createIdempotentRunner', () => {
  it('retries with the same key when fetch fails then succeeds', async () => {
    mockFetch.mockRejectedValueOnce(new Error('NetworkError'));
    mockFetch.mockResolvedValueOnce(okResponse({ id: '1' }));

    const runner = createIdempotentRunner<{ id: string }>('/api/sale', {}, nextKey);
    const firstKey = runner.key;

    // First call: network error
    await expect(runner.run({ amount: 1367 })).rejects.toThrow('NetworkError');

    // Key must not have changed after failure
    expect(runner.key).toBe(firstKey);

    // Second call: succeeds — same key must be sent
    await runner.run({ amount: 1367 });

    const call1Headers = (mockFetch.mock.calls[0][1]!.headers as Record<string, string>);
    const call2Headers = (mockFetch.mock.calls[1][1]!.headers as Record<string, string>);
    expect(call1Headers['Idempotency-Key']).toBe(call2Headers['Idempotency-Key']);
  });

  it('generates a new key after a successful call', async () => {
    mockFetch.mockResolvedValue(okResponse({ id: '2' }));
    const runner = createIdempotentRunner<{ id: string }>('/api/sale', {}, nextKey);
    const firstKey = runner.key;

    await runner.run({ amount: 1200 });
    const keyAfterSuccess = runner.key;
    expect(keyAfterSuccess).not.toBe(firstKey);
  });

  it('generates a new key when payload changes', async () => {
    mockFetch.mockRejectedValue(new Error('NetworkError'));
    const runner = createIdempotentRunner<unknown>('/api/sale', {}, nextKey);
    const keyForPayload1 = runner.key;

    await runner.run({ amount: 500 }).catch(() => {});

    // Same payload: same key
    expect(runner.key).toBe(keyForPayload1);

    // Different payload: new key
    await runner.run({ amount: 867 }).catch(() => {});
    expect(runner.key).not.toBe(keyForPayload1);
  });
});
