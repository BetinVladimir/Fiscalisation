export async function fetchWithTimeout(
  input: string,
  init: RequestInit = {},
  timeoutMs = 4000,
): Promise<Response> {
  if (!Number.isFinite(timeoutMs) || timeoutMs < 100 || timeoutMs > 120_000) {
    throw new Error("HTTP_TIMEOUT_INVALID");
  }
  const controller = new AbortController();
  const upstream = init.signal;
  const abortFromUpstream = () => controller.abort(upstream?.reason);
  if (upstream?.aborted) abortFromUpstream();
  else upstream?.addEventListener("abort", abortFromUpstream, { once: true });
  const timer = setTimeout(() => controller.abort(new Error("HTTP_TIMEOUT")), timeoutMs);
  try {
    return await fetch(input, { ...init, signal: controller.signal });
  } finally {
    clearTimeout(timer);
    upstream?.removeEventListener("abort", abortFromUpstream);
  }
}
