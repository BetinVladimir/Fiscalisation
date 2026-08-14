export type AdvertisedBleDevice = {
  name?: string | null;
  localName?: string | null;
};

export function requireAdvertisingIdentity(expected: string) {
  const identity = expected.trim();
  if (!identity || identity.length > 128)
    throw new Error("BLE_ADVERTISING_IDENTITY_INVALID");
  return identity;
}

export function authorizedAdvertisingIdentity(
  advertisingIdentity: string,
  signedEdgeIdentity: string,
) {
  const identity = requireAdvertisingIdentity(advertisingIdentity);
  const edge = requireAdvertisingIdentity(signedEdgeIdentity);
  if (identity !== edge) throw new Error("BLE_ADVERTISING_AUTHORITY_MISMATCH");
  return identity;
}

export function matchesAdvertisingIdentity(
  device: AdvertisedBleDevice,
  expected: string,
) {
  const identity = requireAdvertisingIdentity(expected);
  return device.name === identity || device.localName === identity;
}
