const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export function isFiscalResourceId(value: string): boolean {
  return uuid.test(value.trim());
}

export function requireFiscalResourceId(value: string, label: string): string {
  const normalized = value.trim();
  if (!isFiscalResourceId(normalized)) throw new Error(`${label} трябва да е валиден UUID`);
  return normalized;
}
