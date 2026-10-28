export type FetchLike = (input: string, init?: RequestInit) => Promise<Response>;

async function requestWithTimeout<T>(
  input: string,
  init: RequestInit,
  timeoutMs: number,
  fetcher: FetchLike,
  consume: (response: Response) => T | Promise<T>,
): Promise<T> {
  if (!Number.isFinite(timeoutMs) || timeoutMs < 100 || timeoutMs > 120_000)
    throw new Error("HTTP_TIMEOUT_INVALID");
  const upstream = init.signal;
  if (upstream?.aborted) throw upstream.reason ?? new Error("HTTP_ABORTED");
  const controller = new AbortController();
  let rejectStop!: (reason: unknown) => void;
  const stopped = new Promise<never>((_, reject) => { rejectStop = reject; });
  const stop = (reason: unknown) => {
    controller.abort(reason);
    rejectStop(reason);
  };
  const abortFromUpstream = () => stop(upstream?.reason ?? new Error("HTTP_ABORTED"));
  upstream?.addEventListener("abort", abortFromUpstream, { once: true });
  const timer = setTimeout(() => stop(new Error("HTTP_TIMEOUT")), timeoutMs);
  try {
    // Race independently of AbortSignal cooperation, including body consumption.
    // Promise.race observes late rejections from the losing request as well.
    const request = Promise.resolve().then(() => {
      if (controller.signal.aborted) throw controller.signal.reason;
      return fetcher(input, { ...init, signal: controller.signal });
    }).then(response => {
      if (controller.signal.aborted) throw controller.signal.reason;
      return consume(response);
    });
    return await Promise.race([request, stopped]);
  } finally {
    clearTimeout(timer);
    upstream?.removeEventListener("abort", abortFromUpstream);
  }
}

/** Headers-only deadline; use fetchTextWithTimeout for API JSON bodies. */
export function fetchWithTimeout(input: string, init: RequestInit = {}, timeoutMs = 4000, fetcher: FetchLike = fetch): Promise<Response> {
  return requestWithTimeout(input, init, timeoutMs, fetcher, response => response);
}

/** One deadline covers both headers and complete body; original Response metadata is preserved. */
export function fetchTextWithTimeout(input: string, init: RequestInit = {}, timeoutMs = 4000, fetcher: FetchLike = fetch): Promise<{response: Response; text: string}> {
  return requestWithTimeout(input, init, timeoutMs, fetcher, async response => ({response, text: await response.text()}));
}
