export function resolveApiBase(env: string | undefined, isProd: boolean): string {
  if (!env || env.trim() === '') {
    if (isProd) throw new Error('API_BASE_REQUIRED');
    return 'http://localhost:8080';
  }
  const url = env.trim().replace(/\/$/, '');
  if (isProd && !url.startsWith('https://')) {
    throw new Error('API_BASE_NOT_HTTPS');
  }
  return url;
}
