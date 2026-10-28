export async function fetchWithTimeout(
  url: string,
  init: RequestInit = {},
  ms = 15000
): Promise<Response> {
  const controller = new AbortController();
  const id = setTimeout(() => controller.abort(), ms);
  try {
    const response = await fetch(url, { ...init, signal: controller.signal });
    return response;
  } finally {
    clearTimeout(id);
  }
}
