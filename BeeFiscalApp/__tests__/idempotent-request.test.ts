import { createIdempotentRunner, idempotentKey } from '../src/idempotentRequest';

let keySeq = 0;
function nextKey() { return `k${++keySeq}`; }
beforeEach(() => { keySeq = 0; });

const mockFetch = jest.fn();
(global as unknown as { fetch: typeof fetch }).fetch = mockFetch;

function okRes(body: unknown) {
  return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve(body), text: () => Promise.resolve('') } as Response);
}
function errRes(status = 500) {
  return Promise.resolve({ ok: false, status, text: () => Promise.resolve('Error'), json: () => Promise.reject(new Error('not ok')) } as Response);
}

afterEach(() => jest.clearAllMocks());

describe('idempotentKey', () => {
  it('generates new key when no current key', () => {
    const r = idempotentKey(null, null, '{"x":1}', nextKey);
    expect(r.key).toBe('k1');
    expect(r.payload).toBe('{"x":1}');
  });

  it('keeps same key for same payload', () => {
    const r = idempotentKey('k1', '{"x":1}', '{"x":1}', nextKey);
    expect(r.key).toBe('k1');
    expect(keySeq).toBe(0); // newKey not called
  });

  it('generates new key when payload changes', () => {
    const r = idempotentKey('k1', '{"x":1}', '{"x":2}', nextKey);
    expect(r.key).toBe('k1'); // nextKey called: k1
    expect(keySeq).toBe(1);
  });
});

describe('createIdempotentRunner', () => {
  it('same key on retry (fetch fails then succeeds)', async () => {
    mockFetch.mockRejectedValueOnce(new Error('NetworkError'));
    mockFetch.mockResolvedValueOnce(okRes({ id: '1' }));

    const runner = createIdempotentRunner<{ id: string }>('/api/test', { fetchFn: mockFetch }, nextKey);
    const firstKey = runner.key;

    await expect(runner.run({ amount: 867 })).rejects.toThrow('NetworkError');
    expect(runner.key).toBe(firstKey);

    await runner.run({ amount: 867 });
    const [, init1] = mockFetch.mock.calls[0];
    const [, init2] = mockFetch.mock.calls[1];
    expect((init1!.headers as Record<string,string>)['Idempotency-Key'])
      .toBe((init2!.headers as Record<string,string>)['Idempotency-Key']);
  });

  it('new key after success', async () => {
    mockFetch.mockResolvedValue(okRes({ id: '2' }));
    const runner = createIdempotentRunner<unknown>('/api/test', { fetchFn: mockFetch }, nextKey);
    const firstKey = runner.key;
    await runner.run({ amount: 500 });
    expect(runner.key).not.toBe(firstKey);
  });

  it('new key when payload changes', async () => {
    mockFetch.mockRejectedValue(new Error('NetworkError'));
    const runner = createIdempotentRunner<unknown>('/api/test', { fetchFn: mockFetch }, nextKey);
    const firstKey = runner.key;
    await runner.run({ amount: 500 }).catch(() => {});
    expect(runner.key).toBe(firstKey); // same payload

    await runner.run({ amount: 867 }).catch(() => {});
    expect(runner.key).not.toBe(firstKey); // different payload
  });
});
