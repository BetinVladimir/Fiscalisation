export type Transport = "EMBEDDED" | "RS232" | "BLE_GATT" | "USB_SERIAL";
export type Adapter = "BLUECASH_ANDROID" | "EDGE_AGENT_S3";

export type DeviceProfile = {
  id: "DATECS_BLUECASH50_EMBEDDED" | "DATECS_DP150_BLUEPAD50" | "DAISY_COMPACT_S01";
  adapter: Adapter;
  fiscal: { vendor: string; model: string; transport: Transport };
  payment?: { vendor: string; model: string; transport: Transport };
};

export const MVP1_DEVICE_PROFILES: readonly DeviceProfile[] = [
  { id: "DATECS_BLUECASH50_EMBEDDED", adapter: "BLUECASH_ANDROID", fiscal: { vendor: "DATECS", model: "BLUECASH-50", transport: "EMBEDDED" }, payment: { vendor: "DATECS", model: "BLUECASH-50", transport: "EMBEDDED" } },
  { id: "DATECS_DP150_BLUEPAD50", adapter: "EDGE_AGENT_S3", fiscal: { vendor: "DATECS", model: "DP-150 MX", transport: "RS232" }, payment: { vendor: "DATECS", model: "BLUEPAD-50 PLUS", transport: "BLE_GATT" } },
  { id: "DAISY_COMPACT_S01", adapter: "EDGE_AGENT_S3", fiscal: { vendor: "DAISY", model: "COMPACT S 01", transport: "USB_SERIAL" } },
] as const;

export function profileById(id: string): DeviceProfile | undefined {
  return MVP1_DEVICE_PROFILES.find((profile) => profile.id === id);
}

export function validateCompositeProfile(value: DeviceProfile): boolean {
  const canonical = profileById(value.id);
  return !!canonical && JSON.stringify(canonical) === JSON.stringify(value);
}
