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

export function devicesBoundToRegister<T extends { id?: unknown }>(
  devices: T[],
  register: { fiscal_device_id?: unknown; payment_terminal_id?: unknown } | undefined,
): T[] {
  if (!register) return [];
  const boundIds = new Set(
    [register.fiscal_device_id, register.payment_terminal_id].filter(
      (id): id is string => typeof id === 'string' && id.length > 0,
    ),
  );
  return devices.filter(device => typeof device.id === 'string' && boundIds.has(device.id));
}
