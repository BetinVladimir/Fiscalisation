import { fetchWithTimeout } from './fetchWithTimeout';

describe('fetchWithTimeout', () => {
  it('rejects with AbortError after timeout', async () => {
    jest.useFakeTimers();
    // Mock fetch that rejects when signal is aborted
    const fetchMock = jest.fn((_, init?: RequestInit) => {
      return new Promise<Response>((_, reject) => {
        const signal = (init as RequestInit & { signal?: AbortSignal })?.signal;
        if (signal) {
          signal.addEventListener('abort', () => {
            const err = new DOMException('Aborted', 'AbortError');
            reject(err);
          });
        }
      });
    });
    global.fetch = fetchMock as unknown as typeof fetch;

    const promise = fetchWithTimeout('/api/test', {}, 15000);
    jest.advanceTimersByTime(15001);
    await Promise.resolve(); // flush microtasks

    await expect(promise).rejects.toMatchObject({ name: 'AbortError' });
    jest.useRealTimers();
  });

  it('resolves when fetch returns before timeout', async () => {
    global.fetch = jest.fn().mockResolvedValue({ ok: true, status: 200 }) as unknown as typeof fetch;
    const result = await fetchWithTimeout('/api/test', {}, 5000);
    expect(result.status).toBe(200);
  });
});
