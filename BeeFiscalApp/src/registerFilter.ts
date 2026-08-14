const uuid =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export function isRegisterId(value: string): boolean {
  return uuid.test(value);
}

export function registerCollectionPath(
  path: string,
  registerId: string,
): string {
  const value = registerId.trim();
  if (!value) return path;
  if (!isRegisterId(value)) throw new Error("INVALID_REGISTER_ID");
  return `${path}?register_id=${encodeURIComponent(value)}`;
}

export function registerResourcePath(registerId: string, suffix = ""): string {
  const value = registerId.trim();
  if (!isRegisterId(value)) throw new Error("INVALID_REGISTER_ID");
  return `/registers/${encodeURIComponent(value)}${suffix}`;
}
