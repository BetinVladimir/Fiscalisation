import { resolveApiBase } from './apiBase';

describe('resolveApiBase', () => {
  it('throws API_BASE_REQUIRED in prod with empty env', () => {
    expect(() => resolveApiBase('', true)).toThrow('API_BASE_REQUIRED');
    expect(() => resolveApiBase(undefined, true)).toThrow('API_BASE_REQUIRED');
  });
  it('throws API_BASE_NOT_HTTPS in prod with http', () => {
    expect(() => resolveApiBase('http://example.com', true)).toThrow('API_BASE_NOT_HTTPS');
  });
  it('returns trimmed url in prod with https', () => {
    expect(resolveApiBase('https://api.example.com/', true)).toBe('https://api.example.com');
  });
  it('returns localhost in dev with empty env', () => {
    expect(resolveApiBase('', false)).toBe('http://localhost:8080');
  });
});
